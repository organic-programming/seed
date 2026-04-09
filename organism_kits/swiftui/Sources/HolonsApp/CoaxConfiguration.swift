import Foundation

private let coaxServerEnabledEnv = "OP_COAX_SERVER_ENABLED"
private let coaxServerListenURIEnv = "OP_COAX_SERVER_LISTEN_URI"

public enum CoaxServerTransport: String, Codable, CaseIterable, Identifiable, Sendable {
    case tcp
    case unix

    public var id: String { rawValue }

    public var title: String {
        switch self {
        case .tcp:
            return "TCP"
        case .unix:
            return "Unix socket"
        }
    }

    public var defaultPort: Int {
        switch self {
        case .tcp:
            return 60000
        case .unix:
            return 0
        }
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        let rawValue = try container.decode(String.self)

        switch rawValue {
        case Self.tcp.rawValue:
            self = .tcp
        case Self.unix.rawValue:
            self = .unix
        case "webSocket", "restSSE":
            self = .tcp
        default:
            self = .tcp
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(rawValue)
    }
}

public struct CoaxSettingsSnapshot: Codable, Sendable {
    public var serverTransport: CoaxServerTransport
    public var serverHost: String
    public var serverPortText: String
    public var serverUnixPath: String

    public static let defaultHost = "127.0.0.1"
    public static let defaultUnixPath = defaultCoaxUnixPath()
    public static let defaults = CoaxSettingsDefaults.standard().snapshot

    public init(
        serverTransport: CoaxServerTransport,
        serverHost: String,
        serverPortText: String,
        serverUnixPath: String
    ) {
        self.serverTransport = serverTransport
        self.serverHost = serverHost
        self.serverPortText = serverPortText
        self.serverUnixPath = serverUnixPath
    }
}

public struct CoaxSettingsDefaults: Sendable {
    public let serverHost: String
    public let serverPortText: String
    public let serverUnixPath: String

    public init(
        serverHost: String = CoaxSettingsSnapshot.defaultHost,
        serverPortText: String = String(CoaxServerTransport.tcp.defaultPort),
        serverUnixPath: String
    ) {
        self.serverHost = serverHost
        self.serverPortText = serverPortText
        self.serverUnixPath = serverUnixPath
    }

    public static func standard(
        socketName: String = "organism-holon-coax.sock"
    ) -> CoaxSettingsDefaults {
        CoaxSettingsDefaults(serverUnixPath: defaultCoaxUnixPath(socketName: socketName))
    }

    public var snapshot: CoaxSettingsSnapshot {
        CoaxSettingsSnapshot(
            serverTransport: .tcp,
            serverHost: serverHost,
            serverPortText: serverPortText,
            serverUnixPath: serverUnixPath
        )
    }
}

public struct CoaxLaunchOverrides: Sendable {
    public let isEnabled: Bool?
    public let snapshot: CoaxSettingsSnapshot?

    public init(isEnabled: Bool?, snapshot: CoaxSettingsSnapshot?) {
        self.isEnabled = isEnabled
        self.snapshot = snapshot
    }
}

public func coaxLaunchOverrides(
    environment: [String: String] = ProcessInfo.processInfo.environment,
    defaults: CoaxSettingsDefaults = .standard()
) -> CoaxLaunchOverrides {
    let listenURI = environment[coaxServerListenURIEnv]?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    let enabled = parseBoolOverride(environment[coaxServerEnabledEnv])
    let snapshot = snapshotFromListenURI(listenURI, defaults: defaults)
    return CoaxLaunchOverrides(isEnabled: enabled, snapshot: snapshot)
}

public func resolvedCoaxEnabled(
    storedValue: Bool,
    overrides: CoaxLaunchOverrides
) -> Bool {
    if let enabled = overrides.isEnabled {
        return enabled
    }
    if overrides.snapshot != nil {
        return true
    }
    return storedValue
}

public func parseBoolOverride(_ value: String?) -> Bool? {
    switch value?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
    case "1", "true", "yes", "on":
        return true
    case "0", "false", "no", "off":
        return false
    default:
        return nil
    }
}

public func snapshotFromListenURI(_ listenURI: String) -> CoaxSettingsSnapshot? {
    snapshotFromListenURI(listenURI, defaults: .standard())
}

public func snapshotFromListenURI(
    _ listenURI: String,
    defaults: CoaxSettingsDefaults
) -> CoaxSettingsSnapshot? {
    guard !listenURI.isEmpty else {
        return nil
    }

    if listenURI.hasPrefix("unix://") {
        return CoaxSettingsSnapshot(
            serverTransport: .unix,
            serverHost: defaults.serverHost,
            serverPortText: defaults.serverPortText,
            serverUnixPath: String(listenURI.dropFirst("unix://".count))
        )
    }

    guard listenURI.hasPrefix("tcp://"),
          let components = URLComponents(string: listenURI) else {
        return nil
    }

    let host = components.host?.trimmingCharacters(in: .whitespacesAndNewlines)
    let port = components.port.map(String.init)
    return CoaxSettingsSnapshot(
        serverTransport: .tcp,
        serverHost: (host?.isEmpty == false ? host! : defaults.serverHost),
        serverPortText: port ?? defaults.serverPortText,
        serverUnixPath: defaults.serverUnixPath
    )
}

public func defaultCoaxUnixPath(
    socketName: String = "organism-holon-coax.sock"
) -> String {
    NSTemporaryDirectory() + socketName
}
