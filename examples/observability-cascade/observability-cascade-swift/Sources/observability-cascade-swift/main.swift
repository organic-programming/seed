import CascadeNodeSwift
import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif
import GRPC
import Holons
import NIOCore

#if os(Linux)
import Glibc
#else
import Darwin
#endif

private let runPhases = 4
private let runTicks = 3
private let roleOrder = ["D", "C", "B", "A"]
private let transports = ["tcp", "unix", "tcp", "unix"]
private let swiftSlug = "observability-cascade-node-swift"
private let goSlug = "observability-cascade-node-go"

private let app = App(roleOrder: roleOrder, transports: transports)
do {
    let rawArgs = Array(CommandLine.arguments.dropFirst())
    let args = Set(rawArgs)
    if let first = rawArgs.first, canonicalCommand(first) == "serve" {
        try app.serveComposite(Array(rawArgs.dropFirst()))
    } else if args.contains("--multi-pattern") {
        _ = try app.runMultiPattern(emit: true)
    } else if args.contains("--live-stream") {
        _ = try app.runLiveStream(emit: true)
    } else {
        _ = try app.runDefault(emit: true)
    }
} catch {
    FileHandle.standardError.write(Data("\nFAIL: \(error)\n".utf8))
    exit(1)
}

private final class App {
    private let roleOrder: [String]
    private let transports: [String]
    private let sourceRoot: String
    private let examplesRoot: String
    private let repoRoot: String

    init(roleOrder: [String], transports: [String]) {
        self.roleOrder = roleOrder
        self.transports = transports
        self.sourceRoot = try! Self.findSourceRoot()
        self.examplesRoot = URL(fileURLWithPath: sourceRoot, isDirectory: true).deletingLastPathComponent().path
        self.repoRoot = try! Self.findRepoRoot(start: sourceRoot)
    }

    func serveComposite(_ args: [String]) throws {
        try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
        let parsed = Serve.parseOptions(args)
        try Serve.runWithOptions(
            normalizedListenURI(parsed.listenURI),
            serviceProviders: [ObservabilityCascadeProvider(app: self)],
            options: Serve.Options(
                reflect: parsed.reflect,
                slug: "observability-cascade-swift"
            )
        )
    }

    private final class ObservabilityCascadeProvider: ObservabilityCascade_V1_ObservabilityCascadeServiceProvider {
        let interceptors: ObservabilityCascade_V1_ObservabilityCascadeServiceServerInterceptorFactoryProtocol? = nil
        private let app: App

        init(app: App) {
            self.app = app
        }

