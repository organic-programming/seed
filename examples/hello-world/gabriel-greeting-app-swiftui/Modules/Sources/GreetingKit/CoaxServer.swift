import Foundation
#if os(macOS)
@preconcurrency import Holons
@preconcurrency import GRPC

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
}

// CoaxServer manages the embedded gRPC surface plus the announced relay and MCP
// surfaces shown in the HostUI. The server surface can start the transports the
// current Swift runtime serves directly here: tcp:// and unix://.
@MainActor
public final class CoaxServer: ObservableObject {
    @Published public var isEnabled: Bool {
        didSet {
            guard oldValue != isEnabled else { return }
            UserDefaults.standard.set(isEnabled, forKey: CoaxServer.enabledKey)
            reconfigureRuntime()
        }
    }

    @Published public var serverEnabled: Bool {
        didSet {
            guard oldValue != serverEnabled else { return }
            persistSettings()
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
            reconfigureRuntimeIfNeeded()
        }
    }

    @Published public var serverHost: String {
        didSet {
            guard oldValue != serverHost else { return }
            persistSettings()
            reconfigureRuntimeIfNeeded()
        }
    }

    @Published public var serverPortText: String {
        didSet {
            guard oldValue != serverPortText else { return }
            persistSettings()
            reconfigureRuntimeIfNeeded()
        }
    }

    @Published public var serverUnixPath: String {
        didSet {
            guard oldValue != serverUnixPath else { return }
            persistSettings()
            reconfigureRuntimeIfNeeded()
        }
    }

    @Published public var relayEnabled: Bool {
        didSet {
            guard oldValue != relayEnabled else { return }
            persistSettings()
        }
    }

    @Published public var relayTransport: CoaxRelayTransport {
        didSet {
            guard oldValue != relayTransport else { return }
            persistSettings()
        }
    }

    @Published public var relayURL: String {
        didSet {
            guard oldValue != relayURL else { return }
            persistSettings()
        }
    }

    @Published public var relayBearerToken: String {
        didSet {
            guard oldValue != relayBearerToken else { return }
            relayTokenStore.save(relayBearerToken)
        }
    }

    @Published public var mcpEnabled: Bool {
        didSet {
            guard oldValue != mcpEnabled else { return }
            persistSettings()
        }
    }

    @Published public var mcpTransport: CoaxMCPTransport {
        didSet {
            guard oldValue != mcpTransport else { return }
            persistSettings()
        }
    }

    @Published public var mcpEndpoint: String {
        didSet {
            guard oldValue != mcpEndpoint else { return }
            persistSettings()
        }
    }

    @Published public var mcpCommand: String {
        didSet {
            guard oldValue != mcpCommand else { return }
            persistSettings()
        }
    }

    @Published public var mcpBearerToken: String {
        didSet {
            guard oldValue != mcpBearerToken else { return }
            mcpTokenStore.save(mcpBearerToken)
        }
    }

    @Published public private(set) var listenURI: String?
    @Published public private(set) var statusDetail: String?

    public var serverPort: Int {
        Self.sanitizedPort(from: serverPortText, fallback: serverTransport.defaultPort)
    }

