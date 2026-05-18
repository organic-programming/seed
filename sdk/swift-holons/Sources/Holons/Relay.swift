import Foundation
import GRPC
import NIOCore

public typealias ChainHop = String

private struct RelayRuntimeError: Error, CustomStringConvertible {
    let description: String
    init(_ description: String) { self.description = description }
}

public final class TransitiveObservabilityRelay: @unchecked Sendable {
    private let obs: Observability
    private let memberSlug: String
    private let memberUid: String
    private let channel: GRPCChannel
    private let lock = NSLock()
    private var stopped = false
    private var logCall: ServerStreamingCall<Holons_V1_LogsRequest, Holons_V1_LogRecord>?
    private var eventCall: ServerStreamingCall<Holons_V1_EventsRequest, Holons_V1_LogRecord>?

    public init(obs: Observability = current(), memberSlug: String, memberUid: String, channel: GRPCChannel) {
        self.obs = obs
        self.memberSlug = memberSlug
        self.memberUid = memberUid
        self.channel = channel
    }

    public func start() {
        if obs.enabled(.logs), obs.logRing != nil {
            DispatchQueue.global(qos: .utility).async { [weak self] in self?.pumpLogs() }
        }
        if obs.enabled(.events), obs.eventBus != nil {
            DispatchQueue.global(qos: .utility).async { [weak self] in self?.pumpEvents() }
        }
    }

    public func stop() {
        lock.lock()
        stopped = true
        let logs = logCall
        let events = eventCall
        lock.unlock()
        logs?.cancel(promise: nil)
        events?.cancel(promise: nil)
    }

    private var isStopped: Bool {
        lock.lock(); defer { lock.unlock() }
        return stopped
    }

    private func pumpLogs() {
        while !isStopped {
            let client = Holons_V1_HolonObservabilityClient(channel: channel)
            var request = Holons_V1_LogsRequest()
            request.minSeverityNumber = .info
            request.follow = true
            let call = client.logs(request) { [weak self] proto in
                guard let self = self else { return }
                var relayed = fromProtoLogRecord(proto)
                relayed.record.chain = appendDirectChild(relayed.record.chain, childSlug: self.memberSlug, childUid: self.memberUid)
                self.obs.logRing?.push(relayed)
            }
            lock.lock()
            logCall = call
            lock.unlock()
            _ = try? call.status.wait()
            if !isStopped { Thread.sleep(forTimeInterval: 0.1) }
        }
    }

    private func pumpEvents() {
        while !isStopped {
            let client = Holons_V1_HolonObservabilityClient(channel: channel)
            var request = Holons_V1_EventsRequest()
            request.follow = true
            let call = client.events(request) { [weak self] proto in
                guard let self = self else { return }
                var relayed = fromProtoLogRecord(proto)
                relayed.record.chain = appendDirectChild(relayed.record.chain, childSlug: self.memberSlug, childUid: self.memberUid)
                self.obs.eventBus?.emit(relayed)
            }
            lock.lock()
            eventCall = call
            lock.unlock()
            _ = try? call.status.wait()
            if !isStopped { Thread.sleep(forTimeInterval: 0.1) }
        }
    }
}

struct RelayIdentity {
    let slug: String
    let uid: String
}

func resolveRelayIdentity(channel: GRPCChannel, fallbackSlug: String) -> RelayIdentity? {
    let client = Holons_V1_HolonObservabilityClient(channel: channel)
    var selected: RelayIdentity?
    let call = client.events(Holons_V1_EventsRequest(), callOptions: timeoutOptions(1)) { event in
        let uid = stringAttribute(event.attributes, key: AttrHolonsInstanceUID)
        let attrSlug = stringAttribute(event.attributes, key: AttrHolonsSlug)
        guard event.chain.isEmpty, !uid.isEmpty else { return }
        let slug = attrSlug.isEmpty ? fallbackSlug : attrSlug
        if !slug.isEmpty, selected == nil {
            selected = RelayIdentity(slug: slug, uid: uid)
        }
    }
    _ = try? call.status.wait()
    if let selected { return selected }

    let logCall = client.logs(Holons_V1_LogsRequest(), callOptions: timeoutOptions(1)) { entry in
        let uid = stringAttribute(entry.attributes, key: AttrHolonsInstanceUID)
        let attrSlug = stringAttribute(entry.attributes, key: AttrHolonsSlug)
        guard entry.chain.isEmpty, !uid.isEmpty else { return }
        let slug = attrSlug.isEmpty ? fallbackSlug : attrSlug
        if !slug.isEmpty, selected == nil {
            selected = RelayIdentity(slug: slug, uid: uid)
        }
    }
    _ = try? logCall.status.wait()
    return selected
}

