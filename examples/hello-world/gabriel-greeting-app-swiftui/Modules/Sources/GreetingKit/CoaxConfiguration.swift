import Foundation

public enum CoaxServerTransport: String, Codable, CaseIterable, Identifiable, Sendable {
    case tcp
    case unix
    case webSocket
    case restSSE

    public var id: String { rawValue }

    public var title: String {
        switch self {
        case .tcp:
            return "TCP"
        case .unix:
            return "Unix socket"
        case .webSocket:
            return "WS / WSS"
        case .restSSE:
            return "SSE + REST"
        }
    }

    public var subtitle: String {
        switch self {
        case .tcp:
            return "grpc-swift TCP listener"
        case .unix:
            return "Unix domain socket bridge"
        case .webSocket:
            return "Future extension"
        case .restSSE:
            return "Future extension"
        }
    }

    public var defaultPort: Int {
        switch self {
        case .tcp:
            return 60000
        case .unix:
            return 0
        case .webSocket, .restSSE:
            return 80
        }
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
