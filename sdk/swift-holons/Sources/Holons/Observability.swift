// Swift reference implementation of the cross-SDK observability layer.
//
// Mirrors sdk/go-holons/pkg/observability. Same activation model
// (OP_OBS env + zero cost when disabled), same public surface
// (HolonLogger / Counter / Gauge / Histogram / EventBus / chain helpers),
// same proto types (holons.v1.HolonObservability). See OBSERVABILITY.md.

import Foundation
import GRPC
import NIOCore
import SwiftProtobuf

// MARK: - Families

public enum Family: String, CaseIterable, Sendable {
    case logs
    case metrics
    case events
    case prom
    case otel // reserved v2
}

private let v1Tokens: Set<String> = ["logs", "metrics", "events", "prom", "all"]

public struct InvalidTokenError: Error, CustomStringConvertible {
    public let variable: String
    public let token: String
    public let reason: String
    public init(token: String, reason: String, variable: String = "OP_OBS") {
        self.variable = variable
        self.token = token
        self.reason = reason
    }
    public var description: String { "\(variable): \(reason): \(token)" }
}

public func parseOpObs(_ raw: String) -> Set<Family> {
    var out = Set<Family>()
    let trimmed = raw.trimmingCharacters(in: .whitespaces)
    if trimmed.isEmpty { return out }
    for part in trimmed.split(separator: ",") {
        let tok = part.trimmingCharacters(in: .whitespaces)
        guard !tok.isEmpty else { continue }
        if tok == "otel" || tok == "sessions" { continue } // dropped by parser
        guard v1Tokens.contains(tok) else { continue }    // unknown dropped
        if tok == "all" {
            out.formUnion([.logs, .metrics, .events, .prom])
        } else if let fam = Family(rawValue: tok) {
            out.insert(fam)
        }
    }
    return out
}

public func checkEnv(_ env: [String: String] = ProcessInfo.processInfo.environment) throws {
    let sessions = (env["OP_SESSIONS"] ?? "").trimmingCharacters(in: .whitespaces)
    if !sessions.isEmpty {
        throw InvalidTokenError(token: sessions, reason: "sessions are reserved for v2; not implemented in v1", variable: "OP_SESSIONS")
    }
    let raw = (env["OP_OBS"] ?? "").trimmingCharacters(in: .whitespaces)
    if raw.isEmpty { return }
    for part in raw.split(separator: ",") {
        let tok = part.trimmingCharacters(in: .whitespaces)
        guard !tok.isEmpty else { continue }
        if tok == "otel" {
            throw InvalidTokenError(token: tok, reason: "otel export is reserved for v2; not implemented in v1")
        }
        if tok == "sessions" {
            throw InvalidTokenError(token: tok, reason: "sessions are reserved for v2; not implemented in v1")
        }
        if !v1Tokens.contains(tok) {
            throw InvalidTokenError(token: tok, reason: "unknown OP_OBS token")
        }
    }
}

// MARK: - Levels + events

public enum Level: Int32, Sendable, Comparable {
    case unset = 0
    case trace = 1
    case debug = 2
    case info = 3
    case warn = 4
    case error = 5
    case fatal = 6

    public static func < (lhs: Level, rhs: Level) -> Bool { lhs.rawValue < rhs.rawValue }

    public var name: String {
        switch self {
        case .trace: return "TRACE"
        case .debug: return "DEBUG"
        case .info: return "INFO"
        case .warn: return "WARN"
        case .error: return "ERROR"
        case .fatal: return "FATAL"
        case .unset: return "UNSPECIFIED"
        }
    }
}

public func parseLevel(_ s: String) -> Level {
    switch s.trimmingCharacters(in: .whitespaces).uppercased() {
    case "TRACE": return .trace
    case "DEBUG": return .debug
    case "INFO": return .info
    case "WARN", "WARNING": return .warn
    case "ERROR": return .error
    case "FATAL": return .fatal
    default: return .info
    }
}

public enum EventType: Int32, Sendable {
    case unspecified = 0
    case instanceSpawned = 1
    case instanceReady = 2
    case instanceExited = 3
    case instanceCrashed = 4
    case sessionStarted = 5
    case sessionEnded = 6
    case handlerPanic = 7
    case configReloaded = 8

