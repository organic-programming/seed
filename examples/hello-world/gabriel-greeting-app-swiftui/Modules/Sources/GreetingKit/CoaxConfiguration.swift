import Foundation

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
    var serverEnabled: Bool
    var serverTransport: CoaxServerTransport
    var serverHost: String
    var serverPortText: String
    var serverUnixPath: String

    static let defaultHost = "127.0.0.1"
    static let defaultUnixPath = "/tmp/gabriel-greeting-coax.sock"

    static let defaults = CoaxSettingsSnapshot(
        serverEnabled: true,
        serverTransport: .tcp,
        serverHost: defaultHost,
        serverPortText: String(CoaxServerTransport.tcp.defaultPort),
        serverUnixPath: defaultUnixPath
    )
}
