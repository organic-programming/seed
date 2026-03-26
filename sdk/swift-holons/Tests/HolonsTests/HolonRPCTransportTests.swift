import Foundation
import XCTest

@testable import Holons

final class HolonRPCTransportTests: XCTestCase {
    func testHolonRPCWSSSecureEchoRoundTrip() async throws {
        try await withGoHolonRPCTransportServer(mode: "wss") { url in
            let client = HolonRPCClient(
                heartbeatInterval: 0.25,
                heartbeatTimeout: 0.25,
                reconnectMinDelay: 0.1,
                reconnectMaxDelay: 0.4
            )

            try await client.connect(url.absoluteString)
            let out = try await client.invoke(
                method: "echo.v1.Echo/Ping",
                params: ["message": "hello-wss"]
            )
            XCTAssertEqual(out["message"] as? String, "hello-wss")
            XCTAssertEqual(out["transport"] as? String, "wss")
            await client.close()
        }
    }

    func testHolonRPCHTTPInvokeAcceptsRestSSEAlias() async throws {
        try await withGoHolonRPCTransportServer(mode: "http") { url in
            let restURL = url.absoluteString.replacingOccurrences(
                of: "http://",
                with: "rest+sse://",
                options: [.anchored]
            )
            let client = try HolonRPCHTTPClient(restURL)
            defer { client.close() }

            let out = try await client.invoke(
                method: "echo.v1.Echo/Ping",
                params: ["message": "hello-http"]
            )
            XCTAssertEqual(out["message"] as? String, "hello-http")
            XCTAssertEqual(out["transport"] as? String, "http")
        }
    }

    func testHolonRPCHTTPStreamPOST() async throws {
        try await withGoHolonRPCTransportServer(mode: "http") { url in
            let client = try HolonRPCHTTPClient(url.absoluteString)
            defer { client.close() }

            let events = try await client.stream(
                method: "echo.v1.Echo/Watch",
                params: ["project": "myapp"]
            )

            XCTAssertEqual(events.count, 3)
            XCTAssertEqual(events[0].event, "message")
            XCTAssertEqual(events[0].id, "1")
            XCTAssertEqual(events[0].result["project"] as? String, "myapp")
            XCTAssertEqual(events[0].result["step"] as? String, "1")
            XCTAssertEqual(events[1].result["step"] as? String, "2")
            XCTAssertEqual(events[2].event, "done")
        }
    }

    func testHolonRPCHTTPStreamGET() async throws {
        try await withGoHolonRPCTransportServer(mode: "http") { url in
            let client = try HolonRPCHTTPClient(url.absoluteString)
            defer { client.close() }

            let events = try await client.streamQuery(
                method: "echo.v1.Echo/Watch",
                params: ["project": "myapp"]
            )

            XCTAssertEqual(events.count, 3)
            XCTAssertEqual(events[0].result["project"] as? String, "myapp")
            XCTAssertEqual(events[0].result["step"] as? String, "1")
            XCTAssertEqual(events[1].result["step"] as? String, "2")
            XCTAssertEqual(events[2].event, "done")
        }
    }

    func testHolonRPCHTTPSecureInvokeUsesCustomCAQuery() async throws {
        try await withGoHolonRPCTransportServer(mode: "https") { url in
            let client = try HolonRPCHTTPClient(url.absoluteString)
            defer { client.close() }

            let out = try await client.invoke(
                method: "echo.v1.Echo/Ping",
                params: ["message": "hello-https"]
            )
            XCTAssertEqual(out["message"] as? String, "hello-https")
            XCTAssertEqual(out["transport"] as? String, "https")
        }
    }
}

private struct GoHolonRPCTransportServer {
    let process: Process
    let helperPath: URL
    let temporaryRoot: URL
    let url: URL

    func stop() {
        if process.isRunning {
            process.terminate()
            process.waitUntilExit()
        }
        try? FileManager.default.removeItem(at: helperPath)
        try? FileManager.default.removeItem(at: temporaryRoot)
    }
}

private func withGoHolonRPCTransportServer(
    mode: String,
    _ body: (URL) async throws -> Void
) async throws {
    do {
        let server = try startGoHolonRPCTransportServer(mode: mode)
        defer { server.stop() }
        try await body(server.url)
    } catch let error as GoHolonRPCTransportHelperError {
        throw XCTSkip(error.description)
    }
}