        func runDefault(
            request: ObservabilityCascade_V1_RunRequest,
            context: StatusOnlyCallContext
        ) -> EventLoopFuture<ObservabilityCascade_V1_CascadeReport> {
            _ = request
            let promise = context.eventLoop.makePromise(of: ObservabilityCascade_V1_CascadeReport.self)
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    let report = try self.app.runDefault(emit: false)
                    context.eventLoop.execute {
                        promise.succeed(toCascadeReport(report))
                    }
                } catch {
                    context.eventLoop.execute {
                        promise.fail(error)
                    }
                }
            }
            return promise.futureResult
        }

        func runLiveStream(
            request: ObservabilityCascade_V1_RunRequest,
            context: StatusOnlyCallContext
        ) -> EventLoopFuture<ObservabilityCascade_V1_CascadeReport> {
            _ = request
            let promise = context.eventLoop.makePromise(of: ObservabilityCascade_V1_CascadeReport.self)
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    let report = try self.app.runLiveStream(emit: false)
                    context.eventLoop.execute {
                        promise.succeed(toCascadeReport(report))
                    }
                } catch {
                    context.eventLoop.execute {
                        promise.fail(error)
                    }
                }
            }
            return promise.futureResult
        }

        func runMultiPattern(
            request: ObservabilityCascade_V1_RunRequest,
            context: StatusOnlyCallContext
        ) -> EventLoopFuture<ObservabilityCascade_V1_MultiPatternReport> {
            _ = request
            let promise = context.eventLoop.makePromise(of: ObservabilityCascade_V1_MultiPatternReport.self)
            DispatchQueue.global(qos: .userInitiated).async {
                do {
                    let report = try self.app.runMultiPattern(emit: false)
                    context.eventLoop.execute {
                        promise.succeed(toMultiPatternReport(report))
                    }
                } catch {
                    context.eventLoop.execute {
                        promise.fail(error)
                    }
                }
            }
            return promise.futureResult
        }
    }

    func runDefault(emit: Bool) throws -> CascadeReportData {
        let binary = try findBinary(swiftSlug)
        let runRoot = try makeRunRoot(prefix: "observability-cascade-swift-")
        output(emit, "=== observability-cascade-swift ===")
        output(emit)
        var totalPass = 0
        var totalFail = 0
        var previous = ""

        for (index, transport) in transports.enumerated() {
            let phase = index + 1
            if previous.isEmpty {
                output(emit, "Phase \(phase)/\(runPhases): transport=\(transport)")
            } else {
                output(emit, "Phase \(phase)/\(runPhases): transport=\(transport) (switching from \(previous))")
            }

            let started = Date()
            let cascade: Cascade
            do {
                cascade = try spawnCascade(phase: phase, transport: transport, specs: allSwiftSpecs(binary), runRoot: runRoot)
            } catch {
                totalFail += runTicks
                output(emit, "  spawn FAIL: \(error)\n")
                previous = transport
                continue
            }
            output(emit, "  spawned 4 nodes in \(elapsed(started))")

            var previousMetric = 0.0
            for tick in 1...runTicks {
                let tickStart = Date()
                let outcome = cascade.runTick(tick: tick, previousMetric: previousMetric)
                if outcome.metric.pass {
                    previousMetric = outcome.metricValue
                }
                let overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
                if overall {
                    totalPass += 1
                } else {
                    totalFail += 1
                }
                output(emit, "  Tick \(tick)/\(runTicks): log \(passText(outcome.log.pass)), event \(passText(outcome.event.pass)), metric \(passText(outcome.metric.pass)) (overall \(passText(overall)) in \(elapsed(tickStart)))")
                if emit {
                    printFailureEvidence("log", outcome.log)
                    printFailureEvidence("event", outcome.event)
                    printFailureEvidence("metric", outcome.metric)
                }
            }
            cascade.stop()
            output(emit)
            previous = transport
        }

        output(emit, "Summary: \(totalPass + totalFail) ticks, \(totalPass) PASS, \(totalFail) FAIL")
        if totalFail > 0 {
            throw RuntimeError("\(totalFail) tick(s) failed")
        }
        return CascadeReportData(
            ticks: totalPass + totalFail,
            pass: totalPass,
            fail: totalFail,
            phases: [PhaseReportData(name: "default", pass: totalPass, fail: totalFail)]
        )
    }

    func runLiveStream(emit: Bool) throws -> CascadeReportData {
        let binary = try findBinary(swiftSlug)
        let runRoot = try makeRunRoot(prefix: "observability-cascade-swift-live-")
        output(emit, "=== observability-cascade-swift --live-stream ===")
        output(emit)
        output(emit, "Setup: opening long-lived Follow:true streams on A")
        output(emit, "       (initial transport: tcp)")
        output(emit)

        var totalPass = 0
        var totalFail = 0
        var cascade: Cascade?
        var streams: LiveStreams?
        let specs = allSwiftSpecs(binary)

        for (index, transport) in transports.enumerated() {
            let phase = index + 1
            if phase == 1 {
                output(emit, "Phase \(phase)/\(runPhases): initial chain (\(transport))")
            } else {
                output(emit, "Phase \(phase)/\(runPhases): respawn on \(transport)")
                let killStart = Date()
                streams?.stop()
                cascade?.stop()
                output(emit, "  killed 4 nodes in \(elapsed(killStart))")
            }

            let spawnStart = Date()
            let phaseCascade: Cascade
            do {
                phaseCascade = try spawnCascade(phase: phase, transport: transport, specs: specs, runRoot: runRoot)
            } catch {
                totalFail += runTicks
                output(emit, "  spawn FAIL: \(error)\n")
                streams = nil
                continue
            }
            output(emit, "  spawned 4 nodes in \(elapsed(spawnStart))")
            if phase > 1 {
                output(emit, "  re-opening Follow:true streams on new A")
            }

            var streamError: String?
            do {
                let opened = try LiveStreams(address: phaseCascade.roles["A"]!.relayAddress)
                opened.start()
                streams = opened
            } catch {
                streams = nil
                streamError = "\(error)"
                output(emit, "  stream re-open failed: \(error)")
            }

            var previousMetric = 0.0
            for tick in 1...runTicks {
                let tickStart = Date()
                let outcome = phaseCascade.runLiveTick(streams: streams, streamOpenError: streamError, tick: tick, previousMetric: previousMetric)
                if outcome.metric.pass {
                    previousMetric = outcome.metricValue
                }
                let overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
                if overall {
                    totalPass += 1
                } else {
                    totalFail += 1
                }
                output(emit, "  Tick \(tick)/\(runTicks): log \(passText(outcome.log.pass)), event \(passText(outcome.event.pass)), metric \(passText(outcome.metric.pass)) (overall \(passText(overall)) in \(elapsed(tickStart)))")
                if emit {
                    printFailureEvidence("log", outcome.log)
                    printFailureEvidence("event", outcome.event)
                    printFailureEvidence("metric", outcome.metric)
                }
            }
            output(emit)
            cascade = phaseCascade
        }

        streams?.stop()
        cascade?.stop()
        output(emit, "Summary: \(totalPass) PASS / \(totalFail) FAIL across \(totalPass + totalFail) ticks")
        if totalFail > 0 {
            throw RuntimeError("\(totalFail) tick(s) failed")
        }
        return CascadeReportData(
            ticks: totalPass + totalFail,
            pass: totalPass,
            fail: totalFail,
            phases: [PhaseReportData(name: "live-stream", pass: totalPass, fail: totalFail)]
        )
    }

    func runMultiPattern(emit: Bool) throws -> MultiPatternReportData {
        let swiftBinary = try findBinary(swiftSlug)
        let goBinary = try findBinary(goSlug)
        let patterns = [
            CascadePattern(name: "swift-swift-swift-swift", roles: allSwiftSpecs(swiftBinary)),
            CascadePattern(name: "swift-swift-go-swift", roles: [
                "A": RoleSpec(slug: swiftSlug, binaryPath: swiftBinary),
                "B": RoleSpec(slug: swiftSlug, binaryPath: swiftBinary),
                "C": RoleSpec(slug: goSlug, binaryPath: goBinary),
                "D": RoleSpec(slug: swiftSlug, binaryPath: swiftBinary),
            ]),
            CascadePattern(name: "swift-swift-go-go", roles: [
                "A": RoleSpec(slug: swiftSlug, binaryPath: swiftBinary),
                "B": RoleSpec(slug: swiftSlug, binaryPath: swiftBinary),
                "C": RoleSpec(slug: goSlug, binaryPath: goBinary),
                "D": RoleSpec(slug: goSlug, binaryPath: goBinary),
            ]),
        ]
        let runRoot = try makeRunRoot(prefix: "observability-cascade-swift-multi-")
        output(emit, "=== observability-cascade-swift (multi-pattern) ===")
        output(emit)

        var totalPass = 0
        var totalFail = 0
        var patternReports: [CascadeReportData] = []
        for (patternIndex, pattern) in patterns.enumerated() {
            output(emit, "Pattern \(patternIndex + 1)/\(patterns.count): \(pattern.name)")
            var patternPass = 0
            var patternFail = 0
            for (index, transport) in transports.enumerated() {
                let phase = index + 1
                let started = Date()
                let cascade: Cascade
                do {
                    cascade = try spawnCascade(phase: phase, transport: transport, specs: pattern.roles, runRoot: runRoot)
                } catch {
                    totalFail += runTicks
                    patternFail += runTicks
                    output(emit, "  Phase \(phase)/\(runPhases) (\(transport)): spawn FAIL (\(error))")
                    continue
                }

                var streams: LiveStreams?
                var streamError: String?
                do {
                    let opened = try LiveStreams(address: cascade.roles["A"]!.relayAddress)
                    opened.start()
                    streams = opened
                    let ready = waitFor(timeout: 5, interval: 0.05) {
                        cascade.checkLiveEvent(streams: opened)
                    }
                    if !ready.pass {
                        streamError = "live relay readiness: \(ready.evidence)"
                    }
                } catch {
                    streamError = "\(error)"
                }

                var previousMetric = 0.0
                var results: [String] = []
                var evidence: [String] = []
                for tick in 1...runTicks {
                    let sender = "\(pattern.name)-phase-\(phase)-tick-\(tick)"
                    let outcome = cascade.runLiveTickWithSender(streams: streams, streamOpenError: streamError, sender: sender, previousMetric: previousMetric)
                    if outcome.metric.pass {
                        previousMetric = outcome.metricValue
                    }
                    let overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
                    if overall {
                        patternPass += 1
                        totalPass += 1
                        results.append("Tick \(tick) PASS")
                    } else {
                        patternFail += 1
                        totalFail += 1
                        results.append("Tick \(tick) FAIL (\(failureSummary(outcome)))")
                        evidence.append("      Tick \(tick) evidence: \(compactEvidence(outcome))")
                    }
                }
                output(emit, "  Phase \(phase)/\(runPhases) (\(transport)): \(results.joined(separator: ", ")) (spawned in \(elapsed(started)))")
                if emit {
                    for line in evidence {
                        print(line)
                    }
                }
                streams?.stop()
                cascade.stop()
            }
            output(emit, "  Subtotal: \(patternPass)/12 PASS")
            output(emit)
            patternReports.append(CascadeReportData(
                ticks: patternPass + patternFail,
                pass: patternPass,
                fail: patternFail,
                phases: [PhaseReportData(name: pattern.name, pass: patternPass, fail: patternFail)]
            ))
        }

        output(emit, "Summary: \(totalPass) PASS / \(totalFail) FAIL across \(totalPass + totalFail) ticks")
        if totalFail > 0 {
            throw RuntimeError("\(totalFail) tick(s) failed")
        }
        return MultiPatternReportData(patterns: patternReports, totalPass: totalPass, totalFail: totalFail)
    }

    private func spawnCascade(phase: Int, transport: String, specs: [String: RoleSpec], runRoot: String) throws -> Cascade {
        var roles: [String: RoleRuntime] = [:]
        for role in roleOrder {
            guard let spec = specs[role] else {
                throw RuntimeError("missing role spec \(role)")
            }
            roles[role] = try newRoleRuntime(phase: phase, transport: transport, role: role, spec: spec)
        }
        for runtime in roles.values {
            try deleteRecursively(path: "\(runRoot)/\(runtime.slug)/\(runtime.uid)")
        }
        let cascade = Cascade(phase: phase, transport: transport, runRoot: runRoot, roles: roles)
        for role in roleOrder {
            guard let runtime = roles[role] else { continue }
            if let child = childRole(role), let childRuntime = roles[child] {
                runtime.memberAddress = childRuntime.relayAddress
                runtime.memberSlug = childRuntime.slug
            }
            try startRole(cascade: cascade, runtime: runtime)
        }
        Thread.sleep(forTimeInterval: 0.15)
        return cascade
    }

    private func newRoleRuntime(phase: Int, transport: String, role: String, spec: RoleSpec) throws -> RoleRuntime {
        let uid = "relay-p\(String(format: "%02d", phase))-\(role.lowercased())"
        switch transport {
        case "tcp":
            return RoleRuntime(role: role, uid: uid, slug: spec.slug, binaryPath: spec.binaryPath, listenURI: "tcp://127.0.0.1:0", relayAddress: "")
        case "unix":
            let socket = "/tmp/observability-cascade-swift-p\(phase)-\(role.lowercased())-\(ProcessInfo.processInfo.processIdentifier).sock"
            if FileManager.default.fileExists(atPath: socket) {
                try? FileManager.default.removeItem(atPath: socket)
            }
            let uri = "unix://\(socket)"
            return RoleRuntime(role: role, uid: uid, slug: spec.slug, binaryPath: spec.binaryPath, listenURI: uri, relayAddress: uri)
        default:
            throw RuntimeError("unknown transport \(transport)")
        }
    }

    private func startRole(cascade: Cascade, runtime: RoleRuntime) throws {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: runtime.binaryPath)
        var arguments = ["serve", "--listen", runtime.listenURI]
        if !runtime.memberAddress.isEmpty {
            arguments += ["--member", "\(runtime.memberSlug)=\(runtime.memberAddress)"]
        }
        process.arguments = arguments
        process.currentDirectoryURL = URL(fileURLWithPath: repoRoot, isDirectory: true)
        var environment = ProcessInfo.processInfo.environment
        environment["OP_OBS"] = "logs,events,metrics,prom"
        environment["OP_RUN_DIR"] = cascade.runRoot
        environment["OP_INSTANCE_UID"] = runtime.uid
        environment["OP_ORGANISM_UID"] = cascade.roles["A"]!.uid
        environment["OP_ORGANISM_SLUG"] = cascade.roles["A"]!.slug
        environment["OP_PROM_ADDR"] = "127.0.0.1:0"
        process.environment = environment

        let stdout = Pipe()
        let stderr = Pipe()
        process.standardOutput = stdout
        process.standardError = stderr
        stdout.fileHandleForReading.readabilityHandler = { handle in
            _ = handle.availableData
        }
        stderr.fileHandleForReading.readabilityHandler = { [weak runtime] handle in
            let data = handle.availableData
            if !data.isEmpty, let text = String(data: data, encoding: .utf8) {
                runtime?.appendStderr(text)
            }
        }

        try process.run()
        runtime.process = process

        do {
            let meta = try waitMeta(runRoot: cascade.runRoot, slug: runtime.slug, uid: runtime.uid, timeout: 15)
            runtime.metricsAddr = meta.metricsAddr
            runtime.relayAddress = meta.address
            let channel = try connect(runtime.relayAddress, options: ConnectOptions(timeout: 5, start: false))
            runtime.channel = channel
            runtime.relayClient = Relay_V1_RelayServiceNIOClient(channel: channel)
        } catch {
            let stderrText = runtime.stderr.trimmingCharacters(in: .whitespacesAndNewlines)
            throw RuntimeError("start \(runtime.role): \(stderrText.isEmpty ? "\(error)" : stderrText)")
        }
    }

    private func waitMeta(runRoot: String, slug: String, uid: String, timeout: TimeInterval) throws -> MetaJson {
        let url = URL(fileURLWithPath: runRoot, isDirectory: true)
            .appendingPathComponent(slug, isDirectory: true)
            .appendingPathComponent(uid, isDirectory: true)
            .appendingPathComponent("meta.json")
        let deadline = Date().addingTimeInterval(timeout)
        var lastError: Error?
        while Date() < deadline {
            do {
                let data = try Data(contentsOf: url)
                let meta = try JSONDecoder().decode(MetaJson.self, from: data)
                if meta.uid == uid, !meta.address.isEmpty, !meta.metricsAddr.isEmpty {
                    return meta
                }
            } catch {
                lastError = error
            }
            Thread.sleep(forTimeInterval: 0.05)
        }
        throw RuntimeError("meta not ready for \(slug)/\(uid): \(lastError.map { "\($0)" } ?? "timeout")")
    }

    private func allSwiftSpecs(_ binary: String) -> [String: RoleSpec] {
        Dictionary(uniqueKeysWithValues: roleOrder.map { ($0, RoleSpec(slug: swiftSlug, binaryPath: binary)) })
    }

    private func findBinary(_ slug: String) throws -> String {
        let suffix = slug.replacingOccurrences(of: "observability-cascade-node-", with: "").uppercased().replacingOccurrences(of: "-", with: "_")
        let envName = "OBSERVABILITY_CASCADE_NODE_\(suffix)_BIN"
        if let fromEnv = ProcessInfo.processInfo.environment[envName]?.trimmingCharacters(in: .whitespacesAndNewlines),
           !fromEnv.isEmpty {
            return fromEnv
        }
        var roots: [String] = []
        if slug == swiftSlug {
            let swiftNode = "\(sourceRoot)/holons/observability-cascade-node"
            roots.append("\(swiftNode)/.op/build/observability-cascade-node.holon/bin")
            roots.append("\(swiftNode)/.op/build/observability-cascade-node-swift.holon/bin")
        }
        if slug == goSlug {
            let goNode = "\(examplesRoot)/observability-cascade-go/holons/observability-cascade-node"
            roots.append("\(goNode)/.op/build/observability-cascade-node.holon/bin")
            roots.append("\(goNode)/.op/build/observability-cascade-node-go.holon/bin")
        }
        for root in roots {
            if let found = findExecutable(root: root, name: slug) {
                return found
            }
        }
        if let output = try? runCommand("/usr/bin/env", ["op", "--bin", slug], cwd: repoRoot).trimmingCharacters(in: .whitespacesAndNewlines),
           !output.isEmpty {
            return output
        }
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        if let found = findExecutable(root: "\(home)/.op/bin/\(slug).holon/bin", name: slug) {
            return found
        }
        throw RuntimeError("\(slug) binary not found; run op build \(slug) --install")
    }

    private func runCommand(_ executable: String, _ arguments: [String], cwd: String) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = arguments
        process.currentDirectoryURL = URL(fileURLWithPath: cwd, isDirectory: true)
        let output = Pipe()
        let error = Pipe()
        process.standardOutput = output
        process.standardError = error
        try process.run()
        let data = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0 else {
            let err = String(data: error.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
            throw RuntimeError(err.trimmingCharacters(in: .whitespacesAndNewlines))
        }
        return String(data: data, encoding: .utf8) ?? ""
    }

    private func findExecutable(root: String, name: String) -> String? {
        guard let enumerator = FileManager.default.enumerator(atPath: root) else {
            return nil
        }
        for case let entry as String in enumerator {
            let path = "\(root)/\(entry)"
            if URL(fileURLWithPath: path).lastPathComponent == name,
               FileManager.default.isExecutableFile(atPath: path) {
                return path
            }
        }
        return nil
    }

    private static func findSourceRoot() throws -> String {
        if let fromEnv = ProcessInfo.processInfo.environment["OBSERVABILITY_CASCADE_SWIFT_SOURCE_ROOT"]?.trimmingCharacters(in: .whitespacesAndNewlines),
           !fromEnv.isEmpty {
            return URL(fileURLWithPath: fromEnv, isDirectory: true).standardizedFileURL.path
        }
        var url = URL(fileURLWithPath: FileManager.default.currentDirectoryPath, isDirectory: true).standardizedFileURL
        while true {
            if isSourceRoot(url.path) {
                return url.path
            }
            let nested = url.appendingPathComponent("examples/observability-cascade/observability-cascade-swift", isDirectory: true)
            if isSourceRoot(nested.path) {
                return nested.path
            }
            let parent = url.deletingLastPathComponent()
            if parent.path == url.path {
                break
            }
            url = parent
        }
        throw RuntimeError("observability-cascade-swift source root not found")
    }

    private static func isSourceRoot(_ path: String) -> Bool {
        FileManager.default.fileExists(atPath: "\(path)/api/v1/holon.proto") &&
            FileManager.default.fileExists(atPath: "\(path)/holons/observability-cascade-node")
    }

    private static func findRepoRoot(start: String) throws -> String {
        var url = URL(fileURLWithPath: start, isDirectory: true).standardizedFileURL
        while true {
            if FileManager.default.fileExists(atPath: url.appendingPathComponent("sdk", isDirectory: true).path),
               FileManager.default.fileExists(atPath: url.appendingPathComponent("examples", isDirectory: true).path) {
                return url.path
            }
            let parent = url.deletingLastPathComponent()
            if parent.path == url.path {
                break
            }
            url = parent
        }
        throw RuntimeError("repository root not found")
    }

    private func childRole(_ role: String) -> String? {
        switch role {
        case "A": return "B"
        case "B": return "C"
        case "C": return "D"
        default: return nil
        }
    }
}