    public var serverPortValidationMessage: String? {
        guard serverTransport == .tcp || serverTransport == .webSocket || serverTransport == .restSSE else { return nil }
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
        case .webSocket:
            return "ws://\(normalizedServerHost):\(serverPort)/grpc"
        case .restSSE:
            return "http://\(normalizedServerHost):\(serverPort)"
        }
    }

    public var relayPreviewEndpoint: String {
        let trimmed = trimmedRelayURL
        if !trimmed.isEmpty {
            return trimmed
        }

        switch relayTransport {
        case .restSSE:
            return "https://relay.example.com"
        case .webSocket:
            return "wss://relay.example.com/grpc"
        case .other:
            return "custom://relay.example.com"
        }
    }

    public var mcpPreviewEndpoint: String {
        switch mcpTransport {
        case .stdio:
            return trimmedMcpCommand.isEmpty ? "stdio: uvx my-mcp-server" : "stdio: \(trimmedMcpCommand)"
        case .streamableHTTP:
            return trimmedMcpEndpoint.isEmpty ? "https://mcp.example.com" : trimmedMcpEndpoint
        case .sse:
            return trimmedMcpEndpoint.isEmpty ? "https://mcp.example.com/sse" : trimmedMcpEndpoint
        case .other:
            if !trimmedMcpEndpoint.isEmpty {
                return trimmedMcpEndpoint
            }
            if !trimmedMcpCommand.isEmpty {
                return trimmedMcpCommand
            }
            return "custom+mcp://surface"
        }
    }

    public var secretStorageNote: String {
        "Bearer tokens are stored separately from visible preferences."
    }

    public var serverTransportNote: String? {
        switch serverTransport {
        case .tcp, .unix:
            return nil
        case .webSocket:
            return "WS / WSS is shown as a future extension. The embedded runtime currently starts TCP and Unix socket only."
        case .restSSE:
            return "SSE + REST is shown as a future extension. The embedded runtime currently starts TCP and Unix socket only."
        }
    }

    public var serverStatus: CoaxSurfaceStatus {
        CoaxSurfaceStatus(
            id: "server",
            title: "Server",
            endpoint: serverEnabled ? (listenURI ?? serverPreviewEndpoint) : serverPreviewEndpoint,
            state: serverSurfaceState
        )
    }

    public var relayStatus: CoaxSurfaceStatus {
        CoaxSurfaceStatus(
            id: "relay",
            title: "Relay",
            endpoint: relayPreviewEndpoint,
            state: relaySurfaceState
        )
    }

    public var mcpStatus: CoaxSurfaceStatus {
        CoaxSurfaceStatus(
            id: "mcp",
            title: "MCP",
            endpoint: mcpPreviewEndpoint,
            state: mcpSurfaceState
        )
    }

    public var visibleSurfaceStatuses: [CoaxSurfaceStatus] {
        var result: [CoaxSurfaceStatus] = []
        if serverEnabled {
            result.append(serverStatus)
        }
        if relayEnabled {
            result.append(relayStatus)
        }
        if mcpEnabled {
            result.append(mcpStatus)
        }
        return result
    }

    private let holon: HolonProcess
    private let relayTokenStore: CoaxSecretStore
    private let mcpTokenStore: CoaxSecretStore
    private var runningServer: Serve.RunningServer?
    private var pendingStartID: UUID?

    private static let enabledKey = "coax.server.enabled"
    private static let settingsKey = "coax.server.settings"

    public init(holon: HolonProcess) {
        let snapshot = CoaxServer.loadSnapshot()
        let relayTokenStore = CoaxSecretStore(account: "relay-bearer-token")
        let mcpTokenStore = CoaxSecretStore(account: "mcp-bearer-token")

        self.holon = holon
        self.relayTokenStore = relayTokenStore
        self.mcpTokenStore = mcpTokenStore

        self.isEnabled = UserDefaults.standard.bool(forKey: CoaxServer.enabledKey)
        self.serverEnabled = snapshot.serverEnabled
        self.serverTransport = snapshot.serverTransport
        self.serverHost = snapshot.serverHost
        self.serverPortText = snapshot.serverPortText
        self.serverUnixPath = snapshot.serverUnixPath
        self.relayEnabled = snapshot.relayEnabled
        self.relayTransport = snapshot.relayTransport
        self.relayURL = snapshot.relayURL
        self.relayBearerToken = relayTokenStore.load()
        self.mcpEnabled = snapshot.mcpEnabled
        self.mcpTransport = snapshot.mcpTransport
        self.mcpEndpoint = snapshot.mcpEndpoint
        self.mcpCommand = snapshot.mcpCommand
        self.mcpBearerToken = mcpTokenStore.load()
        self.listenURI = nil
        self.statusDetail = nil
    }

    public func startIfEnabled() {
        guard isEnabled else { return }
        reconfigureRuntime()
    }

    public func stop() {
        pendingStartID = nil
        stopServer(clearStatus: true)
    }

    private var serverSurfaceState: CoaxSurfaceState {
        if !serverEnabled {
            return .off
        }
        if let _ = statusDetail, isEnabled {
            return .error
        }
        if listenURI != nil {
            return .live
        }
        if !isEnabled {
            return .saved
        }
        return .announced
    }

    private var relaySurfaceState: CoaxSurfaceState {
        if !relayEnabled {
            return .off
        }
        return isEnabled ? .announced : .saved
    }

    private var mcpSurfaceState: CoaxSurfaceState {
        if !mcpEnabled {
            return .off
        }
        return isEnabled ? .announced : .saved
    }

    private var normalizedServerHost: String {
        let trimmed = serverHost.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? CoaxSettingsSnapshot.defaultHost : trimmed
    }

    private var normalizedServerUnixPath: String {
        let trimmed = serverUnixPath.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? CoaxSettingsSnapshot.defaultUnixPath : trimmed
    }

    private var trimmedRelayURL: String {
        relayURL.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedMcpEndpoint: String {
        mcpEndpoint.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedMcpCommand: String {
        mcpCommand.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var runtimeListenURI: String {
        switch serverTransport {
        case .tcp:
            return "tcp://\(normalizedServerHost):\(serverPort)"
        case .unix:
            return "unix://\(normalizedServerUnixPath)"
        case .webSocket, .restSSE:
            return ""
        }
    }

    private func usesDefaultServerPort(for oldTransport: CoaxServerTransport) -> Bool {
        guard oldTransport != .unix else { return serverPortText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
        let trimmed = serverPortText.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return true
        }
        return Self.sanitizedPort(from: trimmed, fallback: oldTransport.defaultPort) == oldTransport.defaultPort
    }

    private func reconfigureRuntimeIfNeeded() {
        guard serverEnabled else { return }
        reconfigureRuntime()
    }

    private var serverTransportSupportsRuntime: Bool {
        switch serverTransport {
        case .tcp, .unix:
            return true
        case .webSocket, .restSSE:
            return false
        }
    }

    private func reconfigureRuntime() {
        pendingStartID = nil

        guard isEnabled, serverEnabled else {
            stopServer(clearStatus: true)
            return
        }

        stopServer(clearStatus: false)
        guard serverTransportSupportsRuntime else { return }
        startServer()
    }

    private func startServer() {
        let coaxProvider = CoaxServiceProvider(holon: holon, coaxServer: self)
        let appProvider = GreetingAppServiceProvider(holon: holon)
        let metaProvider = CoaxDescribeProvider()
        let providers: [CallHandlerProvider] = [metaProvider, coaxProvider, appProvider]
        let providersBox = CallHandlerProvidersBox(providers)
        let listenTarget = runtimeListenURI
        let startID = UUID()

        pendingStartID = startID
        listenURI = nil
        statusDetail = nil

        DispatchQueue.global(qos: .utility).async { [weak self] in
            do {
                let server = try Serve.startWithOptions(
                    listenTarget,
                    serviceProviders: providersBox.value,
                    options: Serve.Options(
                        describe: false,
                        logger: { _ in }
                    )
                )
                let publicURI = server.publicURI

                DispatchQueue.main.async { [weak self] in
                    guard let self else {
                        let staleServer = RunningServerBox(server)
                        DispatchQueue.global(qos: .utility).async {
                            staleServer.value.stop()
                        }
                        return
                    }

                    guard self.pendingStartID == startID,
                          self.isEnabled,
                          self.serverEnabled,
                          self.runtimeListenURI == listenTarget else {
                        let staleServer = RunningServerBox(server)
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
        let runningServer = RunningServerBox(server)
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + .milliseconds(250)) {
            runningServer.value.stop()
        }
    }

    private func persistSettings() {
        let snapshot = CoaxSettingsSnapshot(
            serverEnabled: serverEnabled,
            serverTransport: serverTransport,
            serverHost: serverHost,
            serverPortText: serverPortText,
            serverUnixPath: serverUnixPath,
            relayEnabled: relayEnabled,
            relayTransport: relayTransport,
            relayURL: relayURL,
            mcpEnabled: mcpEnabled,
            mcpTransport: mcpTransport,
            mcpEndpoint: mcpEndpoint,
            mcpCommand: mcpCommand
        )

        let encoder = JSONEncoder()
        guard let data = try? encoder.encode(snapshot) else { return }
        UserDefaults.standard.set(data, forKey: CoaxServer.settingsKey)
    }

    private static func loadSnapshot() -> CoaxSettingsSnapshot {
        guard let data = UserDefaults.standard.data(forKey: settingsKey),
              let snapshot = try? JSONDecoder().decode(CoaxSettingsSnapshot.self, from: data) else {
            return .defaults
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

    init(_ value: [CallHandlerProvider]) {
        self.value = value
    }
}

private final class RunningServerBox: @unchecked Sendable {
    let value: Serve.RunningServer

    init(_ value: Serve.RunningServer) {
        self.value = value
    }
}
#endif
