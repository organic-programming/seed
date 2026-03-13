import Foundation
#if os(macOS)
import Holons
#endif

/// Manages the lifecycle of the bundled greeting daemon connection.
@MainActor
final class DaemonProcess: ObservableObject {
    @Published var isRunning = false
    @Published var connectionError: String?

    private var client: GreetingClient?
#if os(macOS)
    private var stageRoot: URL?
    private var daemonIdentity: GreetingDaemonIdentity?
#endif

    func start() {
        guard client == nil else { return }
        connectionError = nil

        if transport == "mem" {
            let family = assemblyFamily.lowercased()
            let parts = family.split(separator: "-")
            if parts.count >= 3 {
                let ui = parts[1]
                let daemon = String(parts[2].prefix(while: { $0.isLetter }))
                let uiLang = ui.hasPrefix("swift") ? "swift" : String(ui)
                if uiLang != daemon {
                    connectionError = "memory connection mode requires the same language for UI and the daemon"
                    isRunning = false
                    return
                }
            }
        }

#if os(macOS)
        do {
            let daemon = try resolveDaemon()
            daemonIdentity = daemon
            let root = try stageHolonRoot(daemon: daemon)
            stageRoot = root

            let previousDirectory = FileManager.default.currentDirectoryPath
            guard FileManager.default.changeCurrentDirectoryPath(root.path) else {
                throw DaemonStartError.failedToEnterRoot(root.path)
            }
            defer {
                FileManager.default.changeCurrentDirectoryPath(previousDirectory)
            }

            logHostUI("[HostUI] assembly=\(assemblyFamily) daemon=\(daemon.binaryName) transport=\(transport)")
            var options = ConnectOptions()
            options.transport = transport
            
            client = try GreetingClient.connected(to: daemon.slug, options: options)
            logHostUI("[HostUI] connected to \(daemon.binaryName) on \(connectionTarget())")
            isRunning = true
        } catch {
            daemonIdentity = nil
            cleanupStageRoot()
            connectionError = "Failed to start bundled daemon: \(String(describing: error))"
            isRunning = false
        }
#else
        connectionError = GreetingClientError.unsupportedPlatform.localizedDescription
        isRunning = false
#endif
    }

    func stop() {
        let currentClient = client
        client = nil
#if os(macOS)
        daemonIdentity = nil
#endif

        do {
            try currentClient?.close()
        } catch {
            connectionError = "Failed to stop daemon connection: \(error.localizedDescription)"
        }

#if os(macOS)
        cleanupStageRoot()
#endif
        isRunning = false
    }

    func listLanguages() async throws -> [Language] {
        if client == nil { start() }
        guard let client else {
            throw DaemonError.notConnected
        }
        return try await client.listLanguages()
    }

    func sayHello(name: String, langCode: String) async throws -> String {
        guard let client else {
            throw DaemonError.notConnected
        }
        return try await client.sayHello(name: name, langCode: langCode)
    }

    deinit {
        try? client?.close()
#if os(macOS)
        let root = stageRoot
        stageRoot = nil
        daemonIdentity = nil
        if let root {
            try? FileManager.default.removeItem(at: root)
        }
#endif
    }

    var assemblyFamily: String {
        let value = ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]?.trimmingCharacters(in: .whitespacesAndNewlines)
        return (value?.isEmpty == false ? value! : "Greeting-Swiftui-Go")
    }

    @Published var transport: String = {
        let value = ProcessInfo.processInfo.environment["OP_ASSEMBLY_TRANSPORT"]?.trimmingCharacters(in: .whitespacesAndNewlines)
        return (value?.isEmpty == false ? value! : "stdio")
    }() {
        didSet {
            // When transport changes, cleanly stop so the UI can trigger a full reload
            stop()
        }
    }

    var daemonBinaryName: String {
#if os(macOS)
        return daemonIdentity?.binaryName ?? (try? resolveDaemon().binaryName) ?? "gudule-daemon-greeting-swift"
#else
        return "gudule-daemon-greeting-swift"
#endif
    }

    private func connectionTarget() -> String {
        transport == "stdio" ? "stdio" : transport
    }

    private func logHostUI(_ line: String) {
        guard let data = (line + "\n").data(using: .utf8) else {
            return
        }
        FileHandle.standardError.write(data)
    }
}

