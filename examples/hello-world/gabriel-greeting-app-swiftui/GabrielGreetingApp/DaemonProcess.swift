import Combine
import Foundation
#if os(macOS)
import Holons
#endif

@MainActor
final class DaemonProcess: ObservableObject {
    @Published var isRunning = false
    @Published var connectionError: String?
    @Published var availableDaemons: [GabrielDaemonIdentity] = []
    @Published var selectedDaemon: GabrielDaemonIdentity? = nil {
        didSet {
            guard oldValue != selectedDaemon else { return }
            stop()
        }
    }
    @Published var transport: String = {
        let value = ProcessInfo.processInfo.environment["OP_ASSEMBLY_TRANSPORT"]?
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return value?.isEmpty == false ? value! : "stdio"
    }() {
        didSet {
            stop()
        }
    }

    private var client: GreetingClient?
    private var startTask: Task<GreetingClient, Error>?
    private var startTaskID: UUID?
#if os(macOS)
    private var stageRoot: URL?
    private var embeddedSwiftMemDaemon: EmbeddedSwiftMemDaemon?
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
                    connectionError = "Failed to start Gabriel daemon: \(String(describing: error))"
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
                throw DaemonStartError.binaryNotFound("No Gabriel backends found")
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
                connectionError = "Failed to start Gabriel daemon: \(String(describing: error))"
                isRunning = false
            }
            if startTaskID == taskID {
                startTask = nil
                startTaskID = nil
            }
        } catch {
            stopEmbeddedDaemon()
            cleanupStageRoot()
            connectionError = "Failed to start Gabriel daemon: \(String(describing: error))"
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
            connectionError = "Failed to stop Gabriel daemon connection: \(error.localizedDescription)"
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
        let value = ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]?
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return value?.isEmpty == false ? value! : "Gabriel-Greeting-App-SwiftUI"
    }

    var daemonBinaryName: String {
        selectedDaemon?.binaryName ?? "gabriel-greeting-swift"
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
            return "Not connected to the Gabriel greeting daemon"
        }
    }
}

struct GabrielDaemonIdentity: Identifiable, Hashable {
    let variant: String
    let slug: String
    let familyName: String
    let binaryName: String
    let buildRunner: String
    let binaryPath: String
    let displayName: String
    let sortRank: Int
    let holonUUID: String
    let born: String

    var id: String { slug }
}

#if os(macOS)
private extension GabrielDaemonIdentity {
    static let supportedDaemons: [GabrielDaemonDescriptor] = [
        GabrielDaemonDescriptor(
            variant: "swift",
            familyName: "Greeting-Swift",
            displayName: "Gabriel (Swift)",
            binaryName: "gabriel-greeting-swift",
            buildRunner: "swift-package",
            sortRank: 0,
            holonUUID: "9d14585c-a155-4e72-b6fe-3cae3535948b",
            born: "2026-03-15",
            extraRelativePaths: [
                ".build/arm64-apple-macosx/debug/gabriel-greeting-swift",
                ".build/debug/gabriel-greeting-swift",
                ".op/build/swift/arm64-apple-macosx/debug/gabriel-greeting-swift",
            ]
        ),
        GabrielDaemonDescriptor(
            variant: "go",
            familyName: "Greeting-Go",
            displayName: "Gabriel (Go)",
            binaryName: "gabriel-greeting-go",
            buildRunner: "go-module",
            sortRank: 1,
            holonUUID: "3f08b5c3-8931-46d0-847a-a64d8b9ba57e",
            born: "2026-02-20"
        ),
    ]

    static func fromDescriptor(_ descriptor: GabrielDaemonDescriptor, binaryPath: String) -> GabrielDaemonIdentity {
        GabrielDaemonIdentity(
            variant: descriptor.variant,
            slug: "gabriel-greeting-\(descriptor.variant)",
            familyName: descriptor.familyName,
            binaryName: descriptor.binaryName,
            buildRunner: descriptor.buildRunner,
            binaryPath: URL(fileURLWithPath: binaryPath).standardizedFileURL.path,
            displayName: descriptor.displayName,
            sortRank: descriptor.sortRank,
            holonUUID: descriptor.holonUUID,
            born: descriptor.born
        )
    }
}

private struct GabrielDaemonDescriptor {
    let variant: String
    let familyName: String
    let displayName: String
    let binaryName: String
    let buildRunner: String
    let sortRank: Int
    let holonUUID: String
    let born: String
    var extraRelativePaths: [String] = []

    var relativePaths: [String] {
        [
            ".op/build/\(binaryName).holon/bin/darwin_arm64/\(binaryName)",
            ".op/build/bin/\(binaryName)",
            "build/install/\(binaryName)/bin/\(binaryName)",
            "build/scripts/\(binaryName)",
            binaryName,
        ] + extraRelativePaths
    }
}

