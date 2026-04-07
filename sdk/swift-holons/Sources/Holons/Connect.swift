import Dispatch
import Foundation
import GRPC
import Logging
import NIOCore
import NIOPosix
#if os(Linux)
import Glibc
#else
import Darwin
#endif

private protocol RelayHandle: AnyObject {
    var boundURI: String { get }
    var debugSummary: String { get }
    func close()
}

public struct ConnectOptions {
    public var timeout: TimeInterval
    public var transport: String
    public var lifecycle: String
    public var start: Bool
    public var portFile: String?

    public init(
        timeout: TimeInterval = 5.0,
        transport: String = "auto",
        lifecycle: String = "persistent",
        start: Bool = true,
        portFile: String? = nil
    ) {
        self.timeout = timeout
        self.transport = transport
        self.lifecycle = lifecycle
        self.start = start
        self.portFile = portFile
    }
}

public enum ConnectError: Error, CustomStringConvertible {
    case targetRequired
    case unsupportedTransport(String)
    case unsupportedDialTarget(String)
    case invalidDirectTarget(String)
    case holonNotFound(String)
    case holonNotRunning(String)
    case missingManifest(String)
    case missingBinary(String)
    case binaryNotFound(String)
    case packageNotRunnable(String)
    case startupFailed(String)
    case readinessFailed(String)
    case ioFailure(String)

    public var description: String {
        switch self {
        case .targetRequired:
            return "target is required"
        case let .unsupportedTransport(transport):
            return "unsupported transport \"\(transport)\""
        case let .unsupportedDialTarget(target):
            return "unsupported dial target \"\(target)\""
        case let .invalidDirectTarget(target):
            return "invalid direct target \"\(target)\""
        case let .holonNotFound(slug):
            return "holon \"\(slug)\" not found"
        case let .holonNotRunning(slug):
            return "holon \"\(slug)\" is not running"
        case let .missingManifest(slug):
            return "holon \"\(slug)\" has no manifest"
        case let .missingBinary(slug):
            return "holon \"\(slug)\" has no artifacts.binary"
        case let .binaryNotFound(slug):
            return "built binary not found for holon \"\(slug)\""
        case let .packageNotRunnable(message):
            return message
        case let .startupFailed(message):
            return message
        case let .readinessFailed(message):
            return message
        case let .ioFailure(message):
            return message
        }
    }
}

private struct RawBytesPayload: GRPCPayload {
    var data: Data

    init(data: Data = Data()) {
        self.data = data
    }

    init(serializedByteBuffer: inout ByteBuffer) throws {
        self.data = serializedByteBuffer.readData(length: serializedByteBuffer.readableBytes) ?? Data()
    }

    func serialize(into buffer: inout ByteBuffer) throws {
        buffer.writeBytes(self.data)
    }
}

private struct DialTarget {
    enum Kind {
        case hostPort(String, Int)
        case unix(String)
        case connectedSocket(Int32)
    }

    let kind: Kind
}

private struct ConnectionHandle {
    let group: MultiThreadedEventLoopGroup
    let process: Process?
    let relay: (any RelayHandle)?
    let ephemeral: Bool
}

struct LaunchTarget {
    let kind: String
    let executablePath: String
    let arguments: [String]
    let workingDirectory: String?
}

private final class LineQueue {
    private let lock = NSLock()
    private let semaphore = DispatchSemaphore(value: 0)
    private var lines: [String] = []

    func push(_ line: String) {
        lock.lock()
        lines.append(line)
        lock.unlock()
        semaphore.signal()
    }

    func pop(timeout: TimeInterval) -> String? {
        let result = semaphore.wait(timeout: .now() + timeout)
        guard result == .success else {
            return nil
        }

        lock.lock()
        defer { lock.unlock() }
        guard !lines.isEmpty else {
            return nil
        }
        return lines.removeFirst()
    }
}

private final class StringCollector {
    private let lock = NSLock()
    private var lines: [String] = []

    func append(_ line: String) {
        lock.lock()
        lines.append(line)
        lock.unlock()
    }

    var text: String {
        lock.lock()
        defer { lock.unlock() }
        return lines.joined(separator: "\n")
    }
}

private final class ConnectionDiagnostics: NSObject, ClientErrorDelegate, ConnectivityStateDelegate {
    private let collector = StringCollector()

    var text: String {
        collector.text
    }

    func didCatchError(_ error: Error, logger: Logger, file: StaticString, line: Int) {
        _ = logger
        collector.append("client error: \(error) @\(file):\(line)")
    }

    func connectivityStateDidChange(from oldState: ConnectivityState, to newState: ConnectivityState) {
        collector.append("connectivity: \(oldState) -> \(newState)")
    }

    func connectionStartedQuiescing() {
        collector.append("connectivity: quiescing")
    }
}

private final class SocketRelay: RelayHandle {
    private let stateLock = NSLock()
    private let listener: TCPRuntimeListener
    private let upstreamFD: Int32
    private var closed = false
    private var connection: RuntimeConnection?
    private var upstreamPreview = Data()
    private var connectionPreview = Data()

    var boundURI: String {
        listener.boundURI
    }

    var debugSummary: String {
        stateLock.lock()
        defer { stateLock.unlock() }
        return "upstream=\(hexPreview(upstreamPreview)) client=\(hexPreview(connectionPreview))"
    }

