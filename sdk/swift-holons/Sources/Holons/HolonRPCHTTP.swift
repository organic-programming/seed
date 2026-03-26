import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif
#if canImport(Security)
import Security
#endif

private let defaultHolonRPCWebSocketPath = "/rpc"
private let defaultHolonRPCHTTPPath = "/api/v1/rpc"

internal struct HolonRPCResolvedURL {
    let url: URL
    let caFilePath: String?
}

private enum HolonRPCURLKind {
    case webSocket
    case http
}

internal func resolveHolonRPCWebSocketURL(_ raw: String) throws -> HolonRPCResolvedURL {
    try resolveHolonRPCURL(raw, kind: .webSocket)
}

internal func resolveHolonRPCHTTPBaseURL(_ raw: String) throws -> HolonRPCResolvedURL {
    try resolveHolonRPCURL(raw, kind: .http)
}

internal func makeHolonRPCURLSession(
    timeout: TimeInterval,
    caFilePath: String?
) throws -> (session: URLSession, delegate: AnyObject?) {
    let configuration = URLSessionConfiguration.default
    configuration.timeoutIntervalForRequest = timeout
    configuration.timeoutIntervalForResource = max(timeout * 2, timeout)

    guard let caFilePath else {
        return (URLSession(configuration: configuration), nil)
    }

    #if canImport(Security)
    let delegate = try HolonRPCURLSessionDelegate(caFilePath: caFilePath)
    let session = URLSession(
        configuration: configuration,
        delegate: delegate,
        delegateQueue: nil
    )
    return (session, delegate)
    #else
    throw HolonRPCClientError.protocolError(
        "custom CA certificates are unsupported on this platform"
    )
    #endif
}

private func resolveHolonRPCURL(_ raw: String, kind: HolonRPCURLKind) throws -> HolonRPCResolvedURL {
    let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else {
        throw HolonRPCClientError.invalidURL(raw)
    }

    let normalizedRaw: String
    switch kind {
    case .webSocket:
        normalizedRaw = trimmed
    case .http:
        if trimmed.hasPrefix("rest+sse://") {
            normalizedRaw = "http://" + trimmed.dropFirst("rest+sse://".count)
        } else {
            normalizedRaw = trimmed
        }
    }

    guard var components = URLComponents(string: normalizedRaw) else {
        throw HolonRPCClientError.invalidURL(raw)
    }

    let scheme = (components.scheme ?? "").lowercased()
    switch kind {
    case .webSocket:
        guard scheme == "ws" || scheme == "wss" else {
            throw HolonRPCClientError.invalidURL(raw)
        }
        if components.path.isEmpty || components.path == "/" {
            components.path = defaultHolonRPCWebSocketPath
        }
    case .http:
        guard scheme == "http" || scheme == "https" else {
            throw HolonRPCClientError.invalidURL(raw)
        }
        if components.path.isEmpty || components.path == "/" {
            components.path = defaultHolonRPCHTTPPath
        }
        while components.path.count > 1 && components.path.hasSuffix("/") {
            components.path.removeLast()
        }
    }

    let queryItems = components.queryItems ?? []
    let caFilePath = queryItems.first(where: { $0.name == "ca" })?.value
    let retainedQueryItems = queryItems.filter { $0.name != "ca" }
    components.queryItems = retainedQueryItems.isEmpty ? nil : retainedQueryItems

    guard let url = components.url else {
        throw HolonRPCClientError.invalidURL(raw)
    }

    return HolonRPCResolvedURL(url: url, caFilePath: caFilePath)
}

internal func decodeHolonRPCResponse(statusCode: Int, body: Data) throws -> [String: Any] {
    if !body.isEmpty,
       let payload = try? JSONSerialization.jsonObject(with: body) as? [String: Any] {
        if let errorPayload = payload["error"] as? [String: Any] {
            throw holonRPCResponseError(from: errorPayload)
        }
        if payload.keys.contains("result") {
            return normalizeHolonRPCResult(payload["result"])
        }
        if statusCode < 400 {
            return payload
        }
    }

    if statusCode >= 400 {
        throw HolonRPCClientError.protocolError("http status \(statusCode)")
    }

    return [:]
}

internal func normalizeHolonRPCResult(_ raw: Any?) -> [String: Any] {
    guard let raw else {
        return [:]
    }
    if raw is NSNull {
        return [:]
    }
    if let object = raw as? [String: Any] {
        return object
    }
    return ["value": raw]
}

private func holonRPCResponseError(from payload: [String: Any]) -> HolonRPCResponseError {
    HolonRPCResponseError(
        code: payload["code"] as? Int ?? 13,
        message: payload["message"] as? String ?? "internal error",
        data: payload["data"]
    )
}

