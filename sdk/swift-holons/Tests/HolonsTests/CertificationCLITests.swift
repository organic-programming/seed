import Foundation
import XCTest
@testable import Holons

final class CertificationCLITests: XCTestCase {
    private var packageRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent() // HolonsTests
            .deletingLastPathComponent() // Tests
            .deletingLastPathComponent() // swift-holons
    }

    func testResolveGoBinaryUsesEnvironmentOverride() {
        let binary = CertificationCLI.resolveGoBinary(
            environment: ["GO_BIN": " /opt/custom/go "]
        )
        XCTAssertEqual(binary, "/opt/custom/go")
    }

    func testEchoClientInvocationDefaults() {
        let invocation = CertificationCLI.makeEchoClientInvocation(
            userArgs: ["stdio://", "--message", "cert"],
            environment: ["GO_BIN": "go-custom"],
            packageRoot: packageRoot
        )

        let helperPath = packageRoot
            .appendingPathComponent("cmd")
            .appendingPathComponent("echo-client-go")
            .appendingPathComponent("main.go")

        XCTAssertEqual(invocation.command, "go-custom")
        XCTAssertEqual(
            invocation.currentDirectoryPath,
            packageRoot.deletingLastPathComponent().appendingPathComponent("go-holons").path
        )
        XCTAssertEqual(invocation.arguments[0], "run")
        XCTAssertEqual(invocation.arguments[1], helperPath.path)
        XCTAssertEqual(invocation.arguments[2], "--sdk")
        XCTAssertEqual(invocation.arguments[3], "swift-holons")
        XCTAssertEqual(invocation.arguments[4], "--server-sdk")
        XCTAssertEqual(invocation.arguments[5], "go-holons")
        XCTAssertEqual(Array(invocation.arguments.suffix(3)), ["stdio://", "--message", "cert"])
        XCTAssertEqual(invocation.environment["GOCACHE"], "/tmp/go-cache")
    }

    func testEchoClientInvocationPreservesGOCache() {
        let invocation = CertificationCLI.makeEchoClientInvocation(
            userArgs: [],
            environment: ["GO_BIN": "go-custom", "GOCACHE": "/custom/cache"],
            packageRoot: packageRoot
        )

        XCTAssertEqual(invocation.environment["GOCACHE"], "/custom/cache")
    }

    func testEchoServerInvocationAddsDefaults() {
        let invocation = CertificationCLI.makeEchoServerInvocation(
            userArgs: ["--listen", "stdio://"],
            version: "1.2.3",
            environment: ["GO_BIN": "go-custom"],
            packageRoot: packageRoot
        )

        XCTAssertEqual(invocation.command, "go-custom")
        XCTAssertEqual(invocation.arguments[0], "run")
        XCTAssertEqual(invocation.arguments[1], "./cmd/echo-server")
        XCTAssertTrue(invocation.arguments.contains("--sdk"))
        XCTAssertTrue(invocation.arguments.contains("swift-holons"))
        XCTAssertTrue(invocation.arguments.contains("--version"))
        XCTAssertTrue(invocation.arguments.contains("1.2.3"))
    }

    func testEchoServerInvocationRespectsOverrides() {
        let invocation = CertificationCLI.makeEchoServerInvocation(
            userArgs: [
                "--sdk", "custom-sdk",
                "--version=9.9.9",
            ],
            environment: ["GO_BIN": "go-custom"],
            packageRoot: packageRoot
        )

        XCTAssertEqual(invocation.arguments.filter { $0 == "--sdk" }.count, 1)
        XCTAssertEqual(invocation.arguments.filter { $0 == "--version" }.count, 0)
        XCTAssertTrue(invocation.arguments.contains("--version=9.9.9"))
    }

    func testEchoServerInvocationUsesDelayHelperWhenRequested() {
        let invocation = CertificationCLI.makeEchoServerInvocation(
            userArgs: [
                "--listen", "tcp://127.0.0.1:9999",
                "--handler-delay-ms", "5000",
            ],
            environment: ["GO_BIN": "go-custom"],
            packageRoot: packageRoot
        )

        let helperPath = packageRoot
            .appendingPathComponent("cmd")
            .appendingPathComponent("echo-server-delay")
            .appendingPathComponent("main.go")

        XCTAssertEqual(invocation.arguments[0], "run")
        XCTAssertEqual(invocation.arguments[1], helperPath.path)
        XCTAssertTrue(invocation.arguments.contains("--handler-delay-ms"))
        XCTAssertTrue(invocation.arguments.contains("5000"))
        XCTAssertTrue(invocation.arguments.contains("--sdk"))
        XCTAssertTrue(invocation.arguments.contains("swift-holons"))
    }

    func testHolonRPCServerInvocationAddsDefaults() {
        let invocation = CertificationCLI.makeHolonRPCServerInvocation(
            userArgs: ["ws://127.0.0.1:0/rpc"],
            version: "9.9.9",
            environment: ["GO_BIN": "go-custom"],
            packageRoot: packageRoot
        )

        let helperPath = packageRoot
            .appendingPathComponent("cmd")
            .appendingPathComponent("holon-rpc-server-go")
            .appendingPathComponent("main.go")

        XCTAssertEqual(invocation.command, "go-custom")
        XCTAssertEqual(invocation.arguments[0], "run")
        XCTAssertEqual(invocation.arguments[1], helperPath.path)
        XCTAssertTrue(invocation.arguments.contains("ws://127.0.0.1:0/rpc"))
        XCTAssertTrue(invocation.arguments.contains("--sdk"))
        XCTAssertTrue(invocation.arguments.contains("swift-holons"))
        XCTAssertTrue(invocation.arguments.contains("--version"))
        XCTAssertTrue(invocation.arguments.contains("9.9.9"))
        XCTAssertEqual(invocation.environment["GOCACHE"], "/tmp/go-cache")
    }

    func testEchoClientSupportsStdioRoundTrip() throws {
        let invocation = CertificationCLI.makeEchoClientInvocation(
            userArgs: ["--message", "cert-stdio", "stdio://"],
            packageRoot: packageRoot
        )

        let result = try runInvocation(invocation)
        if result.status != 0, isInfrastructureFailure(result.stderr) {
            throw XCTSkip("stdio invocation blocked by environment: \(result.stderr)")
        }

        XCTAssertEqual(result.status, 0, "stderr:\n\(result.stderr)")
        XCTAssertTrue(result.stdout.contains("\"status\":\"pass\""), "stdout:\n\(result.stdout)")
    }

    func testEchoClientSupportsWebSocketDial() throws {
        let invocation = CertificationCLI.makeEchoClientInvocation(
            userArgs: ["--message", "cert-ws", "ws://127.0.0.1:0/grpc"],
            packageRoot: packageRoot
        )
        let result = try runInvocation(invocation)
        if result.status != 0, isInfrastructureFailure(result.stderr) {
            throw XCTSkip("ws invocation blocked by environment: \(result.stderr)")
        }

        XCTAssertEqual(result.status, 0, "stderr:\n\(result.stderr)")
        XCTAssertTrue(result.stdout.contains("\"status\":\"pass\""), "stdout:\n\(result.stdout)")
    }

    func testHolonRPCServerExecutableEcho() async throws {
        let invocation = CertificationCLI.makeHolonRPCServerInvocation(
            userArgs: ["ws://127.0.0.1:0/rpc"],
            packageRoot: packageRoot
        )
        let server = try startInvocationServer(invocation)
        defer { stopRunningProcess(server) }

        let client = HolonRPCClient(
            heartbeatInterval: 10,
            heartbeatTimeout: 2,
            reconnectMinDelay: 0.2,
            reconnectMaxDelay: 0.5
        )

        try await client.connect(server.address.absoluteString)
        let out = try await client.invoke(
            method: "echo.v1.Echo/Ping",
            params: ["message": "hello-cert"]
        )
        XCTAssertEqual(out["message"] as? String, "hello-cert")
        XCTAssertEqual(out["sdk"] as? String, "swift-holons")
        await client.close()
    }

    func testHolonRPCServerFanoutRoutingAggregatesResponses() async throws {
        let invocation = CertificationCLI.makeHolonRPCServerInvocation(
            userArgs: ["ws://127.0.0.1:0/rpc"],
            packageRoot: packageRoot
        )
        let server = try startInvocationServer(invocation)
        defer { stopRunningProcess(server) }

        var responders: [HolonRPCClient] = []
        let caller = HolonRPCClient(
            heartbeatInterval: 10,
            heartbeatTimeout: 2,
            reconnectMinDelay: 0.2,
            reconnectMaxDelay: 0.5
        )

        do {
            for index in 0..<3 {
                let responder = HolonRPCClient(
                    heartbeatInterval: 10,
                    heartbeatTimeout: 2,
                    reconnectMinDelay: 0.2,
                    reconnectMaxDelay: 0.5
                )
                let responderID = "responder-\(index)"
                await responder.register(method: "Echo/Ping") { params in
                    var out = params
                    out["responder"] = responderID
                    return out
                }
                try await responder.connect(server.address.absoluteString)
                responders.append(responder)
            }

            try await caller.connect(server.address.absoluteString)

            var aggregated: [String: Any]?
            var lastError: Error?
            let deadline = Date().addingTimeInterval(4.0)
            while Date() < deadline {
                do {
                    let out = try await caller.invoke(
                        method: "*.Echo/Ping",
                        params: ["message": "fanout-check"]
                    )
                    if let entries = out["value"] as? [Any], entries.count == 3 {
                        aggregated = out
                        break
                    }
                } catch {
                    lastError = error
                }
                try await Task.sleep(nanoseconds: 120_000_000)
            }

            if let lastError, aggregated == nil {
                XCTFail("fanout invocation failed: \(lastError)")
            }

            guard let aggregated else {
                XCTFail("fanout invocation did not return 3 entries before timeout")
                return
            }

            guard let entries = aggregated["value"] as? [Any] else {
                XCTFail("fanout response missing value array: \(aggregated)")
                return
            }
            XCTAssertEqual(entries.count, 3)

            var peers = Set<String>()
            var respondersSeen = Set<String>()

            for raw in entries {
                guard let entry = raw as? [String: Any] else {
                    XCTFail("fanout entry must be an object: \(raw)")
                    return
                }
                guard let peer = entry["peer"] as? String else {
                    XCTFail("fanout entry missing peer: \(entry)")
                    return
                }
                guard let result = entry["result"] as? [String: Any] else {
                    XCTFail("fanout entry missing result payload: \(entry)")
                    return
                }
                XCTAssertEqual(result["message"] as? String, "fanout-check")
                if let responderID = result["responder"] as? String {
                    respondersSeen.insert(responderID)
                }
                peers.insert(peer)
            }

            XCTAssertEqual(peers.count, 3)
            XCTAssertEqual(respondersSeen, Set(["responder-0", "responder-1", "responder-2"]))
        } catch {
            for responder in responders {
                await responder.close()
            }
            await caller.close()
            throw error
        }

        for responder in responders {
            await responder.close()
        }
        await caller.close()
    }
}