public struct CheckOutcome: Sendable {
    public var pass: Bool
    public var evidence: String

    public init(pass: Bool = false, evidence: String = "") {
        self.pass = pass
        self.evidence = evidence
    }
}

public struct LogCheckOptions {
    public var channel: GRPCChannel?
    public var sender: String
    public var leafUid: String
    public var expectedChain: [ChainHop]
    public var timeout: TimeInterval
    public var pollInterval: TimeInterval

    public init(channel: GRPCChannel? = nil, sender: String, leafUid: String, expectedChain: [ChainHop],
                timeout: TimeInterval = 3, pollInterval: TimeInterval = 0.1) {
        self.channel = channel
        self.sender = sender
        self.leafUid = leafUid
        self.expectedChain = expectedChain
        self.timeout = timeout
        self.pollInterval = pollInterval
    }
}

public struct EventCheckOptions {
    public var channel: GRPCChannel?
    public var eventName: String
    public var leafUid: String
    public var expectedChain: [ChainHop]
    public var timeout: TimeInterval
    public var pollInterval: TimeInterval

    public init(channel: GRPCChannel? = nil, eventName: String = EventInstanceReady, leafUid: String,
                expectedChain: [ChainHop], timeout: TimeInterval = 3, pollInterval: TimeInterval = 0.1) {
        self.channel = channel
        self.eventName = eventName
        self.leafUid = leafUid
        self.expectedChain = expectedChain
        self.timeout = timeout
        self.pollInterval = pollInterval
    }
}

public func checkRelayedLog(_ opts: LogCheckOptions) -> CheckOutcome {
    let deadline = Date().addingTimeInterval(opts.timeout > 0 ? opts.timeout : 3)
    var last = CheckOutcome(evidence: "no log check attempted")
    repeat {
        do {
            last = matchRelayedLog(try readLogEntries(channel: opts.channel), opts: opts)
            if last.pass { return last }
        } catch {
            last = CheckOutcome(evidence: compactCheckEvidence("\(error)"))
        }
        Thread.sleep(forTimeInterval: opts.pollInterval > 0 ? opts.pollInterval : 0.1)
    } while Date() < deadline
    return last
}

public func checkRelayedEvent(_ opts: EventCheckOptions) -> CheckOutcome {
    let deadline = Date().addingTimeInterval(opts.timeout > 0 ? opts.timeout : 3)
    var last = CheckOutcome(evidence: "no event check attempted")
    repeat {
        do {
            last = matchRelayedEvent(try readEventEntries(channel: opts.channel), opts: opts)
            if last.pass { return last }
        } catch {
            last = CheckOutcome(evidence: compactCheckEvidence("\(error)"))
        }
        Thread.sleep(forTimeInterval: opts.pollInterval > 0 ? opts.pollInterval : 0.1)
    } while Date() < deadline
    return last
}

private func readLogEntries(channel: GRPCChannel?) throws -> [LogRecord] {
    guard let channel else {
        guard let ring = current().logRing else {
            throw RelayRuntimeError("logs family is not enabled")
        }
        return ring.drain()
    }
    var entries: [LogRecord] = []
    let client = Holons_V1_HolonObservabilityClient(channel: channel)
    var request = Holons_V1_LogsRequest()
    request.minSeverityNumber = .info
    let call = client.logs(request, callOptions: timeoutOptions(2)) { entry in
        entries.append(fromProtoLogRecord(entry))
    }
    _ = try call.status.wait()
    return entries
}

private func readEventEntries(channel: GRPCChannel?) throws -> [Event] {
    guard let channel else {
        guard let bus = current().eventBus else {
            throw RelayRuntimeError("events family is not enabled")
        }
        return bus.drain()
    }
    var events: [Event] = []
    let client = Holons_V1_HolonObservabilityClient(channel: channel)
    let call = client.events(Holons_V1_EventsRequest(), callOptions: timeoutOptions(2)) { event in
        events.append(fromProtoLogRecord(event))
    }
    _ = try call.status.wait()
    return events
}

