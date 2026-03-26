import Foundation
#if os(Linux)
import Glibc
#else
import Darwin
#endif

public enum TransportError: Error, CustomStringConvertible {
    case unsupportedURI(String)
    case invalidURI(String)
    case runtimeUnsupported(uri: String, reason: String)
    case listenFailed(String)
    case acceptFailed(String)
    case ioFailure(String)
    case listenerClosed(String)

    public var description: String {
        switch self {
        case let .unsupportedURI(uri):
            return "unsupported transport URI: \(uri)"
        case let .invalidURI(uri):
            return "invalid transport URI: \(uri)"
        case let .runtimeUnsupported(uri, reason):
            return "unsupported runtime transport for \(uri): \(reason)"
        case let .listenFailed(message):
            return "listen failed: \(message)"
        case let .acceptFailed(message):
            return "accept failed: \(message)"
        case let .ioFailure(message):
            return "I/O failure: \(message)"
        case let .listenerClosed(message):
            return "listener closed: \(message)"
        }
    }
}

public enum TransportScheme: String, CaseIterable {
    case tcp
    case unix
    case stdio
    case ws
    case wss

    public static func from(_ uri: String) -> String {
        guard let idx = uri.range(of: "://") else {
            return uri
        }
        return String(uri[..<idx.lowerBound])
    }
}

public struct TransportURI: Equatable {
    public let raw: String
    public let scheme: String
    public let host: String?
    public let port: Int?
    public let path: String?

    public init(raw: String, scheme: String, host: String? = nil, port: Int? = nil, path: String? = nil) {
        self.raw = raw
        self.scheme = scheme
        self.host = host
        self.port = port
        self.path = path
    }
}

public enum Listener: Equatable {
    case tcp(host: String, port: Int)
    case unix(path: String)
    case stdio
    case ws(host: String, port: Int, path: String, secure: Bool)
}

public protocol RuntimeConnection: AnyObject {
    func read(maxBytes: Int) throws -> Data
    func write(_ data: Data) throws
    func close() throws
}

public protocol RuntimeTransportListener: AnyObject {
    var boundURI: String { get }
    func accept() throws -> RuntimeConnection
    func close() throws
}

public enum RuntimeListener {
    case tcp(TCPRuntimeListener)
    case unix(UnixRuntimeListener)
    case stdio(StdioRuntimeListener)

    public var boundURI: String {
        switch self {
        case let .tcp(listener):
            return listener.boundURI
        case let .unix(listener):
            return listener.boundURI
        case let .stdio(listener):
            return listener.boundURI
        }
    }

    public func accept() throws -> RuntimeConnection {
        switch self {
        case let .tcp(listener):
            return try listener.accept()
        case let .unix(listener):
            return try listener.accept()
        case let .stdio(listener):
            return try listener.accept()
        }
    }

    public func close() throws {
        switch self {
        case let .tcp(listener):
            try listener.close()
        case let .unix(listener):
            try listener.close()
        case let .stdio(listener):
            try listener.close()
        }
    }
}

public final class POSIXRuntimeConnection: RuntimeConnection {
    private let readFD: Int32
    private let writeFD: Int32
    private let ownsReadFD: Bool
    private let ownsWriteFD: Bool
    private let stateLock = NSLock()
    private var closed = false

    init(readFD: Int32, writeFD: Int32, ownsReadFD: Bool, ownsWriteFD: Bool) {
        self.readFD = readFD
        self.writeFD = writeFD
        self.ownsReadFD = ownsReadFD
        self.ownsWriteFD = ownsWriteFD
    }

    deinit {
        try? close()
    }

    public func read(maxBytes: Int) throws -> Data {
        guard maxBytes > 0 else {
            return Data()
        }

        stateLock.lock()
        let isClosed = closed
        stateLock.unlock()

        if isClosed {
            throw TransportError.listenerClosed("connection already closed")
        }

        var buffer = [UInt8](repeating: 0, count: maxBytes)
        let readCount = buffer.withUnsafeMutableBytes { ptr in
            sysRead(readFD, ptr.baseAddress, ptr.count)
        }
        if readCount < 0 {
            throw TransportError.ioFailure(sysErrnoMessage())
        }
        if readCount == 0 {
            return Data()
        }

        return Data(buffer.prefix(Int(readCount)))
    }

