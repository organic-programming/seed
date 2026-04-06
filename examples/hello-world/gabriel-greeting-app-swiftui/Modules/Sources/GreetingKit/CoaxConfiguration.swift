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

struct CoaxSettingsSnapshot: Codable, Sendable {
    var serverTransport: CoaxServerTransport
    var serverHost: String
    var serverPortText: String
    var serverUnixPath: String

    static let defaultHost = "127.0.0.1"
    static let defaultUnixPath = "/tmp/gabriel-greeting-coax.sock"

    static let defaults = CoaxSettingsSnapshot(
        serverTransport: .tcp,
        serverHost: defaultHost,
        serverPortText: String(CoaxServerTransport.tcp.defaultPort),
        serverUnixPath: defaultUnixPath
    )
}

struct CoaxLaunchOverrides: Sendable {
    let isEnabled: Bool?
    let snapshot: CoaxSettingsSnapshot?
}

func coaxLaunchOverrides(
    environment: [String: String] = ProcessInfo.processInfo.environment
) -> CoaxLaunchOverrides {
    let listenURI = environment[coaxServerListenURIEnv]?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    let enabled = parseBoolOverride(environment[coaxServerEnabledEnv])
    let snapshot = snapshotFromListenURI(listenURI)
    return CoaxLaunchOverrides(isEnabled: enabled, snapshot: snapshot)
}

func resolvedCoaxEnabled(
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

private func parseBoolOverride(_ value: String?) -> Bool? {
    switch value?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
    case "1", "true", "yes", "on":
        return true
    case "0", "false", "no", "off":
        return false
    default:
        return nil
    }
}

private func snapshotFromListenURI(_ listenURI: String) -> CoaxSettingsSnapshot? {
    guard !listenURI.isEmpty else {
        return nil
    }

    if listenURI.hasPrefix("unix://") {
        return CoaxSettingsSnapshot(
            serverTransport: .unix,
            serverHost: CoaxSettingsSnapshot.defaultHost,
            serverPortText: CoaxSettingsSnapshot.defaults.serverPortText,
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
        serverHost: (host?.isEmpty == false ? host! : CoaxSettingsSnapshot.defaultHost),
        serverPortText: port ?? CoaxSettingsSnapshot.defaults.serverPortText,
        serverUnixPath: CoaxSettingsSnapshot.defaultUnixPath
    )
}