    public var protoName: String {
        switch self {
        case .instanceSpawned: return "INSTANCE_SPAWNED"
        case .instanceReady: return "INSTANCE_READY"
        case .instanceExited: return "INSTANCE_EXITED"
        case .instanceCrashed: return "INSTANCE_CRASHED"
        case .sessionStarted: return "SESSION_STARTED"
        case .sessionEnded: return "SESSION_ENDED"
        case .handlerPanic: return "HANDLER_PANIC"
        case .configReloaded: return "CONFIG_RELOADED"
        case .unspecified: return "UNSPECIFIED"
        }
    }
}

// MARK: - Chain helpers

public struct Hop: Sendable {
    public let slug: String
    public let instanceUid: String
    public init(slug: String, instanceUid: String) {
        self.slug = slug
        self.instanceUid = instanceUid
    }
}

public func appendDirectChild(_ src: [Hop], childSlug: String, childUid: String) -> [Hop] {
    src + [Hop(slug: childSlug, instanceUid: childUid)]
}

public func enrichForMultilog(_ wire: [Hop], streamSourceSlug: String, streamSourceUid: String) -> [Hop] {
    appendDirectChild(wire, childSlug: streamSourceSlug, childUid: streamSourceUid)
}

// MARK: - Log entries + ring

public struct LogEntry: Sendable {
    public let timestamp: Date
    public let level: Level
    public let slug: String
    public let instanceUid: String
    public let sessionId: String
    public let rpcMethod: String
    public let message: String
    public let fields: [String: String]
    public let caller: String
    public let chain: [Hop]

    public init(timestamp: Date, level: Level, slug: String, instanceUid: String,
                sessionId: String = "", rpcMethod: String = "",
                message: String, fields: [String: String] = [:], caller: String = "",
                chain: [Hop] = []) {
        self.timestamp = timestamp
        self.level = level
        self.slug = slug
        self.instanceUid = instanceUid
        self.sessionId = sessionId
        self.rpcMethod = rpcMethod
        self.message = message
        self.fields = fields
        self.caller = caller
        self.chain = chain
    }
}

public final class LogRing: @unchecked Sendable {
    private let capacity: Int
    private var buf: [LogEntry] = []
    private var subs: [(LogEntry) -> Void] = []
    private let lock = NSLock()

    public init(capacity: Int = 1024) {
        self.capacity = max(1, capacity)
    }

    public func push(_ e: LogEntry) {
        lock.lock()
        buf.append(e)
        if buf.count > capacity {
            buf.removeFirst(buf.count - capacity)
        }
        let snapshot = subs
        lock.unlock()
        for fn in snapshot {
            fn(e)
        }
    }

    public func drain() -> [LogEntry] {
        lock.lock(); defer { lock.unlock() }
        return buf
    }

    public func drainSince(_ cutoff: Date) -> [LogEntry] {
        lock.lock(); defer { lock.unlock() }
        return buf.filter { $0.timestamp >= cutoff }
    }

    @discardableResult
    public func subscribe(_ fn: @escaping (LogEntry) -> Void) -> () -> Void {
        lock.lock()
        subs.append(fn)
        let index = subs.count - 1
        lock.unlock()
        return { [weak self] in
            guard let self = self else { return }
            self.lock.lock()
            if index < self.subs.count {
                self.subs.remove(at: index)
            }
            self.lock.unlock()
        }
    }

    public var count: Int {
        lock.lock(); defer { lock.unlock() }
        return buf.count
    }
}

// MARK: - Events

public struct Event: Sendable {
    public let timestamp: Date
    public let type: EventType
    public let slug: String
    public let instanceUid: String
    public let sessionId: String
    public let payload: [String: String]
    public let chain: [Hop]

    public init(timestamp: Date, type: EventType, slug: String, instanceUid: String,
                sessionId: String = "", payload: [String: String] = [:], chain: [Hop] = []) {
        self.timestamp = timestamp
        self.type = type
        self.slug = slug
        self.instanceUid = instanceUid
        self.sessionId = sessionId
        self.payload = payload
        self.chain = chain
    }
}

public final class EventBus: @unchecked Sendable {
    private let capacity: Int
    private var buf: [Event] = []
    private var subs: [(Event) -> Void] = []
    private var closed = false
    private let lock = NSLock()

    public init(capacity: Int = 256) {
        self.capacity = max(1, capacity)
    }