private final class Cascade {
    let phase: Int
    let transport: String
    let runRoot: String
    let roles: [String: RoleRuntime]

    init(phase: Int, transport: String, runRoot: String, roles: [String: RoleRuntime]) {
        self.phase = phase
        self.transport = transport
        self.runRoot = runRoot
        self.roles = roles
    }

    func runTick(tick: Int, previousMetric: Double) -> TickOutcome {
        runTickWithSender(sender: "phase-\(phase)-tick-\(tick)", previousMetric: previousMetric)
    }

    func runTickWithSender(sender: String, previousMetric: Double) -> TickOutcome {
        var request = Relay_V1_TickRequest()
        request.sender = sender
        request.note = transport
        do {
            _ = try roles["D"]!.relayClient!.tick(request, callOptions: timeoutOptions(5)).response.wait()
        } catch {
            let failed = CheckResult(pass: false, evidence: "\(error)")
            return TickOutcome(log: failed, event: failed, metric: failed, metricValue: previousMetric)
        }
        let log = waitFor(timeout: 3, interval: 0.1) { self.checkLog(sender: sender) }
        let event = waitFor(timeout: 3, interval: 0.1) { self.checkEvent() }
        let metricCheck = MetricCheck(previous: previousMetric)
        let metric = waitFor(timeout: 3, interval: 0.1) { metricCheck.check(cascade: self) }
        return TickOutcome(log: log, event: event, metric: metric, metricValue: metricCheck.value)
    }

