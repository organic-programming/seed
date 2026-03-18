import Combine
import Foundation
#if os(macOS)
import Holons
#endif

@MainActor
public final class HolonProcess: ObservableObject {
    @Published public var isRunning = false
    @Published public var connectionError: String?
    @Published public var availableHolons: [GabrielHolonIdentity] = []
    @Published public var selectedHolon: GabrielHolonIdentity? = nil {
        didSet {
            guard oldValue != selectedHolon else { return }
            stop()
        }
    }
    @Published public var transport: String = {
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
#endif

    public init() {
#if os(macOS)
        refreshHolons()
#endif
    }

    public func start() async {
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
                throw HolonStartError.holonNotFound
            }

            if selectedHolon != holon {
                selectedHolon = holon
            }

            logHostUI("[HostUI] assembly=\(assemblyFamily) holon=\(holon.binaryName) transport=\(transport)")

            var options = ConnectOptions()
            options.transport = transport
            options.lifecycle = "ephemeral"
            options.timeout = 5.0

            let taskID = UUID()
            startTaskID = taskID
            let connectTask = Task.detached(priority: .userInitiated) { [slug = holon.slug, options] in
                try connectClient(holonSlug: slug, options: options)
            }
            startTask = connectTask

            do {
                let connectedClient = try await connectTask.value
                guard startTaskID == taskID else {
                    try? connectedClient.close()
                    return
                }
                client = connectedClient
                logHostUI("[HostUI] connected to \(holon.binaryName) on \(connectionTarget())")
                isRunning = true
            } catch {
                guard startTaskID == taskID else {
                    return
                }
                connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
                isRunning = false
            }

            if startTaskID == taskID {
                startTask = nil
                startTaskID = nil
            }
        } catch {
            connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
            isRunning = false
        }
#else
        connectionError = GreetingClientError.unsupportedPlatform.localizedDescription
        isRunning = false
#endif
    }

    public func stop() {
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
#endif
        isRunning = false
    }

    public func listLanguages() async throws -> [Greeting_V1_Language] {
        if client == nil { await start() }
        guard let client else {
            throw HolonError.notConnected
        }
        return try await client.listLanguages()
    }

    public func sayHello(name: String, langCode: String) async throws -> String {
        guard let client else {
            throw HolonError.notConnected
        }
        return try await client.sayHello(name: name, langCode: langCode)
    }

    deinit {
        try? client?.close()
    }

    public var assemblyFamily: String {
        let value = ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]?
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return value?.isEmpty == false ? value! : "Gabriel-Greeting-App-SwiftUI"
    }

    public var holonBinaryName: String {
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
    options: ConnectOptions
) throws -> GreetingClient {
    connectClientLock.lock()
    defer { connectClientLock.unlock() }
    return try GreetingClient.connected(to: holonSlug, options: options)
}

public enum HolonError: LocalizedError {
    case notConnected

    public var errorDescription: String? {
        switch self {
        case .notConnected:
            return "Not connected to the Gabriel greeting holon"
        }
    }
}

public struct GabrielHolonIdentity: Identifiable, Hashable {
    public let slug: String
    public let familyName: String
    public let binaryName: String
    public let buildRunner: String
    public let displayName: String
    public let sortRank: Int
    public let holonUUID: String
    public let born: String
    public let sourceKind: String
    public let discoveryPath: String
    public let hasSource: Bool

    public var id: String { slug }
    public var variant: String { slug.replacingOccurrences(of: "gabriel-greeting-", with: "") }
}

#if os(macOS)
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

        do {
            var seen = Set<String>()
            let results = try discoverAll()
                .filter(isGabrielHolon(_:))
                .compactMap { entry in
                    let identity = GabrielHolonIdentity(entry: entry)
                    return seen.insert(identity.slug).inserted ? identity : nil
                }
                .sorted(by: holonSort)

            availableHolons = results
            if let previousSelection,
               let holon = availableHolons.first(where: { $0.slug == previousSelection }) {
                selectedHolon = holon
            } else {
                selectedHolon = preferredHolon(in: availableHolons)
            }
        } catch {
            availableHolons = []
            selectedHolon = nil
            connectionError = "Failed to discover Gabriel holons: \(error.localizedDescription)"
        }
    }

    func isGabrielHolon(_ entry: HolonEntry) -> Bool {
        entry.slug.hasPrefix("gabriel-greeting-") && entry.slug != "gabriel-greeting-app-swiftui"
    }


}

private enum HolonStartError: LocalizedError {
    case holonNotFound

    var errorDescription: String? {
        switch self {
        case .holonNotFound:
            return "No Gabriel holons found"
        }
    }
}

private extension GabrielHolonIdentity {
    init(entry: HolonEntry) {
        let runner = entry.runner.isEmpty ? (entry.manifest?.build.runner ?? "") : entry.runner
        let binaryName = {
            let candidate = entry.entrypoint.isEmpty ? (entry.manifest?.artifacts.binary ?? entry.slug) : entry.entrypoint
            return (candidate as NSString).lastPathComponent
        }()

        self.init(
            slug: entry.slug,
            familyName: entry.identity.familyName,
            binaryName: binaryName,
            buildRunner: runner,
            displayName: Self.displayName(for: entry.slug),
            sortRank: Self.sortRank(for: entry.slug),
            holonUUID: entry.uuid,
            born: entry.identity.born,
            sourceKind: entry.sourceKind,
            discoveryPath: entry.dir.path,
            hasSource: entry.hasSource
        )
    }

    static func displayName(for slug: String) -> String {
        switch slug.replacingOccurrences(of: "gabriel-greeting-", with: "") {
        case "cpp":
            return "Gabriel (C++)"
        case "csharp":
            return "Gabriel (C#)"
        case "node":
            return "Gabriel (Node.js)"
        default:
            let variant = slug
                .replacingOccurrences(of: "gabriel-greeting-", with: "")
                .split(separator: "-")
                .map { $0.capitalized }
                .joined(separator: " ")
            return "Gabriel (\(variant))"
        }
    }

    static func sortRank(for slug: String) -> Int {
        let order = [
            "gabriel-greeting-swift": 0,
            "gabriel-greeting-go": 1,
            "gabriel-greeting-rust": 2,
            "gabriel-greeting-python": 3,
            "gabriel-greeting-c": 4,
            "gabriel-greeting-cpp": 5,
            "gabriel-greeting-csharp": 6,
            "gabriel-greeting-dart": 7,
            "gabriel-greeting-java": 8,
            "gabriel-greeting-kotlin": 9,
            "gabriel-greeting-node": 10,
            "gabriel-greeting-ruby": 11,
        ]
        return order[slug] ?? 999
    }
}
#endif