    public func emit(_ e: Event) {
        lock.lock()
        if closed { lock.unlock(); return }
        buf.append(e)
        if buf.count > capacity {
            buf.removeFirst(buf.count - capacity)
        }
        let snapshot = subs
        lock.unlock()
        for fn in snapshot { fn(e) }
    }

    public func drain() -> [Event] {
        lock.lock(); defer { lock.unlock() }
        return buf
    }

    public func drainSince(_ cutoff: Date) -> [Event] {
        lock.lock(); defer { lock.unlock() }
        return buf.filter { $0.timestamp >= cutoff }
    }

    @discardableResult
    public func subscribe(_ fn: @escaping (Event) -> Void) -> () -> Void {
        lock.lock()
        subs.append(fn)
        let index = subs.count - 1
        lock.unlock()
        return { [weak self] in
            guard let self = self else { return }
            self.lock.lock()
            if index < self.subs.count {
                self.subs.remove(at: index)
            }
            self.lock.unlock()
        }
    }

    public func close() {
        lock.lock()
        closed = true
        subs.removeAll()
        lock.unlock()
    }
}

// MARK: - Metrics

public final class Counter: @unchecked Sendable {
    public let name: String
    public let help: String
    public let labels: [String: String]
    private var value: Int64 = 0
    private let lock = NSLock()

    fileprivate init(name: String, help: String, labels: [String: String]) {
        self.name = name
        self.help = help
        self.labels = labels
    }

    public func inc(_ n: Int64 = 1) {
        if n < 0 { return }
        lock.lock(); value &+= n; lock.unlock()
    }
    public func add(_ n: Int64) { inc(n) }
    public func read() -> Int64 {
        lock.lock(); defer { lock.unlock() }
        return value
    }
}

public final class Gauge: @unchecked Sendable {
    public let name: String
    public let help: String
    public let labels: [String: String]
    private var value: Double = 0
    private let lock = NSLock()

    fileprivate init(name: String, help: String, labels: [String: String]) {
        self.name = name
        self.help = help
        self.labels = labels
    }

    public func set(_ v: Double) { lock.lock(); value = v; lock.unlock() }
    public func add(_ d: Double) { lock.lock(); value += d; lock.unlock() }
    public func read() -> Double {
        lock.lock(); defer { lock.unlock() }
        return value
    }
}

public struct HistogramSnapshot: Sendable {
    public let bounds: [Double]
    public let counts: [Int64]
    public let total: Int64
    public let sum: Double

    public func quantile(_ q: Double) -> Double {
        if total == 0 { return .nan }
        let target = Double(total) * q
        for i in 0..<counts.count {
            if Double(counts[i]) >= target { return bounds[i] }
        }
        return .infinity
    }
}

public let defaultBuckets: [Double] = [
    50e-6, 100e-6, 250e-6, 500e-6,
    1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
    1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
]

public final class Histogram: @unchecked Sendable {
    public let name: String
    public let help: String
    public let labels: [String: String]
    private let bounds: [Double]
    private var counts: [Int64]
    private var total: Int64 = 0
    private var sum: Double = 0
    private let lock = NSLock()

    fileprivate init(name: String, help: String, labels: [String: String], bounds: [Double]?) {
        self.name = name
        self.help = help
        self.labels = labels
        var b = bounds ?? defaultBuckets
        b.sort()
        self.bounds = b
        self.counts = [Int64](repeating: 0, count: b.count)
    }

    public func observe(_ v: Double) {
        lock.lock()
        total &+= 1
        sum += v
        for i in 0..<bounds.count {
            if v <= bounds[i] { counts[i] &+= 1 }
        }
        lock.unlock()
    }

    public func observeDuration(_ d: TimeInterval) { observe(d) }

    public func snapshot() -> HistogramSnapshot {
        lock.lock(); defer { lock.unlock() }
        return HistogramSnapshot(bounds: bounds, counts: counts, total: total, sum: sum)
    }
}

private func metricKey(_ name: String, _ labels: [String: String]) -> String {
    if labels.isEmpty { return name }
    let keys = labels.keys.sorted()
    var s = name
    for k in keys {
        s += "|\(k)=\(labels[k] ?? "")"
    }
    return s
}