    func runLiveTick(streams: LiveStreams?, streamOpenError: String?, tick: Int, previousMetric: Double) -> TickOutcome {
        runLiveTickWithSender(streams: streams, streamOpenError: streamOpenError, sender: "phase-\(phase)-tick-\(tick)", previousMetric: previousMetric)
    }

    func runLiveTickWithSender(streams: LiveStreams?, streamOpenError: String?, sender: String, previousMetric: Double) -> TickOutcome {
        var request = Relay_V1_TickRequest()
        request.sender = sender
        request.note = transport
        do {
            _ = try roles["D"]!.relayClient!.tick(request, callOptions: timeoutOptions(5)).response.wait()
        } catch {
            let failed = CheckResult(pass: false, evidence: "\(error)")
            return TickOutcome(log: failed, event: failed, metric: failed, metricValue: previousMetric)
        }

        let log: CheckResult
        let event: CheckResult
        if let streams, streamOpenError == nil {
            log = waitFor(timeout: 1, interval: 0.05) { self.checkLiveLog(streams: streams, sender: sender) }
            event = waitFor(timeout: 1, interval: 0.05) { self.checkLiveEvent(streams: streams) }
        } else {
            let evidence = "stream re-open failed: \(streamOpenError ?? "streams not open")"
            log = CheckResult(pass: false, evidence: evidence)
            event = CheckResult(pass: false, evidence: evidence)
        }
        let metricCheck = MetricCheck(previous: previousMetric)
        let metric = waitFor(timeout: 1, interval: 0.05) { metricCheck.check(cascade: self) }
        return TickOutcome(log: log, event: event, metric: metric, metricValue: metricCheck.value)
    }

