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

// CoaxServer manages the embedded gRPC surface shown in the HostUI.
// The server surface can start the transports the current Swift runtime
// serves directly here: tcp:// and unix://.
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
            endpoint: serverEnabled ? (listenURI ?? serverPreviewEndpoint) : serverPreviewEndpoint,
            state: serverSurfaceState
        )
    }

    private let holon: HolonProcess
    private var runningServer: Serve.RunningServer?
    private var pendingStartID: UUID?

    private static let enabledKey = "coax.server.enabled"
    private static let settingsKey = "coax.server.settings"

    public init(holon: HolonProcess) {
        let overrides = coaxLaunchOverrides()
        let snapshot = overrides.snapshot ?? CoaxServer.loadSnapshot()
        let storedEnabled = UserDefaults.standard.bool(forKey: CoaxServer.enabledKey)

        self.holon = holon

        self.isEnabled = resolvedCoaxEnabled(storedValue: storedEnabled, overrides: overrides)
        self.serverEnabled = snapshot.serverEnabled
        self.serverTransport = snapshot.serverTransport
        self.serverHost = snapshot.serverHost
        self.serverPortText = snapshot.serverPortText
        self.serverUnixPath = snapshot.serverUnixPath
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

    private var normalizedServerHost: String {
        let trimmed = serverHost.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? CoaxSettingsSnapshot.defaultHost : trimmed
    }

    private var normalizedServerUnixPath: String {
        let trimmed = serverUnixPath.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? CoaxSettingsSnapshot.defaultUnixPath : trimmed
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

    private func reconfigureRuntime() {
        pendingStartID = nil

        guard isEnabled, serverEnabled else {
            stopServer(clearStatus: true)
            return
        }

        stopServer(clearStatus: false)
        startServer()
    }

    private func startServer() {
        let coaxProvider = CoaxServiceProvider(holon: holon, coaxServer: self)
        let appProvider = GreetingAppServiceProvider(holon: holon)
        let providers: [CallHandlerProvider] = [coaxProvider, appProvider]
        let providersBox = CallHandlerProvidersBox(providers)
        let listenTarget = runtimeListenURI
        let startID = UUID()

        pendingStartID = startID
        listenURI = nil
        statusDetail = nil

        DispatchQueue.global(qos: .utility).async { [weak self] in
            do {
                try CoaxDescribeRegistration.register()
                let server = try Serve.startWithOptions(
                    listenTarget,
                    serviceProviders: providersBox.value,
                    options: Serve.Options(
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
            serverUnixPath: serverUnixPath
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