public final class Registry: @unchecked Sendable {
    private var counters: [String: Counter] = [:]
    private var gauges: [String: Gauge] = [:]
    private var histograms: [String: Histogram] = [:]
    private let lock = NSLock()

    public func counter(_ name: String, help: String = "", labels: [String: String] = [:]) -> Counter {
        let k = metricKey(name, labels)
        lock.lock(); defer { lock.unlock() }
        if let c = counters[k] { return c }
        let c = Counter(name: name, help: help, labels: labels)
        counters[k] = c
        return c
    }

    public func gauge(_ name: String, help: String = "", labels: [String: String] = [:]) -> Gauge {
        let k = metricKey(name, labels)
        lock.lock(); defer { lock.unlock() }
        if let g = gauges[k] { return g }
        let g = Gauge(name: name, help: help, labels: labels)
        gauges[k] = g
        return g
    }

    public func histogram(_ name: String, help: String = "", labels: [String: String] = [:],
                          bounds: [Double]? = nil) -> Histogram {
        let k = metricKey(name, labels)
        lock.lock(); defer { lock.unlock() }
        if let h = histograms[k] { return h }
        let h = Histogram(name: name, help: help, labels: labels, bounds: bounds)
        histograms[k] = h
        return h
    }

    public func listCounters() -> [Counter] {
        lock.lock(); defer { lock.unlock() }
        return counters.values.sorted(by: { $0.name < $1.name })
    }

    public func listGauges() -> [Gauge] {
        lock.lock(); defer { lock.unlock() }
        return gauges.values.sorted(by: { $0.name < $1.name })
    }

    public func listHistograms() -> [Histogram] {
        lock.lock(); defer { lock.unlock() }
        return histograms.values.sorted(by: { $0.name < $1.name })
    }
}

// MARK: - Config + Observability

public struct ObsConfig: Sendable {
    public var slug: String
    public var defaultLogLevel: Level
    public var promAddr: String
    public var redactedFields: [String]
    public var logsRingSize: Int
    public var eventsRingSize: Int
    public var runDir: String
    public var instanceUid: String
    public var organismUid: String
    public var organismSlug: String

    public init(slug: String = "",
                defaultLogLevel: Level = .info,
                promAddr: String = "",
                redactedFields: [String] = [],
                logsRingSize: Int = 1024,
                eventsRingSize: Int = 256,
                runDir: String = "",
                instanceUid: String = "",
                organismUid: String = "",
                organismSlug: String = "") {
        self.slug = slug
        self.defaultLogLevel = defaultLogLevel
        self.promAddr = promAddr
        self.redactedFields = redactedFields
        self.logsRingSize = logsRingSize
        self.eventsRingSize = eventsRingSize
        self.runDir = runDir
        self.instanceUid = instanceUid
        self.organismUid = organismUid
        self.organismSlug = organismSlug
    }
}

public final class HolonLogger: @unchecked Sendable {
    fileprivate weak var obs: Observability?
    public let name: String
    private var level: Level
    private let lock = NSLock()

    fileprivate init(obs: Observability?, name: String) {
        self.obs = obs
        self.name = name
        self.level = obs?.cfg.defaultLogLevel ?? .fatal
    }

    public func setLevel(_ l: Level) { lock.lock(); level = l; lock.unlock() }

    public func enabled(_ l: Level) -> Bool {
        lock.lock(); defer { lock.unlock() }
        return obs != nil && l >= level
    }

    public func log(_ lvl: Level, _ message: String, fields: [String: String] = [:],
                    file: String = #fileID, line: Int = #line) {
        guard enabled(lvl), let obs = obs else { return }
        let redact = Set(obs.cfg.redactedFields)
        var out: [String: String] = [:]
        for (k, v) in fields {
            if k.isEmpty { continue }
            out[k] = redact.contains(k) ? "<redacted>" : v
        }
        let entry = LogEntry(
            timestamp: Date(),
            level: lvl,
            slug: obs.cfg.slug,
            instanceUid: obs.cfg.instanceUid,
            message: message,
            fields: out,
            caller: "\(file):\(line)"
        )
        obs.logRing?.push(entry)
    }