    func checkLog(sender: String) -> CheckResult {
        do {
            let entries = try readLogs(channel: roles["A"]!.channel!)
            for entry in entries {
                guard entry.message == "tick received" else { continue }
                guard entry.fields["sender"] == sender else { continue }
                guard entry.fields["responder_uid"] == roles["D"]!.uid else { continue }
                let err = checkChain(entry.chain)
                return err.isEmpty
                    ? CheckResult(pass: true, evidence: "\(entry)")
                    : CheckResult(pass: false, evidence: "matching log has bad chain: \(err) entry=\(entry)")
            }
            return CheckResult(pass: false, evidence: "no relayed D tick log for sender=\(sender) in \(entries.count) A log entries")
        } catch {
            return CheckResult(pass: false, evidence: "\(error)")
        }
    }

    func checkEvent() -> CheckResult {
        do {
            let events = try readEvents(channel: roles["A"]!.channel!)
            for event in events {
                guard event.type == .instanceReady, event.instanceUid == roles["D"]!.uid else { continue }
                let err = checkChain(event.chain)
                return err.isEmpty
                    ? CheckResult(pass: true, evidence: "\(event)")
                    : CheckResult(pass: false, evidence: "matching event has bad chain: \(err) event=\(event)")
            }
            return CheckResult(pass: false, evidence: "no relayed D INSTANCE_READY event in \(events.count) A events")
        } catch {
            return CheckResult(pass: false, evidence: "\(error)")
        }
    }