    public func write(_ data: Data) throws {
        if data.isEmpty {
            return
        }

        stateLock.lock()
        let isClosed = closed
        stateLock.unlock()

        if isClosed {
            throw TransportError.listenerClosed("connection already closed")
        }

        try data.withUnsafeBytes { ptr in
            guard let base = ptr.baseAddress else {
                return
            }
            var sent = 0
            while sent < ptr.count {
                let written = sysWrite(
                    writeFD,
                    base.advanced(by: sent),
                    ptr.count - sent
                )
                if written < 0 {
                    throw TransportError.ioFailure(sysErrnoMessage())
                }
                if written == 0 {
                    throw TransportError.ioFailure("zero-byte write")
                }
                sent += Int(written)
            }
        }
    }

    public func close() throws {
        stateLock.lock()
        if closed {
            stateLock.unlock()
            return
        }
        closed = true
        stateLock.unlock()

        if ownsReadFD {
            _ = sysClose(readFD)
        }
        if ownsWriteFD, writeFD != readFD {
            _ = sysClose(writeFD)
        }
    }
}

public final class TCPRuntimeListener: RuntimeTransportListener {
    private let stateLock = NSLock()
    private var fd: Int32
    private var isClosed = false

    public let host: String
    public let port: Int
    public let boundHost: String
    public let boundPort: Int

    public var boundURI: String {
        "tcp://\(formatHostForURI(boundHost)):\(boundPort)"
    }

    public init(host: String, port: Int) throws {
        let bindResult = try Transport.bindTCP(host: host, port: port)
        self.fd = bindResult.fd
        self.host = host
        self.port = port
        self.boundHost = bindResult.boundHost
        self.boundPort = bindResult.boundPort
    }

    deinit {
        try? close()
    }

    public func accept() throws -> RuntimeConnection {
        while true {
            stateLock.lock()
            let listenerFD = fd
            let closed = isClosed
            stateLock.unlock()

            if closed {
                throw TransportError.listenerClosed(boundURI)
            }

            let acceptedFD = sysAccept(listenerFD)
            if acceptedFD >= 0 {
                return POSIXRuntimeConnection(
                    readFD: acceptedFD,
                    writeFD: acceptedFD,
                    ownsReadFD: true,
                    ownsWriteFD: true
                )
            }

            if errno == EINTR {
                continue
            }

            stateLock.lock()
            let closedAfterError = isClosed
            stateLock.unlock()
            if closedAfterError {
                throw TransportError.listenerClosed(boundURI)
            }
            throw TransportError.acceptFailed(sysErrnoMessage())
        }
    }

    public func close() throws {
        stateLock.lock()
        if isClosed {
            stateLock.unlock()
            return
        }
        isClosed = true
        let listenerFD = fd
        fd = -1
        stateLock.unlock()

        if listenerFD >= 0 {
            _ = sysClose(listenerFD)
        }
    }
}

public final class UnixRuntimeListener: RuntimeTransportListener {
    private let stateLock = NSLock()
    private var fd: Int32
    private var isClosed = false

    public let path: String
    public var boundURI: String {
        "unix://\(path)"
    }

    public init(path: String) throws {
        self.path = path
        self.fd = try Transport.bindUnix(path: path)
    }

    deinit {
        try? close()
    }

    public func accept() throws -> RuntimeConnection {
        while true {
            stateLock.lock()
            let listenerFD = fd
            let closed = isClosed
            stateLock.unlock()

            if closed {
                throw TransportError.listenerClosed(boundURI)
            }

            let acceptedFD = sysAccept(listenerFD)
            if acceptedFD >= 0 {
                return POSIXRuntimeConnection(
                    readFD: acceptedFD,
                    writeFD: acceptedFD,
                    ownsReadFD: true,
                    ownsWriteFD: true
                )
            }

            if errno == EINTR {
                continue
            }

            stateLock.lock()
            let closedAfterError = isClosed
            stateLock.unlock()
            if closedAfterError {
                throw TransportError.listenerClosed(boundURI)
            }
            throw TransportError.acceptFailed(sysErrnoMessage())
        }
    }

    public func close() throws {
        stateLock.lock()
        if isClosed {
            stateLock.unlock()
            return
        }
        isClosed = true
        let listenerFD = fd
        fd = -1
        stateLock.unlock()

        if listenerFD >= 0 {
            _ = sysClose(listenerFD)
        }
        _ = sysUnlink(path)
    }
}