    public func trace(_ m: String, _ f: [String: String] = [:], file: String = #fileID, line: Int = #line) { log(.trace, m, fields: f, file: file, line: line) }
    public func debug(_ m: String, _ f: [String: String] = [:], file: String = #fileID, line: Int = #line) { log(.debug, m, fields: f, file: file, line: line) }
    public func info(_ m: String, _ f: [String: String] = [:], file: String = #fileID, line: Int = #line) { log(.info, m, fields: f, file: file, line: line) }
    public func warn(_ m: String, _ f: [String: String] = [:], file: String = #fileID, line: Int = #line) { log(.warn, m, fields: f, file: file, line: line) }
    public func error(_ m: String, _ f: [String: String] = [:], file: String = #fileID, line: Int = #line) { log(.error, m, fields: f, file: file, line: line) }
    public func fatal(_ m: String, _ f: [String: String] = [:], file: String = #fileID, line: Int = #line) { log(.fatal, m, fields: f, file: file, line: line) }
}

public final class Observability: @unchecked Sendable {
    public let cfg: ObsConfig
    public let families: Set<Family>
    public let logRing: LogRing?
    public let eventBus: EventBus?
    public let registry: Registry?
    private var loggers: [String: HolonLogger] = [:]
    private let lock = NSLock()

    fileprivate init(cfg: ObsConfig, families: Set<Family>) {
        self.cfg = cfg
        self.families = families
        self.logRing = families.contains(.logs) ? LogRing(capacity: cfg.logsRingSize) : nil
        self.eventBus = families.contains(.events) ? EventBus(capacity: cfg.eventsRingSize) : nil
        self.registry = families.contains(.metrics) ? Registry() : nil
    }

    public func enabled(_ f: Family) -> Bool { families.contains(f) }

    public var isOrganismRoot: Bool {
        !cfg.organismUid.isEmpty && cfg.organismUid == cfg.instanceUid
    }

    public func logger(_ name: String) -> HolonLogger {
        if !families.contains(.logs) { return disabledLogger }
        lock.lock(); defer { lock.unlock() }
        if let l = loggers[name] { return l }
        let l = HolonLogger(obs: self, name: name)
        loggers[name] = l
        return l
    }

    public func counter(_ name: String, help: String = "", labels: [String: String] = [:]) -> Counter? {
        registry?.counter(name, help: help, labels: labels)
    }

    public func gauge(_ name: String, help: String = "", labels: [String: String] = [:]) -> Gauge? {
        registry?.gauge(name, help: help, labels: labels)
    }

    public func histogram(_ name: String, help: String = "", labels: [String: String] = [:],
                          bounds: [Double]? = nil) -> Histogram? {
        registry?.histogram(name, help: help, labels: labels, bounds: bounds)
    }

    public func emit(_ type: EventType, payload: [String: String] = [:]) {
        guard let bus = eventBus else { return }
        let redact = Set(cfg.redactedFields)
        var p: [String: String] = [:]
        for (k, v) in payload {
            p[k] = redact.contains(k) ? "<redacted>" : v
        }
        bus.emit(Event(
            timestamp: Date(),
            type: type,
            slug: cfg.slug,
            instanceUid: cfg.instanceUid,
            payload: p
        ))
    }

    public func close() {
        eventBus?.close()
    }
}

private let disabledLogger: HolonLogger = {
    let cfg = ObsConfig(defaultLogLevel: .fatal)
    let obs = Observability(cfg: cfg, families: [])
    return HolonLogger(obs: obs, name: "")
}()

// MARK: - Package-scope singleton

private let obsLock = NSLock()
private nonisolated(unsafe) var _current: Observability?

@discardableResult
public func configure(_ cfg: ObsConfig, env: [String: String] = ProcessInfo.processInfo.environment) -> Observability {
    let families = parseOpObs(env["OP_OBS"] ?? "")
    var normalized = cfg
    if normalized.slug.isEmpty {
        normalized.slug = CommandLine.arguments.first.map { (($0 as NSString).lastPathComponent) } ?? ""
    }
    if normalized.instanceUid.isEmpty {
        normalized.instanceUid = UUID().uuidString
    }
    if !normalized.runDir.isEmpty {
        normalized.runDir = deriveRunDir(root: normalized.runDir, slug: normalized.slug, uid: normalized.instanceUid)
    }
    let obs = Observability(cfg: normalized, families: families)
    obsLock.lock()
    _current = obs
    obsLock.unlock()
    return obs
}

@discardableResult
public func fromEnv(_ base: ObsConfig = ObsConfig(), env: [String: String] = ProcessInfo.processInfo.environment) -> Observability {
    var cfg = base
    if cfg.instanceUid.isEmpty { cfg.instanceUid = env["OP_INSTANCE_UID"] ?? "" }
    if cfg.organismUid.isEmpty { cfg.organismUid = env["OP_ORGANISM_UID"] ?? "" }
    if cfg.organismSlug.isEmpty { cfg.organismSlug = env["OP_ORGANISM_SLUG"] ?? "" }
    if cfg.promAddr.isEmpty { cfg.promAddr = env["OP_PROM_ADDR"] ?? "" }
    if cfg.runDir.isEmpty { cfg.runDir = env["OP_RUN_DIR"] ?? "" }
    return configure(cfg, env: env)
}

public func current() -> Observability {
    obsLock.lock(); defer { obsLock.unlock() }
    return _current ?? Observability(cfg: ObsConfig(defaultLogLevel: .fatal), families: [])
}

public func reset() {
    obsLock.lock()
    _current?.close()
    _current = nil
    obsLock.unlock()
}

public func deriveRunDir(root: String, slug: String, uid: String) -> String {
    if root.isEmpty || slug.isEmpty || uid.isEmpty { return root }
    return (root as NSString).appendingPathComponent((slug as NSString).appendingPathComponent(uid))
}

// MARK: - Proto conversion + gRPC service

private func timestamp(_ date: Date) -> Google_Protobuf_Timestamp {
    Google_Protobuf_Timestamp(date: date)
}

private func hopToProto(_ hop: Hop) -> Holons_V1_ChainHop {
    var out = Holons_V1_ChainHop()
    out.slug = hop.slug
    out.instanceUid = hop.instanceUid
    return out
}

private func levelToProto(_ level: Level) -> Holons_V1_LogLevel {
    Holons_V1_LogLevel(rawValue: Int(level.rawValue)) ?? .unspecified
}

private func eventTypeToProto(_ type: EventType) -> Holons_V1_EventType {
    Holons_V1_EventType(rawValue: Int(type.rawValue)) ?? .unspecified
}

public func toProtoLogEntry(_ entry: LogEntry) -> Holons_V1_LogEntry {
    var out = Holons_V1_LogEntry()
    out.ts = timestamp(entry.timestamp)
    out.level = levelToProto(entry.level)
    out.slug = entry.slug
    out.instanceUid = entry.instanceUid
    out.sessionID = entry.sessionId
    out.rpcMethod = entry.rpcMethod
    out.message = entry.message
    out.fields = entry.fields
    out.caller = entry.caller
    out.chain = entry.chain.map(hopToProto)
    return out
}

private func histogramToProto(_ snapshot: HistogramSnapshot) -> Holons_V1_HistogramSample {
    var out = Holons_V1_HistogramSample()
    out.buckets = zip(snapshot.bounds, snapshot.counts).map { upper, count in
        var bucket = Holons_V1_Bucket()
        bucket.upperBound = upper
        bucket.count = count
        return bucket
    }
    out.count = snapshot.total
    out.sum = snapshot.sum
    return out
}

public func toProtoMetricSamples(_ registry: Registry) -> [Holons_V1_MetricSample] {
    var samples: [Holons_V1_MetricSample] = []
    for counter in registry.listCounters() {
        var sample = Holons_V1_MetricSample()
        sample.name = counter.name
        sample.help = counter.help
        sample.labels = counter.labels
        sample.counter = counter.read()
        samples.append(sample)
    }
    for gauge in registry.listGauges() {
        var sample = Holons_V1_MetricSample()
        sample.name = gauge.name
        sample.help = gauge.help
        sample.labels = gauge.labels
        sample.gauge = gauge.read()
        samples.append(sample)
    }
    for histogram in registry.listHistograms() {
        var sample = Holons_V1_MetricSample()
        sample.name = histogram.name
        sample.help = histogram.help
        sample.labels = histogram.labels
        sample.histogram = histogramToProto(histogram.snapshot())
        samples.append(sample)
    }
    return samples
}

public func toProtoEvent(_ event: Event) -> Holons_V1_EventInfo {
    var out = Holons_V1_EventInfo()
    out.ts = timestamp(event.timestamp)
    out.type = eventTypeToProto(event.type)
    out.slug = event.slug
    out.instanceUid = event.instanceUid
    out.sessionID = event.sessionId
    out.payload = event.payload
    out.chain = event.chain.map(hopToProto)
    return out
}

public final class HolonObservabilityService: Holons_V1_HolonObservabilityProvider {
    private let obs: Observability

    public init(_ obs: Observability = current()) {
        self.obs = obs
    }

    public func logs(
        request: Holons_V1_LogsRequest,
        context: StreamingResponseCallContext<Holons_V1_LogEntry>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.logs), let ring = obs.logRing else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "logs family is not enabled (OP_OBS)")
            )
        }
        let minLevel = request.minLevel.rawValue == 0 ? Int(Level.info.rawValue) : request.minLevel.rawValue
        let entries = request.hasSince
            ? ring.drainSince(Date().addingTimeInterval(-durationSeconds(request.since)))
            : ring.drain()
        var future = context.eventLoop.makeSucceededFuture(())
        for entry in entries where matchLog(entry, minLevel: minLevel, sessionIds: request.sessionIds, rpcMethods: request.rpcMethods) {
            future = future.flatMap { context.sendResponse(toProtoLogEntry(entry)) }
        }
        return future.map { .ok }
    }

    public func metrics(
        request: Holons_V1_MetricsRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_MetricsSnapshot> {
        guard obs.enabled(.metrics), let registry = obs.registry else {
            return context.eventLoop.makeFailedFuture(
                GRPCStatus(code: .failedPrecondition, message: "metrics family is not enabled (OP_OBS)")
            )
        }
        var samples = toProtoMetricSamples(registry)
        if !request.namePrefixes.isEmpty {
            samples = samples.filter { sample in
                request.namePrefixes.contains { prefix in sample.name.hasPrefix(prefix) }
            }
        }
        var snapshot = Holons_V1_MetricsSnapshot()
        snapshot.capturedAt = timestamp(Date())
        snapshot.slug = obs.cfg.slug
        snapshot.instanceUid = obs.cfg.instanceUid
        snapshot.samples = samples
        return context.eventLoop.makeSucceededFuture(snapshot)
    }

    public func events(
        request: Holons_V1_EventsRequest,
        context: StreamingResponseCallContext<Holons_V1_EventInfo>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.events), let bus = obs.eventBus else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "events family is not enabled (OP_OBS)")
            )
        }
        let wanted = Set(request.types.map { $0.rawValue })
        let events = request.hasSince
            ? bus.drainSince(Date().addingTimeInterval(-durationSeconds(request.since)))
            : bus.drain()
        var future = context.eventLoop.makeSucceededFuture(())
        for event in events where matchEvent(event, wanted: wanted) {
            future = future.flatMap { context.sendResponse(toProtoEvent(event)) }
        }
        return future.map { .ok }
    }
}