    func checkLiveLog(streams: LiveStreams, sender: String) -> CheckResult {
        let entries = streams.logEntries()
        for entry in entries {
            guard entry.message == "tick received" else { continue }
            guard entry.fields["sender"] == sender else { continue }
            guard entry.fields["responder_uid"] == roles["D"]!.uid else { continue }
            let err = checkChain(entry.chain)
            return err.isEmpty
                ? CheckResult(pass: true, evidence: "\(entry)")
                : CheckResult(pass: false, evidence: "matching live log has bad chain: \(err) entry=\(entry)")
        }
        return CheckResult(pass: false, evidence: "no live log found for sender=\(sender); buffer=\(entries.count) errors=\(streams.errors().joined(separator: ","))")
    }

    func checkLiveEvent(streams: LiveStreams) -> CheckResult {
        let events = streams.eventEntries()
        for event in events {
            guard event.type == .instanceReady, event.instanceUid == roles["D"]!.uid else { continue }
            let err = checkChain(event.chain)
            return err.isEmpty
                ? CheckResult(pass: true, evidence: "\(event)")
                : CheckResult(pass: false, evidence: "matching live event has bad chain: \(err) event=\(event)")
        }
        return CheckResult(pass: false, evidence: "no live INSTANCE_READY event for D; buffer=\(events.count) errors=\(streams.errors().joined(separator: ","))")
    }

    func checkMetric(_ metricCheck: MetricCheck) -> CheckResult {
        do {
            let body = try fetchMetrics(roles["D"]!.metricsAddr)
            guard let value = parseCascadeTicks(body: body, uid: roles["D"]!.uid) else {
                return CheckResult(pass: false, evidence: body)
            }
            metricCheck.value = value
            if value <= metricCheck.previous {
                return CheckResult(pass: false, evidence: "cascade_ticks_total=\(value) did not increase beyond \(metricCheck.previous)\n\(body)")
            }
            return CheckResult(pass: true, evidence: "cascade_ticks_total=\(value)")
        } catch {
            return CheckResult(pass: false, evidence: "\(error)")
        }
    }

    func checkChain(_ chain: [Holons_V1_ChainHop]) -> String {
        let expected = ["D", "C", "B"]
        guard chain.count == expected.count else {
            return "chain length \(chain.count), want \(expected.count)"
        }
        for (index, role) in expected.enumerated() {
            let want = roles[role]!
            let hop = chain[index]
            if hop.slug != want.slug || hop.instanceUid != want.uid {
                return "hop \(index) = \(hop.slug)/\(hop.instanceUid), want \(want.slug)/\(want.uid)"
            }
        }
        return ""
    }

    func stop() {
        for role in ["A", "B", "C", "D"] {
            guard let runtime = roles[role] else { continue }
            if let channel = runtime.channel {
                try? disconnect(channel)
                runtime.channel = nil
            }
            if let process = runtime.process, process.isRunning {
                process.terminate()
                Thread.sleep(forTimeInterval: 0.1)
                if process.isRunning {
                    killProcess(process)
                }
            }
        }
    }
}

private final class RoleRuntime {
    let role: String
    let uid: String
    let slug: String
    let binaryPath: String
    let listenURI: String
    var relayAddress: String
    var memberAddress = ""
    var memberSlug = ""
    var metricsAddr = ""
    var process: Process?
    var channel: GRPCChannel?
    var relayClient: Relay_V1_RelayServiceNIOClient?

