import Foundation
#if os(macOS)
import GreetingDaemonSwiftSupport
import Holons
#endif

/// Manages the lifecycle of the bundled greeting daemon connection.
@MainActor
final class DaemonProcess: ObservableObject {
    @Published var isRunning = false
    @Published var connectionError: String?

    private var client: GreetingClient?
    private var startTask: Task<GreetingClient, Error>?
    private var startTaskID: UUID?
#if os(macOS)
    private var stageRoot: URL?
    private var embeddedSwiftMemDaemon: EmbeddedSwiftMemDaemon?
    
    @Published var availableDaemons: [GreetingDaemonIdentity] = []
    @Published var selectedDaemon: GreetingDaemonIdentity? = nil {
        didSet {
            guard oldValue != selectedDaemon else { return }
            stop()
        }
    }
#endif

    init() {
#if os(macOS)
        refreshDaemons()
#endif
    }

    func start() async {
        guard client == nil else { return }
        if let startTask {
            do {
                _ = try await startTask.value
            } catch {
                if connectionError == nil {
                    connectionError = "Failed to start bundled daemon: \(String(describing: error))"
                }
                isRunning = false
            }
            return
        }
        connectionError = nil

#if os(macOS)
        do {
            if availableDaemons.isEmpty {
                refreshDaemons()
            }
            guard let daemon = selectedDaemon ?? preferredDaemon(in: availableDaemons) else {
                throw DaemonStartError.binaryNotFound("No daemons found")
            }
            
            if selectedDaemon != daemon {
                selectedDaemon = daemon
            }

            let root = try stageHolonRoot(daemon: daemon)
            stageRoot = root
            logHostUI("[HostUI] assembly=\(assemblyFamily) daemon=\(daemon.binaryName) transport=\(transport)")
            try prepareEmbeddedDaemonIfNeeded(for: daemon, stageRoot: root)

            var options = ConnectOptions()
            options.transport = transport
            options.timeout = transport == "stdio" ? 1.5 : 2.0

            let taskID = UUID()
            startTaskID = taskID
            let connectTask = Task.detached(priority: .userInitiated) {
                try connectClient(
                    daemonSlug: daemon.slug,
                    stageRoot: root,
                    options: options
                )
            }
            startTask = connectTask

            do {
                let connectedClient = try await connectTask.value
                guard startTaskID == taskID else {
                    try? connectedClient.close()
                    if stageRoot?.path == root.path {
                        cleanupStageRoot()
                    }
                    return
                }
                client = connectedClient
                logHostUI("[HostUI] connected to \(daemon.binaryName) on \(connectionTarget())")
                isRunning = true
            } catch {
                guard startTaskID == taskID else {
                    return
                }
                stopEmbeddedDaemon()
                cleanupStageRoot()
                connectionError = "Failed to start bundled daemon: \(String(describing: error))"
                isRunning = false
            }
            if startTaskID == taskID {
                startTask = nil
                startTaskID = nil
            }
        } catch {
            stopEmbeddedDaemon()
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
        startTaskID = nil
        startTask?.cancel()
        startTask = nil

        let currentClient = client
        client = nil

        do {
            try currentClient?.close()
        } catch {
            connectionError = "Failed to stop daemon connection: \(error.localizedDescription)"
        }

#if os(macOS)
        stopEmbeddedDaemon()
        cleanupStageRoot()
#endif
        isRunning = false
    }

    func listLanguages() async throws -> [Language] {
        if client == nil { await start() }
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
        return selectedDaemon?.binaryName ?? "gudule-daemon-greeting-swift"
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

private let connectClientLock = NSLock()

private func connectClient(
    daemonSlug: String,
    stageRoot: URL,
    options: ConnectOptions
) throws -> GreetingClient {
    connectClientLock.lock()
    defer { connectClientLock.unlock() }

    let previousDirectory = FileManager.default.currentDirectoryPath
    guard FileManager.default.changeCurrentDirectoryPath(stageRoot.path) else {
        throw DaemonStartError.failedToEnterRoot(stageRoot.path)
    }
    defer {
        FileManager.default.changeCurrentDirectoryPath(previousDirectory)
    }

    return try GreetingClient.connected(to: daemonSlug, options: options)
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
struct GreetingDaemonIdentity: Identifiable, Hashable {
    static let binaryPrefix = "gudule-daemon-greeting-"

    let variant: String
    let slug: String
    let familyName: String
    let binaryName: String
    let buildRunner: String
    let binaryPath: String
    let displayName: String
    let sortRank: Int
    
    var id: String { slug }

    fileprivate static let supportedDaemons: [GreetingDaemonDescriptor] = [
        GreetingDaemonDescriptor(
            variant: "go",
            familySuffix: "Go",
            displayName: "Daemon in Go lang",
            buildRunner: "go-module",
            sortRank: 0
        ),
        GreetingDaemonDescriptor(
            variant: "cpp",
            familySuffix: "Cpp",
            displayName: "Daemon in C++",
            buildRunner: "cmake",
            sortRank: 1
        ),
        GreetingDaemonDescriptor(
            variant: "swift",
            familySuffix: "Swift",
            displayName: "Daemon in Swift",
            buildRunner: "swift-package",
            sortRank: 2,
            extraRelativePaths: [
                ".build/arm64-apple-macosx/debug/gudule-daemon-greeting-swift",
                ".op/build/swift/arm64-apple-macosx/debug/gudule-daemon-greeting-swift",
            ]
        ),
        GreetingDaemonDescriptor(
            variant: "rust",
            familySuffix: "Rust",
            displayName: "Daemon in Rust",
            buildRunner: "cargo",
            sortRank: 3,
            extraRelativePaths: [
                ".op/build/cargo/debug/gudule-daemon-greeting-rust",
            ]
        ),
        GreetingDaemonDescriptor(
            variant: "c",
            familySuffix: "C",
            displayName: "Daemon in C",
            buildRunner: "cmake",
            sortRank: 4
        ),
        GreetingDaemonDescriptor(
            variant: "csharp",
            familySuffix: "Csharp",
            displayName: "Daemon in C#",
            buildRunner: "dotnet",
            sortRank: 5
        ),
        GreetingDaemonDescriptor(
            variant: "dart",
            familySuffix: "Dart",
            displayName: "Daemon in Dart",
            buildRunner: "dart",
            sortRank: 6
        ),
        GreetingDaemonDescriptor(
            variant: "java",
            familySuffix: "Java",
            displayName: "Daemon in Java",
            buildRunner: "gradle",
            sortRank: 7
        ),
        GreetingDaemonDescriptor(
            variant: "kotlin",
            familySuffix: "Kotlin",
            displayName: "Daemon in Kotlin",
            buildRunner: "gradle",
            sortRank: 8
        ),
        GreetingDaemonDescriptor(
            variant: "node",
            familySuffix: "Node",
            displayName: "Daemon in Node.js",
            buildRunner: "npm",
            sortRank: 9
        ),
        GreetingDaemonDescriptor(
            variant: "python",
            familySuffix: "Python",
            displayName: "Daemon in Python",
            buildRunner: "python",
            sortRank: 10
        ),
        GreetingDaemonDescriptor(
            variant: "ruby",
            familySuffix: "Ruby",
            displayName: "Daemon in Ruby",
            buildRunner: "recipe",
            sortRank: 11
        ),
    ]

    fileprivate static func fromDescriptor(_ descriptor: GreetingDaemonDescriptor, binaryPath: String) -> GreetingDaemonIdentity {
        GreetingDaemonIdentity(
            variant: descriptor.variant,
            slug: "gudule-greeting-daemon-\(descriptor.variant)",
            familyName: "Greeting-Daemon-\(descriptor.familySuffix)",
            binaryName: descriptor.binaryName,
            buildRunner: descriptor.buildRunner,
            binaryPath: URL(fileURLWithPath: binaryPath).standardizedFileURL.path,
            displayName: descriptor.displayName,
            sortRank: descriptor.sortRank
        )
    }
}

private struct GreetingDaemonDescriptor {
    let variant: String
    let familySuffix: String
    let displayName: String
    let buildRunner: String
    let sortRank: Int
    var extraRelativePaths: [String] = []

    var binaryName: String {
        GreetingDaemonIdentity.binaryPrefix + variant
    }

    var relativePaths: [String] {
        [
            ".op/build/bin/\(binaryName)",
            "build/install/\(binaryName)/bin/\(binaryName)",
            "build/scripts/\(binaryName)",
            binaryName,
        ] + extraRelativePaths
    }
}

private extension DaemonProcess {
    static let holonUUID = "61aa59e8-e4fc-425f-a799-48ff7a6d02d2"

    func preferredDaemon(in daemons: [GreetingDaemonIdentity]) -> GreetingDaemonIdentity? {
        daemons.sorted(by: daemonSort).first
    }

    func daemonSort(_ lhs: GreetingDaemonIdentity, _ rhs: GreetingDaemonIdentity) -> Bool {
        if lhs.sortRank != rhs.sortRank {
            return lhs.sortRank < rhs.sortRank
        }
        return lhs.displayName.localizedCaseInsensitiveCompare(rhs.displayName) == .orderedAscending
    }

    func refreshDaemons() {
        let previousSelection = selectedDaemon?.slug
        let bundledRoots = bundledDaemonRoots()
        let sourceRoots = sourceTreeDaemonRoots()
        var results: [GreetingDaemonIdentity] = []

        logHostUI("[HostUI] refreshDaemons() scanning \(GreetingDaemonIdentity.supportedDaemons.count) known daemons")
        for descriptor in GreetingDaemonIdentity.supportedDaemons {
            if let daemon = resolveDaemon(
                descriptor: descriptor,
                bundledRoots: bundledRoots,
                sourceRoots: sourceRoots
            ) {
                results.append(daemon)
            }
        }

        self.availableDaemons = results.sorted(by: daemonSort)
        if let previousSelection,
           let daemon = availableDaemons.first(where: { $0.slug == previousSelection }) {
            self.selectedDaemon = daemon
        } else {
            self.selectedDaemon = preferredDaemon(in: availableDaemons)
        }
    }

    func resolveDaemon(
        descriptor: GreetingDaemonDescriptor,
        bundledRoots: [URL],
        sourceRoots: [URL]
    ) -> GreetingDaemonIdentity? {
        for candidate in bundledBinaryCandidates(for: descriptor, bundledRoots: bundledRoots) {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logHostUI("[HostUI] resolved \(descriptor.binaryName) -> \(candidate)")
                return GreetingDaemonIdentity.fromDescriptor(descriptor, binaryPath: candidate)
            }
        }
        for candidate in sourceTreeBinaryCandidates(for: descriptor, sourceRoots: sourceRoots) {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logHostUI("[HostUI] resolved \(descriptor.binaryName) -> \(candidate)")
                return GreetingDaemonIdentity.fromDescriptor(descriptor, binaryPath: candidate)
            }
        }
        return nil
    }

    func bundledBinaryCandidates(for descriptor: GreetingDaemonDescriptor, bundledRoots: [URL]) -> [String] {
        bundledRoots.map {
            $0.appendingPathComponent(descriptor.binaryName).standardizedFileURL.path
        }
    }

    func sourceTreeBinaryCandidates(for descriptor: GreetingDaemonDescriptor, sourceRoots: [URL]) -> [String] {
        var seen = Set<String>()
        var results: [String] = []
        for root in sourceRoots {
            let daemonDir = root.appendingPathComponent(descriptor.binaryName, isDirectory: true)
            for relativePath in descriptor.relativePaths {
                let candidate = daemonDir.appendingPathComponent(relativePath).standardizedFileURL.path
                if seen.insert(candidate).inserted {
                    results.append(candidate)
                }
            }
        }
        return results
    }

    func sourceTreeDaemonRoots() -> [URL] {
        let fileManager = FileManager.default
        let currentDirectory = URL(fileURLWithPath: fileManager.currentDirectoryPath, isDirectory: true)
        var candidates: [URL] = [
            currentDirectory.appendingPathComponent("../../daemons", isDirectory: true),
            currentDirectory.appendingPathComponent("recipes/daemons", isDirectory: true),
            currentDirectory.appendingPathComponent("daemons", isDirectory: true),
        ]

        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            for base in ancestorDirectories(of: executableDir, maxDepth: 8) {
                candidates.append(base.appendingPathComponent("recipes/daemons", isDirectory: true))
                candidates.append(base.appendingPathComponent("daemons", isDirectory: true))
            }
        }

        return existingDirectories(in: candidates)
    }

    func bundledDaemonRoots() -> [URL] {
        var candidates: [URL] = []
        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            candidates.append(executableDir.appendingPathComponent("daemon", isDirectory: true))
            candidates.append(executableDir)
        }
        if let resourceURL = Bundle.main.resourceURL {
            candidates.append(resourceURL.appendingPathComponent("daemon", isDirectory: true))
            candidates.append(resourceURL)
        }
        return existingDirectories(in: candidates)
    }