    init(upstreamFD: Int32) throws {
        self.listener = try TCPRuntimeListener(host: "127.0.0.1", port: 0)
        self.upstreamFD = upstreamFD

        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            self?.acceptAndRelay()
        }
    }

    func close() {
        stateLock.lock()
        if closed {
            stateLock.unlock()
            return
        }
        closed = true
        let connection = self.connection
        self.connection = nil
        stateLock.unlock()

        _ = sysClose(upstreamFD)
        try? connection?.close()
        try? listener.close()
    }

    private func acceptAndRelay() {
        let accepted: RuntimeConnection
        do {
            accepted = try listener.accept()
        } catch {
            close()
            return
        }

        stateLock.lock()
        if closed {
            stateLock.unlock()
            try? accepted.close()
            return
        }
        connection = accepted
        stateLock.unlock()

        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            self?.forwardUpstream(to: accepted)
        }
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            self?.forwardConnection(from: accepted)
        }
    }

    private func forwardUpstream(to connection: RuntimeConnection) {
        var buffer = [UInt8](repeating: 0, count: 16 * 1024)

        while true {
            let readCount = buffer.withUnsafeMutableBytes { ptr in
                sysRead(upstreamFD, ptr.baseAddress, ptr.count)
            }

            if readCount > 0 {
                do {
                    let chunk = Data(buffer.prefix(Int(readCount)))
                    appendPreview(&upstreamPreview, bytes: chunk)
                    try connection.write(chunk)
                } catch {
                    close()
                    return
                }
            } else if readCount == 0 {
                close()
                return
            } else if currentErrno() == EINTR {
                continue
            } else {
                close()
                return
            }
        }
    }

    private func forwardConnection(from connection: RuntimeConnection) {
        while true {
            do {
                let data = try connection.read(maxBytes: 16 * 1024)
                if data.isEmpty {
                    close()
                    return
                }

                appendPreview(&connectionPreview, bytes: data)
                try data.withUnsafeBytes { ptr in
                    guard let base = ptr.baseAddress else {
                        return
                    }
                    try writeAll(fd: upstreamFD, base: base, count: ptr.count)
                }
            } catch {
                close()
                return
            }
        }
    }

    private func appendPreview(_ preview: inout Data, bytes: Data) {
        stateLock.lock()
        defer { stateLock.unlock() }
        guard preview.count < 64 else {
            return
        }

        let remaining = 64 - preview.count
        preview.append(bytes.prefix(remaining))
    }

    private func hexPreview(_ data: Data) -> String {
        if data.isEmpty {
            return "-"
        }
        return data.map { String(format: "%02x", $0) }.joined()
    }
}

private let connectStateLock = NSLock()
private var connectHandles: [ObjectIdentifier: ConnectionHandle] = [:]

public func connect(
    scope: Int,
    expression: String,
    root: String?,
    specifiers: Int,
    timeout: Int
) async -> ConnectResult {
    let normalized = expression.trimmingCharacters(in: .whitespacesAndNewlines)
    if normalized.isEmpty {
        return ConnectResult(error: "expression is required")
    }

    let resolved = resolve(
        scope: scope,
        expression: normalized,
        root: root,
        specifiers: specifiers,
        timeout: timeout
    )

    if let error = resolved.error {
        return ConnectResult(origin: resolved.ref, error: error)
    }

    guard let ref = resolved.ref else {
        return ConnectResult(error: "holon \"\(normalized)\" not found")
    }

    return connectResolvedRef(ref, timeout: timeout)
}

public func connect(_ target: String) throws -> GRPCChannel {
    try connectInternal(target, options: ConnectOptions())
}

public func connect(_ target: String, options: ConnectOptions) throws -> GRPCChannel {
    try connectInternal(target, options: options)
}

public func disconnect(_ channel: GRPCChannel) throws {
    guard let connection = channel as? ClientConnection else {
        try waitForClose(channel.close())
        return
    }

    let key = ObjectIdentifier(connection)

    connectStateLock.lock()
    let handle = connectHandles.removeValue(forKey: key)
    connectStateLock.unlock()

    var firstError: Error?

    do {
        try waitForClose(connection.close())
    } catch {
        firstError = error
    }

    handle?.relay?.close()

    if let group = handle?.group {
        do {
            try group.syncShutdownGracefully()
        } catch {
            if firstError == nil {
                firstError = error
            }
        }
    }

    if let process = handle?.process {
        do {
            if handle?.ephemeral == true {
                try stopProcess(process)
            } else {
                reapProcess(process)
            }
        } catch {
            if firstError == nil {
                firstError = error
            }
        }
    }

    if let firstError {
        throw firstError
    }
}

public func disconnect(_ result: ConnectResult) {
    guard let channel = result.channel else {
        return
    }
    try? disconnect(channel)
}

private func connectResolvedRef(_ ref: HolonRef, timeout: Int) -> ConnectResult {
    let timeoutSeconds = uniformConnectTimeout(timeout)

    if isResolvedDirectTarget(ref.url) {
        do {
            let channel = try dialReady(
                target: try normalizeDialTarget(ref.url),
                timeout: timeoutSeconds,
                process: nil,
                ephemeral: false,
                stderr: nil
            )
            return ConnectResult(channel: channel, origin: ref)
        } catch {
            return ConnectResult(origin: ref, error: String(describing: error))
        }
    }

    let entry: HolonEntry
    do {
        entry = try holonEntry(from: ref)
    } catch {
        return ConnectResult(origin: ref, error: String(describing: error))
    }

    var errorsSeen: [String] = []
    for transport in launchTransportAttempts(for: ref, entry: entry) {
        do {
            let attempt = try connectResolvedEntry(
                ref: ref,
                entry: entry,
                transport: transport,
                timeout: timeoutSeconds
            )
            return attempt
        } catch {
            errorsSeen.append("\(transport)-error: \(error)")
        }
    }

    if errorsSeen.isEmpty {
        return ConnectResult(origin: ref, error: "target unreachable")
    }
    return ConnectResult(origin: ref, error: errorsSeen.joined(separator: "; "))
}

private func connectResolvedEntry(
    ref: HolonRef,
    entry: HolonEntry,
    transport: String,
    timeout: TimeInterval
) throws -> ConnectResult {
    let launchTarget = try resolveLaunchTarget(entry)
    switch transport {
    case "stdio":
        let channel = try connectStdioHolon(
            launchTarget: launchTarget,
            timeout: timeout
        )
        return ConnectResult(channel: channel, origin: ref)

    case "tcp":
        let started = try startTCPHolon(
            launchTarget: launchTarget,
            timeout: timeout
        )
        let channel = try dialReady(
            target: try normalizeDialTarget(started.uri),
            timeout: timeout,
            process: started.process,
            ephemeral: true,
            stderr: started.stderr
        )
        return ConnectResult(
            channel: channel,
            origin: HolonRef(url: started.uri, info: ref.info, error: ref.error)
        )

    case "unix":
        let slug = resolvedTransportSlug(ref: ref, entry: entry)
        let started = try startUnixHolon(
            launchTarget: launchTarget,
            slug: slug,
            portFile: normalizedPortFilePath(nil, slug: slug),
            timeout: timeout
        )
        let channel = try dialReady(
            target: try normalizeDialTarget(started.uri),
            timeout: timeout,
            process: started.process,
            ephemeral: true,
            stderr: started.stderr
        )
        return ConnectResult(
            channel: channel,
            origin: HolonRef(url: started.uri, info: ref.info, error: ref.error)
        )

    default:
        throw ConnectError.unsupportedTransport(transport)
    }
}