enum DaemonError: LocalizedError {
    case notConnected

    var errorDescription: String? {
        switch self {
        case .notConnected:
            return "Not connected to the greeting daemon"
        }
    }
}

#if os(macOS)
private struct GreetingDaemonIdentity {
    static let binaryPrefix = "gudule-daemon-greeting-"

    let slug: String
    let familyName: String
    let binaryName: String
    let buildRunner: String
    let binaryPath: String

    static func fromBinaryPath(_ path: String) -> GreetingDaemonIdentity? {
        fromBinaryName(URL(fileURLWithPath: path).lastPathComponent, binaryPath: path)
    }

    static func fromBinaryName(_ binaryName: String, binaryPath: String) -> GreetingDaemonIdentity? {
        let normalized = binaryName.hasSuffix(".exe")
            ? String(binaryName.dropLast(4))
            : binaryName
        guard normalized.hasPrefix(binaryPrefix) else {
            return nil
        }

        let variant = String(normalized.dropFirst(binaryPrefix.count))
        return GreetingDaemonIdentity(
            slug: "gudule-greeting-daemon-\(variant)",
            familyName: "Greeting-Daemon-\(displayVariant(variant))",
            binaryName: normalized,
            buildRunner: runner(for: variant),
            binaryPath: URL(fileURLWithPath: binaryPath).standardizedFileURL.path
        )
    }

    private static func displayVariant(_ variant: String) -> String {
        let overrides: [String: String] = [
            "cpp": "CPP",
            "js": "JS",
            "qt": "Qt",
        ]

        return variant
            .split(separator: "-")
            .map { token in
                let value = String(token)
                if let override = overrides[value] {
                    return override
                }
                guard let first = value.first else {
                    return value
                }
                return String(first).uppercased() + value.dropFirst()
            }
            .joined(separator: "-")
    }

    private static func runner(for variant: String) -> String {
        switch variant {
        case "go":
            return "go-module"
        case "rust":
            return "cargo"
        case "swift":
            return "swift-package"
        case "kotlin":
            return "gradle"
        case "dart":
            return "dart"
        case "python":
            return "python"
        case "csharp":
            return "dotnet"
        case "node":
            return "npm"
        default:
            return "go-module"
        }
    }
}

private extension DaemonProcess {
    static let holonUUID = "61aa59e8-e4fc-425f-a799-48ff7a6d02d2"

    func resolveDaemon() throws -> GreetingDaemonIdentity {
        for candidate in daemonCandidates() {
            if let daemon = GreetingDaemonIdentity.fromBinaryPath(candidate),
               FileManager.default.isExecutableFile(atPath: daemon.binaryPath) {
                return daemon
            }
        }
        throw DaemonStartError.binaryNotFound(GreetingDaemonIdentity.binaryPrefix + "*")
    }

    func daemonCandidates() -> [String] {
        let fileManager = FileManager.default
        let currentDirectory = URL(
            fileURLWithPath: fileManager.currentDirectoryPath,
            isDirectory: true
        )

        var directories: [URL] = [
            currentDirectory.appendingPathComponent("build", isDirectory: true),
            currentDirectory.appendingPathComponent("../build", isDirectory: true),
            currentDirectory.appendingPathComponent("../../daemons", isDirectory: true),
        ]

        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            directories.append(executableDir)
            directories.append(executableDir.appendingPathComponent("daemon", isDirectory: true))
            directories.append(executableDir.appendingPathComponent("../Resources", isDirectory: true))
            directories.append(executableDir.appendingPathComponent("../Resources/daemon", isDirectory: true))
        }

        if let resourceURL = Bundle.main.resourceURL {
            directories.append(resourceURL)
            directories.append(resourceURL.appendingPathComponent("daemon", isDirectory: true))
        }

