import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

public struct HolonRPCResponseError: Error, CustomStringConvertible {
    public let code: Int
    public let message: String
    public let data: Any?

    public init(code: Int, message: String, data: Any? = nil) {
        self.code = code
        self.message = message
        self.data = data
    }

    public var description: String {
        "rpc error \(code): \(message)"
    }
}

public enum HolonRPCClientError: Error, CustomStringConvertible {
    case invalidURL(String)
    case notConnected
    case protocolError(String)
    case timeout
    case serialization(String)

    public var description: String {
        switch self {
        case let .invalidURL(url):
            return "invalid URL: \(url)"
        case .notConnected:
            return "holon-rpc client is not connected"
        case let .protocolError(message):
            return "holon-rpc protocol error: \(message)"
        case .timeout:
            return "operation timed out"
        case let .serialization(message):
            return "serialization error: \(message)"
        }
    }
}

public actor HolonRPCClient {
    public typealias Params = [String: Any]
    public typealias Handler = @Sendable (Params) async throws -> Params

    private let heartbeatInterval: TimeInterval
    private let heartbeatTimeout: TimeInterval
    private let reconnectMinDelay: TimeInterval
    private let reconnectMaxDelay: TimeInterval
    private let reconnectFactor: Double
    private let reconnectJitter: Double

    private var urlString: String?
    private var session: URLSession?
    private var sessionDelegate: AnyObject?
    private var webSocketTask: URLSessionWebSocketTask?
    private var receiveTask: Task<Void, Never>?
    private var heartbeatTask: Task<Void, Never>?
    private var reconnectTask: Task<Void, Never>?

    private var handlers: [String: Handler] = [:]
    private var pending: [String: CheckedContinuation<Params, Error>] = [:]
    private var nextClientID: Int = 0
    private var isClosed = false

    public init(
        heartbeatInterval: TimeInterval = 15.0,
        heartbeatTimeout: TimeInterval = 5.0,
        reconnectMinDelay: TimeInterval = 0.5,
        reconnectMaxDelay: TimeInterval = 30.0,
        reconnectFactor: Double = 2.0,
        reconnectJitter: Double = 0.1
    ) {
        self.heartbeatInterval = heartbeatInterval
        self.heartbeatTimeout = heartbeatTimeout
        self.reconnectMinDelay = reconnectMinDelay
        self.reconnectMaxDelay = reconnectMaxDelay
        self.reconnectFactor = reconnectFactor
        self.reconnectJitter = reconnectJitter
    }

    public func connect(_ url: String) async throws {
        urlString = url
        isClosed = false
        try await connectOnce()
        startRuntimeTasks()
    }

    public func register(method: String, handler: @escaping Handler) {
        handlers[method] = handler
    }

    public func invoke(method: String, params: Params = [:]) async throws -> Params {
        guard !method.isEmpty else {
            throw HolonRPCClientError.protocolError("method is required")
        }

        guard webSocketTask != nil else {
            throw HolonRPCClientError.notConnected
        }

        nextClientID += 1
        let id = "c\(nextClientID)"

        let request: [String: Any] = [
            "jsonrpc": "2.0",
            "id": id,
            "method": method,
            "params": params,
        ]

        return try await withCheckedThrowingContinuation { continuation in
            pending[id] = continuation
            Task {
                do {
                    try await sendJSON(request)
                } catch {
                    failPending(id: id, error: error)
                }
            }
        }
    }

    public func close() async {
        isClosed = true

        reconnectTask?.cancel()
        reconnectTask = nil

        heartbeatTask?.cancel()
        heartbeatTask = nil

        receiveTask?.cancel()
        receiveTask = nil

        webSocketTask?.cancel(with: .normalClosure, reason: nil)
        webSocketTask = nil

        session?.invalidateAndCancel()
        session = nil
        sessionDelegate = nil

        failAllPending(error: HolonRPCClientError.notConnected)
    }

    private func connectOnce() async throws {
        guard let urlString else {
            throw HolonRPCClientError.invalidURL("<nil>")
        }
        let resolved = try resolveHolonRPCWebSocketURL(urlString)
        let built = try makeHolonRPCURLSession(timeout: 15, caFilePath: resolved.caFilePath)
        let session = built.session
        let task = session.webSocketTask(with: resolved.url, protocols: ["holon-rpc"])
        task.resume()

        // Wait briefly for handshake to complete so response headers are available.
        var selectedProtocol: String?
        for _ in 0..<50 {
            if let response = task.response as? HTTPURLResponse,
               let protoHeader = response.value(forHTTPHeaderField: "Sec-WebSocket-Protocol") {
                selectedProtocol = protoHeader
                break
            }
            try await Task.sleep(nanoseconds: 40_000_000)
        }

        guard selectedProtocol == "holon-rpc" else {
            task.cancel(with: .protocolError, reason: nil)
            session.invalidateAndCancel()
            throw HolonRPCClientError.protocolError("server did not negotiate holon-rpc")
        }

        self.session = session
        self.sessionDelegate = built.delegate
        self.webSocketTask = task
    }

    private func startRuntimeTasks() {
        receiveTask?.cancel()
        receiveTask = Task { [weak self] in
            guard let self else { return }
            await self.receiveLoop()
        }

        heartbeatTask?.cancel()
        heartbeatTask = Task { [weak self] in
            guard let self else { return }
            await self.heartbeatLoop()
        }
    }

    private func receiveLoop() async {
        while !Task.isCancelled {
            guard let task = webSocketTask else {
                return
            }

            do {
                let msg = try await task.receive()
                switch msg {
                case let .string(text):
                    try await handleIncomingText(text)
                case let .data(data):
                    if let text = String(data: data, encoding: .utf8) {
                        try await handleIncomingText(text)
                    }
                @unknown default:
                    break
                }
            } catch {
                await handleDisconnect()
                return
            }
        }
    }

    private func heartbeatLoop() async {
        while !Task.isCancelled {
            do {
                try await Task.sleep(nanoseconds: UInt64(heartbeatInterval * 1_000_000_000))
            } catch {
                return
            }

            if isClosed {
                return
            }

            do {
                _ = try await invokeWithTimeout(method: "rpc.heartbeat", params: [:], timeout: heartbeatTimeout)
            } catch {
                await handleDisconnect()
                return
            }
        }
    }

    private func invokeWithTimeout(method: String, params: Params, timeout: TimeInterval) async throws -> Params {
        try await withThrowingTaskGroup(of: Params.self) { group in
            group.addTask { [weak self] in
                guard let self else { throw HolonRPCClientError.notConnected }
                return try await self.invoke(method: method, params: params)
            }

            group.addTask {
                try await Task.sleep(nanoseconds: UInt64(timeout * 1_000_000_000))
                throw HolonRPCClientError.timeout
            }

            let result = try await group.next()!
            group.cancelAll()
            return result
        }
    }

    private func handleIncomingText(_ text: String) async throws {
        guard let data = text.data(using: .utf8) else {
            throw HolonRPCClientError.serialization("invalid UTF-8 payload")
        }
        guard let payload = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw HolonRPCClientError.serialization("invalid JSON payload")
        }

        if payload["method"] != nil {
            try await handleIncomingRequest(payload)
            return
        }

        if payload["result"] != nil || payload["error"] != nil {
            handleIncomingResponse(payload)
        }
    }

    private func handleIncomingRequest(_ payload: [String: Any]) async throws {
        guard let method = payload["method"] as? String,
              (payload["jsonrpc"] as? String) == "2.0" else {
            if let id = payload["id"] {
                try await sendError(id: id, code: -32600, message: "invalid request")
            }
            return
        }

        if method == "rpc.heartbeat" {
            if let id = payload["id"] {
                try await sendResult(id: id, result: [:])
            }
            return
        }

        if let id = payload["id"], !String(describing: id).hasPrefix("s") {
            try await sendError(id: id, code: -32600, message: "server request id must start with 's'")
            return
        }

        guard let handler = handlers[method] else {
            if let id = payload["id"] {
                try await sendError(id: id, code: -32601, message: "method \(method) not found")
            }
            return
        }

        let params = payload["params"] as? [String: Any] ?? [:]

        do {
            let result = try await handler(params)
            if let id = payload["id"] {
                try await sendResult(id: id, result: result)
            }
        } catch let rpcError as HolonRPCResponseError {
            if let id = payload["id"] {
                try await sendError(id: id, code: rpcError.code, message: rpcError.message, data: rpcError.data)
            }
        } catch {
            if let id = payload["id"] {
                try await sendError(id: id, code: 13, message: String(describing: error))
            }
        }
    }

    private func handleIncomingResponse(_ payload: [String: Any]) {
        guard let rawID = payload["id"] else {
            return
        }
        let id = String(describing: rawID)

        guard let continuation = pending.removeValue(forKey: id) else {
            return
        }

        if let err = payload["error"] as? [String: Any] {
            let code = err["code"] as? Int ?? -32603
            let message = err["message"] as? String ?? "internal error"
            continuation.resume(throwing: HolonRPCResponseError(code: code, message: message, data: err["data"]))
            return
        }

        guard payload.keys.contains("result") else {
            continuation.resume(returning: [:])
            return
        }

        let rawResult = payload["result"]
        if rawResult is NSNull || rawResult == nil {
            continuation.resume(returning: [:])
            return
        }

        if let result = rawResult as? [String: Any] {
            continuation.resume(returning: result)
            return
        }

        // Keep object results as-is and wrap scalar/array values so callers
        // can consume fan-out aggregate arrays through result["value"].
        continuation.resume(returning: ["value": rawResult as Any])
    }

    private func sendResult(id: Any, result: Params) async throws {
        try await sendJSON([
            "jsonrpc": "2.0",
            "id": id,
            "result": result,
        ])
    }

    private func sendError(id: Any, code: Int, message: String, data: Any? = nil) async throws {
        var err: [String: Any] = [
            "code": code,
            "message": message,
        ]
        if let data {
            err["data"] = data
        }

        try await sendJSON([
            "jsonrpc": "2.0",
            "id": id,
            "error": err,
        ])
    }

    private func sendJSON(_ payload: [String: Any]) async throws {
        guard let task = webSocketTask else {
            throw HolonRPCClientError.notConnected
        }

        let data = try JSONSerialization.data(withJSONObject: payload, options: [])
        guard let text = String(data: data, encoding: .utf8) else {
            throw HolonRPCClientError.serialization("invalid JSON payload")
        }

        try await task.send(.string(text))
    }

    private func handleDisconnect() async {
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
        session?.invalidateAndCancel()
        session = nil
        sessionDelegate = nil

        failAllPending(error: HolonRPCClientError.notConnected)

        if isClosed {
            return
        }

        if reconnectTask == nil || reconnectTask?.isCancelled == true {
            reconnectTask = Task { [weak self] in
                guard let self else { return }
                await self.reconnectLoop()
            }
        }
    }

    private func reconnectLoop() async {
        var attempt = 0

        while !Task.isCancelled {
            if isClosed {
                return
            }

            do {
                try await connectOnce()
                startRuntimeTasks()
                reconnectTask = nil
                return
            } catch {
                let base = min(reconnectMinDelay * pow(reconnectFactor, Double(attempt)), reconnectMaxDelay)
                let jitter = base * reconnectJitter * Double.random(in: 0...1)
                let delay = base + jitter

                do {
                    try await Task.sleep(nanoseconds: UInt64(delay * 1_000_000_000))
                } catch {
                    return
                }
                attempt += 1
            }
        }
    }

    private func failPending(id: String, error: Error) {
        guard let continuation = pending.removeValue(forKey: id) else {
            return
        }
        continuation.resume(throwing: error)
    }

    private func failAllPending(error: Error) {
        let values = pending.values
        pending.removeAll()

        for continuation in values {
            continuation.resume(throwing: error)
        }
    }
}