private func connectInternal(_ target: String, options: ConnectOptions) throws -> GRPCChannel {
    let trimmed = target.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else {
        throw ConnectError.targetRequired
    }

    let timeout = options.timeout > 0 ? options.timeout : 5.0

    if isDirectTarget(trimmed) {
        return try dialReady(
            target: try normalizeDialTarget(trimmed),
            timeout: timeout,
            process: nil,
            ephemeral: false,
            stderr: nil
        )
    }

    let transport = try normalizedTransport(options.transport)
    let lifecycle = try normalizedLifecycle(options.lifecycle)

    guard let entry = try findBySlug(trimmed) else {
        throw ConnectError.holonNotFound(trimmed)
    }

    let portFile = normalizedPortFilePath(options.portFile, slug: entry.slug)

    if transportSupportsPortFileReuse(transport),
       let reusable = try usablePortFile(portFile, timeout: timeout) {
        return try dialReady(
            target: try normalizeDialTarget(reusable),
            timeout: timeout,
            process: nil,
            ephemeral: false,
            stderr: nil
        )
    }

    var errorsSeen: [String] = []
    let attempts = transportAttempts(transport: transport, lifecycle: lifecycle)

    for currentTransport in attempts {
        do {
            return try connectViaTransport(
                target: trimmed,
                entry: entry,
                portFile: portFile,
                transport: currentTransport,
                options: options,
                timeout: timeout,
                lifecycle: lifecycle
            )
        } catch {
            errorsSeen.append("\(currentTransport)-error: \(error)")
        }
    }

    if errorsSeen.isEmpty {
        throw ConnectError.ioFailure("holon \"\(trimmed)\" is not reachable")
    }
    throw ConnectError.ioFailure("connect \"\(trimmed)\" failed: \(errorsSeen.joined(separator: "; "))")
}

private func connectViaTransport(
    target: String,
    entry: HolonEntry,
    portFile: String,
    transport: String,
    options: ConnectOptions,
    timeout: TimeInterval,
    lifecycle: String
) throws -> GRPCChannel {
    switch transport {
    case "stdio":
        if lifecycle != "ephemeral" {
            throw ConnectError.ioFailure("stdio transport only supports ephemeral connect()")
        }
        guard options.start else {
            throw ConnectError.holonNotRunning(target)
        }
        let launchTarget = try resolveLaunchTarget(entry)
        return try connectStdioHolon(
            launchTarget: launchTarget,
            timeout: timeout
        )

    case "tcp", "unix":
        guard options.start else {
            throw ConnectError.holonNotRunning(target)
        }

        let launchTarget = try resolveLaunchTarget(entry)
        let started: StartedHolon
        
        switch transport {
        case "tcp":
            started = try startTCPHolon(
                launchTarget: launchTarget,
                timeout: timeout
            )
        case "unix":
            started = try startUnixHolon(
                launchTarget: launchTarget,
                slug: entry.slug,
                portFile: portFile,
                timeout: timeout
            )
        default:
            throw ConnectError.unsupportedTransport(transport)
        }
        
        let ephemeral = lifecycle == "ephemeral"
        do {
            let channel = try dialReady(
                target: try normalizeDialTarget(started.uri),
                timeout: timeout,
                process: started.process,
                ephemeral: ephemeral,
                stderr: started.stderr
            )
            
            if !ephemeral {
                do {
                    try writePortFile(path: portFile, uri: started.uri)
                } catch {
                    try? disconnect(channel)
                    try? stopProcess(started.process)
                    throw error
                }
            }
            
            return channel
        } catch {
            throw error
        }

    default:
        throw ConnectError.unsupportedTransport(transport)
    }
}

private func normalizedLifecycle(_ value: String) throws -> String {
    let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    if normalized.isEmpty {
        return "persistent"
    }
    switch normalized {
    case "ephemeral", "persistent":
        return normalized
    default:
        throw ConnectError.ioFailure("unsupported lifecycle \"\(value)\"")
    }
}

private func normalizedTransport(_ transport: String) throws -> String {
    let normalized = transport.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    if normalized.isEmpty {
        return "auto"
    }

    switch normalized {
    case "auto", "stdio", "tcp", "unix":
        return normalized
    default:
        throw ConnectError.unsupportedTransport(transport)
    }
}

private func transportSupportsPortFileReuse(_ transport: String) -> Bool {
    switch transport {
    case "auto", "unix", "tcp":
        return true
    default:
        return false
    }
}

private func transportAttempts(transport: String, lifecycle: String) -> [String] {
    if transport != "auto" {
        return [transport]
    }
    
    var attempts: [String] = []
    if lifecycle == "ephemeral" {
        attempts.append("stdio")
    }
    #if !os(Windows)
    attempts.append("unix")
    #endif
    attempts.append("tcp")
    return attempts
}

private func dialReady(
    target: DialTarget,
    timeout: TimeInterval,
    process: Process?,
    relay: (any RelayHandle)? = nil,
    ephemeral: Bool,
    stderr: StringCollector?
) throws -> GRPCChannel {
    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
    let diagnostics = ConnectionDiagnostics()
    let connection: ClientConnection
    do {
        connection = try makeConnection(target: target, group: group, diagnostics: diagnostics)
    } catch {
        closeConnectedSocketIfNeeded(target)
        try? group.syncShutdownGracefully()
        throw error
    }

    do {
        try waitForReady(
            channel: connection,
            timeout: timeout,
            process: process,
            stderr: stderr,
            diagnostics: diagnostics,
            relay: relay
        )
    } catch {
        relay?.close()
        try? waitForClose(connection.close())
        try? group.syncShutdownGracefully()
        if let process {
            try? stopProcess(process)
        }
        throw error
    }

    let handle = ConnectionHandle(
        group: group,
        process: process,
        relay: relay,
        ephemeral: ephemeral
    )

    connectStateLock.lock()
    connectHandles[ObjectIdentifier(connection)] = handle
    connectStateLock.unlock()

    return connection
}

private func makeConnection(
    target: DialTarget,
    group: MultiThreadedEventLoopGroup,
    diagnostics: ConnectionDiagnostics? = nil
) throws -> ClientConnection {
    switch target.kind {
    case let .hostPort(host, port):
        return ClientConnection.insecure(group: group)
            .withErrorDelegate(diagnostics)
            .withConnectivityStateDelegate(diagnostics)
            .connect(host: host, port: port)
    case let .unix(path):
        var configuration = ClientConnection.Configuration.default(
            target: .unixDomainSocket(path),
            eventLoopGroup: group
        )
        configuration.connectionBackoff = nil
        configuration.errorDelegate = diagnostics
        configuration.connectivityStateDelegate = diagnostics
        return ClientConnection(configuration: configuration)
    case let .connectedSocket(socket):
        return ClientConnection.insecure(group: group)
            .withConnectionReestablishment(enabled: false)
            .withErrorDelegate(diagnostics)
            .withConnectivityStateDelegate(diagnostics)
            .withConnectedSocket(socket)
    }
}

