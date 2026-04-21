import Foundation
import Dispatch
#if os(Linux)
import Glibc
#else
import Darwin
#endif

public struct CertificationInvocation: Equatable {
    public let command: String
    public let arguments: [String]
    public let currentDirectoryPath: String
    public let environment: [String: String]

    public init(
        command: String,
        arguments: [String],
        currentDirectoryPath: String,
        environment: [String: String]
    ) {
        self.command = command
        self.arguments = arguments
        self.currentDirectoryPath = currentDirectoryPath
        self.environment = environment
    }
}

public enum CertificationCLIError: Error, CustomStringConvertible {
    case launchFailed(String)

    public var description: String {
        switch self {
        case let .launchFailed(message):
            return "process launch failed: \(message)"
        }
    }
}

public enum CertificationCLI {
    public static let sdkName = "swift-holons"
    public static let serverSDKName = "go-holons"
    public static let preferredGoBinary = "/Users/bpds/go/go1.25.1/bin/go"
    public static let defaultGoCache = "/tmp/go-cache"

    public static func packageRoot(from sourceFilePath: String = #filePath) -> URL {
        URL(fileURLWithPath: sourceFilePath)
            .deletingLastPathComponent() // Holons
            .deletingLastPathComponent() // Sources
            .deletingLastPathComponent() // swift-holons
    }

    public static func sdkRoot(packageRoot: URL = packageRoot()) -> URL {
        packageRoot.deletingLastPathComponent()
    }

    public static func goHolonsDirectory(packageRoot: URL = packageRoot()) -> URL {
        sdkRoot(packageRoot: packageRoot).appendingPathComponent("go-holons", isDirectory: true)
    }

    public static func echoClientHelperPath(packageRoot: URL = packageRoot()) -> URL {
        packageRoot
            .appendingPathComponent("cmd", isDirectory: true)
            .appendingPathComponent("echo-client-go", isDirectory: true)
            .appendingPathComponent("main.go")
    }

    public static func holonRPCServerHelperPath(packageRoot: URL = packageRoot()) -> URL {
        packageRoot
            .appendingPathComponent("cmd", isDirectory: true)
            .appendingPathComponent("holon-rpc-server-go", isDirectory: true)
            .appendingPathComponent("main.go")
    }

    public static func echoServerDelayHelperPath(packageRoot: URL = packageRoot()) -> URL {
        packageRoot
            .appendingPathComponent("cmd", isDirectory: true)
            .appendingPathComponent("echo-server-delay", isDirectory: true)
            .appendingPathComponent("main.go")
    }

    public static func resolveGoBinary(
        environment: [String: String] = ProcessInfo.processInfo.environment
    ) -> String {
        if let configured = environment["GO_BIN"]?.trimmingCharacters(in: .whitespacesAndNewlines),
           !configured.isEmpty {
            return configured
        }
        if FileManager.default.isExecutableFile(atPath: preferredGoBinary) {
            return preferredGoBinary
        }
        return "go"
    }

    public static func makeEchoClientInvocation(
        userArgs: [String],
        environment: [String: String] = ProcessInfo.processInfo.environment,
        packageRoot: URL = packageRoot()
    ) -> CertificationInvocation {
        var arguments: [String] = [
            "run",
            echoClientHelperPath(packageRoot: packageRoot).path,
            "--sdk", sdkName,
            "--server-sdk", serverSDKName,
        ]
        arguments.append(contentsOf: userArgs)

        var env = environment
        if env["GOCACHE"]?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true {
            env["GOCACHE"] = defaultGoCache
        }

        return CertificationInvocation(
            command: resolveGoBinary(environment: environment),
            arguments: arguments,
            currentDirectoryPath: goHolonsDirectory(packageRoot: packageRoot).path,
            environment: env
        )
    }