    private let stderrLock = NSLock()
    private var stderrBuffer = ""

    init(role: String, uid: String, slug: String, binaryPath: String, listenURI: String, relayAddress: String) {
        self.role = role
        self.uid = uid
        self.slug = slug
        self.binaryPath = binaryPath
        self.listenURI = listenURI
        self.relayAddress = relayAddress
    }

    var stderr: String {
        stderrLock.lock()
        defer { stderrLock.unlock() }
        return stderrBuffer
    }

    func appendStderr(_ text: String) {
        stderrLock.lock()
        stderrBuffer += text
        stderrLock.unlock()
    }
}

private final class MetricCheck {
    let previous: Double
    var value: Double

    init(previous: Double) {
        self.previous = previous
        self.value = previous
    }

    func check(cascade: Cascade) -> CheckResult {
        cascade.checkMetric(self)
    }
}

private final class LiveStreams {
    private let address: String
    private let logs = LockedArray<Holons_V1_LogEntry>()
    private let events = LockedArray<Holons_V1_EventInfo>()
    private let streamErrors = LockedArray<String>()
    private var channel: GRPCChannel?
    private var logCall: ServerStreamingCall<Holons_V1_LogsRequest, Holons_V1_LogEntry>?
    private var eventCall: ServerStreamingCall<Holons_V1_EventsRequest, Holons_V1_EventInfo>?

    init(address: String) throws {
        self.address = address
    }

    func start() {
        do {
            let channel = try connect(address, options: ConnectOptions(timeout: 5, start: false))
            self.channel = channel
            let client = Holons_V1_HolonObservabilityClient(channel: channel)

            var logRequest = Holons_V1_LogsRequest()
            logRequest.minLevel = .info
            logRequest.follow = true
            let logs = client.logs(logRequest) { [weak self] entry in
                self?.logs.append(entry)
            }
            logs.status.whenComplete { [weak self] result in
                self?.streamErrors.append("logs stream ended: \(result)")
            }
            self.logCall = logs

            var eventRequest = Holons_V1_EventsRequest()
            eventRequest.follow = true
            let events = client.events(eventRequest) { [weak self] event in
                self?.events.append(event)
            }
            events.status.whenComplete { [weak self] result in
                self?.streamErrors.append("events stream ended: \(result)")
            }
            self.eventCall = events
        } catch {
            streamErrors.append("\(error)")
        }
    }

    func stop() {
        if let channel {
            try? disconnect(channel)
        }
        channel = nil
        logCall = nil
        eventCall = nil
    }

    func logEntries() -> [Holons_V1_LogEntry] { logs.snapshot() }
    func eventEntries() -> [Holons_V1_EventInfo] { events.snapshot() }
    func errors() -> [String] { streamErrors.snapshot() }
}

private final class LockedArray<Element> {
    private let lock = NSLock()
    private var values: [Element] = []

    func append(_ value: Element) {
        lock.lock()
        values.append(value)
        lock.unlock()
    }

    func snapshot() -> [Element] {
        lock.lock()
        defer { lock.unlock() }
        return values
    }
}

private struct RoleSpec {
    let slug: String
    let binaryPath: String
}

private struct CascadePattern {
    let name: String
    let roles: [String: RoleSpec]
}

private struct PhaseReportData {
    let name: String
    let pass: Int
    let fail: Int
    let failures: [String]

    init(name: String, pass: Int, fail: Int, failures: [String] = []) {
        self.name = name
        self.pass = pass
        self.fail = fail
        self.failures = failures
    }
}

private struct CascadeReportData {
    let ticks: Int
    let pass: Int
    let fail: Int
    let phases: [PhaseReportData]
}

private struct MultiPatternReportData {
    let patterns: [CascadeReportData]
    let totalPass: Int
    let totalFail: Int
}

private struct CheckResult {
    let pass: Bool
    let evidence: String
}

private struct TickOutcome {
    let log: CheckResult
    let event: CheckResult
    let metric: CheckResult
    let metricValue: Double
}

private struct MetaJson: Decodable {
    let uid: String
    let address: String
    let metricsAddr: String

    enum CodingKeys: String, CodingKey {
        case uid
        case address
        case metricsAddr = "metrics_addr"
    }
}

private struct RuntimeError: Error, CustomStringConvertible {
    let description: String
    init(_ description: String) {
        self.description = description
    }
}

private func readLogs(channel: GRPCChannel) throws -> [Holons_V1_LogEntry] {
    let client = Holons_V1_HolonObservabilityClient(channel: channel)
    var request = Holons_V1_LogsRequest()
    request.minLevel = .info
    var entries: [Holons_V1_LogEntry] = []
    let call = client.logs(request, callOptions: timeoutOptions(2)) { entry in
        entries.append(entry)
    }
    let status = try call.status.wait()
    guard status.code == .ok else {
        throw RuntimeError("logs status \(status)")
    }
    return entries
}

private func readEvents(channel: GRPCChannel) throws -> [Holons_V1_EventInfo] {
    let client = Holons_V1_HolonObservabilityClient(channel: channel)
    var entries: [Holons_V1_EventInfo] = []
    let call = client.events(Holons_V1_EventsRequest(), callOptions: timeoutOptions(2)) { entry in
        entries.append(entry)
    }
    let status = try call.status.wait()
    guard status.code == .ok else {
        throw RuntimeError("events status \(status)")
    }
    return entries
}