public func registerObservabilityService(_ obs: Observability = current()) -> CallHandlerProvider {
    HolonObservabilityService(obs)
}

private func durationSeconds(_ duration: Google_Protobuf_Duration) -> TimeInterval {
    TimeInterval(duration.seconds) + TimeInterval(duration.nanos) / 1_000_000_000
}

private func matchLog(_ entry: LogEntry, minLevel: Int, sessionIds: [String], rpcMethods: [String]) -> Bool {
    if Int(entry.level.rawValue) < minLevel { return false }
    if !sessionIds.isEmpty && !sessionIds.contains(entry.sessionId) { return false }
    if !rpcMethods.isEmpty && !rpcMethods.contains(entry.rpcMethod) { return false }
    return true
}

private func matchEvent(_ event: Event, wanted: Set<Int>) -> Bool {
    wanted.isEmpty || wanted.contains(Int(event.type.rawValue))
}

// MARK: - Disk writers

public func enableDiskWriters(_ runDir: String) {
    obsLock.lock()
    let obs = _current
    obsLock.unlock()
    guard let obs = obs, !runDir.isEmpty else { return }
    try? FileManager.default.createDirectory(atPath: runDir, withIntermediateDirectories: true)

    if obs.enabled(.logs), let ring = obs.logRing {
        let fp = (runDir as NSString).appendingPathComponent("stdout.log")
        ring.subscribe { entry in
            var rec: [String: Any] = [
                "kind": "log",
                "ts": ISO8601DateFormatter().string(from: entry.timestamp),
                "level": entry.level.name,
                "slug": entry.slug,
                "instance_uid": entry.instanceUid,
                "message": entry.message,
            ]
            if !entry.sessionId.isEmpty { rec["session_id"] = entry.sessionId }
            if !entry.rpcMethod.isEmpty { rec["rpc_method"] = entry.rpcMethod }
            if !entry.fields.isEmpty { rec["fields"] = entry.fields }
            if !entry.caller.isEmpty { rec["caller"] = entry.caller }
            if !entry.chain.isEmpty {
                rec["chain"] = entry.chain.map { ["slug": $0.slug, "instance_uid": $0.instanceUid] }
            }
            appendJSONL(fp, rec)
        }
    }

    if obs.enabled(.events), let bus = obs.eventBus {
        let fp = (runDir as NSString).appendingPathComponent("events.jsonl")
        bus.subscribe { e in
            var rec: [String: Any] = [
                "kind": "event",
                "ts": ISO8601DateFormatter().string(from: e.timestamp),
                "type": e.type.protoName,
                "slug": e.slug,
                "instance_uid": e.instanceUid,
            ]
            if !e.sessionId.isEmpty { rec["session_id"] = e.sessionId }
            if !e.payload.isEmpty { rec["payload"] = e.payload }
            if !e.chain.isEmpty {
                rec["chain"] = e.chain.map { ["slug": $0.slug, "instance_uid": $0.instanceUid] }
            }
            appendJSONL(fp, rec)
        }
    }
}

