import Combine
import Foundation
#if os(macOS)
import Holons
#endif

@MainActor
final class HolonProcess: ObservableObject {
    @Published var isRunning = false
    @Published var connectionError: String?
    @Published var availableHolons: [GabrielHolonIdentity] = []
    @Published var selectedHolon: GabrielHolonIdentity? = nil {
        didSet {
            guard oldValue != selectedHolon else { return }
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
    private var embeddedSwiftMemHolon: EmbeddedSwiftMemHolon?
#endif

    init() {
#if os(macOS)
        refreshHolons()
#endif
    }

    func start() async {
        guard client == nil else { return }
        if let startTask {
            do {
                _ = try await startTask.value
            } catch {
                if connectionError == nil {
                    connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
                }
                isRunning = false
            }
            return
        }
        connectionError = nil

#if os(macOS)
        do {
            if availableHolons.isEmpty {
                refreshHolons()
            }
            guard let holon = selectedHolon ?? preferredHolon(in: availableHolons) else {
                throw HolonStartError.binaryNotFound("No Gabriel holons found")
            }

            if selectedHolon != holon {
                selectedHolon = holon
            }

            let root = try stageHolonRoot(holon: holon)
            stageRoot = root
            logHostUI("[HostUI] assembly=\(assemblyFamily) holon=\(holon.binaryName) transport=\(transport)")
            try prepareEmbeddedHolonIfNeeded(for: holon, stageRoot: root)

            var options = ConnectOptions()
            options.transport = transport
            options.timeout = transport == "stdio" ? 1.5 : 2.0

            let taskID = UUID()
            startTaskID = taskID
            let connectTask = Task.detached(priority: .userInitiated) {
                try connectClient(
                    holonSlug: holon.slug,
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
                logHostUI("[HostUI] connected to \(holon.binaryName) on \(connectionTarget())")
                isRunning = true
            } catch {
                guard startTaskID == taskID else {
                    return
                }
                stopEmbeddedHolon()
                cleanupStageRoot()
                connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
                isRunning = false
            }

            if startTaskID == taskID {
                startTask = nil
                startTaskID = nil
            }
        } catch {
            stopEmbeddedHolon()
            cleanupStageRoot()
            connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
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
            connectionError = "Failed to stop Gabriel holon connection: \(error.localizedDescription)"
        }

#if os(macOS)
        stopEmbeddedHolon()
        cleanupStageRoot()
#endif
        isRunning = false
    }

    func listLanguages() async throws -> [Language] {
        if client == nil { await start() }
        guard let client else {
            throw HolonError.notConnected
        }
        return try await client.listLanguages()
    }

    func sayHello(name: String, langCode: String) async throws -> String {
        guard let client else {
            throw HolonError.notConnected
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

    var holonBinaryName: String {
        selectedHolon?.binaryName ?? "gabriel-greeting-swift"
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
    holonSlug: String,
    stageRoot: URL,
    options: ConnectOptions
) throws -> GreetingClient {
    connectClientLock.lock()
    defer { connectClientLock.unlock() }

    let previousDirectory = FileManager.default.currentDirectoryPath
    guard FileManager.default.changeCurrentDirectoryPath(stageRoot.path) else {
        throw HolonStartError.failedToEnterRoot(stageRoot.path)
    }
    defer {
        FileManager.default.changeCurrentDirectoryPath(previousDirectory)
    }

    return try GreetingClient.connected(to: holonSlug, options: options)
}

enum HolonError: LocalizedError {
    case notConnected

    var errorDescription: String? {
        switch self {
        case .notConnected:
            return "Not connected to the Gabriel greeting holon"
        }
    }
}

struct GabrielHolonIdentity: Identifiable, Hashable {
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
private extension GabrielHolonIdentity {
    static let supportedHolons: [GabrielHolonDescriptor] = [
        GabrielHolonDescriptor(
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
        GabrielHolonDescriptor(
            variant: "go",
            familyName: "Greeting-Go",
            displayName: "Gabriel (Go)",
            binaryName: "gabriel-greeting-go",
            buildRunner: "go-module",
            sortRank: 1,
            holonUUID: "3f08b5c3-8931-46d0-847a-a64d8b9ba57e",
            born: "2026-02-20"
        ),
        GabrielHolonDescriptor(
            variant: "rust",
            familyName: "Greeting-Rust",
            displayName: "Gabriel (Rust)",
            binaryName: "gabriel-greeting-rust",
            buildRunner: "cargo",
            sortRank: 2,
            holonUUID: "f6f82422-f0fa-4d7e-a417-67602e749d09",
            born: "2026-03-16",
            extraRelativePaths: [
                "target/debug/gabriel-greeting-rust",
                "target/release/gabriel-greeting-rust",
            ]
        ),
        GabrielHolonDescriptor(
            variant: "python",
            familyName: "Greeting-Python",
            displayName: "Gabriel (Python)",
            binaryName: "gabriel-greeting-python",
            buildRunner: "python",
            sortRank: 3,
            holonUUID: "21c76e23-bdf2-4dbe-85fe-850c5d17118b",
            born: "2026-03-16",
            extraRelativePaths: [
                "build/scripts/gabriel-greeting-python",
            ]
        ),
        GabrielHolonDescriptor(
            variant: "c",
            familyName: "Greeting-C",
            displayName: "Gabriel (C)",
            binaryName: "gabriel-greeting-c",
            buildRunner: "cmake",
            sortRank: 4,
            holonUUID: "a9b515e3-0b75-44f5-ab68-70c4d53e4947",
            born: "2026-03-16",
            extraRelativePaths: [
                "build/gabriel-greeting-c",
            ]
        ),
        GabrielHolonDescriptor(
            variant: "cpp",
            familyName: "Greeting-Cpp",
            displayName: "Gabriel (C++)",
            binaryName: "gabriel-greeting-cpp",
            buildRunner: "cmake",
            sortRank: 5,
            holonUUID: "05af5ed2-79d3-438d-b8b1-6a252d09ab38",
            born: "2026-03-16",
            extraRelativePaths: [
                "build/gabriel-greeting-cpp",
            ]
        ),
        GabrielHolonDescriptor(
            variant: "csharp",
            familyName: "Greeting-Csharp",
            displayName: "Gabriel (C#)",
            binaryName: "gabriel-greeting-csharp",
            buildRunner: "dotnet",
            sortRank: 6,
            holonUUID: "846d57c7-d6e8-4be1-a1f0-89250bd87759",
            born: "2026-03-16"
        ),
        GabrielHolonDescriptor(
            variant: "dart",
            familyName: "Greeting-Dart",
            displayName: "Gabriel (Dart)",
            binaryName: "gabriel-greeting-dart",
            buildRunner: "dart",
            sortRank: 7,
            holonUUID: "0e98310c-c48f-4d3b-aa4c-25e133b4880f",
            born: "2026-03-16"
        ),
        GabrielHolonDescriptor(
            variant: "java",
            familyName: "Greeting-Java",
            displayName: "Gabriel (Java)",
            binaryName: "gabriel-greeting-java",
            buildRunner: "gradle",
            sortRank: 8,
            holonUUID: "43a1d6d9-952b-447c-8e47-4c9f0837d8bd",
            born: "2026-03-16",
            extraRelativePaths: [
                "build/install/gabriel-greeting-java/bin/gabriel-greeting-java",
            ]
        ),
        GabrielHolonDescriptor(
            variant: "kotlin",
            familyName: "Greeting-Kotlin",
            displayName: "Gabriel (Kotlin)",
            binaryName: "gabriel-greeting-kotlin",
            buildRunner: "gradle",
            sortRank: 9,
            holonUUID: "e7c6a320-8c93-4ae6-8c65-03efa8fa759e",
            born: "2026-03-16",
            extraRelativePaths: [
                "build/install/gabriel-greeting-kotlin/bin/gabriel-greeting-kotlin",
            ]
        ),
        GabrielHolonDescriptor(
            variant: "node",
            familyName: "Greeting-Node",
            displayName: "Gabriel (Node.js)",
            binaryName: "gabriel-greeting-node",
            buildRunner: "npm",
            sortRank: 10,
            holonUUID: "1d02f9a4-77e5-4082-bbd7-b6f9c4bb28db",
            born: "2026-03-16"
        ),
        GabrielHolonDescriptor(
            variant: "ruby",
            familyName: "Greeting-Ruby",
            displayName: "Gabriel (Ruby)",
            binaryName: "gabriel-greeting-ruby",
            buildRunner: "ruby",
            sortRank: 11,
            holonUUID: "0d371dd4-2948-4192-8638-cee294fb8320",
            born: "2026-03-16",
            extraRelativePaths: [
                "build/scripts/gabriel-greeting-ruby",
                "cmd/main.rb",
            ]
        ),
    ]

    static func fromDescriptor(_ descriptor: GabrielHolonDescriptor, binaryPath: String) -> GabrielHolonIdentity {
        GabrielHolonIdentity(
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

private struct GabrielHolonDescriptor {
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

private extension HolonProcess {
    func preferredHolon(in holons: [GabrielHolonIdentity]) -> GabrielHolonIdentity? {
        holons.sorted(by: holonSort).first
    }

    func holonSort(_ lhs: GabrielHolonIdentity, _ rhs: GabrielHolonIdentity) -> Bool {
        if lhs.sortRank != rhs.sortRank {
            return lhs.sortRank < rhs.sortRank
        }
        return lhs.displayName.localizedCaseInsensitiveCompare(rhs.displayName) == .orderedAscending
    }

    func refreshHolons() {
        let previousSelection = selectedHolon?.slug
        let bundledRoots = bundledHolonRoots()
        let sourceRoots = sourceTreeHolonRoots()
        var results: [GabrielHolonIdentity] = []

        logHostUI("[HostUI] refreshHolons() scanning \(GabrielHolonIdentity.supportedHolons.count) Gabriel holons")
        for descriptor in GabrielHolonIdentity.supportedHolons {
            if let holon = resolveHolon(
                descriptor: descriptor,
                bundledRoots: bundledRoots,
                sourceRoots: sourceRoots
            ) {
                results.append(holon)
            }
        }

        availableHolons = results.sorted(by: holonSort)
        if let previousSelection,
           let holon = availableHolons.first(where: { $0.slug == previousSelection }) {
            selectedHolon = holon
        } else {
            selectedHolon = preferredHolon(in: availableHolons)
        }
    }

    func resolveHolon(
        descriptor: GabrielHolonDescriptor,
        bundledRoots: [URL],
        sourceRoots: [URL]
    ) -> GabrielHolonIdentity? {
        for candidate in bundledBinaryCandidates(for: descriptor, bundledRoots: bundledRoots) {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logHostUI("[HostUI] resolved \(descriptor.binaryName) -> \(candidate)")
                return GabrielHolonIdentity.fromDescriptor(descriptor, binaryPath: candidate)
            }
        }
        for candidate in sourceTreeBinaryCandidates(for: descriptor, sourceRoots: sourceRoots) {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                logHostUI("[HostUI] resolved \(descriptor.binaryName) -> \(candidate)")
                return GabrielHolonIdentity.fromDescriptor(descriptor, binaryPath: candidate)
            }
        }
        return nil
    }

    func bundledBinaryCandidates(for descriptor: GabrielHolonDescriptor, bundledRoots: [URL]) -> [String] {
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

    func sourceTreeBinaryCandidates(for descriptor: GabrielHolonDescriptor, sourceRoots: [URL]) -> [String] {
        var seen = Set<String>()
        var results: [String] = []
        for root in sourceRoots {
            let holonDir = root.appendingPathComponent(descriptor.binaryName, isDirectory: true)
            for relativePath in descriptor.relativePaths {
                let candidate = holonDir.appendingPathComponent(relativePath).standardizedFileURL.path
                if seen.insert(candidate).inserted {
                    results.append(candidate)
                }
            }
        }
        return results
    }

    func sourceTreeHolonRoots() -> [URL] {
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

    func bundledHolonRoots() -> [URL] {
        var candidates: [URL] = []
        if let executableURL = Bundle.main.executableURL {
            let executableDir = executableURL.deletingLastPathComponent()
            candidates.append(executableDir.appendingPathComponent("holons", isDirectory: true))
            candidates.append(executableDir)
        }
        if let resourceURL = Bundle.main.resourceURL {
            candidates.append(resourceURL.appendingPathComponent("holons", isDirectory: true))
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

    func stageHolonRoot(holon: GabrielHolonIdentity) throws -> URL {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("gabriel-greeting-app-swiftui-\(UUID().uuidString)", isDirectory: true)

        do {
            let holonDir = root
                .appendingPathComponent("holons", isDirectory: true)
                .appendingPathComponent(holon.slug, isDirectory: true)
            try FileManager.default.createDirectory(at: holonDir, withIntermediateDirectories: true)
            try stageGabrielProtos(into: holonDir, holon: holon)
            return root
        } catch {
            try? FileManager.default.removeItem(at: root)
            throw HolonStartError.failedToStageRoot(error.localizedDescription)
        }
    }

    func stageGabrielProtos(into holonDir: URL, holon: GabrielHolonIdentity) throws {
        if let source = holonManifestProtoURL(for: holon) {
            let manifestProtoDir = holonDir
                .appendingPathComponent("api", isDirectory: true)
                .appendingPathComponent("v1", isDirectory: true)
            try FileManager.default.createDirectory(at: manifestProtoDir, withIntermediateDirectories: true)
            let data = try Data(contentsOf: source)
            try data.write(to: manifestProtoDir.appendingPathComponent("holon.proto"), options: .atomic)
        } else {
            logHostUI("[HostUI] no holon manifest proto found for \(holon.slug)")
        }

        guard let greetingSource = sharedGreetingProtoURL() else {
            logHostUI("[HostUI] no shared greeting.proto found for staged holon metadata")
            return
        }

        guard let manifestSource = sharedManifestProtoURL() else {
            logHostUI("[HostUI] no shared holons/v1/manifest.proto found for staged holon metadata")
            return
        }

        let protoRoots = ["protos", "_protos"]
        for protoRoot in protoRoots {
            let greetingTargetDir = holonDir
                .appendingPathComponent(protoRoot, isDirectory: true)
                .appendingPathComponent("v1", isDirectory: true)
            try FileManager.default.createDirectory(at: greetingTargetDir, withIntermediateDirectories: true)
            let greetingData = try Data(contentsOf: greetingSource)
            try greetingData.write(to: greetingTargetDir.appendingPathComponent("greeting.proto"), options: .atomic)

            let manifestTargetDir = holonDir
                .appendingPathComponent(protoRoot, isDirectory: true)
                .appendingPathComponent("holons", isDirectory: true)
                .appendingPathComponent("v1", isDirectory: true)
            try FileManager.default.createDirectory(at: manifestTargetDir, withIntermediateDirectories: true)
            let manifestData = try Data(contentsOf: manifestSource)
            try manifestData.write(to: manifestTargetDir.appendingPathComponent("manifest.proto"), options: .atomic)
        }
    }

    func holonManifestProtoURL(for holon: GabrielHolonIdentity) -> URL? {
        var candidates: [URL] = []

        let binaryURL = URL(fileURLWithPath: holon.binaryPath, isDirectory: false)
        for base in ancestorDirectories(of: binaryURL.deletingLastPathComponent(), maxDepth: 8) {
            candidates.append(base.appendingPathComponent("api/v1/holon.proto", isDirectory: false))
        }

        for root in sourceTreeHolonRoots() {
            candidates.append(
                root
                    .appendingPathComponent(holon.binaryName, isDirectory: true)
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
            base.appendingPathComponent("protos/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("examples/_protos/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("recipes/protos/greeting/v1/greeting.proto", isDirectory: false),
            base.appendingPathComponent("Protos/greeting.proto", isDirectory: false),
            base.appendingPathComponent("greeting.proto", isDirectory: false),
        ]
    }

    func manifestProtoCandidates(from base: URL) -> [URL] {
        [
            base.appendingPathComponent("_protos/holons/v1/manifest.proto", isDirectory: false),
            base.appendingPathComponent("protos/holons/v1/manifest.proto", isDirectory: false),
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

    func prepareEmbeddedHolonIfNeeded(for holon: GabrielHolonIdentity, stageRoot: URL) throws {
        guard transport == "mem" else {
            stopEmbeddedHolon()
            return
        }

        guard holon.variant == "swift" else {
            throw HolonStartError.unsupportedMemoryHolon(
                "memory connection mode currently requires the Swift Gabriel backend"
            )
        }

        let embeddedHolon = embeddedSwiftMemHolon ?? EmbeddedSwiftMemHolon()
        try embeddedHolon.start(
            slug: holon.slug,
            stageRoot: stageRoot,
            logger: logHostUI(_:)
        )
        embeddedSwiftMemHolon = embeddedHolon
    }

    func stopEmbeddedHolon() {
        embeddedSwiftMemHolon?.stop(logger: logHostUI(_:))
        embeddedSwiftMemHolon = nil
    }
}

private enum HolonStartError: LocalizedError {
    case binaryNotFound(String)
    case failedToStageRoot(String)
    case failedToEnterRoot(String)
    case unsupportedMemoryHolon(String)

    var errorDescription: String? {
        switch self {
        case let .binaryNotFound(binaryName):
            return "Holon binary not found: \(binaryName)"
        case let .failedToStageRoot(message):
            return "Failed to stage holon root: \(message)"
        case let .failedToEnterRoot(path):
            return "Failed to enter staged holon root: \(path)"
        case let .unsupportedMemoryHolon(message):
            return message
        }
    }
}
#endif