private func waitForReady(
    channel: ClientConnection,
    timeout: TimeInterval,
    process: Process?,
    stderr: StringCollector?,
    diagnostics: ConnectionDiagnostics?,
    relay: (any RelayHandle)?
) throws {
    do {
        _ = try describe(channel: channel, timeout: timeout)
        return
    } catch {
        // UNIMPLEMENTED means the server is alive and processing RPCs,
        // it just doesn't register HolonMeta (e.g. running from a package
        // without proto files). Treat as ready.
        if isGRPCUnimplemented(error) {
            return
        }
        if let process, !process.isRunning {
            let stderrText = stderr?.text.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if !stderrText.isEmpty {
                throw ConnectError.startupFailed("holon exited before becoming ready: \(stderrText)")
            }
            throw ConnectError.startupFailed("holon exited before becoming ready")
        }
        let stderrText = stderr?.text.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let diagnosticsText = diagnostics?.text.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let relayText = relay?.debugSummary ?? ""
        if !stderrText.isEmpty {
            throw ConnectError.readinessFailed("timed out waiting for holon readiness: \(error) [stderr: \(stderrText)] [client: \(diagnosticsText)] [relay: \(relayText)]")
        }
        if !diagnosticsText.isEmpty {
            throw ConnectError.readinessFailed("timed out waiting for holon readiness: \(error) [client: \(diagnosticsText)] [relay: \(relayText)]")
        }
        if !relayText.isEmpty {
            throw ConnectError.readinessFailed("timed out waiting for holon readiness: \(error) [relay: \(relayText)]")
        }
        throw ConnectError.readinessFailed("timed out waiting for holon readiness: \(error)")
    }
}

func describeLaunchTarget(
    _ launchTarget: LaunchTarget,
    timeout: TimeInterval
) throws -> Holons_V1_DescribeResponse {
    do {
        let channel = try connectStdioHolon(
            launchTarget: launchTarget,
            timeout: timeout
        )
        defer {
            try? disconnect(channel)
        }
        return try describeResponse(channel: channel, timeout: timeout)
    } catch {
        let started = try startTCPHolon(
            launchTarget: launchTarget,
            timeout: timeout
        )
        let channel = try dialReady(
            target: try normalizeDialTarget(started.uri),
            timeout: timeout,
            process: started.process,
            ephemeral: true,
            stderr: started.stderr
        )
        defer {
            try? disconnect(channel)
        }
        return try describeResponse(channel: channel, timeout: timeout)
    }
}

func describeResponse(
    channel: GRPCChannel,
    timeout: TimeInterval
) throws -> Holons_V1_DescribeResponse {
    let payload = try describe(channel: channel, timeout: timeout)
    return try Holons_V1_DescribeResponse(serializedBytes: payload.data)
}

private func describe(channel: GRPCChannel, timeout: TimeInterval) throws -> RawBytesPayload {
    let call = channel.makeUnaryCall(
        path: "/holons.v1.HolonMeta/Describe",
        request: RawBytesPayload(),
        callOptions: CallOptions(
            timeLimit: .timeout(.nanoseconds(Int64(timeout * 1_000_000_000)))
        )
    ) as UnaryCall<RawBytesPayload, RawBytesPayload>
    return try call.response.wait()
}

private func isGRPCUnimplemented(_ error: Error) -> Bool {
    guard let status = error as? GRPCStatus else { return false }
    return status.code == .unimplemented
}

private struct StartedHolon {
    let uri: String
    let process: Process
    let stderr: StringCollector
}

private func startTCPHolon(
    launchTarget: LaunchTarget,
    timeout: TimeInterval
) throws -> StartedHolon {
    let process = makeLaunchProcess(launchTarget, listenURI: "tcp://127.0.0.1:0")

    let stdout = Pipe()
    let stderr = Pipe()
    let stderrCollector = StringCollector()
    let lineQueue = LineQueue()

    process.standardOutput = stdout
    process.standardError = stderr

    try process.run()

    startLineReader(handle: stdout.fileHandleForReading, queue: lineQueue, collector: nil)
    startLineReader(handle: stderr.fileHandleForReading, queue: lineQueue, collector: stderrCollector)

    let deadline = Date().addingTimeInterval(timeout)
    while Date() < deadline {
        if !process.isRunning {
            let stderrText = stderrCollector.text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !stderrText.isEmpty {
                throw ConnectError.startupFailed("holon exited before advertising an address: \(stderrText)")
            }
            throw ConnectError.startupFailed("holon exited before advertising an address")
        }

        if let line = lineQueue.pop(timeout: 0.05), let uri = firstURI(in: line) {
            return StartedHolon(uri: uri, process: process, stderr: stderrCollector)
        }
    }

    try? stopProcess(process)
    let stderrText = stderrCollector.text.trimmingCharacters(in: .whitespacesAndNewlines)
    if !stderrText.isEmpty {
        throw ConnectError.startupFailed("timed out waiting for holon startup: \(stderrText)")
    }
    throw ConnectError.startupFailed("timed out waiting for holon startup")
}

private func startUnixHolon(
    launchTarget: LaunchTarget,
    slug: String,
    portFile: String,
    timeout: TimeInterval
) throws -> StartedHolon {
    let socketURI = defaultUnixSocketURI(slug: slug, portFile: portFile)
    let socketPath = String(socketURI.dropFirst("unix://".count))
    if FileManager.default.fileExists(atPath: socketPath) {
        try? FileManager.default.removeItem(atPath: socketPath)
    }
    let process = makeLaunchProcess(launchTarget, listenURI: socketURI)

    let stdout = Pipe()
    let stderr = Pipe()
    let stderrCollector = StringCollector()

    process.standardOutput = stdout
    process.standardError = stderr

    try process.run()

    startLineReader(handle: stdout.fileHandleForReading, queue: nil, collector: nil)
    startLineReader(handle: stderr.fileHandleForReading, queue: nil, collector: stderrCollector)

    let deadline = Date().addingTimeInterval(timeout)
    while Date() < deadline {
        if FileManager.default.fileExists(atPath: socketPath) {
            return StartedHolon(uri: socketURI, process: process, stderr: stderrCollector)
        }

        if !process.isRunning {
            let stderrText = stderrCollector.text.trimmingCharacters(in: .whitespacesAndNewlines)
            if !stderrText.isEmpty {
                throw ConnectError.startupFailed("holon exited before binding unix socket: \(stderrText)")
            }
            throw ConnectError.startupFailed("holon exited before binding unix socket")
        }

        Thread.sleep(forTimeInterval: 0.02)
    }

    try? stopProcess(process)
    let stderrText = stderrCollector.text.trimmingCharacters(in: .whitespacesAndNewlines)
    if !stderrText.isEmpty {
        throw ConnectError.startupFailed("timed out waiting for unix holon startup: \(stderrText)")
    }
    throw ConnectError.startupFailed("timed out waiting for unix holon startup")
}

