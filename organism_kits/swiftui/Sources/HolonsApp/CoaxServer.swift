import Foundation
#if os(macOS)
@preconcurrency import GRPC
@preconcurrency import Holons

public enum CoaxSurfaceState: String, Sendable {
    case off
    case saved
    case announced
    case live
    case error

    public var badgeTitle: String {
        switch self {
        case .off:
            return "OFF"
        case .saved:
            return "SAVED"
        case .announced:
            return "ANNOUNCED"
        case .live:
            return "LIVE"
        case .error:
            return "ERROR"
        }
    }
}

public struct CoaxSurfaceStatus: Identifiable, Hashable, Sendable {
    public let id: String
    public let title: String
    public let endpoint: String?
    public let state: CoaxSurfaceState

    public init(id: String, title: String, endpoint: String?, state: CoaxSurfaceState) {
        self.id = id
        self.title = title
        self.endpoint = endpoint
        self.state = state
    }
}

@MainActor
public final class CoaxManager: ObservableObject {
    @Published public var isEnabled: Bool {
        didSet {
            guard oldValue != isEnabled else { return }
            settingsStore.writeBool(enabledKey, isEnabled)
            reconfigureRuntime()
        }
    }

    @Published public var serverTransport: CoaxServerTransport {
        didSet {
            guard oldValue != serverTransport else { return }
            if usesDefaultServerPort(for: oldValue) {
                serverPortText = String(serverTransport.defaultPort)
                return
            }
            persistSettings()
            reconfigureRuntime()
        }
    }

    @Published public var serverHost: String {
        didSet {
            guard oldValue != serverHost else { return }
            persistSettings()
            reconfigureRuntime()
        }
    }

    @Published public var serverPortText: String {
        didSet {
            guard oldValue != serverPortText else { return }
            persistSettings()
            reconfigureRuntime()
        }
    }

    @Published public var serverUnixPath: String {
        didSet {
            guard oldValue != serverUnixPath else { return }
            persistSettings()
            reconfigureRuntime()
        }
    }

    @Published public private(set) var listenURI: String?
    @Published public private(set) var statusDetail: String?

    public var serverPort: Int {
        Self.sanitizedPort(from: serverPortText, fallback: serverTransport.defaultPort)
    }