private func matchRelayedLog(_ entries: [LogRecord], opts: LogCheckOptions) -> CheckOutcome {
    for entry in entries {
        guard entry.bodyString == "tick received",
              entry.attribute("sender") == opts.sender,
              entry.attribute("responder_uid") == opts.leafUid else {
            continue
        }
        if let evidence = compareChain(entry.record.chain, opts.expectedChain) {
            return CheckOutcome(evidence: compactCheckEvidence("matching log bad chain: \(evidence)"))
        }
        return CheckOutcome(pass: true)
    }
    return CheckOutcome(evidence: compactCheckEvidence("no relayed tick log sender=\(opts.sender) leaf_uid=\(opts.leafUid) entries=\(entries.count)"))
}

private func matchRelayedEvent(_ events: [Event], opts: EventCheckOptions) -> CheckOutcome {
    for event in events {
        guard event.record.eventName == opts.eventName,
              event.attribute(AttrHolonsInstanceUID) == opts.leafUid else {
            continue
        }
        if let evidence = compareChain(event.record.chain, opts.expectedChain) {
            return CheckOutcome(evidence: compactCheckEvidence("matching event bad chain: \(evidence)"))
        }
        return CheckOutcome(pass: true)
    }
    return CheckOutcome(evidence: compactCheckEvidence("no relayed \(opts.eventName) event leaf_uid=\(opts.leafUid) events=\(events.count)"))
}

private func compareChain(_ got: [ChainHop], _ want: [ChainHop]) -> String? {
    guard got.count == want.count else {
        return "chain length \(got.count) want \(want.count)"
    }
    for idx in want.indices {
        if got[idx] != want[idx] {
            return "hop \(idx)=\(got[idx]) want \(want[idx])"
        }
    }
    return nil
}

private func compactCheckEvidence(_ value: String) -> String {
    let compact = value.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
    if compact.isEmpty { return "<empty>" }
    if compact.count <= 240 { return compact }
    return String(compact.prefix(240)) + "..."
}

public final class CanonicalRelayServiceProvider: Relay_V1_RelayServiceProvider {
    public let interceptors: Relay_V1_RelayServiceServerInterceptorFactoryProtocol? = nil
    private let downstream: GRPCChannel?
    private let lock = NSLock()
    private var received: Int64 = 0

    public init(downstream: GRPCChannel? = nil) {
        self.downstream = downstream
    }

    public func tick(
        request: Relay_V1_TickRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Relay_V1_TickResponse> {
        let obs = current()
        let slug = responderSlug(obs)
        let uid = obs.cfg.instanceUid
        let count = nextReceived()
        obs.logger("tick").info(
            "tick received",
            [
                "sender": .string(request.sender),
                "note": .string(request.note),
                "responder_slug": .string(slug),
                "responder_uid": .string(uid),
            ]
        )
        obs.counter(
            "cascade_ticks_total",
            help: "Ticks received by this cascade node.",
            labels: ["responder_uid": uid]
        )?.inc()

        let childResponse: EventLoopFuture<Relay_V1_TickResponse>
        if let downstream {
            childResponse = Relay_V1_RelayServiceNIOClient(channel: downstream)
                .tick(request, callOptions: timeoutOptions(5))
                .response
        } else {
            childResponse = context.eventLoop.makeSucceededFuture(Relay_V1_TickResponse())
        }

        let promise = context.eventLoop.makePromise(of: Relay_V1_TickResponse.self)
        childResponse.whenComplete { result in
            context.eventLoop.execute {
                switch result {
                case .success(let child):
                    var response = Relay_V1_TickResponse()
                    response.responderSlug = slug
                    response.responderInstanceUid = uid
                    response.hops = child.hops
                    var hop = Relay_V1_HopReceipt()
                    hop.slug = slug
                    hop.uid = uid
                    hop.received = count
                    response.hops.append(hop)
                    promise.succeed(response)
                case .failure(let error):
                    promise.fail(error)
                }
            }
        }
        return promise.futureResult
    }

    private func nextReceived() -> Int64 {
        lock.lock()
        received += 1
        let value = received
        lock.unlock()
        return value
    }

    private func responderSlug(_ obs: Observability) -> String {
        let slug = obs.cfg.slug.trimmingCharacters(in: .whitespacesAndNewlines)
        if !slug.isEmpty { return slug }
        return ""
    }
}

public func canonicalRelayServiceProvider(downstream: GRPCChannel? = nil) -> CallHandlerProvider {
    CanonicalRelayServiceProvider(downstream: downstream)
}

private func timeoutOptions(_ seconds: TimeInterval) -> CallOptions {
    CallOptions(timeLimit: .timeout(.nanoseconds(Int64(seconds * 1_000_000_000))))
}