private func connectStdioHolon(
    launchTarget: LaunchTarget,
    timeout: TimeInterval
) throws -> GRPCChannel {
    let sockets = try makeSocketPair()
    let childInputFD = try duplicateDescriptor(sockets.child)
    let childOutputFD = try duplicateDescriptor(sockets.child)
    let childInput = FileHandle(fileDescriptor: childInputFD, closeOnDealloc: true)
    let childOutput = FileHandle(fileDescriptor: childOutputFD, closeOnDealloc: true)
    let stderrPipe = Pipe()
    let stderrCollector = StringCollector()
    let relay: SocketRelay
    do {
        relay = try SocketRelay(upstreamFD: sockets.client)
    } catch {
        try? childInput.close()
        try? childOutput.close()
        _ = sysClose(sockets.child)
        _ = sysClose(sockets.client)
        throw error
    }

    let process = makeLaunchProcess(launchTarget, listenURI: "stdio://")
    process.standardInput = childInput
    process.standardOutput = childOutput
    process.standardError = stderrPipe

    do {
        try process.run()
    } catch {
        relay.close()
        try? childInput.close()
        try? childOutput.close()
        _ = sysClose(sockets.child)
        throw error
    }

    startLineReader(handle: stderrPipe.fileHandleForReading, queue: nil, collector: stderrCollector)
    try? childInput.close()
    try? childOutput.close()
    _ = sysClose(sockets.child)
    do {
        return try dialReady(
            target: try normalizeDialTarget(relay.boundURI),
            timeout: timeout,
            process: process,
            relay: relay,
            ephemeral: true,
            stderr: stderrCollector
        )
    } catch {
        relay.close()
        try? stopProcess(process)
        throw error
    }
}

private func makeLaunchProcess(_ launchTarget: LaunchTarget, listenURI: String) -> Process {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: launchTarget.executablePath)
    process.arguments = launchTarget.arguments + ["serve", "--listen", listenURI]
    if let directoryURL = launchWorkingDirectoryURL(launchTarget.workingDirectory) {
        process.currentDirectoryURL = directoryURL
    }
    return process
}

func launchWorkingDirectoryURL(_ workingDirectory: String?) -> URL? {
    guard let workingDirectory, !workingDirectory.isEmpty else {
        return nil
    }

    let directoryURL = URL(fileURLWithPath: workingDirectory, isDirectory: true)
    if FileManager.default.isWritableFile(atPath: directoryURL.path) {
        return directoryURL
    }

    return URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true)
}

private func usablePortFile(_ path: String, timeout: TimeInterval) throws -> String? {
    guard let data = FileManager.default.contents(atPath: path),
          let rawTarget = String(data: data, encoding: .utf8)?
            .trimmingCharacters(in: .whitespacesAndNewlines) else {
        return nil
    }

    guard !rawTarget.isEmpty else {
        try? FileManager.default.removeItem(atPath: path)
        return nil
    }

    let probeTimeout = min(max(timeout / 4.0, 0.25), 1.0)
    do {
        let channel = try dialReady(
            target: try normalizeDialTarget(rawTarget),
            timeout: probeTimeout,
            process: nil,
            ephemeral: false,
            stderr: nil
        )
        try disconnect(channel)
        return rawTarget
    } catch {
        try? FileManager.default.removeItem(atPath: path)
        return nil
    }
}

func resolveLaunchTarget(_ entry: HolonEntry) throws -> LaunchTarget {
    if entry.sourceKind == "package" {
        return try resolvePackageLaunchTarget(entry)
    }
    return try resolveSourceLaunchTarget(entry)
}

private func resolvePackageLaunchTarget(_ entry: HolonEntry) throws -> LaunchTarget {
    var entrypoint = entry.entrypoint.trimmingCharacters(in: .whitespacesAndNewlines)
    if entrypoint.isEmpty {
        entrypoint = entry.manifest?.artifacts.binary.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }
    guard !entrypoint.isEmpty else {
        throw ConnectError.packageNotRunnable("holon \"\(entry.slug)\" package has no entrypoint")
    }

    let archDir = currentArchDirectory()
    let binaryName = (entrypoint as NSString).lastPathComponent
    let binaryCandidate = entry.dir
        .appendingPathComponent("bin", isDirectory: true)
        .appendingPathComponent(archDir, isDirectory: true)
        .appendingPathComponent(binaryName, isDirectory: false)

    if FileManager.default.isExecutableFile(atPath: binaryCandidate.path) {
        return LaunchTarget(
            kind: "package-bin",
            executablePath: binaryCandidate.path,
            arguments: [],
            workingDirectory: entry.dir.path
        )
    }

    let distCandidate = entry.dir
        .appendingPathComponent("dist", isDirectory: true)
        .appendingPathComponent(entrypoint, isDirectory: false)
        .standardizedFileURL
    if FileManager.default.fileExists(atPath: distCandidate.path) {
        let runner = entry.runner.isEmpty ? (entry.manifest?.build.runner ?? "") : entry.runner
        guard let interpreter = interpreterForRunner(runner) else {
            throw ConnectError.packageNotRunnable(
                "holon \"\(entry.slug)\" package dist is not runnable for runner \"\(runner)\""
            )
        }
        return LaunchTarget(
            kind: "package-dist",
            executablePath: interpreter,
            arguments: [distCandidate.path],
            workingDirectory: entry.dir.path
        )
    }

    let gitRoot = entry.dir.appendingPathComponent("git", isDirectory: true).standardizedFileURL
    if isDirectory(gitRoot) {
        let sourceEntry = try sourceEntryFromPackageGit(entry, gitRoot: gitRoot)
        return try resolveSourceLaunchTarget(sourceEntry)
    }

    throw ConnectError.packageNotRunnable(
        "holon \"\(entry.slug)\" package is not runnable for arch \"\(archDir)\": missing bin/\(archDir)/\(binaryName), dist/\(entrypoint), and git/"
    )
}