    func ancestorDirectories(of url: URL, maxDepth: Int) -> [URL] {
        var current = url.standardizedFileURL
        var results: [URL] = []
        for _ in 0..<maxDepth {
            results.append(current)
            let parent = current.deletingLastPathComponent()
            if parent == current {
                break
            }
            current = parent
        }
        return results
    }

    func existingDirectories(in candidates: [URL]) -> [URL] {
        let fileManager = FileManager.default
        var seen = Set<String>()
        var results: [URL] = []
        for candidate in candidates {
            let normalized = candidate.standardizedFileURL
            let path = normalized.path
            guard seen.insert(path).inserted else { continue }
            var isDirectory: ObjCBool = false
            guard fileManager.fileExists(atPath: path, isDirectory: &isDirectory), isDirectory.boolValue else {
                continue
            }
            results.append(normalized)
        }
        return results
    }

    func stageHolonRoot(daemon: GreetingDaemonIdentity) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("gudule-greeting-hostui-universal-swiftui-\(UUID().uuidString)", isDirectory: true)

        do {
            let holonDir = root
                .appendingPathComponent("holons", isDirectory: true)
                .appendingPathComponent(daemon.slug, isDirectory: true)
            try FileManager.default.createDirectory(at: holonDir, withIntermediateDirectories: true)
            try manifest(for: daemon)
                .write(to: holonDir.appendingPathComponent("holon.yaml"), atomically: true, encoding: .utf8)
            try stageGreetingProto(into: holonDir)
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

    func stageGreetingProto(into holonDir: URL) throws {
        guard let source = sharedGreetingProtoURL() else {
            logHostUI("[HostUI] no shared greeting.proto found for staged holon metadata")
            return
        }

        let protoDir = holonDir
            .appendingPathComponent("protos", isDirectory: true)
            .appendingPathComponent("greeting", isDirectory: true)
            .appendingPathComponent("v1", isDirectory: true)
        let destination = protoDir.appendingPathComponent("greeting.proto")

        try FileManager.default.createDirectory(at: protoDir, withIntermediateDirectories: true)
        let data = try Data(contentsOf: source)
        try data.write(to: destination, options: .atomic)
    }

    func sharedGreetingProtoURL() -> URL? {
        var candidates: [URL] = []

        let currentDirectory = URL(
            fileURLWithPath: FileManager.default.currentDirectoryPath,
            isDirectory: true
        )
        candidates.append(contentsOf: greetingProtoCandidates(from: currentDirectory))

        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            for base in ancestorDirectories(of: executableDir, maxDepth: 8) {
                candidates.append(contentsOf: greetingProtoCandidates(from: base))
            }
        }

        var seen = Set<String>()
        for candidate in candidates {
            let normalized = candidate.standardizedFileURL
            guard seen.insert(normalized.path).inserted else { continue }
            if FileManager.default.fileExists(atPath: normalized.path) {
                return normalized
            }
        }

        return nil
    }