public struct HolonRPCSSEEvent {
    public let event: String
    public let id: String
    public let result: [String: Any]
    public let error: HolonRPCResponseError?

    public init(
        event: String,
        id: String,
        result: [String: Any] = [:],
        error: HolonRPCResponseError? = nil
    ) {
        self.event = event
        self.id = id
        self.result = result
        self.error = error
    }
}

internal func readHolonRPCSSEEvents(statusCode: Int, body: Data) throws -> [HolonRPCSSEEvent] {
    if statusCode >= 400 {
        _ = try decodeHolonRPCResponse(statusCode: statusCode, body: body)
        return []
    }

    guard let rawBody = String(data: body, encoding: .utf8) else {
        throw HolonRPCClientError.serialization("invalid UTF-8 payload")
    }

    let lines = rawBody
        .replacingOccurrences(of: "\r\n", with: "\n")
        .replacingOccurrences(of: "\r", with: "\n")
        .split(separator: "\n", omittingEmptySubsequences: false)

    var events: [HolonRPCSSEEvent] = []
    var currentEvent = ""
    var currentID = ""
    var currentData: [String] = []

    func trimSSEFieldValue(_ line: Substring, prefixLength: Int) -> String {
        var value = String(line.dropFirst(prefixLength))
        if value.first == " " {
            value.removeFirst()
        }
        return value
    }

    func flushCurrentEvent() throws -> Bool {
        if currentEvent.isEmpty && currentID.isEmpty && currentData.isEmpty {
            return false
        }

        defer {
            currentEvent = ""
            currentID = ""
            currentData.removeAll(keepingCapacity: true)
        }

        if currentEvent == "done" {
            events.append(HolonRPCSSEEvent(event: "done", id: currentID))
            return true
        }

        let payloadText = currentData.joined(separator: "\n")
        var event = HolonRPCSSEEvent(event: currentEvent, id: currentID)

        if (currentEvent == "message" || currentEvent == "error"),
           let payloadData = payloadText.data(using: .utf8),
           !payloadData.isEmpty {
            guard let payload = try JSONSerialization.jsonObject(with: payloadData) as? [String: Any] else {
                throw HolonRPCClientError.serialization("invalid JSON payload")
            }
            if let errorPayload = payload["error"] as? [String: Any] {
                event = HolonRPCSSEEvent(
                    event: currentEvent,
                    id: currentID,
                    error: holonRPCResponseError(from: errorPayload)
                )
            } else {
                event = HolonRPCSSEEvent(
                    event: currentEvent,
                    id: currentID,
                    result: normalizeHolonRPCResult(payload["result"])
                )
            }
        }

        events.append(event)
        return false
    }

    for line in lines {
        if line.isEmpty {
            if try flushCurrentEvent() {
                return events
            }
            continue
        }

        if line.hasPrefix("event:") {
            currentEvent = trimSSEFieldValue(line, prefixLength: "event:".count)
            continue
        }
        if line.hasPrefix("id:") {
            currentID = trimSSEFieldValue(line, prefixLength: "id:".count)
            continue
        }
        if line.hasPrefix("data:") {
            currentData.append(trimSSEFieldValue(line, prefixLength: "data:".count))
        }
    }

    _ = try flushCurrentEvent()
    return events
}

public final class HolonRPCHTTPClient {
    public typealias Params = [String: Any]

    private let normalizedBaseURL: String
    private let session: URLSession
    private let sessionDelegate: AnyObject?
    private let timeout: TimeInterval

    public init(_ baseURL: String, timeout: TimeInterval = 10.0) throws {
        let resolved = try resolveHolonRPCHTTPBaseURL(baseURL)
        let built = try makeHolonRPCURLSession(timeout: timeout, caFilePath: resolved.caFilePath)
        self.normalizedBaseURL = resolved.url.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        self.session = built.session
        self.sessionDelegate = built.delegate
        self.timeout = timeout
    }

    public convenience init(baseURL: String, timeout: TimeInterval = 10.0) throws {
        try self.init(baseURL, timeout: timeout)
    }

    deinit {
        session.invalidateAndCancel()
    }

    public func close() {
        session.invalidateAndCancel()
    }

    public func invoke(method: String, params: Params = [:]) async throws -> Params {
        var request = try URLRequest(url: methodURL(method))
        request.httpMethod = "POST"
        request.timeoutInterval = timeout
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.httpBody = try JSONSerialization.data(withJSONObject: params, options: [])

        let (body, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw HolonRPCClientError.protocolError("expected HTTP response")
        }
        return try decodeHolonRPCResponse(statusCode: httpResponse.statusCode, body: body)
    }

