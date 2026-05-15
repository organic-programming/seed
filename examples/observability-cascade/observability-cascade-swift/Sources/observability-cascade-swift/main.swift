import Dispatch
import Foundation
import GRPC
import Holons
import NIOCore

#if os(Linux)
import Glibc
#else
import Darwin
#endif

private let swiftSlug = "observability-cascade-swift-node"
private let goSlug = "observability-cascade-go-node"
private let runTicks = 3

do {
    try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    let rawArgs = Array(CommandLine.arguments.dropFirst())
    let args = Set(rawArgs)
    if ProcessInfo.processInfo.environment["OP_COAX_SERVER_ENABLED"] == "1" {
        let listenURI = ProcessInfo.processInfo.environment["OP_COAX_SERVER_LISTEN_URI"] ?? "tcp://127.0.0.1:0"
        try serveComposite(["--listen", listenURI])
    } else if let first = rawArgs.first, canonicalCommand(first) == "serve" {
        try serveComposite(Array(rawArgs.dropFirst()))
    } else if args.contains("--multi-pattern") {
        let report = runMultiPatternReport(emit: true)
        if report.totalFail > 0 { exit(1) }
    } else {
        let live = args.contains("--live-stream")
        let report = runReport(name: live ? "live-stream" : "default", members: ownLanguageMembers(), live: live, emit: true)
        if report.fail > 0 { exit(1) }
    }
} catch {
    FileHandle.standardError.write(Data("FAIL: \(error)\n".utf8))
    exit(1)
}

private func serveComposite(_ args: [String]) throws {
    let parsed = Serve.parseOptions(args)
    try Serve.runWithOptions(
        normalizedListenURI(parsed.listenURI),
        serviceProviders: [ObservabilityCascadeProvider()],
        options: Serve.Options(reflect: parsed.reflect, slug: "observability-cascade-swift")
    )
}

private final class ObservabilityCascadeProvider: ObservabilityCascade_V1_ObservabilityCascadeServiceProvider {
    let interceptors: ObservabilityCascade_V1_ObservabilityCascadeServiceServerInterceptorFactoryProtocol? = nil