    public var serverPortValidationMessage: String? {
        guard serverTransport == .tcp else { return nil }
        let trimmed = serverPortText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return "Empty port. Falling back to \(serverTransport.defaultPort)."
        }
        guard let value = Int(trimmed), (1...65535).contains(value) else {
            return "Invalid port. Falling back to \(serverTransport.defaultPort)."
        }
        return nil
    }

    public var serverPreviewEndpoint: String {
        switch serverTransport {
        case .tcp:
            return "tcp://\(normalizedServerHost):\(serverPort)"
        case .unix:
            return "unix://\(normalizedServerUnixPath)"
        }
    }

    public var serverStatus: CoaxSurfaceStatus {
        CoaxSurfaceStatus(
            id: "server",
            title: "Server",
            endpoint: isEnabled ? (listenURI ?? serverPreviewEndpoint) : serverPreviewEndpoint,
            state: serverSurfaceState
        )
    }

    private let providers: () -> [CallHandlerProvider]
    private let registerDescribe: @Sendable () throws -> Void
    private let settingsStore: SettingsStore
    private let coaxDefaults: CoaxSettingsDefaults
    private let enabledKey: String
    private let settingsKey: String
    private var runningServer: Serve.RunningServer?
    private var pendingStartID: UUID?

    public init(
        providers: @escaping () -> [CallHandlerProvider],
        registerDescribe: @escaping @Sendable () throws -> Void,
        settingsStore: SettingsStore,
        coaxDefaults: CoaxSettingsDefaults = .standard(),
        enabledKey: String = "coax.server.enabled",
        settingsKey: String = "coax.server.settings"
    ) {
        let overrides = coaxLaunchOverrides(defaults: coaxDefaults)
        let snapshot = overrides.snapshot ?? CoaxManager.loadSnapshot(
            settingsStore: settingsStore,
            settingsKey: settingsKey,
            coaxDefaults: coaxDefaults
        )
        let storedEnabled = settingsStore.readBool(enabledKey, defaultValue: false)

        self.providers = providers
        self.registerDescribe = registerDescribe
        self.settingsStore = settingsStore
        self.coaxDefaults = coaxDefaults
        self.enabledKey = enabledKey
        self.settingsKey = settingsKey
        self.isEnabled = resolvedCoaxEnabled(storedValue: storedEnabled, overrides: overrides)
        self.serverTransport = snapshot.serverTransport
        self.serverHost = snapshot.serverHost
        self.serverPortText = snapshot.serverPortText
        self.serverUnixPath = snapshot.serverUnixPath
        self.listenURI = nil
        self.statusDetail = nil
    }

    public convenience init(
        providers: @escaping () -> [CallHandlerProvider],
        registerDescribe: @escaping @Sendable () throws -> Void,
        coaxDefaults: CoaxSettingsDefaults = .standard(),
        defaults: UserDefaults = .standard,
        enabledKey: String = "coax.server.enabled",
        settingsKey: String = "coax.server.settings"
    ) {
        self.init(
            providers: providers,
            registerDescribe: registerDescribe,
            settingsStore: UserDefaultsSettingsStore(defaults: defaults),
            coaxDefaults: coaxDefaults,
            enabledKey: enabledKey,
            settingsKey: settingsKey
        )
    }

    public func startIfEnabled() {
        guard isEnabled else { return }
        reconfigureRuntime()
    }

    public func stop() {
        pendingStartID = nil
        stopServer(clearStatus: true)
    }

    public func turnOffAfterRpc() {
        DispatchQueue.main.asyncAfter(deadline: .now() + .milliseconds(100)) { [weak self] in
            self?.isEnabled = false
        }
    }

    public var defaultUnixPath: String {
        coaxDefaults.serverUnixPath
    }

    private var serverSurfaceState: CoaxSurfaceState {
        if !isEnabled {
            return .off
        }
        if statusDetail != nil {
            return .error
        }
        if listenURI != nil {
            return .live
        }
        return .announced
    }

    private var normalizedServerHost: String {
        let trimmed = serverHost.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? coaxDefaults.serverHost : trimmed
    }

    private var normalizedServerUnixPath: String {
        let trimmed = serverUnixPath.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? coaxDefaults.serverUnixPath : trimmed
    }

    private var runtimeListenURI: String {
        switch serverTransport {
        case .tcp:
            return "tcp://\(normalizedServerHost):\(serverPort)"
        case .unix:
            return "unix://\(normalizedServerUnixPath)"
        }
    }

    private func usesDefaultServerPort(for oldTransport: CoaxServerTransport) -> Bool {
        guard oldTransport != .unix else {
            return serverPortText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }
        let trimmed = serverPortText.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return true
        }
        return Self.sanitizedPort(from: trimmed, fallback: oldTransport.defaultPort) == oldTransport.defaultPort
    }

    private func reconfigureRuntime() {
        pendingStartID = nil

        guard isEnabled else {
            stopServer(clearStatus: true)
            return
        }

        stopServer(clearStatus: false)
        startServer()
    }

    private func startServer() {
        let providersBox = CallHandlerProvidersBox(value: providers())
        let listenTarget = runtimeListenURI
        let startID = UUID()
        let registerDescribe = self.registerDescribe

        pendingStartID = startID
        listenURI = nil
        statusDetail = nil

        DispatchQueue.global(qos: .utility).async { [weak self] in
            do {
                try registerDescribe()
                let server = try Serve.startWithOptions(
                    listenTarget,
                    serviceProviders: providersBox.value,
                    options: Serve.Options(logger: { _ in })
                )
                let publicURI = server.publicURI

                DispatchQueue.main.async { [weak self] in
                    guard let self else {
                        let staleServer = RunningServerBox(value: server)
                        DispatchQueue.global(qos: .utility).async {
                            staleServer.value.stop()
                        }
                        return
                    }

                    guard self.pendingStartID == startID,
                          self.isEnabled,
                          self.runtimeListenURI == listenTarget else {
                        let staleServer = RunningServerBox(value: server)
                        DispatchQueue.global(qos: .utility).async {
                            staleServer.value.stop()
                        }
                        return
                    }

                    self.runningServer = server
                    self.listenURI = publicURI
                    self.statusDetail = nil
                    self.logCoax("[COAX] server listening on \(publicURI)")
                }
            } catch {
                DispatchQueue.main.async { [weak self] in
                    guard let self, self.pendingStartID == startID else { return }
                    self.listenURI = nil
                    self.statusDetail = "Server surface failed to start: \(error)"
                    self.logCoax("[COAX] failed to start server: \(error)")
                }
            }
        }
    }

    private func stopServer(clearStatus: Bool) {
        let server = runningServer
        runningServer = nil
        listenURI = nil
        if clearStatus {
            statusDetail = nil
        }

        guard let server else { return }

        logCoax("[COAX] server stopped")
        let runningServer = RunningServerBox(value: server)
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + .milliseconds(250)) {
            runningServer.value.stop()
        }
    }

    private func persistSettings() {
        let snapshot = CoaxSettingsSnapshot(
            serverTransport: serverTransport,
            serverHost: serverHost,
            serverPortText: serverPortText,
            serverUnixPath: serverUnixPath
        )

        let encoder = JSONEncoder()
        guard let data = try? encoder.encode(snapshot),
              let value = String(data: data, encoding: .utf8) else {
            return
        }
        settingsStore.writeString(settingsKey, value)
    }

    private static func loadSnapshot(
        settingsStore: SettingsStore,
        settingsKey: String,
        coaxDefaults: CoaxSettingsDefaults
    ) -> CoaxSettingsSnapshot {
        let value = settingsStore.readString(settingsKey, defaultValue: "")
        guard let data = value.data(using: .utf8),
              let snapshot = try? JSONDecoder().decode(CoaxSettingsSnapshot.self, from: data) else {
            return coaxDefaults.snapshot
        }
        return snapshot
    }

    private static func sanitizedPort(from text: String, fallback: Int) -> Int {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let value = Int(trimmed), (1...65535).contains(value) else {
            return fallback
        }
        return value
    }

    private func logCoax(_ line: String) {
        guard let data = (line + "\n").data(using: .utf8) else { return }
        FileHandle.standardError.write(data)
    }
}

private struct CallHandlerProvidersBox: @unchecked Sendable {
    let value: [CallHandlerProvider]
}

private struct RunningServerBox: @unchecked Sendable {
    let value: Serve.RunningServer
}

@available(*, deprecated, renamed: "CoaxManager")
public typealias CoaxServer = CoaxManager
#endif