    public func stream(method: String, params: Params = [:]) async throws -> [HolonRPCSSEEvent] {
        var request = try URLRequest(url: methodURL(method))
        request.httpMethod = "POST"
        request.timeoutInterval = timeout
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        request.httpBody = try JSONSerialization.data(withJSONObject: params, options: [])

        let (body, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw HolonRPCClientError.protocolError("expected HTTP response")
        }
        return try readHolonRPCSSEEvents(statusCode: httpResponse.statusCode, body: body)
    }

    public func streamQuery(method: String, params: [String: String]) async throws -> [HolonRPCSSEEvent] {
        var components = URLComponents(url: try methodURL(method), resolvingAgainstBaseURL: false)
        components?.queryItems = params.map { URLQueryItem(name: $0.key, value: $0.value) }
        guard let url = components?.url else {
            throw HolonRPCClientError.invalidURL(method)
        }

        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        request.timeoutInterval = timeout
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")

        let (body, response) = try await session.data(for: request)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw HolonRPCClientError.protocolError("expected HTTP response")
        }
        return try readHolonRPCSSEEvents(statusCode: httpResponse.statusCode, body: body)
    }

    private func methodURL(_ method: String) throws -> URL {
        let trimmedMethod = method
            .trimmingCharacters(in: .whitespacesAndNewlines)
            .trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        guard !trimmedMethod.isEmpty else {
            throw HolonRPCClientError.protocolError("method is required")
        }

        guard let url = URL(string: normalizedBaseURL + "/" + trimmedMethod) else {
            throw HolonRPCClientError.invalidURL(method)
        }
        return url
    }
}

#if canImport(Security)
private final class HolonRPCURLSessionDelegate: NSObject, URLSessionDelegate {
    private let anchorCertificate: SecCertificate

    init(caFilePath: String) throws {
        self.anchorCertificate = try Self.loadAnchorCertificate(from: caFilePath)
    }

    func urlSession(
        _ session: URLSession,
        didReceive challenge: URLAuthenticationChallenge,
        completionHandler: @escaping (URLSession.AuthChallengeDisposition, URLCredential?) -> Void
    ) {
        guard challenge.protectionSpace.authenticationMethod == NSURLAuthenticationMethodServerTrust,
              let trust = challenge.protectionSpace.serverTrust else {
            completionHandler(.performDefaultHandling, nil)
            return
        }

        let host = challenge.protectionSpace.host
        let policy = SecPolicyCreateSSL(true, host.isEmpty ? nil : host as CFString)

        guard SecTrustSetPolicies(trust, policy) == errSecSuccess,
              SecTrustSetAnchorCertificates(trust, [anchorCertificate] as CFArray) == errSecSuccess,
              SecTrustSetAnchorCertificatesOnly(trust, true) == errSecSuccess else {
            completionHandler(.cancelAuthenticationChallenge, nil)
            return
        }

        var error: CFError?
        if SecTrustEvaluateWithError(trust, &error) {
            completionHandler(.useCredential, URLCredential(trust: trust))
            return
        }

        completionHandler(.cancelAuthenticationChallenge, nil)
    }

    private static func loadAnchorCertificate(from path: String) throws -> SecCertificate {
        let rawData = try Data(contentsOf: URL(fileURLWithPath: path))
        let certificateData = try normalizedCertificateData(from: rawData)
        guard let certificate = SecCertificateCreateWithData(nil, certificateData as CFData) else {
            throw HolonRPCClientError.protocolError("unable to load CA certificate at \(path)")
        }
        return certificate
    }

    private static func normalizedCertificateData(from rawData: Data) throws -> Data {
        guard let text = String(data: rawData, encoding: .utf8),
              text.contains("BEGIN CERTIFICATE") else {
            return rawData
        }

        let beginMarker = "-----BEGIN CERTIFICATE-----"
        let endMarker = "-----END CERTIFICATE-----"
        guard let beginRange = text.range(of: beginMarker),
              let endRange = text.range(of: endMarker) else {
            throw HolonRPCClientError.protocolError("invalid PEM certificate")
        }

        let base64Range = beginRange.upperBound..<endRange.lowerBound
        let base64Text = text[base64Range]
            .components(separatedBy: .whitespacesAndNewlines)
            .joined()

        guard let decoded = Data(base64Encoded: base64Text) else {
            throw HolonRPCClientError.protocolError("invalid PEM certificate")
        }
        return decoded
    }
}
#endif