private func appendJSONL(_ path: String, _ rec: [String: Any]) {
    guard let data = try? JSONSerialization.data(withJSONObject: rec, options: []) else { return }
    var payload = data
    payload.append(0x0a) // newline
    if FileManager.default.fileExists(atPath: path) {
        if let h = try? FileHandle(forWritingTo: URL(fileURLWithPath: path)) {
            defer { try? h.close() }
            _ = try? h.seekToEnd()
            try? h.write(contentsOf: payload)
        }
    } else {
        try? payload.write(to: URL(fileURLWithPath: path))
    }
}

public struct MetaJson: Codable {
    public var slug: String
    public var uid: String
    public var pid: Int
    public var startedAt: Date
    public var mode: String
    public var transport: String
    public var address: String
    public var metricsAddr: String
    public var logPath: String
    public var logBytesRotated: Int64
    public var organismUid: String
    public var organismSlug: String
    public var isDefault: Bool

    enum CodingKeys: String, CodingKey {
        case slug, uid, pid
        case startedAt = "started_at"
        case mode, transport, address
        case metricsAddr = "metrics_addr"
        case logPath = "log_path"
        case logBytesRotated = "log_bytes_rotated"
        case organismUid = "organism_uid"
        case organismSlug = "organism_slug"
        case isDefault = "default"
    }