    public static func makeEchoServerInvocation(
        userArgs: [String],
        version: String = Holons.version,
        environment: [String: String] = ProcessInfo.processInfo.environment,
        packageRoot: URL = packageRoot()
    ) -> CertificationInvocation {
        let executable = containsFlag(userArgs, named: "--handler-delay-ms")
            ? echoServerDelayHelperPath(packageRoot: packageRoot).path
            : "./cmd/echo-server"

        var arguments: [String] = ["run", executable]
        arguments.append(contentsOf: userArgs)

        if !containsFlag(userArgs, named: "--sdk") {
            arguments.append(contentsOf: ["--sdk", sdkName])
        }
        if !containsFlag(userArgs, named: "--version") {
            arguments.append(contentsOf: ["--version", version])
        }

        var env = environment
        if env["GOCACHE"]?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true {
            env["GOCACHE"] = defaultGoCache
        }

        return CertificationInvocation(
            command: resolveGoBinary(environment: environment),
            arguments: arguments,
            currentDirectoryPath: goHolonsDirectory(packageRoot: packageRoot).path,
            environment: env
        )
    }

    public static func makeHolonRPCServerInvocation(
        userArgs: [String],
        version: String = Holons.version,
        environment: [String: String] = ProcessInfo.processInfo.environment,
        packageRoot: URL = packageRoot()
    ) -> CertificationInvocation {
        var arguments: [String] = [
            "run",
            holonRPCServerHelperPath(packageRoot: packageRoot).path,
        ]

        if !containsFlag(userArgs, named: "--sdk") {
            arguments.append(contentsOf: ["--sdk", sdkName])
        }
        if !containsFlag(userArgs, named: "--version") {
            arguments.append(contentsOf: ["--version", version])
        }
        arguments.append(contentsOf: userArgs)

        var env = environment
        if env["GOCACHE"]?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true {
            env["GOCACHE"] = defaultGoCache
        }

        return CertificationInvocation(
            command: resolveGoBinary(environment: environment),
            arguments: arguments,
            currentDirectoryPath: goHolonsDirectory(packageRoot: packageRoot).path,
            environment: env
        )
    }

    public static func run(_ invocation: CertificationInvocation) throws -> Int32 {
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
        process.standardInput = FileHandle.standardInput
        process.standardOutput = FileHandle.standardOutput
        process.standardError = FileHandle.standardError

        do {
            try process.run()
        } catch {
            throw CertificationCLIError.launchFailed("\(invocation.command): \(error)")
        }

        signal(SIGTERM, SIG_IGN)
        signal(SIGINT, SIG_IGN)

        let signalQueue = DispatchQueue(label: "holons.certification-cli.signal-forwarding")
        let termSource = makeSignalForwarder(
            signalNumber: SIGTERM,
            process: process,
            queue: signalQueue
        )
        let intSource = makeSignalForwarder(
            signalNumber: SIGINT,
            process: process,
            queue: signalQueue
        )

        defer {
            termSource.cancel()
            intSource.cancel()
            signal(SIGTERM, SIG_DFL)
            signal(SIGINT, SIG_DFL)
        }

        process.waitUntilExit()
        return process.terminationStatus
    }

    public static func containsFlag(_ args: [String], named name: String) -> Bool {
        args.contains { arg in
            arg == name || arg.hasPrefix("\(name)=")
        }
    }

    private static func makeSignalForwarder(
        signalNumber: Int32,
        process: Process,
        queue: DispatchQueue
    ) -> DispatchSourceSignal {
        let source = DispatchSource.makeSignalSource(signal: signalNumber, queue: queue)
        source.setEventHandler {
            forwardSignal(signalNumber, for: process)
        }
        source.resume()
        return source
    }

    private static func forwardSignal(_ signalNumber: Int32, for process: Process) {
        guard process.isRunning else { return }

        let rootPID = process.processIdentifier
        let targetPID = firstChildProcessID(of: rootPID) ?? rootPID
        _ = kill(targetPID, signalNumber)
    }

    private static func firstChildProcessID(of parentPID: pid_t) -> pid_t? {
        let lookup = Process()
        lookup.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        lookup.arguments = ["pgrep", "-P", String(parentPID)]

        let stdout = Pipe()
        lookup.standardOutput = stdout
        lookup.standardError = Pipe()
        lookup.standardInput = nil

        do {
            try lookup.run()
        } catch {
            return nil
        }

        lookup.waitUntilExit()
        guard lookup.terminationStatus == 0 else {
            return nil
        }

        let data = stdout.fileHandleForReading.readDataToEndOfFile()
        guard let output = String(data: data, encoding: .utf8) else {
            return nil
        }

        let pids = output
            .split(whereSeparator: \.isNewline)
            .compactMap { Int32($0.trimmingCharacters(in: .whitespaces)) }
            .filter { $0 > 0 }

        return pids.max()
    }
}