private extension DaemonProcess {
    func preferredDaemon(in daemons: [GabrielDaemonIdentity]) -> GabrielDaemonIdentity? {
        daemons.sorted(by: daemonSort).first
    }

    func daemonSort(_ lhs: GabrielDaemonIdentity, _ rhs: GabrielDaemonIdentity) -> Bool {
        if lhs.sortRank != rhs.sortRank {
            return lhs.sortRank < rhs.sortRank
        }
        return lhs.displayName.localizedCaseInsensitiveCompare(rhs.displayName) == .orderedAscending
    }

    func refreshDaemons() {
        let previousSelection = selectedDaemon?.slug
        let bundledRoots = bundledDaemonRoots()
        let sourceRoots = sourceTreeDaemonRoots()
        var results: [GabrielDaemonIdentity] = []

        logHostUI("[HostUI] refreshDaemons() scanning \(GabrielDaemonIdentity.supportedDaemons.count) Gabriel backends")
        for descriptor in GabrielDaemonIdentity.supportedDaemons {
            if let daemon = resolveDaemon(
                descriptor: descriptor,
                bundledRoots: bundledRoots,
                sourceRoots: sourceRoots
            ) {
                results.append(daemon)
            }
        }

        availableDaemons = results.sorted(by: daemonSort)
        if let previousSelection,
           let daemon = availableDaemons.first(where: { $0.slug == previousSelection }) {
            selectedDaemon = daemon
        } else {
            selectedDaemon = preferredDaemon(in: availableDaemons)
        }
    }