private func resolveSourceLaunchTarget(_ entry: HolonEntry) throws -> LaunchTarget {
    guard let manifest = entry.manifest else {
        throw ConnectError.missingManifest(entry.slug)
    }

    let binary = manifest.artifacts.binary.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !binary.isEmpty else {
        throw ConnectError.missingBinary(entry.slug)
    }

    if binary.hasPrefix("/"), FileManager.default.isExecutableFile(atPath: binary) {
        return LaunchTarget(
            kind: "path",
            executablePath: binary,
            arguments: [],
            workingDirectory: entry.dir.path
        )
    }

    let candidate = entry.dir
        .appendingPathComponent(".op", isDirectory: true)
        .appendingPathComponent("build", isDirectory: true)
        .appendingPathComponent("bin", isDirectory: true)
        .appendingPathComponent((binary as NSString).lastPathComponent, isDirectory: false)

    if FileManager.default.isExecutableFile(atPath: candidate.path) {
        return LaunchTarget(
            kind: "source-built",
            executablePath: candidate.path,
            arguments: [],
            workingDirectory: entry.dir.path
        )
    }

    let runner = manifest.build.runner.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    let mainPath = manifest.build.main.trimmingCharacters(in: .whitespacesAndNewlines)
    if (runner == "go" || runner == "go-module"), !mainPath.isEmpty {
        guard let goBinary = which("go") else {
            throw ConnectError.binaryNotFound(entry.slug)
        }
        return LaunchTarget(
            kind: "source-go-run",
            executablePath: goBinary,
            arguments: ["run", mainPath],
            workingDirectory: entry.dir.path
        )
    }

    if let resolved = which((binary as NSString).lastPathComponent) {
        return LaunchTarget(
            kind: "path",
            executablePath: resolved,
            arguments: [],
            workingDirectory: entry.dir.path
        )
    }

    throw ConnectError.binaryNotFound(entry.slug)
}

private func sourceEntryFromPackageGit(_ entry: HolonEntry, gitRoot: URL) throws -> HolonEntry {
    let discovered = try discover(root: gitRoot)
    var fallback: HolonEntry?

    for candidate in discovered where candidate.sourceKind == "source" {
        if fallback == nil {
            fallback = candidate
        }
        if !entry.uuid.isEmpty, candidate.uuid == entry.uuid {
            return candidate
        }
        if candidate.slug == entry.slug {
            return candidate
        }
    }

    if let fallback {
        return fallback
    }

    throw ConnectError.packageNotRunnable(
        "holon \"\(entry.slug)\" package git/ does not contain a runnable source holon"
    )
}

private func holonEntry(from ref: HolonRef) throws -> HolonEntry {
    let path = try localPath(from: ref.url)
    let url = URL(fileURLWithPath: path, isDirectory: false).standardizedFileURL

    var isDirectoryFlag: ObjCBool = false
    guard FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectoryFlag) else {
        throw ConnectError.ioFailure("local target \"\(ref.url)\" does not exist")
    }

    if isDirectoryFlag.boolValue {
        if url.lastPathComponent.hasSuffix(".holon") || hasHolonJSONAtPath(url) {
            guard let info = ref.info else {
                throw ConnectError.ioFailure("holon metadata unavailable")
            }
            return packageEntry(from: info, directory: url)
        }

        if let discovered = try discoverSourceEntry(at: url, ref: ref) {
            return discovered
        }

        guard let info = ref.info else {
            throw ConnectError.ioFailure("holon metadata unavailable")
        }
        return sourceEntry(from: info, directory: url)
    }

    guard let info = ref.info else {
        throw ConnectError.ioFailure("holon metadata unavailable")
    }
    return binaryEntry(from: info, path: url)
}

private func discoverSourceEntry(at directory: URL, ref: HolonRef) throws -> HolonEntry? {
    let discovered = try discover(root: directory)
    var fallback: HolonEntry?

    for candidate in discovered where candidate.sourceKind == "source" {
        if fallback == nil {
            fallback = candidate
        }
        if let uuid = ref.info?.uuid, !uuid.isEmpty, candidate.uuid == uuid {
            return candidate
        }
        if let slug = ref.info?.slug, !slug.isEmpty, candidate.slug == slug {
            return candidate
        }
        if candidate.dir.standardizedFileURL.path == directory.standardizedFileURL.path {
            return candidate
        }
    }

    return fallback
}

private func packageEntry(from info: HolonInfo, directory: URL) -> HolonEntry {
    var identity = HolonIdentity()
    identity.uuid = info.uuid
    identity.givenName = info.identity.givenName
    identity.familyName = info.identity.familyName
    identity.motto = info.identity.motto
    identity.aliases = info.identity.aliases
    identity.lang = info.lang
    identity.status = info.status

    var manifest = HolonManifest()
    manifest.kind = info.kind
    manifest.build.runner = info.runner
    manifest.artifacts.binary = info.entrypoint

    return HolonEntry(
        slug: info.slug,
        uuid: info.uuid,
        dir: directory.standardizedFileURL,
        relativePath: ".",
        origin: "resolve",
        identity: identity,
        manifest: manifest,
        sourceKind: "package",
        packageRoot: directory.standardizedFileURL,
        runner: info.runner,
        transport: info.transport,
        entrypoint: info.entrypoint,
        architectures: info.architectures,
        hasDist: info.hasDist,
        hasSource: info.hasSource
    )
}

private func sourceEntry(from info: HolonInfo, directory: URL) -> HolonEntry {
    var identity = HolonIdentity()
    identity.uuid = info.uuid
    identity.givenName = info.identity.givenName
    identity.familyName = info.identity.familyName
    identity.motto = info.identity.motto
    identity.aliases = info.identity.aliases
    identity.lang = info.lang
    identity.status = info.status

    var manifest = HolonManifest()
    manifest.kind = info.kind
    manifest.build.runner = info.runner
    manifest.artifacts.binary = info.entrypoint

    return HolonEntry(
        slug: info.slug,
        uuid: info.uuid,
        dir: directory.standardizedFileURL,
        relativePath: ".",
        origin: "resolve",
        identity: identity,
        manifest: manifest,
        sourceKind: "source",
        packageRoot: nil,
        runner: info.runner,
        transport: info.transport,
        entrypoint: info.entrypoint,
        architectures: info.architectures,
        hasDist: info.hasDist,
        hasSource: info.hasSource
    )
}

private func binaryEntry(from info: HolonInfo, path: URL) -> HolonEntry {
    var identity = HolonIdentity()
    identity.uuid = info.uuid
    identity.givenName = info.identity.givenName
    identity.familyName = info.identity.familyName
    identity.motto = info.identity.motto
    identity.aliases = info.identity.aliases
    identity.lang = info.lang
    identity.status = info.status

    var manifest = HolonManifest()
    manifest.kind = info.kind
    manifest.build.runner = info.runner
    manifest.artifacts.binary = path.path

    return HolonEntry(
        slug: info.slug,
        uuid: info.uuid,
        dir: path.deletingLastPathComponent().standardizedFileURL,
        relativePath: ".",
        origin: "resolve",
        identity: identity,
        manifest: manifest,
        sourceKind: "binary",
        packageRoot: nil,
        runner: info.runner,
        transport: info.transport,
        entrypoint: path.path,
        architectures: info.architectures,
        hasDist: info.hasDist,
        hasSource: info.hasSource
    )
}