        var seen = Set<String>()
        var candidates: [String] = []
        for directory in directories {
            let normalizedDir = directory.standardizedFileURL.path
            guard seen.insert(normalizedDir).inserted else {
                continue
            }

            var isDirectory: ObjCBool = false
            guard fileManager.fileExists(atPath: normalizedDir, isDirectory: &isDirectory),
                  isDirectory.boolValue else {
                continue
            }

            if directory.lastPathComponent == "daemons" {
                appendSourceTreeDaemonCandidates(from: directory, into: &candidates)
                continue
            }

            appendBundledBinaries(from: directory, into: &candidates)
        }

        return dedupeSortedPaths(candidates)
    }

    func appendBundledBinaries(from directory: URL, into results: inout [String]) {
        guard let entries = try? FileManager.default.contentsOfDirectory(
            at: directory,
            includingPropertiesForKeys: [.isRegularFileKey],
            options: [.skipsHiddenFiles]
        ) else {
            return
        }

        for entry in entries where entry.lastPathComponent.hasPrefix(GreetingDaemonIdentity.binaryPrefix) {
            results.append(entry.standardizedFileURL.path)
        }
    }

    func appendSourceTreeDaemonCandidates(from daemonsDir: URL, into results: inout [String]) {
        guard let entries = try? FileManager.default.contentsOfDirectory(
            at: daemonsDir,
            includingPropertiesForKeys: [.isDirectoryKey],
            options: [.skipsHiddenFiles]
        ) else {
            return
        }

        for entry in entries where entry.lastPathComponent.hasPrefix(GreetingDaemonIdentity.binaryPrefix) {
            results.append(
                entry
                    .appendingPathComponent(".op/build/bin", isDirectory: true)
                    .appendingPathComponent(entry.lastPathComponent)
                    .standardizedFileURL
                    .path
            )
            results.append(entry.appendingPathComponent(entry.lastPathComponent).standardizedFileURL.path)
        }
    }

    func dedupeSortedPaths(_ paths: [String]) -> [String] {
        var seen = Set<String>()
        return paths
            .map { URL(fileURLWithPath: $0).standardizedFileURL.path }
            .filter { seen.insert($0).inserted }
            .sorted()
    }

    func stageHolonRoot(daemon: GreetingDaemonIdentity) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("gudule-greeting-hostui-swiftui-\(UUID().uuidString)", isDirectory: true)

        do {
            let holonDir = root
                .appendingPathComponent("holons", isDirectory: true)
                .appendingPathComponent(daemon.slug, isDirectory: true)
            try FileManager.default.createDirectory(at: holonDir, withIntermediateDirectories: true)
            try manifest(for: daemon)
                .write(to: holonDir.appendingPathComponent("holon.yaml"), atomically: true, encoding: .utf8)
            return root
        } catch {
            try? FileManager.default.removeItem(at: root)
            throw DaemonStartError.failedToStageRoot(error.localizedDescription)
        }
    }

    func manifest(for daemon: GreetingDaemonIdentity) -> String {
        """
        schema: holon/v0
        uuid: "\(Self.holonUUID)"
        given_name: "gudule"
        family_name: "\(daemon.familyName)"
        motto: "Greets users in 56 languages."
        composer: "Codex"
        clade: deterministic/pure
        status: draft
        born: "2026-03-12"
        generated_by: manual
        kind: native
        build:
          runner: \(daemon.buildRunner)
        artifacts:
          binary: "\(yamlEscape(daemon.binaryPath))"
        """ + "\n"
    }

    func yamlEscape(_ value: String) -> String {
        value.replacingOccurrences(of: "\\", with: "\\\\")
            .replacingOccurrences(of: "\"", with: "\\\"")
    }

    func cleanupStageRoot() {
        guard let root = stageRoot else { return }
        stageRoot = nil
        try? FileManager.default.removeItem(at: root)
    }
}

private enum DaemonStartError: LocalizedError {
    case binaryNotFound(String)
    case failedToStageRoot(String)
    case failedToEnterRoot(String)

    var errorDescription: String? {
        switch self {
        case let .binaryNotFound(binaryName):
            return "Daemon binary not found: \(binaryName)"
        case let .failedToStageRoot(message):
            return "Failed to stage holon root: \(message)"
        case let .failedToEnterRoot(path):
            return "Failed to enter staged holon root: \(path)"
        }
    }
}
#endif