    public init(slug: String, uid: String, pid: Int, startedAt: Date,
                mode: String = "persistent", transport: String = "",
                address: String = "", metricsAddr: String = "",
                logPath: String = "", logBytesRotated: Int64 = 0,
                organismUid: String = "", organismSlug: String = "",
                isDefault: Bool = false) {
        self.slug = slug
        self.uid = uid
        self.pid = pid
        self.startedAt = startedAt
        self.mode = mode
        self.transport = transport
        self.address = address
        self.metricsAddr = metricsAddr
        self.logPath = logPath
        self.logBytesRotated = logBytesRotated
        self.organismUid = organismUid
        self.organismSlug = organismSlug
        self.isDefault = isDefault
    }
}

public func writeMetaJson(_ runDir: String, _ meta: MetaJson) throws {
    try FileManager.default.createDirectory(atPath: runDir, withIntermediateDirectories: true)
    let url = URL(fileURLWithPath: (runDir as NSString).appendingPathComponent("meta.json"))
    let tmp = url.appendingPathExtension("tmp")
    let enc = JSONEncoder()
    enc.outputFormatting = [.prettyPrinted, .sortedKeys]
    enc.dateEncodingStrategy = .iso8601
    let data = try enc.encode(meta)
    try data.write(to: tmp)
    if FileManager.default.fileExists(atPath: url.path) {
        _ = try? FileManager.default.removeItem(at: url)
    }
    try FileManager.default.moveItem(at: tmp, to: url)
}