public final class StdioRuntimeListener: RuntimeTransportListener {
    private let stateLock = NSLock()
    private var isClosed = false
    private var consumed = false

    public var boundURI: String {
        "stdio://"
    }

    public init() {}

    public func accept() throws -> RuntimeConnection {
        stateLock.lock()
        defer {
            stateLock.unlock()
        }

        if isClosed {
            throw TransportError.listenerClosed(boundURI)
        }
        if consumed {
            throw TransportError.acceptFailed("stdio:// accepts exactly one connection")
        }

        consumed = true
        return POSIXRuntimeConnection(
            readFD: STDIN_FILENO,
            writeFD: STDOUT_FILENO,
            ownsReadFD: false,
            ownsWriteFD: false
        )
    }

    public func close() throws {
        stateLock.lock()
        isClosed = true
        stateLock.unlock()
    }
}

public enum Transport {
    public static let defaultURI = "tcp://:9090"

    public static func scheme(_ uri: String) -> String {
        TransportScheme.from(uri)
    }

    public static func parse(_ uri: String) throws -> TransportURI {
        let s = scheme(uri)

        switch s {
        case "tcp":
            guard uri.hasPrefix("tcp://") else {
                throw TransportError.invalidURI(uri)
            }
            let value = String(uri.dropFirst("tcp://".count))
            let split = splitHostPort(value, defaultPort: 9090)
            return TransportURI(raw: uri, scheme: s, host: split.host, port: split.port)
        case "unix":
            guard uri.hasPrefix("unix://") else {
                throw TransportError.invalidURI(uri)
            }
            let path = String(uri.dropFirst("unix://".count))
            guard !path.isEmpty else { throw TransportError.invalidURI(uri) }
            return TransportURI(raw: uri, scheme: s, path: path)
        case "stdio":
            return TransportURI(raw: "stdio://", scheme: "stdio")
        case "ws", "wss":
            let secure = s == "wss"
            let prefix = secure ? "wss://" : "ws://"
            guard uri.hasPrefix(prefix) else {
                throw TransportError.invalidURI(uri)
            }
            let trimmed = String(uri.dropFirst(secure ? "wss://".count : "ws://".count))
            let pieces = trimmed.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false)
            let addr = String(pieces.first ?? "")
            let path = pieces.count > 1 ? "/" + String(pieces[1]) : "/grpc"
            let split = splitHostPort(addr, defaultPort: secure ? 443 : 80)
            return TransportURI(raw: uri, scheme: s, host: split.host, port: split.port, path: path)
        default:
            throw TransportError.unsupportedURI(uri)
        }
    }

    public static func listen(_ uri: String) throws -> Listener {
        let parsed = try parse(uri)

        switch parsed.scheme {
        case "tcp":
            return .tcp(host: parsed.host ?? "0.0.0.0", port: parsed.port ?? 9090)
        case "unix":
            return .unix(path: parsed.path ?? "")
        case "stdio":
            return .stdio
        case "ws", "wss":
            return .ws(
                host: parsed.host ?? "0.0.0.0",
                port: parsed.port ?? (parsed.scheme == "wss" ? 443 : 80),
                path: parsed.path ?? "/grpc",
                secure: parsed.scheme == "wss"
            )
        default:
            throw TransportError.unsupportedURI(uri)
        }
    }

    public static func listenRuntime(_ uri: String) throws -> RuntimeListener {
        let parsed = try parse(uri)

        switch parsed.scheme {
        case "tcp":
            let host = parsed.host ?? "0.0.0.0"
            let port = parsed.port ?? 9090
            return .tcp(try TCPRuntimeListener(host: host, port: port))
        case "unix":
            guard let path = parsed.path, !path.isEmpty else {
                throw TransportError.invalidURI(uri)
            }
            return .unix(try UnixRuntimeListener(path: path))
        case "stdio":
            return .stdio(StdioRuntimeListener())
        case "ws", "wss":
            throw TransportError.runtimeUnsupported(
                uri: uri,
                reason: "grpc-swift does not provide an official WebSocket server transport for HTTP/2 gRPC framing"
            )
        default:
            throw TransportError.unsupportedURI(uri)
        }
    }

    private static func splitHostPort(_ value: String, defaultPort: Int) -> (host: String, port: Int) {
        if value.isEmpty {
            return ("0.0.0.0", defaultPort)
        }

        if value.hasPrefix("["),
           let endIdx = value.firstIndex(of: "]") {
            let hostStart = value.index(after: value.startIndex)
            let host = String(value[hostStart..<endIdx])
            let rest = value[value.index(after: endIdx)...]
            if rest.isEmpty {
                return (host.isEmpty ? "::" : host, defaultPort)
            }
            if rest.first == ":" {
                let portText = String(rest.dropFirst())
                let port = Int(portText) ?? defaultPort
                return (host.isEmpty ? "::" : host, port)
            }
            return (host.isEmpty ? "::" : host, defaultPort)
        }

        guard let idx = value.lastIndex(of: ":") else {
            return (value, defaultPort)
        }

        let host = String(value[..<idx])
        let portString = String(value[value.index(after: idx)...])
        let hostValue = host.isEmpty ? "0.0.0.0" : host
        let portValue = Int(portString) ?? defaultPort
        return (hostValue, portValue)
    }

    fileprivate static func bindTCP(host: String, port: Int) throws -> (fd: Int32, boundHost: String, boundPort: Int) {
        var hints = addrinfo(
            ai_flags: AI_PASSIVE,
            ai_family: AF_UNSPEC,
            ai_socktype: socketStreamType,
            ai_protocol: 0,
            ai_addrlen: 0,
            ai_canonname: nil,
            ai_addr: nil,
            ai_next: nil
        )

        let serviceCString = strdup(String(port))
        let nodeCString: UnsafeMutablePointer<CChar>? =
            (host == "0.0.0.0" || host == "::") ? nil : strdup(host)

        defer {
            if let serviceCString {
                free(serviceCString)
            }
            if let nodeCString {
                free(nodeCString)
            }
        }

        guard serviceCString != nil else {
            throw TransportError.listenFailed("unable to allocate service string")
        }

        var infos: UnsafeMutablePointer<addrinfo>?
        let gai = getaddrinfo(nodeCString, serviceCString, &hints, &infos)
        guard gai == 0 else {
            throw TransportError.listenFailed("getaddrinfo failed: \(sysGAIMessage(gai))")
        }
        defer {
            if let infos {
                freeaddrinfo(infos)
            }
        }

        var current = infos
        var lastError = "unable to bind TCP listener"

        while let infoPtr = current {
            let info = infoPtr.pointee
            let fd = sysSocket(info.ai_family, info.ai_socktype, info.ai_protocol)
            if fd < 0 {
                lastError = sysErrnoMessage()
                current = info.ai_next
                continue
            }

            var one: Int32 = 1
            _ = setsockopt(
                fd,
                SOL_SOCKET,
                SO_REUSEADDR,
                &one,
                socklen_t(MemoryLayout<Int32>.size)
            )

            if sysBind(fd, info.ai_addr, info.ai_addrlen) == 0, sysListen(fd, 16) == 0 {
                let bound = socketBoundAddress(
                    fd: fd,
                    fallbackHost: host,
                    fallbackPort: port
                )
                return (fd, bound.host, bound.port)
            }

            lastError = sysErrnoMessage()
            _ = sysClose(fd)
            current = info.ai_next
        }

        throw TransportError.listenFailed("tcp://\(host):\(port): \(lastError)")
    }

    fileprivate static func bindUnix(path: String) throws -> Int32 {
        if path.isEmpty {
            throw TransportError.invalidURI("unix://")
        }

        let fd = sysSocket(AF_UNIX, socketStreamType, 0)
        if fd < 0 {
            throw TransportError.listenFailed(sysErrnoMessage())
        }

        _ = sysUnlink(path)

        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        let maxPathLength = MemoryLayout.size(ofValue: addr.sun_path)
        if path.utf8.count >= maxPathLength {
            _ = sysClose(fd)
            throw TransportError.listenFailed("unix path too long")
        }

        _ = path.withCString { cString in
            withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
                ptr.withMemoryRebound(to: CChar.self, capacity: maxPathLength) { dest in
                    strncpy(dest, cString, maxPathLength - 1)
                }
            }
        }

        let bindResult: Int32 = withUnsafePointer(to: &addr) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
                sysBind(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        if bindResult != 0 {
            let error = sysErrnoMessage()
            _ = sysClose(fd)
            throw TransportError.listenFailed(error)
        }

        if sysListen(fd, 16) != 0 {
            let error = sysErrnoMessage()
            _ = sysClose(fd)
            throw TransportError.listenFailed(error)
        }

        return fd
    }

    fileprivate static func createPipe() throws -> (readFD: Int32, writeFD: Int32) {
        var fds: [Int32] = [0, 0]
        if sysPipe(&fds) != 0 {
            throw TransportError.listenFailed(sysErrnoMessage())
        }
        return (fds[0], fds[1])
    }

    private static func socketBoundAddress(
        fd: Int32,
        fallbackHost: String,
        fallbackPort: Int
    ) -> (host: String, port: Int) {
        var storage = sockaddr_storage()
        var length = socklen_t(MemoryLayout<sockaddr_storage>.size)
        let rc = withUnsafeMutablePointer(to: &storage) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
                getsockname(fd, sockaddrPtr, &length)
            }
        }
        if rc != 0 {
            return (fallbackHost, fallbackPort)
        }

        var hostBuffer = [CChar](repeating: 0, count: Int(NI_MAXHOST))
        var serviceBuffer = [CChar](repeating: 0, count: Int(NI_MAXSERV))
        let nameInfoRC = withUnsafePointer(to: &storage) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
                getnameinfo(
                    sockaddrPtr,
                    length,
                    &hostBuffer,
                    socklen_t(hostBuffer.count),
                    &serviceBuffer,
                    socklen_t(serviceBuffer.count),
                    NI_NUMERICHOST | NI_NUMERICSERV
                )
            }
        }

        if nameInfoRC != 0 {
            return (fallbackHost, fallbackPort)
        }

        let host = String(cString: hostBuffer)
        let port = Int(String(cString: serviceBuffer)) ?? fallbackPort
        return (host.isEmpty ? fallbackHost : host, port)
    }
}