private struct InvocationResult {
    let status: Int32
    let stdout: String
    let stderr: String
}

private struct RunningProcess {
    let process: Process
    let stdout: Pipe
    let stderr: Pipe
    let address: URL
}

private enum CertificationTestError: Error, CustomStringConvertible {
    case launch(String)
    case startup(String)
    case timeout(String)

    var description: String {
        switch self {
        case let .launch(message):
            return "launch failed: \(message)"
        case let .startup(message):
            return "startup failed: \(message)"
        case let .timeout(message):
            return "timeout: \(message)"
        }
    }
}

private func runInvocation(
    _ invocation: CertificationInvocation,
    timeout: TimeInterval = 30
) throws -> InvocationResult {
    let stdout = Pipe()
    let stderr = Pipe()
    let process = makeProcess(invocation, stdout: stdout, stderr: stderr)

    do {
        try process.run()
    } catch {
        throw CertificationTestError.launch("\(invocation.command): \(error)")
    }

    let deadline = Date().addingTimeInterval(timeout)
    while process.isRunning, Date() < deadline {
        Thread.sleep(forTimeInterval: 0.05)
    }
    if process.isRunning {
        process.terminate()
        process.waitUntilExit()
        throw CertificationTestError.timeout("process exceeded \(timeout)s")
    }
    process.waitUntilExit()

    let out = String(data: stdout.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
    let err = String(data: stderr.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""

    return InvocationResult(
        status: process.terminationStatus,
        stdout: out.trimmingCharacters(in: .whitespacesAndNewlines),
        stderr: err.trimmingCharacters(in: .whitespacesAndNewlines)
    )
}

private func startInvocationServer(_ invocation: CertificationInvocation) throws -> RunningProcess {
    let stdout = Pipe()
    let stderr = Pipe()
    let process = makeProcess(invocation, stdout: stdout, stderr: stderr)

    do {
        try process.run()
    } catch {
        throw CertificationTestError.launch("\(invocation.command): \(error)")
    }

    let firstLine: String
    do {
        firstLine = try readFirstLine(from: stdout.fileHandleForReading)
    } catch {
        let stderrText = String(data: stderr.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        let details = stderrText.isEmpty ? "\(error)" : "\(error)\n\(stderrText)"
        if isInfrastructureFailure(details) {
            throw XCTSkip(details)
        }
        throw CertificationTestError.startup(details)
    }

    guard let url = URL(string: firstLine) else {
        let stderrText = String(data: stderr.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        throw CertificationTestError.startup("invalid URL emitted: \(firstLine)\n\(stderrText)")
    }

    return RunningProcess(process: process, stdout: stdout, stderr: stderr, address: url)
}

private func stopRunningProcess(_ server: RunningProcess) {
    if server.process.isRunning {
        server.process.terminate()
        server.process.waitUntilExit()
    }
    _ = server.stdout.fileHandleForReading.readDataToEndOfFile()
    _ = server.stderr.fileHandleForReading.readDataToEndOfFile()
}

private func makeProcess(_ invocation: CertificationInvocation, stdout: Pipe, stderr: Pipe) -> Process {
    let process = Process()
    if invocation.command.contains("/") {
        process.executableURL = URL(fileURLWithPath: invocation.command)
        process.arguments = invocation.arguments
    } else {
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        process.arguments = [invocation.command] + invocation.arguments
    }
    process.currentDirectoryURL = URL(fileURLWithPath: invocation.currentDirectoryPath)
    process.environment = invocation.environment
    process.standardOutput = stdout
    process.standardError = stderr
    process.standardInput = nil
    return process
}

private func readFirstLine(from handle: FileHandle) throws -> String {
    var bytes = Data()

    while true {
        guard let chunk = try handle.read(upToCount: 1), !chunk.isEmpty else {
            break
        }
        bytes.append(chunk)
        if chunk.first == 0x0A {
            break
        }
    }

    guard let line = String(data: bytes, encoding: .utf8)?
        .trimmingCharacters(in: .whitespacesAndNewlines), !line.isEmpty else {
        throw CertificationTestError.startup("helper did not output a URL")
    }

    return line
}

private func isInfrastructureFailure(_ details: String) -> Bool {
    let lower = details.lowercased()
    return lower.contains("operation not permitted")
        || lower.contains("permission denied")
        || lower.contains("unable to start")
        || lower.contains("no such file or directory")
        || lower.contains("command not found")
        || lower.contains("executable file not found")
        || lower.contains("proxy.golang.org")
        || lower.contains("address already in use")
}