private func hasHolonJSONAtPath(_ directory: URL) -> Bool {
    var isDirectoryFlag: ObjCBool = false
    let manifestURL = directory.appendingPathComponent(".holon.json", isDirectory: false)
    guard FileManager.default.fileExists(atPath: manifestURL.path, isDirectory: &isDirectoryFlag) else {
        return false
    }
    return !isDirectoryFlag.boolValue
}

private func localPath(from url: String) throws -> String {
    guard let parsed = URL(string: url), parsed.scheme?.lowercased() == "file" else {
        throw ConnectError.unsupportedDialTarget(url)
    }
    guard !parsed.path.isEmpty else {
        throw ConnectError.invalidDirectTarget(url)
    }
    return URL(fileURLWithPath: parsed.path).standardizedFileURL.path
}

private func launchTransportAttempts(for ref: HolonRef, entry: HolonEntry) -> [String] {
    var transports: [String] = []
    var seen = Set<String>()

    func add(_ transport: String) {
        let normalized = transport.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard ["stdio", "unix", "tcp"].contains(normalized) else {
            return
        }
        if seen.insert(normalized).inserted {
            transports.append(normalized)
        }
    }

    add(ref.info?.transport ?? entry.transport)
    add("stdio")
    #if !os(Windows)
    add("unix")
    #endif
    add("tcp")
    return transports
}

private func resolvedTransportSlug(ref: HolonRef, entry: HolonEntry) -> String {
    let slug = (ref.info?.slug ?? entry.slug).trimmingCharacters(in: .whitespacesAndNewlines)
    return slug.isEmpty ? "holon" : slug
}

private func interpreterForRunner(_ runner: String) -> String? {
    let normalized = runner.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    switch normalized {
    case "python":
        return which("python3")
    case "node", "typescript":
        return which("node")
    case "ruby":
        return which("ruby")
    default:
        return nil
    }
}

private func currentArchDirectory() -> String {
    let osName: String
    #if os(macOS)
    osName = "darwin"
    #elseif os(Linux)
    osName = "linux"
    #else
    osName = "unknown"
    #endif

    let archName: String
    #if arch(arm64)
    archName = "arm64"
    #elseif arch(x86_64)
    archName = "amd64"
    #else
    archName = "unknown"
    #endif

    return "\(osName)_\(archName)"
}

private func isDirectory(_ url: URL) -> Bool {
    var isDirectory: ObjCBool = false
    guard FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectory) else {
        return false
    }
    return isDirectory.boolValue
}

func normalizedPortFilePath(_ override: String?, slug: String) -> String {
    let trimmed = override?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    if !trimmed.isEmpty {
        return trimmed
    }

    if bundledHolonsRootURLForConnect() != nil,
       let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first {
        return caches
            .appendingPathComponent("holons")
            .appendingPathComponent("run")
            .appendingPathComponent("\(slug).port")
            .path
    }

    return URL(fileURLWithPath: FileManager.default.currentDirectoryPath, isDirectory: true)
        .appendingPathComponent(".op")
        .appendingPathComponent("run")
        .appendingPathComponent("\(slug).port")
        .path
}

private func bundledHolonsRootURLForConnect() -> URL? {
    guard let resourceURL = discoverBundleResourceURLProvider()?.standardizedFileURL else {
        return nil
    }
    let holonsRoot = resourceURL.appendingPathComponent("Holons", isDirectory: true)
    return isDirectory(holonsRoot) ? holonsRoot : nil
}

func defaultUnixSocketURI(slug: String, portFile: String) -> String {
    let hash = fnv1a64(Array(portFile.utf8))
    var tempDir = NSTemporaryDirectory()
    if tempDir.hasSuffix("/") {
        tempDir = String(tempDir.dropLast())
    }
    return "unix://\(tempDir)/h\(String(format: "%08llx", hash & 0xffffffff)).s"
}

private func fnv1a64(_ bytes: [UInt8]) -> UInt64 {
    var hash: UInt64 = 0xcbf29ce484222325
    for byte in bytes {
        hash ^= UInt64(byte)
        hash &*= 0x100000001b3
    }
    return hash
}

private func writePortFile(path: String, uri: String) throws {
    let fileURL = URL(fileURLWithPath: path)
    try FileManager.default.createDirectory(
        at: fileURL.deletingLastPathComponent(),
        withIntermediateDirectories: true
    )
    try (uri.trimmingCharacters(in: .whitespacesAndNewlines) + "\n")
        .write(to: fileURL, atomically: true, encoding: .utf8)
}

private func normalizeDialTarget(_ target: String) throws -> DialTarget {
    let trimmed = target.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else {
        throw ConnectError.invalidDirectTarget(target)
    }

    guard trimmed.contains("://") else {
        let hostPort = try parseHostPort(trimmed)
        return DialTarget(kind: .hostPort(hostPort.host, hostPort.port))
    }

    let parsed = try Transport.parse(trimmed)
    switch parsed.scheme {
    case "tcp":
        let host = normalizedLoopbackHost(parsed.host ?? "127.0.0.1")
        return DialTarget(kind: .hostPort(host, parsed.port ?? 9090))
    case "unix":
        guard let path = parsed.path, !path.isEmpty else {
            throw ConnectError.invalidDirectTarget(trimmed)
        }
        return DialTarget(kind: .unix(path))
    default:
        throw ConnectError.unsupportedDialTarget(trimmed)
    }
}

private func normalizedLoopbackHost(_ host: String) -> String {
    let trimmed = host.trimmingCharacters(in: .whitespacesAndNewlines)
    switch trimmed {
    case "", "0.0.0.0", "::", "[::]":
        return "127.0.0.1"
    default:
        return trimmed
    }
}