    func runDefault(
        request: ObservabilityCascade_V1_RunRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<ObservabilityCascade_V1_CascadeReport> {
        _ = request
        return runAsync(context) {
            runReport(name: "default", members: ownLanguageMembers(), live: false, emit: false)
        }
    }

    func runLiveStream(
        request: ObservabilityCascade_V1_RunRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<ObservabilityCascade_V1_CascadeReport> {
        _ = request
        return runAsync(context) {
            runReport(name: "live-stream", members: ownLanguageMembers(), live: true, emit: false)
        }
    }

    func runMultiPattern(
        request: ObservabilityCascade_V1_RunRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<ObservabilityCascade_V1_MultiPatternReport> {
        _ = request
        let promise = context.eventLoop.makePromise(of: ObservabilityCascade_V1_MultiPatternReport.self)
        DispatchQueue.global(qos: .userInitiated).async {
            let report = runMultiPatternReport(emit: false)
            context.eventLoop.execute { promise.succeed(report) }
        }
        return promise.futureResult
    }

    private func runAsync(
        _ context: StatusOnlyCallContext,
        _ body: @escaping () -> ObservabilityCascade_V1_CascadeReport
    ) -> EventLoopFuture<ObservabilityCascade_V1_CascadeReport> {
        let promise = context.eventLoop.makePromise(of: ObservabilityCascade_V1_CascadeReport.self)
        DispatchQueue.global(qos: .userInitiated).async {
            let report = body()
            context.eventLoop.execute { promise.succeed(report) }
        }
        return promise.futureResult
    }
}

private struct LanguageMember {
    let lang: String
    let slug: String
    let binary: String
}

private struct NamedPattern {
    let name: String
    let members: [LanguageMember]
}

private func runMultiPatternReport(emit: Bool) -> ObservabilityCascade_V1_MultiPatternReport {
    let started = Date()
    let patterns = swiftPatterns()
    var out = ObservabilityCascade_V1_MultiPatternReport()
    if emit {
        print("=== observability-cascade-swift --multi-pattern ===")
        print()
    }
    for (idx, pattern) in patterns.enumerated() {
        if emit {
            print("Pattern \(idx + 1)/\(patterns.count): \(pattern.name)")
        }
        let report = runReport(name: pattern.name, members: pattern.members, live: true, emit: emit)
        out.patterns.append(report)
        out.totalPass += report.pass
        out.totalFail += report.fail
        if emit {
            print("Pattern \(pattern.name): \(report.pass)/\(report.ticks) \(passText(report.fail == 0)) (elapsed=\(elapsedText(report.elapsedUs)))")
            print()
        }
    }
    out.totalElapsedUs = elapsedUS(since: started)
    if emit {
        print("Summary: \(out.totalPass) PASS / \(out.totalFail) FAIL across \(out.totalPass + out.totalFail) ticks (total elapsed=\(elapsedText(out.totalElapsedUs)))")
    }
    return out
}

private func runReport(name: String, members: [LanguageMember], live: Bool, emit: Bool) -> ObservabilityCascade_V1_CascadeReport {
    ensureCascadeObservability()
    let reportStart = Date()
    var report = ObservabilityCascade_V1_CascadeReport()
    report.name = name
    let timeout: TimeInterval = live ? 1.0 : 3.0
    let poll: TimeInterval = live ? 0.05 : 0.1

    if emit {
        print("=== observability-cascade-swift \(name == "default" ? "" : "--\(name)") ===")
        print()
    }

    for phaseIndex in TransportCoverageSequence.indices {
        let phaseStart = Date()
        let to = TransportCoverageSequence[phaseIndex]
        let from = phaseIndex == 0 ? to : TransportCoverageSequence[phaseIndex - 1]
        var phase = ObservabilityCascade_V1_PhaseResult()
        phase.name = String(format: "%02d-%@→%@", phaseIndex + 1, from, to)
        if emit {
            print("Phase \(phaseIndex + 1)/\(TransportCoverageSequence.count): \(phase.name)")
        }

        let cascade: Cascade
        do {
            cascade = try Composite.buildCascade(CascadeOptions(
                transport: to,
                members: childSpecs(members),
                extraEnv: [
                    "OP_OBS": "logs,events,metrics,prom",
                    "OP_PROM_ADDR": "127.0.0.1:0",
                ]
            ))
        } catch {
            phase.fail += Int32(runTicks)
            let evidence = compactEvidence("\(error)")
            for tick in 1...runTicks {
                phase.failures.append("tick=\(tick) log=spawn event=spawn hops=\(evidence)")
            }
            phase.elapsedUs = elapsedUS(since: phaseStart)
            addPhase(phase, to: &report)
            if emit {
                FileHandle.standardError.write(Data("  Spawn failed: \(evidence)\n".utf8))
                printPhaseSummary(phase)
            }
            continue
        }

        var previous: [String: Int64] = [:]
        for tick in 1...runTicks {
            let sender = "\(name)-phase-\(String(format: "%02d", phaseIndex + 1))-tick-\(tick)"
            let result = runTick(cascade: cascade, sender: sender, note: to, members: members, previous: &previous, timeout: timeout, poll: poll)
            if result.pass {
                phase.pass += 1
            } else {
                phase.fail += 1
                phase.failures.append(result.evidenceLine(tick: tick))
            }
            if emit {
                print("  Tick \(tick)/\(runTicks): \(passText(result.pass))")
                if !result.pass {
                    FileHandle.standardError.write(Data("    \(result.evidenceLine(tick: tick))\n".utf8))
                }
            }
        }
        cascade.stop()
        phase.elapsedUs = elapsedUS(since: phaseStart)
        addPhase(phase, to: &report)
        if emit { printPhaseSummary(phase) }
    }

    report.elapsedUs = elapsedUS(since: reportStart)
    if emit {
        print()
        print("Summary: \(report.ticks) ticks, \(report.pass) PASS, \(report.fail) FAIL (total elapsed=\(elapsedText(report.elapsedUs)))")
    }
    return report
}

private struct TickResult {
    let pass: Bool
    let log: CheckOutcome
    let event: CheckOutcome
    let hops: CheckOutcome

    func evidenceLine(tick: Int) -> String {
        "tick=\(tick) log=\(evidenceText(log)) event=\(evidenceText(event)) hops=\(evidenceText(hops))"
    }
}

private func runTick(
    cascade: Cascade,
    sender: String,
    note: String,
    members: [LanguageMember],
    previous: inout [String: Int64],
    timeout: TimeInterval,
    poll: TimeInterval
) -> TickResult {
    var request = Relay_V1_TickRequest()
    request.sender = sender
    request.note = note
    let client = Relay_V1_RelayServiceNIOClient(channel: cascade.top.channel)
    let response: Relay_V1_TickResponse
    do {
        response = try client.tick(request, callOptions: timeoutOptions(5)).response.wait()
    } catch {
        let failed = CheckOutcome(evidence: compactEvidence("\(error)"))
        return TickResult(pass: false, log: failed, event: failed, hops: failed)
    }

    let hops = checkHops(response.hops, members: members, previous: &previous)
    guard hops.pass, let leaf = response.hops.first else {
        return TickResult(pass: false, log: CheckOutcome(evidence: "skipped"), event: CheckOutcome(evidence: "skipped"), hops: hops)
    }
    let expected = hopChain(response.hops)
    let log = checkRelayedLog(LogCheckOptions(sender: sender, leafUid: leaf.uid, expectedChain: expected, timeout: timeout, pollInterval: poll))
    let event = checkRelayedEvent(EventCheckOptions(eventType: .instanceReady, leafUid: leaf.uid, expectedChain: expected, timeout: timeout, pollInterval: poll))
    return TickResult(pass: hops.pass && log.pass && event.pass, log: log, event: event, hops: hops)
}

private func checkHops(_ hops: [Relay_V1_HopReceipt], members: [LanguageMember], previous: inout [String: Int64]) -> CheckOutcome {
    guard hops.count == members.count else {
        return CheckOutcome(evidence: "hops length \(hops.count) want \(members.count)")
    }
    for idx in hops.indices {
        let want = members[members.count - 1 - idx]
        let hop = hops[idx]
        if hop.slug != want.slug {
            return CheckOutcome(evidence: "hop \(idx) slug=\(hop.slug) want \(want.slug)")
        }
        if hop.uid.isEmpty {
            return CheckOutcome(evidence: "hop \(idx) uid empty")
        }
        if hop.received <= (previous[hop.uid] ?? 0) {
            return CheckOutcome(evidence: "hop \(idx) received=\(hop.received) previous=\(previous[hop.uid] ?? 0)")
        }
        previous[hop.uid] = hop.received
    }
    return CheckOutcome(pass: true)
}

private func hopChain(_ hops: [Relay_V1_HopReceipt]) -> [ChainHop] {
    hops.map { ChainHop(slug: $0.slug, instanceUid: $0.uid) }
}

private func ownLanguageMembers() -> [LanguageMember] {
    let binary = (try? Composite.member("swift-node")) ?? ""
    return [
        LanguageMember(lang: "swift", slug: swiftSlug, binary: binary),
        LanguageMember(lang: "swift", slug: swiftSlug, binary: binary),
        LanguageMember(lang: "swift", slug: swiftSlug, binary: binary),
    ]
}

private func swiftPatterns() -> [NamedPattern] {
    let swiftBin = (try? Composite.member("swift-node")) ?? ""
    let goBin = (try? Composite.member("go-node")) ?? ""
    let bins: [String: LanguageMember] = [
        "swift": LanguageMember(lang: "swift", slug: swiftSlug, binary: swiftBin),
        "go": LanguageMember(lang: "go", slug: goSlug, binary: goBin),
    ]
    let names = [
        "swift-swift-swift", "swift-swift-go", "swift-go-swift", "swift-go-go",
        "go-swift-swift", "go-swift-go", "go-go-swift", "go-go-go",
    ]
    return names.map { name in
        let parts = name.split(separator: "-").map(String.init)
        return NamedPattern(name: name, members: parts.compactMap { bins[$0] })
    }
}

private func childSpecs(_ members: [LanguageMember]) -> [ChildSpec] {
    members.map { ChildSpec(slug: $0.slug, binary: $0.binary) }
}

private func addPhase(_ phase: ObservabilityCascade_V1_PhaseResult, to report: inout ObservabilityCascade_V1_CascadeReport) {
    report.phases.append(phase)
    report.pass += phase.pass
    report.fail += phase.fail
    report.ticks += phase.pass + phase.fail
}

private func ensureCascadeObservability() {
    let obs = current()
    if obs.enabled(.logs), obs.enabled(.events) {
        return
    }
    setenv("OP_OBS", "logs,events,metrics,prom", 1)
    setenv("OP_PROM_ADDR", "127.0.0.1:0", 0)
    _ = try? fromEnv(ObsConfig(slug: "observability-cascade-swift"))
}

private func evidenceText(_ out: CheckOutcome) -> String {
    out.pass ? "ok" : compactEvidence(out.evidence)
}

private func compactEvidence(_ value: String) -> String {
    let compact = value.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
    if compact.isEmpty { return "<empty>" }
    if compact.count <= 240 { return compact }
    return String(compact.prefix(240)) + "..."
}

private func passText(_ pass: Bool) -> String {
    pass ? "PASS" : "FAIL"
}

private func printPhaseSummary(_ phase: ObservabilityCascade_V1_PhaseResult) {
    print("Phase \(phase.name): \(phase.pass)/\(phase.pass + phase.fail) \(passText(phase.fail == 0)) (elapsed=\(elapsedText(phase.elapsedUs)))")
}

private func elapsedUS(since started: Date) -> Int64 {
    Int64(Date().timeIntervalSince(started) * 1_000_000)
}

private func elapsedText(_ elapsedUs: Int64) -> String {
    let seconds = Double(elapsedUs) / 1_000_000.0
    if seconds < 1 {
        return "\(Int(seconds * 1_000))ms"
    }
    if seconds < 60 {
        return String(format: "%.2fs", seconds)
    }
    return String(format: "%.1fm", seconds / 60.0)
}

private func normalizedListenURI(_ listenURI: String) -> String {
    if listenURI.hasPrefix("tcp://:") {
        return "tcp://0.0.0.0:\(listenURI.dropFirst("tcp://:".count))"
    }
    return listenURI
}

private func timeoutOptions(_ seconds: TimeInterval) -> CallOptions {
    CallOptions(timeLimit: .timeout(.nanoseconds(Int64(seconds * 1_000_000_000))))
}

private func canonicalCommand(_ raw: String) -> String {
    raw
        .lowercased()
        .replacingOccurrences(of: "-", with: "")
        .replacingOccurrences(of: "_", with: "")
        .replacingOccurrences(of: " ", with: "")
}