    func resolveDaemon(
        descriptor: GabrielDaemonDescriptor,
        bundledRoots: [URL],
        sourceRoots: [URL]
    ) -> GabrielDaemonIdentity? {
        for candidate in bundledBinaryCandidates(for: descriptor, bundledRoots: bundledRoots) {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logHostUI("[HostUI] resolved \(descriptor.binaryName) -> \(candidate)")
                return GabrielDaemonIdentity.fromDescriptor(descriptor, binaryPath: candidate)
            }
        }
        for candidate in sourceTreeBinaryCandidates(for: descriptor, sourceRoots: sourceRoots) {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logHostUI("[HostUI] resolved \(descriptor.binaryName) -> \(candidate)")
                return GabrielDaemonIdentity.fromDescriptor(descriptor, binaryPath: candidate)
            }
        }
        return nil
    }

    func bundledBinaryCandidates(for descriptor: GabrielDaemonDescriptor, bundledRoots: [URL]) -> [String] {
        bundledRoots.flatMap { root in
            [
                root.appendingPathComponent(descriptor.binaryName).standardizedFileURL.path,
                root
                    .appendingPathComponent("\(descriptor.binaryName).holon", isDirectory: true)
                    .appendingPathComponent("bin/darwin_arm64/\(descriptor.binaryName)")
                    .standardizedFileURL.path,
            ]
        }
    }

    func sourceTreeBinaryCandidates(for descriptor: GabrielDaemonDescriptor, sourceRoots: [URL]) -> [String] {
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
            currentDirectory.deletingLastPathComponent(),
            currentDirectory.appendingPathComponent("examples/hello-world", isDirectory: true),
        ]

        for base in ancestorDirectories(of: currentDirectory, maxDepth: 8) {
            candidates.append(base.appendingPathComponent("examples/hello-world", isDirectory: true))
        }

        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            for base in ancestorDirectories(of: executableDir, maxDepth: 10) {
                candidates.append(base.appendingPathComponent("examples/hello-world", isDirectory: true))
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
            candidates.append(resourceURL.appendingPathComponent("Holons", isDirectory: true))
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

    func stageHolonRoot(daemon: GabrielDaemonIdentity) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("gabriel-greeting-app-swiftui-\(UUID().uuidString)", isDirectory: true)

        do {
            let holonDir = root
                .appendingPathComponent("holons", isDirectory: true)
                .appendingPathComponent(daemon.slug, isDirectory: true)
            try FileManager.default.createDirectory(at: holonDir, withIntermediateDirectories: true)
            try manifest(for: daemon)
                .write(to: holonDir.appendingPathComponent("holon.yaml"), atomically: true, encoding: .utf8)
            try stageGabrielProtos(into: holonDir, daemon: daemon)
            return root
        } catch {
            try? FileManager.default.removeItem(at: root)
            throw DaemonStartError.failedToStageRoot(error.localizedDescription)
        }
    }

    func manifest(for daemon: GabrielDaemonIdentity) -> String {
        """
        schema: holon/v1
        uuid: "\(daemon.holonUUID)"
        given_name: "Gabriel"
        family_name: "\(daemon.familyName)"
        motto: "Greets users in 56 languages."
        composer: "Codex"
        clade: deterministic/pure
        status: draft
        born: "\(daemon.born)"
        lang: "\(daemon.variant)"
        reproduction: manual
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

    func stageGabrielProtos(into holonDir: URL, daemon: GabrielDaemonIdentity) throws {
        if let source = daemonHolonProtoURL(for: daemon) {
            let daemonProtoDir = holonDir
                .appendingPathComponent("api", isDirectory: true)
                .appendingPathComponent("v1", isDirectory: true)
            try FileManager.default.createDirectory(at: daemonProtoDir, withIntermediateDirectories: true)
            let data = try Data(contentsOf: source)
            try data.write(to: daemonProtoDir.appendingPathComponent("holon.proto"), options: .atomic)
        } else {
            logHostUI("[HostUI] no daemon holon.proto found for \(daemon.slug)")
        }

        guard let greetingSource = sharedGreetingProtoURL() else {
            logHostUI("[HostUI] no shared greeting.proto found for staged holon metadata")
            return
        }

        let sharedProtoDir = holonDir.appendingPathComponent("_protos", isDirectory: true)
        let sharedGreetingDir = sharedProtoDir
            .appendingPathComponent("v1", isDirectory: true)
        try FileManager.default.createDirectory(at: sharedGreetingDir, withIntermediateDirectories: true)
        let greetingData = try Data(contentsOf: greetingSource)
        try greetingData.write(to: sharedGreetingDir.appendingPathComponent("greeting.proto"), options: .atomic)

        guard let manifestSource = sharedManifestProtoURL() else {
            logHostUI("[HostUI] no shared holons/v1/manifest.proto found for staged holon metadata")
            return
        }

        let sharedManifestDir = sharedProtoDir
            .appendingPathComponent("holons", isDirectory: true)
            .appendingPathComponent("v1", isDirectory: true)
        try FileManager.default.createDirectory(at: sharedManifestDir, withIntermediateDirectories: true)
        let manifestData = try Data(contentsOf: manifestSource)
        try manifestData.write(to: sharedManifestDir.appendingPathComponent("manifest.proto"), options: .atomic)
    }

    func daemonHolonProtoURL(for daemon: GabrielDaemonIdentity) -> URL? {
        var candidates: [URL] = []

        let binaryURL = URL(fileURLWithPath: daemon.binaryPath, isDirectory: false)
        for base in ancestorDirectories(of: binaryURL.deletingLastPathComponent(), maxDepth: 8) {
            candidates.append(base.appendingPathComponent("api/v1/holon.proto", isDirectory: false))
        }

        for root in sourceTreeDaemonRoots() {
            candidates.append(
                root
                    .appendingPathComponent(daemon.binaryName, isDirectory: true)
                    .appendingPathComponent("api/v1/holon.proto", isDirectory: false)
            )
        }

        return firstExistingURL(in: candidates)
    }

    func sharedGreetingProtoURL() -> URL? {
        var candidates: [URL] = []

        for base in sharedProtoSearchBases() {
            candidates.append(contentsOf: greetingProtoCandidates(from: base))
        }

        return firstExistingURL(in: candidates)
    }

    func sharedManifestProtoURL() -> URL? {
        var candidates: [URL] = []
        for base in sharedProtoSearchBases() {
            candidates.append(contentsOf: manifestProtoCandidates(from: base))
        }
        return firstExistingURL(in: candidates)
    }

    func sharedProtoSearchBases() -> [URL] {
        let currentDirectory = URL(fileURLWithPath: FileManager.default.currentDirectoryPath, isDirectory: true)
        var candidates = ancestorDirectories(of: currentDirectory, maxDepth: 8)

        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            candidates.append(contentsOf: ancestorDirectories(of: executableDir, maxDepth: 10))
        }

        return candidates
    }

    func greetingProtoCandidates(from base: URL) -> [URL] {
        [
            base.appendingPathComponent("_protos/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("examples/_protos/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("recipes/protos/greeting/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("Protos/greeting.proto", isDirectory: false),
            base.appendingPathComponent("greeting.proto", isDirectory: false),
        ]
    }

    func manifestProtoCandidates(from base: URL) -> [URL] {
        [
            base.appendingPathComponent("_protos/holons/v1/manifest.proto", isDirectory: false),
            base.appendingPathComponent("holons/grace-op/_protos/holons/v1/manifest.proto", isDirectory: false),
        ]
    }

    func firstExistingURL(in candidates: [URL]) -> URL? {
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

    func cleanupStageRoot() {
        guard let root = stageRoot else { return }
        stageRoot = nil
        try? FileManager.default.removeItem(at: root)
    }

    func prepareEmbeddedDaemonIfNeeded(for daemon: GabrielDaemonIdentity, stageRoot: URL) throws {
        guard transport == "mem" else {
            stopEmbeddedDaemon()
            return
        }

        guard daemon.variant == "swift" else {
            throw DaemonStartError.unsupportedMemoryDaemon(
                "memory connection mode currently requires the Swift Gabriel backend"
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