var socketStreamType: Int32 {
    #if os(Linux)
    return Int32(SOCK_STREAM.rawValue)
    #else
    return SOCK_STREAM
    #endif
}

private func formatHostForURI(_ host: String) -> String {
    if host.contains(":"), !host.hasPrefix("[") {
        return "[\(host)]"
    }
    return host
}

private func sysErrnoMessage() -> String {
    String(cString: strerror(errno))
}

private func sysGAIMessage(_ code: Int32) -> String {
    String(cString: gai_strerror(code))
}

private func sysSocket(_ domain: Int32, _ type: Int32, _ proto: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.socket(domain, type, proto)
    #else
    return Darwin.socket(domain, type, proto)
    #endif
}

private func sysBind(_ fd: Int32, _ addr: UnsafePointer<sockaddr>?, _ len: socklen_t) -> Int32 {
    #if os(Linux)
    return Glibc.bind(fd, addr, len)
    #else
    return Darwin.bind(fd, addr, len)
    #endif
}

private func sysListen(_ fd: Int32, _ backlog: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.listen(fd, backlog)
    #else
    return Darwin.listen(fd, backlog)
    #endif
}

private func sysAccept(_ fd: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.accept(fd, nil, nil)
    #else
    return Darwin.accept(fd, nil, nil)
    #endif
}

private func sysClose(_ fd: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.close(fd)
    #else
    return Darwin.close(fd)
    #endif
}

private func sysRead(_ fd: Int32, _ buf: UnsafeMutableRawPointer?, _ count: Int) -> Int {
    #if os(Linux)
    return Glibc.read(fd, buf, count)
    #else
    return Darwin.read(fd, buf, count)
    #endif
}

private func sysWrite(_ fd: Int32, _ buf: UnsafeRawPointer?, _ count: Int) -> Int {
    #if os(Linux)
    return Glibc.write(fd, buf, count)
    #else
    return Darwin.write(fd, buf, count)
    #endif
}

private func sysPipe(_ fds: UnsafeMutablePointer<Int32>?) -> Int32 {
    #if os(Linux)
    return Glibc.pipe(fds)
    #else
    return Darwin.pipe(fds)
    #endif
}

private func sysUnlink(_ path: String) -> Int32 {
    #if os(Linux)
    return Glibc.unlink(path)
    #else
    return Darwin.unlink(path)
    #endif
}
