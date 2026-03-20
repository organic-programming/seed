import Foundation
#if canImport(Security)
import Security
#endif

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

public enum CoaxRelayTransport: String, Codable, CaseIterable, Identifiable, Sendable {
    case restSSE
    case webSocket
    case other

    public var id: String { rawValue }

    public var title: String {
        switch self {
        case .restSSE:
            return "SSE + REST"
        case .webSocket:
            return "WS / WSS"
        case .other:
            return "Other"
        }
    }
}

public enum CoaxMCPTransport: String, Codable, CaseIterable, Identifiable, Sendable {
    case stdio
    case streamableHTTP
    case sse
    case other

    public var id: String { rawValue }

    public var title: String {
        switch self {
        case .stdio:
            return "stdio"
        case .streamableHTTP:
            return "Streamable HTTP"
        case .sse:
            return "SSE"
        case .other:
            return "Other"
        }
    }
}

struct CoaxSettingsSnapshot: Codable, Sendable {
    var serverEnabled: Bool
    var serverTransport: CoaxServerTransport
    var serverHost: String
    var serverPortText: String
    var serverUnixPath: String
    var relayEnabled: Bool
    var relayTransport: CoaxRelayTransport
    var relayURL: String
    var mcpEnabled: Bool
    var mcpTransport: CoaxMCPTransport
    var mcpEndpoint: String
    var mcpCommand: String

    static let defaultHost = "127.0.0.1"
    static let defaultUnixPath = "/tmp/gabriel-greeting-coax.sock"

    static let defaults = CoaxSettingsSnapshot(
        serverEnabled: true,
        serverTransport: .tcp,
        serverHost: defaultHost,
        serverPortText: String(CoaxServerTransport.tcp.defaultPort),
        serverUnixPath: defaultUnixPath,
        relayEnabled: false,
        relayTransport: .restSSE,
        relayURL: "",
        mcpEnabled: false,
        mcpTransport: .stdio,
        mcpEndpoint: "",
        mcpCommand: ""
    )
}

struct CoaxSecretStore {
    private let service = "com.compilons.gabriel.greeting-app-swiftui.coax"
    private let account: String

    init(account: String) {
        self.account = account
    }

    func load() -> String {
#if canImport(Security)
        var query = baseQuery()
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        query[kSecReturnData as String] = true

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        guard status == errSecSuccess,
              let data = item as? Data,
              let token = String(data: data, encoding: .utf8) else {
            return ""
        }
        return token
#else
        return ""
#endif
    }

    func save(_ token: String) {
#if canImport(Security)
        let trimmed = token.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            delete()
            return
        }

        let data = Data(trimmed.utf8)
        let query = baseQuery()
        let attributes = [kSecValueData as String: data]
        let status = SecItemCopyMatching(query as CFDictionary, nil)

        if status == errSecSuccess {
            SecItemUpdate(query as CFDictionary, attributes as CFDictionary)
        } else {
            var insert = query
            insert[kSecValueData as String] = data
            SecItemAdd(insert as CFDictionary, nil)
        }
#endif
    }

    func delete() {
#if canImport(Security)
        SecItemDelete(baseQuery() as CFDictionary)
#endif
    }

#if canImport(Security)
    private func baseQuery() -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account
        ]
    }
#endif
}