private func parseHostPort(_ target: String) throws -> (host: String, port: Int) {
    if target.hasPrefix("["),
       let end = target.firstIndex(of: "]"),
       target.index(after: end) < target.endIndex,
       target[target.index(after: end)] == ":" {
        let host = String(target[target.index(after: target.startIndex)..<end])
        let portText = String(target[target.index(end, offsetBy: 2)...])
        guard let port = Int(portText) else {
            throw ConnectError.invalidDirectTarget(target)
        }
        return (host, port)
    }

    guard let colon = target.lastIndex(of: ":") else {
        throw ConnectError.invalidDirectTarget(target)
    }

    let host = String(target[..<colon]).trimmingCharacters(in: .whitespacesAndNewlines)
    let portText = String(target[target.index(after: colon)...]).trimmingCharacters(in: .whitespacesAndNewlines)
    guard !host.isEmpty, let port = Int(portText) else {
        throw ConnectError.invalidDirectTarget(target)
    }
    return (host, port)
}

private func isDirectTarget(_ target: String) -> Bool {
    target.contains("://") || target.contains(":")
}

private func isResolvedDirectTarget(_ target: String) -> Bool {
    let trimmed = target.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.lowercased().hasPrefix("file://") {
        return false
    }
    return isDirectTarget(trimmed)
}

private func uniformConnectTimeout(_ timeout: Int) -> TimeInterval {
    timeout > 0 ? TimeInterval(timeout) / 1000.0 : 5.0
}

private func firstURI(in line: String) -> String? {
    for field in line.split(whereSeparator: \.isWhitespace) {
        let candidate = field.trimmingCharacters(in: CharacterSet(charactersIn: "\"'()[]{}.,"))
        if candidate.hasPrefix("tcp://") ||
            candidate.hasPrefix("unix://") ||
            candidate.hasPrefix("stdio://") ||
            candidate.hasPrefix("ws://") ||
            candidate.hasPrefix("wss://") {
            return candidate
        }
    }
    return nil
}

private func startLineReader(handle: FileHandle, queue: LineQueue?, collector: StringCollector?) {
    DispatchQueue.global(qos: .utility).async {
        var buffer = Data()

        while true {
            let chunk = handle.availableData
            if chunk.isEmpty {
                if !buffer.isEmpty, let line = String(data: buffer, encoding: .utf8) {
                    collector?.append(line)
                    queue?.push(line)
                }
                return
            }

            buffer.append(chunk)
            while let newline = buffer.firstIndex(of: 0x0A) {
                let lineData = buffer.prefix(upTo: newline)
                buffer.removeSubrange(...newline)
                guard let line = String(data: lineData, encoding: .utf8) else {
                    continue
                }
                collector?.append(line)
                queue?.push(line)
            }
        }
    }
}

private func stopProcess(_ process: Process) throws {
    guard process.isRunning else {
        return
    }

    let pid = process.processIdentifier
    if pid > 0 {
        if sysKill(pid, SIGTERM) != 0 && currentErrno() != ESRCH {
            throw ConnectError.ioFailure("failed to send SIGTERM: \(sysErrnoMessage())")
        }
    }

    let termDeadline = Date().addingTimeInterval(2.0)
    while process.isRunning && Date() < termDeadline {
        Thread.sleep(forTimeInterval: 0.05)
    }

    if process.isRunning, pid > 0 {
        if sysKill(pid, SIGKILL) != 0 && currentErrno() != ESRCH {
            throw ConnectError.ioFailure("failed to send SIGKILL: \(sysErrnoMessage())")
        }
    }

    let killDeadline = Date().addingTimeInterval(2.0)
    while process.isRunning && Date() < killDeadline {
        Thread.sleep(forTimeInterval: 0.05)
    }
}

private func reapProcess(_ process: Process) {
    DispatchQueue.global(qos: .background).async {
        process.waitUntilExit()
    }
}

private func waitForClose(_ future: EventLoopFuture<Void>) throws {
    try future.wait()
}

private func writeAll(fd: Int32, base: UnsafeRawPointer, count: Int) throws {
    var written = 0
    while written < count {
        let result = sysWrite(fd, base.advanced(by: written), count - written)
        if result > 0 {
            written += result
        } else if result < 0 && currentErrno() == EINTR {
            continue
        } else if result < 0 {
            throw ConnectError.ioFailure(sysErrnoMessage())
        } else {
            throw ConnectError.ioFailure("zero-byte write")
        }
    }
}

private func closeConnectedSocketIfNeeded(_ target: DialTarget) {
    guard case let .connectedSocket(socket) = target.kind else {
        return
    }
    _ = sysClose(socket)
}

private func makeSocketPair() throws -> (client: Int32, child: Int32) {
    var fds: [Int32] = [0, 0]
    let rc = fds.withUnsafeMutableBufferPointer { buffer in
        sysSocketPair(AF_UNIX, connectedSocketType, 0, buffer.baseAddress)
    }
    if rc != 0 {
        throw ConnectError.ioFailure("socketpair failed: \(sysErrnoMessage())")
    }
    return (client: fds[0], child: fds[1])
}

private func duplicateDescriptor(_ fd: Int32) throws -> Int32 {
    let duplicated = sysDup(fd)
    if duplicated < 0 {
        throw ConnectError.ioFailure("dup failed: \(sysErrnoMessage())")
    }
    return duplicated
}

private func which(_ executable: String) -> String? {
    let path = ProcessInfo.processInfo.environment["PATH"] ?? ""
    for directory in path.split(separator: ":") {
        let candidate = URL(fileURLWithPath: String(directory), isDirectory: true)
            .appendingPathComponent(executable)
            .path
        if FileManager.default.isExecutableFile(atPath: candidate) {
            return candidate
        }
    }
    return nil
}

private func currentErrno() -> Int32 {
    #if os(Linux)
    return errno
    #else
    return Darwin.errno
    #endif
}

private var connectedSocketType: Int32 {
    #if os(Linux)
    return Int32(SOCK_STREAM.rawValue)
    #else
    return SOCK_STREAM
    #endif
}

private func sysErrnoMessage() -> String {
    String(cString: strerror(currentErrno()))
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

private func sysClose(_ fd: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.close(fd)
    #else
    return Darwin.close(fd)
    #endif
}

private func sysDup(_ fd: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.dup(fd)
    #else
    return Darwin.dup(fd)
    #endif
}

private func sysFcntl(_ fd: Int32, _ command: Int32, _ value: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.fcntl(fd, command, value)
    #else
    return Darwin.fcntl(fd, command, value)
    #endif
}

private func sysSocketPair(_ domain: Int32, _ type: Int32, _ proto: Int32, _ fds: UnsafeMutablePointer<Int32>?) -> Int32 {
    #if os(Linux)
    return Glibc.socketpair(domain, type, proto, fds)
    #else
    return Darwin.socketpair(domain, type, proto, fds)
    #endif
}

private func sysKill(_ pid: Int32, _ signal: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.kill(pid, signal)
    #else
    return Darwin.kill(pid, signal)
    #endif
}