private func startGoHolonRPCTransportServer(mode: String) throws -> GoHolonRPCTransportServer {
    let thisFile = URL(fileURLWithPath: #filePath)
    let swiftRepo = thisFile
        .deletingLastPathComponent() // HolonsTests
        .deletingLastPathComponent() // Tests
        .deletingLastPathComponent() // swift-holons
    let sdkDir = swiftRepo.deletingLastPathComponent()
    let goRepo = sdkDir.appendingPathComponent("go-holons", isDirectory: true)
    let fixtureSource = swiftRepo
        .appendingPathComponent("Tests", isDirectory: true)
        .appendingPathComponent("HolonsTests", isDirectory: true)
        .appendingPathComponent("Fixtures", isDirectory: true)
        .appendingPathComponent("go-holonrpc-transport", isDirectory: true)
        .appendingPathComponent("main.go")

    let temporaryRoot = FileManager.default.temporaryDirectory
        .appendingPathComponent("swift-holons-holonrpc-\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: temporaryRoot, withIntermediateDirectories: true)

    let helperPath = goRepo.appendingPathComponent("tmp-holonrpc-transport-\(UUID().uuidString).go")
    try Data(contentsOf: fixtureSource).write(to: helperPath)

    let stdout = Pipe()
    let stderr = Pipe()

    let process = Process()
    process.currentDirectoryURL = goRepo
    process.executableURL = URL(fileURLWithPath: resolveGoBinary())

    var arguments = ["run", helperPath.path, mode]
    if mode == "wss" || mode == "https" {
        let certFile = temporaryRoot.appendingPathComponent("server-cert.pem")
        let keyFile = temporaryRoot.appendingPathComponent("server-key.pem")
        arguments.append(contentsOf: [certFile.path, keyFile.path])
    }

    process.arguments = arguments
    process.standardOutput = stdout
    process.standardError = stderr

    do {
        try process.run()
    } catch {
        try? FileManager.default.removeItem(at: helperPath)
        try? FileManager.default.removeItem(at: temporaryRoot)
        throw GoHolonRPCTransportHelperError.unavailable("unable to start go helper: \(error)")
    }

    let firstLine: String
    do {
        firstLine = try readFirstTransportLine(from: stdout.fileHandleForReading)
    } catch {
        let stderrText = drainTransportStderr(process: process, stderrHandle: stderr.fileHandleForReading)
        let details = buildTransportHelperFailureDetails(
            base: String(describing: error),
            stderrText: stderrText
        )
        try? FileManager.default.removeItem(at: helperPath)
        try? FileManager.default.removeItem(at: temporaryRoot)
        if isTransportInfrastructureFailure(details) {
            throw GoHolonRPCTransportHelperError.unavailable(details)
        }
        throw NSError(
            domain: "HolonRPCTransportTests",
            code: 1,
            userInfo: [NSLocalizedDescriptionKey: details]
        )
    }

    guard let url = URL(string: firstLine) else {
        let stderrText = drainTransportStderr(process: process, stderrHandle: stderr.fileHandleForReading)
        throw NSError(
            domain: "HolonRPCTransportTests",
            code: 2,
            userInfo: [NSLocalizedDescriptionKey: "invalid helper URL: \(firstLine)\n\(stderrText)"]
        )
    }

    return GoHolonRPCTransportServer(
        process: process,
        helperPath: helperPath,
        temporaryRoot: temporaryRoot,
        url: url
    )
}

private enum GoHolonRPCTransportHelperError: Error, CustomStringConvertible {
    case unavailable(String)

    var description: String {
        switch self {
        case let .unavailable(message):
            return "Go Holon-RPC transport helper unavailable in this environment: \(message)"
        }
    }
}

private func resolveGoBinary() -> String {
    let preferred = "/Users/bpds/go/go1.25.1/bin/go"
    if FileManager.default.isExecutableFile(atPath: preferred) {
        return preferred
    }
    return "go"
}

private func readFirstTransportLine(from handle: FileHandle) throws -> String {
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
        .trimmingCharacters(in: .whitespacesAndNewlines),
          !line.isEmpty else {
        throw NSError(
            domain: "HolonRPCTransportTests",
            code: 3,
            userInfo: [NSLocalizedDescriptionKey: "helper did not output URL"]
        )
    }

    return line
}

private func isTransportInfrastructureFailure(_ details: String) -> Bool {
    let lower = details.lowercased()
    return lower.contains("operation not permitted")
        || lower.contains("permission denied")
        || lower.contains("unable to start go helper")
        || lower.contains("no such file or directory")
        || lower.contains("no such host")
        || lower.contains("proxy.golang.org")
        || lower.contains("executable file not found")
        || lower.contains("command not found")
}

private func drainTransportStderr(process: Process, stderrHandle: FileHandle) -> String {
    if process.isRunning {
        process.terminate()
    }
    process.waitUntilExit()

    let data = stderrHandle.readDataToEndOfFile()
    return String(data: data, encoding: .utf8)?
        .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
}

private func buildTransportHelperFailureDetails(base: String, stderrText: String) -> String {
    if stderrText.isEmpty {
        return base
    }
    return "\(base)\n\(stderrText)"
}