private func fetchMetrics(_ addr: String) throws -> String {
    guard let url = URL(string: addr) else {
        throw RuntimeError("invalid metrics addr \(addr)")
    }
    let semaphore = DispatchSemaphore(value: 0)
    var result: Result<String, Error>?
    let task = URLSession.shared.dataTask(with: url) { data, _, error in
        if let error {
            result = .failure(error)
        } else {
            result = .success(String(data: data ?? Data(), encoding: .utf8) ?? "")
        }
        semaphore.signal()
    }
    task.resume()
    if semaphore.wait(timeout: .now() + 2) == .timedOut {
        task.cancel()
        throw RuntimeError("metrics request timed out")
    }
    return try result?.get() ?? ""
}

private func parseCascadeTicks(body: String, uid: String) -> Double? {
    let needle = "responder_uid=\"\(uid)\""
    for line in body.split(separator: "\n") {
        guard line.hasPrefix("cascade_ticks_total{"), line.contains(needle) else { continue }
        let parts = line.split(whereSeparator: { $0 == " " || $0 == "\t" })
        if let raw = parts.last, let value = Double(raw) {
            return value
        }
    }
    return nil
}

private func waitFor(timeout: TimeInterval, interval: TimeInterval, _ fn: () -> CheckResult) -> CheckResult {
    let deadline = Date().addingTimeInterval(timeout)
    var last = CheckResult(pass: false, evidence: "")
    while true {
        last = fn()
        if last.pass || Date() > deadline {
            return last
        }
        Thread.sleep(forTimeInterval: interval)
    }
}

private func toCascadeReport(_ report: CascadeReportData) -> ObservabilityCascade_V1_CascadeReport {
    var output = ObservabilityCascade_V1_CascadeReport()
    output.ticks = Int32(report.ticks)
    output.pass = Int32(report.pass)
    output.fail = Int32(report.fail)
    output.phases = report.phases.map { phase in
        var item = ObservabilityCascade_V1_PhaseResult()
        item.name = phase.name
        item.pass = Int32(phase.pass)
        item.fail = Int32(phase.fail)
        item.failures = phase.failures
        return item
    }
    return output
}

private func toMultiPatternReport(_ report: MultiPatternReportData) -> ObservabilityCascade_V1_MultiPatternReport {
    var output = ObservabilityCascade_V1_MultiPatternReport()
    output.patterns = report.patterns.map(toCascadeReport)
    output.totalPass = Int32(report.totalPass)
    output.totalFail = Int32(report.totalFail)
    return output
}

private func normalizedListenURI(_ listenURI: String) -> String {
    if listenURI.hasPrefix("tcp://:") {
        return "tcp://0.0.0.0:\(listenURI.dropFirst("tcp://:".count))"
    }
    return listenURI
}

private func output(_ emit: Bool, _ value: String = "") {
    if emit {
        print(value)
    }
}

private func canonicalCommand(_ raw: String) -> String {
    raw.trimmingCharacters(in: .whitespacesAndNewlines)
        .lowercased()
        .replacingOccurrences(of: "-", with: "")
        .replacingOccurrences(of: "_", with: "")
        .replacingOccurrences(of: " ", with: "")
}

private func timeoutOptions(_ seconds: TimeInterval) -> CallOptions {
    CallOptions(timeLimit: .timeout(.nanoseconds(Int64(seconds * 1_000_000_000))))
}

private func makeRunRoot(prefix: String) throws -> String {
    let url = URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true)
        .appendingPathComponent("\(prefix)\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
    return url.path
}

private func elapsed(_ start: Date) -> String {
    let millis = max(0, Int(Date().timeIntervalSince(start) * 1000))
    if millis < 1000 {
        return "\(millis)ms"
    }
    return String(format: "%.1fs", Double(millis) / 1000.0)
}

private func passText(_ value: Bool) -> String {
    value ? "PASS" : "FAIL"
}

private func printFailureEvidence(_ family: String, _ result: CheckResult) {
    if !result.pass {
        print("    \(family) evidence: \(result.evidence.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? "<empty>" : result.evidence)")
    }
}

private func failureSummary(_ outcome: TickOutcome) -> String {
    var missing: [String] = []
    if !outcome.log.pass { missing.append("log family") }
    if !outcome.event.pass { missing.append("event family") }
    if !outcome.metric.pass { missing.append("metric family") }
    return missing.isEmpty ? "unknown" : missing.joined(separator: ", ")
}

private func compactEvidence(_ outcome: TickOutcome) -> String {
    var parts: [String] = []
    if !outcome.log.pass { parts.append("log=\(outcome.log.evidence)") }
    if !outcome.event.pass { parts.append("event=\(outcome.event.evidence)") }
    if !outcome.metric.pass { parts.append("metric=\(outcome.metric.evidence)") }
    return parts.joined(separator: " | ")
}

private func deleteRecursively(path: String) throws {
    let fm = FileManager.default
    guard fm.fileExists(atPath: path) else { return }
    try fm.removeItem(atPath: path)
}

private func killProcess(_ process: Process) {
    #if os(Linux)
    _ = Glibc.kill(pid_t(process.processIdentifier), SIGKILL)
    #else
    _ = Darwin.kill(pid_t(process.processIdentifier), SIGKILL)
    #endif
}
