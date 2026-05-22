#if os(macOS)
import Combine
import Foundation
import Holons

extension ConnectOptions: @unchecked @retroactive Sendable {}

@MainActor
open class HolonProcessManager<Holon: Identifiable & Hashable, Client: Sendable>: ObservableObject {
    public typealias ClientFactory = @Sendable (String, ConnectOptions) throws -> Client
    public typealias ClientClose = @Sendable (Client) throws -> Void

    @Published public var isRunning = false
    @Published public var connectionError: String?
    @Published public var availableHolons: [Holon] = []
    @Published public var selectedHolon: Holon? {
        didSet {
            guard oldValue != selectedHolon else { return }
            stop()
        }
    }
    @Published public var transport: String {
        didSet {
            guard oldValue != transport else { return }
            stop()
        }
    }

    public private(set) var observability: Observability?
    public private(set) var processLogger: HolonLogger?

    private let holons: any Holons<Holon>
    private let clientFactory: ClientFactory
    private let closeClient: ClientClose
    private let slugOf: (Holon) -> String
    private let displayNameOf: (Holon) -> String
    private let sortRankOf: (Holon) -> Int
    private let connectionName: String
    private let noHolonsError: @Sendable () -> Error
    private let notConnectedError: @Sendable () -> Error

    private var client: Client?
    private var startTask: Task<Client, Error>?
    private var startTaskID: UUID?

    public init(
        holons: any Holons<Holon>,
        clientFactory: @escaping ClientFactory,
        closeClient: @escaping ClientClose,
        slugOf: @escaping (Holon) -> String,
        displayNameOf: @escaping (Holon) -> String,
        sortRankOf: @escaping (Holon) -> Int = { _ in 999 },
        defaultTransport: String = HolonTransportName.normalize(
            ProcessInfo.processInfo.environment["OP_ASSEMBLY_TRANSPORT"]
        ).rawValue,
        connectionName: String = "holon",
        noHolonsError: @escaping @Sendable () -> Error = {
            HolonProcessManagerError.noHolonsFound
        },
        notConnectedError: @escaping @Sendable () -> Error = {
            HolonProcessManagerError.notConnected
        },
        autoRefresh: Bool = true
    ) {
        self.holons = holons
        self.clientFactory = clientFactory
        self.closeClient = closeClient
        self.slugOf = slugOf
        self.displayNameOf = displayNameOf
        self.sortRankOf = sortRankOf
        self.transport = defaultTransport
        self.connectionName = connectionName
        self.noHolonsError = noHolonsError
        self.notConnectedError = notConnectedError

        if autoRefresh {
            refreshHolons()
        }
    }

    open var observabilityLoggerName: String {
        "holon-process-manager"
    }

    open func attachObservability(_ observability: Observability) {
        self.observability = observability
        self.processLogger = observability.logger(observabilityLoggerName)
    }

    public var connectedClient: Client? {
        client
    }

    public func refreshHolons() {
        let previousSelection = selectedHolon.map(slugOf)

        do {
            let results = try holons.list()
            availableHolons = results

            if let previousSelection,
               let holon = availableHolons.first(where: { slugOf($0) == previousSelection })
            {
                selectedHolon = holon
            } else {
                selectedHolon = preferredHolon(in: availableHolons)
            }
        } catch {
            availableHolons = []
            selectedHolon = nil
            connectionError = "Failed to discover \(connectionName)s: \(error.localizedDescription)"
        }
    }

    open func preferredHolon(in holons: [Holon]) -> Holon? {
        holons.sorted(by: holonSort).first
    }

    open func connectionOptions(for holon: Holon) -> ConnectOptions {
        _ = holon
        var options = ConnectOptions()
        options.transport = transport
        options.lifecycle = "ephemeral"
        options.timeout = 5.0
        return options
    }

    open func invokeRPC(
        on client: Client,
        method: String,
        payload: Data
    ) async throws -> Data {
        _ = client
        _ = method
        _ = payload
        throw HolonProcessManagerError.rpcNotImplemented
    }

    public func start() async {
        guard client == nil else { return }
        if let startTask {
            do {
                _ = try await startTask.value
            } catch {
                if connectionError == nil {
                    connectionError = failedToStartMessage(error)
                }
                isRunning = false
            }
            return
        }
        connectionError = nil

        do {
            if availableHolons.isEmpty {
                refreshHolons()
            }
            guard let holon = selectedHolon ?? preferredHolon(in: availableHolons) else {
                throw noHolonsError()
            }

            if selectedHolon != holon {
                selectedHolon = holon
            }

            let options = connectionOptions(for: holon)
            let taskID = UUID()
            startTaskID = taskID
            let factory = clientFactory
            let connectTask = Task.detached(priority: .userInitiated) { [slug = slugOf(holon), options] in
                try factory(slug, options)
            }
            startTask = connectTask

            do {
                let connectedClient = try await connectTask.value
                guard startTaskID == taskID else {
                    try? closeClient(connectedClient)
                    return
                }
                client = connectedClient
                processLogger?.info("Holon connection ready", [
                    "holon": .string(slugOf(holon)),
                    "transport": .string(connectionTarget()),
                ])
                isRunning = true
            } catch {
                guard startTaskID == taskID else {
                    return
                }
                connectionError = failedToStartMessage(error)
                processLogger?.error("Holon connection failed", [
                    "holon": .string(slugOf(holon)),
                    "transport": .string(connectionTarget()),
                    "error": .string(String(describing: error)),
                ])
                isRunning = false
            }

            if startTaskID == taskID {
                startTask = nil
                startTaskID = nil
            }
        } catch {
            connectionError = failedToStartMessage(error)
            isRunning = false
        }
    }

    public func stop() {
        startTaskID = nil
        startTask?.cancel()
        startTask = nil

        let currentClient = client
        client = nil

        do {
            if let currentClient {
                try closeClient(currentClient)
            }
        } catch {
            connectionError = "Failed to stop \(connectionName) connection: \(error.localizedDescription)"
        }

        isRunning = false
    }

    public func connectionFailure(_ fallback: Error? = nil) -> Error {
        if let connectionError, !connectionError.isEmpty {
            return NSError(
                domain: "HolonsApp.HolonProcessManager",
                code: 1,
                userInfo: [NSLocalizedDescriptionKey: connectionError]
            )
        }
        return fallback ?? notConnectedError()
    }

    private func holonSort(_ lhs: Holon, _ rhs: Holon) -> Bool {
        let leftRank = sortRankOf(lhs)
        let rightRank = sortRankOf(rhs)
        if leftRank != rightRank {
            return leftRank < rightRank
        }
        return displayNameOf(lhs).localizedCaseInsensitiveCompare(displayNameOf(rhs))
            == .orderedAscending
    }

    private func connectionTarget() -> String {
        HolonTransportName.normalize(transport).rawValue
    }

    private func failedToStartMessage(_ error: Error) -> String {
        "Failed to start \(connectionName): \(String(describing: error))"
    }

    deinit {
        if let client {
            try? closeClient(client)
        }
    }
}

public enum HolonProcessManagerError: LocalizedError {
    case noHolonsFound
    case notConnected
    case rpcNotImplemented

    public var errorDescription: String? {
        switch self {
        case .noHolonsFound:
            return "No holons found"
        case .notConnected:
            return "Not connected to a holon"
        case .rpcNotImplemented:
            return "Holon RPC invocation is not implemented"
        }
    }
}
#endif
