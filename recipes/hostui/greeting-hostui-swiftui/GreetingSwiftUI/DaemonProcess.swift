import Foundation

/// Manages the lifecycle of the packaged greeting daemon connection.
@MainActor
final class DaemonProcess: ObservableObject {
    @Published var isRunning = false
    @Published var connectionError: String?

    private var client: GreetingClient?
#if os(macOS)
    private var stageRoot: URL?
#endif

    func start() {
        guard client == nil else { return }
        connectionError = nil

        if let target = Self.trimmedDaemonTargetOverride() {
            do {
                client = try GreetingClient.connected(to: target)
                isRunning = true
            } catch {
                connectionError = "Failed to connect to \(target): \(String(describing: error))"
                isRunning = false
            }
            return
        }

#if os(macOS)
        do {
            let daemonPath = try resolveDaemonPath()
            let root = try stageHolonRoot(binaryPath: daemonPath)
            stageRoot = root

            let previousDirectory = FileManager.default.currentDirectoryPath
            guard FileManager.default.changeCurrentDirectoryPath(root.path) else {
                throw DaemonStartError.failedToEnterRoot(root.path)
            }
            defer {
                FileManager.default.changeCurrentDirectoryPath(previousDirectory)
            }

            client = try GreetingClient.connected(to: Self.packagedHolonSlug)
            isRunning = true
        } catch {
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
        if let root {
            try? FileManager.default.removeItem(at: root)
        }
#endif
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
private extension DaemonProcess {
    static let packagedHolonSlug = "greeting-daemon"
    static let packagedHolonUUID = "6492d55a-55b8-4ecb-a406-2a2a401f7c01"
    static let packagedFamilyName = "daemon"
    static let packagedBinaryName = "gudule-greeting-daemon"
    static let devBinaryName = "gudule-daemon-greeting-go"
    static let buildRunner = "recipe"

    static func trimmedDaemonTargetOverride() -> String? {
        let value = ProcessInfo.processInfo.environment["GUDULE_DAEMON_TARGET"]?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return value.isEmpty ? nil : value
    }

    func resolveDaemonPath() throws -> String {
        for candidate in daemonCandidates() where FileManager.default.isExecutableFile(atPath: candidate) {
            return candidate
        }
        throw DaemonStartError.binaryNotFound(Self.packagedBinaryName)
    }

    func daemonCandidates() -> [String] {
        var candidates: [String] = []
        let currentDirectory = URL(
            fileURLWithPath: FileManager.default.currentDirectoryPath,
            isDirectory: true
        )

        candidates.append(
            currentDirectory
                .appendingPathComponent(Self.packagedBinaryName)
                .path
        )
        candidates.append(
            currentDirectory
                .appendingPathComponent("../../daemons/greeting-daemon-go")
                .appendingPathComponent(".op/build/bin")
                .appendingPathComponent(Self.devBinaryName)
                .path
        )
        candidates.append(
            currentDirectory
                .appendingPathComponent("../../daemons/greeting-daemon-go")
                .appendingPathComponent(Self.devBinaryName)
                .path
        )

        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            candidates.append(executableDir.appendingPathComponent(Self.packagedBinaryName).path)
            candidates.append(
                executableDir
                    .appendingPathComponent("../Resources")
                    .appendingPathComponent(Self.packagedBinaryName)
                    .path
            )
        }

        if let resourceURL = Bundle.main.resourceURL {
            candidates.append(resourceURL.appendingPathComponent(Self.packagedBinaryName).path)
        }

        var seen = Set<String>()
        return candidates.compactMap { path in
            let normalized = URL(fileURLWithPath: path).standardizedFileURL.path
            return seen.insert(normalized).inserted ? normalized : nil
        }
    }

    func stageHolonRoot(binaryPath: String) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("greeting-daemon-stage-\(UUID().uuidString)", isDirectory: true)
        let holonDir = root
            .appendingPathComponent("holons", isDirectory: true)
            .appendingPathComponent(Self.packagedHolonSlug, isDirectory: true)

        do {
            try FileManager.default.createDirectory(at: holonDir, withIntermediateDirectories: true)
            try manifest(for: binaryPath)
                .write(to: holonDir.appendingPathComponent("holon.yaml"), atomically: true, encoding: .utf8)
            return root
        } catch {
            try? FileManager.default.removeItem(at: root)
            throw DaemonStartError.failedToStageRoot(error.localizedDescription)
        }
    }

    func manifest(for binaryPath: String) -> String {
        """
        schema: holon/v0
        uuid: "\(Self.packagedHolonUUID)"
        given_name: "greeting"
        family_name: "\(Self.packagedFamilyName)"
        motto: "Packaged greeting daemon fallback."
        composer: "Codex"
        clade: deterministic/pure
        status: draft
        born: "2026-03-11"
        generated_by: codex
        kind: native
        build:
          runner: \(Self.buildRunner)
        artifacts:
          binary: "\(yamlEscape(binaryPath))"
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