    func greetingProtoCandidates(from base: URL) -> [URL] {
        [
            base.appendingPathComponent("recipes/protos/greeting/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("Protos/greeting.proto", isDirectory: false),
            base.appendingPathComponent("greeting.proto", isDirectory: false),
        ]
    }

    func cleanupStageRoot() {
        guard let root = stageRoot else { return }
        stageRoot = nil
        try? FileManager.default.removeItem(at: root)
    }

    func prepareEmbeddedDaemonIfNeeded(for daemon: GreetingDaemonIdentity, stageRoot: URL) throws {
        guard transport == "mem" else {
            stopEmbeddedDaemon()
            return
        }

        guard daemon.variant == "swift" else {
            throw DaemonStartError.unsupportedMemoryDaemon(
                "memory connection mode currently requires the Swift daemon in the SwiftUI HostUI"
            )
        }

        let embeddedDaemon = embeddedSwiftMemDaemon ?? EmbeddedSwiftMemDaemon()
        try embeddedDaemon.start(
            slug: daemon.slug,
            stageRoot: stageRoot,
            logger: logHostUI(_:)
        )
        embeddedSwiftMemDaemon = embeddedDaemon
    }

    func stopEmbeddedDaemon() {
        embeddedSwiftMemDaemon?.stop(logger: logHostUI(_:))
        embeddedSwiftMemDaemon = nil
    }
}

private enum DaemonStartError: LocalizedError {
    case binaryNotFound(String)
    case failedToStageRoot(String)
    case failedToEnterRoot(String)
    case unsupportedMemoryDaemon(String)

    var errorDescription: String? {
        switch self {
        case let .binaryNotFound(binaryName):
            return "Daemon binary not found: \(binaryName)"
        case let .failedToStageRoot(message):
            return "Failed to stage holon root: \(message)"
        case let .failedToEnterRoot(path):
            return "Failed to enter staged holon root: \(path)"
        case let .unsupportedMemoryDaemon(message):
            return message
        }
    }
}
#endif
