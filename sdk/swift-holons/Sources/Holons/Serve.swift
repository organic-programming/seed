import Dispatch
import Foundation
import GRPC
import NIOPosix

#if os(Linux)
  import Glibc
#else
  import Darwin
#endif

public enum Serve {
  public struct ParsedFlags {
    public var listenURI: String
    public var reflect: Bool

    public init(listenURI: String, reflect: Bool) {
      self.listenURI = listenURI
      self.reflect = reflect
    }
  }

  public struct Options {
    public var reflect: Bool
    public var logger: (String) -> Void
    public var onListen: ((String) -> Void)?
    public var shutdownGracePeriodSeconds: TimeInterval
    public var protoDir: String?

    public init(
      reflect: Bool = false,
      logger: @escaping (String) -> Void = { message in
        guard let data = (message + "\n").data(using: .utf8) else {
          return
        }
        try? FileHandle.standardError.write(contentsOf: data)
      },
      onListen: ((String) -> Void)? = nil,
      shutdownGracePeriodSeconds: TimeInterval = 10,
      protoDir: String? = nil
    ) {
      self.reflect = reflect
      self.logger = logger
      self.onListen = onListen
      self.shutdownGracePeriodSeconds = shutdownGracePeriodSeconds
      self.protoDir = protoDir
    }
  }

  public final class RunningServer {
    fileprivate let server: Server
    fileprivate let group: MultiThreadedEventLoopGroup
    private let logger: (String) -> Void
    private let defaultGracePeriodSeconds: TimeInterval
    private let auxiliaryStop: (() -> Void)?
    private let stateLock = NSLock()
    private var stopped = false

    public let publicURI: String

    fileprivate init(
      server: Server,
      group: MultiThreadedEventLoopGroup,
      publicURI: String,
      logger: @escaping (String) -> Void,
      defaultGracePeriodSeconds: TimeInterval,
      auxiliaryStop: (() -> Void)? = nil
    ) {
      self.server = server
      self.group = group
      self.publicURI = publicURI
      self.logger = logger
      self.defaultGracePeriodSeconds = defaultGracePeriodSeconds
      self.auxiliaryStop = auxiliaryStop
    }

    public func await() throws {
      try server.onClose.wait()
    }

    public func stop(gracePeriodSeconds: TimeInterval? = nil) {
      stateLock.lock()
      if stopped {
        stateLock.unlock()
        return
      }
      stopped = true
      stateLock.unlock()

      auxiliaryStop?()
      _ = gracePeriodSeconds ?? defaultGracePeriodSeconds
      do {
        try server.initiateGracefulShutdown().wait()
      } catch {
        logger("graceful stop failed: \(error); forcing hard stop")
        _ = try? server.close().wait()
      }

      try? group.syncShutdownGracefully()
    }
  }

  public static func parseFlags(_ args: [String]) -> String {
    parseOptions(args).listenURI
  }

  public static func parseOptions(_ args: [String]) -> ParsedFlags {
    var listenURI = Transport.defaultURI
    var reflect = false
    var idx = 0
    while idx < args.count {
      if args[idx] == "--listen", idx + 1 < args.count {
        listenURI = args[idx + 1]
      }
      if args[idx] == "--port", idx + 1 < args.count {
        listenURI = "tcp://:\(args[idx + 1])"
      }
      if args[idx] == "--reflect" {
        reflect = true
      }
      idx += 1
    }
    return ParsedFlags(listenURI: listenURI, reflect: reflect)
  }

  public static func run(
    _ listenURI: String,
    serviceProviders: [CallHandlerProvider]
  ) throws {
    try runWithOptions(listenURI, serviceProviders: serviceProviders)
  }

  public static func runWithOptions(
    _ listenURI: String,
    serviceProviders: [CallHandlerProvider],
    options: Options = Options()
  ) throws {
    let running = try startWithOptions(
      listenURI,
      serviceProviders: serviceProviders,
      options: options
    )

    signal(SIGTERM, SIG_IGN)
    signal(SIGINT, SIG_IGN)

    let queue = DispatchQueue(label: "holons.serve.signal-forwarding")
    let stopServer = {
      options.logger("shutting down gRPC server")
      running.stop(gracePeriodSeconds: options.shutdownGracePeriodSeconds)
    }

    let termSource = DispatchSource.makeSignalSource(signal: SIGTERM, queue: queue)
    termSource.setEventHandler(handler: stopServer)
    termSource.resume()

    let intSource = DispatchSource.makeSignalSource(signal: SIGINT, queue: queue)
    intSource.setEventHandler(handler: stopServer)
    intSource.resume()

    defer {
      termSource.cancel()
      intSource.cancel()
      signal(SIGTERM, SIG_DFL)
      signal(SIGINT, SIG_DFL)
    }

    try running.await()
  }

  public static func startWithOptions(
    _ listenURI: String,
    serviceProviders: [CallHandlerProvider],
    options: Options = Options()
  ) throws -> RunningServer {
    let parsed = try Transport.parse(listenURI.isEmpty ? Transport.defaultURI : listenURI)
    var providers = serviceProviders
    let describeEnabled: Bool
    do {
      describeEnabled = try maybeAddDescribe(&providers)
    } catch {
      options.logger("HolonMeta registration failed: \(error)")
      throw error
    }
    let reflectionEnabled = try maybeAddReflection(&providers, options: options)
    switch parsed.scheme {
    case "tcp":
      let host = parsed.host ?? "0.0.0.0"
      let port = parsed.port ?? 9090
      return try startTCPServer(
        host: host,
        port: port,
        publicURI: nil,
        serviceProviders: providers,
        describeEnabled: describeEnabled,
        reflectionEnabled: reflectionEnabled,
        options: options
      )
    case "stdio":
      let backing = try startTCPServer(
        host: "127.0.0.1",
        port: 0,
        publicURI: nil,
        serviceProviders: providers,
        describeEnabled: describeEnabled,
        reflectionEnabled: reflectionEnabled,
        options: options,
        suppressAnnouncement: true
      )
      let parsedBacking = try Transport.parse(backing.publicURI)
      let bridge = try StdioBridge(
        host: parsedBacking.host ?? "127.0.0.1",
        port: parsedBacking.port ?? 0
      ) { [weak backing] in
        backing?.stop(gracePeriodSeconds: options.shutdownGracePeriodSeconds)
      }
      bridge.start()
      let mode = formatMode(describeEnabled: describeEnabled, reflectionEnabled: reflectionEnabled)
      options.onListen?("stdio://")
      options.logger("gRPC server listening on stdio:// (\(mode))")
      return RunningServer(
        server: backing.server,
        group: backing.group,
        publicURI: "stdio://",
        logger: options.logger,
        defaultGracePeriodSeconds: options.shutdownGracePeriodSeconds,
        auxiliaryStop: { bridge.stop() }
      )
    case "unix":
      let path = parsed.path ?? ""
      let backing = try startTCPServer(
        host: "127.0.0.1",
        port: 0,
        publicURI: nil,
        serviceProviders: providers,
        describeEnabled: describeEnabled,
        reflectionEnabled: reflectionEnabled,
        options: options,
        suppressAnnouncement: true
      )
      let parsedBacking = try Transport.parse(backing.publicURI)
      let bridge = try UnixBridge(
        path: path,
        host: parsedBacking.host ?? "127.0.0.1",
        port: parsedBacking.port ?? 0
      )
      bridge.start()
      let publicURI = "unix://\(path)"
      let mode = formatMode(describeEnabled: describeEnabled, reflectionEnabled: reflectionEnabled)
      options.onListen?(publicURI)
      options.logger("gRPC server listening on \(publicURI) (\(mode))")
      return RunningServer(
        server: backing.server,
        group: backing.group,
        publicURI: publicURI,
        logger: options.logger,
        defaultGracePeriodSeconds: options.shutdownGracePeriodSeconds,
        auxiliaryStop: { bridge.stop() }
      )
    default:
      throw TransportError.runtimeUnsupported(
        uri: listenURI,
        reason: "Serve.run(...) currently supports tcp://, unix://, and stdio:// only"
      )
    }
  }

  private static func startTCPServer(
    host: String,
    port: Int,
    publicURI: String?,
    serviceProviders: [CallHandlerProvider],
    describeEnabled: Bool,
    reflectionEnabled: Bool,
    options: Options,
    suppressAnnouncement: Bool = false
  ) throws -> RunningServer {
    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)

    do {
      let server = try Server.insecure(group: group)
        .withServiceProviders(serviceProviders)
        .bind(host: host, port: port)
        .wait()
      let actualPort = server.channel.localAddress?.port ?? port
      let advertised = publicURI ?? "tcp://\(advertisedHost(host)):\(actualPort)"
      let mode = formatMode(describeEnabled: describeEnabled, reflectionEnabled: reflectionEnabled)

      if !suppressAnnouncement {
        options.onListen?(advertised)
        options.logger("gRPC server listening on \(advertised) (\(mode))")
      }

      return RunningServer(
        server: server,
        group: group,
        publicURI: advertised,
        logger: options.logger,
        defaultGracePeriodSeconds: options.shutdownGracePeriodSeconds
      )
    } catch {
      try? group.syncShutdownGracefully()
      throw error
    }
  }

  private static func maybeAddDescribe(_ providers: inout [CallHandlerProvider]) throws -> Bool {
    providers.append(try Describe.register())
    return true
  }

  private static func maybeAddReflection(
    _ providers: inout [CallHandlerProvider],
    options: Options
  ) throws -> Bool {
    guard options.reflect else {
      return false
    }

    providers.append(
      try ReflectionProvider(
        protoDir: options.protoDir ?? "."
      )
    )
    return true
  }

  private static func formatMode(describeEnabled: Bool, reflectionEnabled: Bool) -> String {
    "\(describeEnabled ? "Describe ON" : "Describe OFF"), \(reflectionEnabled ? "reflection ON" : "reflection OFF")"
  }
}

private func advertisedHost(_ host: String) -> String {
  switch host {
  case "", "0.0.0.0":
    return "127.0.0.1"
  case "::":
    return "::1"
  default:
    return host
  }
}

private final class StdioBridge {
  private let stateLock = NSLock()
  private let onDisconnect: () -> Void
  private var socketFD: Int32
  private var stopped = false
  private var started = false
  private let completion = DispatchGroup()

  init(host: String, port: Int, onDisconnect: @escaping () -> Void) throws {
    self.socketFD = try connectLoopback(host: host, port: port)
    self.onDisconnect = onDisconnect
  }

  func start() {
    stateLock.lock()
    if started {
      stateLock.unlock()
      return
    }
    started = true
    stateLock.unlock()

    completion.enter()
    DispatchQueue.global(qos: .userInitiated).async {
      self.pumpStdinToSocket()
      self.completion.leave()
    }

    completion.enter()
    DispatchQueue.global(qos: .userInitiated).async {
      self.pumpSocketToStdout()
      self.completion.leave()
    }

    completion.notify(queue: .global(qos: .utility)) { [onDisconnect] in
      onDisconnect()
    }
  }

  func stop() {
    stateLock.lock()
    if stopped {
      stateLock.unlock()
      return
    }
    stopped = true
    let fd = socketFD
    socketFD = -1
    stateLock.unlock()

    if fd >= 0 {
      _ = bridgeShutdown(fd, bridgeShutdownReadWrite)
      _ = bridgeClose(fd)
    }
  }

  private func pumpStdinToSocket() {
    var buffer = [UInt8](repeating: 0, count: 16 * 1024)
    while true {
      let readCount = buffer.withUnsafeMutableBytes { ptr in
        bridgeRead(STDIN_FILENO, ptr.baseAddress, ptr.count)
      }
      if readCount < 0 {
        let currentErrno = errno
        if isRetryableBridgeErrno(currentErrno) {
          Thread.sleep(forTimeInterval: bridgeRetryDelaySeconds)
          continue
        }
        return
      }
      if readCount == 0 {
        let fd = currentSocketFD()
        if fd >= 0 {
          _ = bridgeShutdown(fd, bridgeShutdownWrite)
        }
        return
      }

      if writeAll(buffer, count: readCount, to: currentSocketFD()) == false {
        return
      }
    }
  }

  private func pumpSocketToStdout() {
    var buffer = [UInt8](repeating: 0, count: 16 * 1024)
    while true {
      let fd = currentSocketFD()
      if fd < 0 {
        return
      }

      let readCount = buffer.withUnsafeMutableBytes { ptr in
        bridgeRead(fd, ptr.baseAddress, ptr.count)
      }
      if readCount < 0 {
        let currentErrno = errno
        if isRetryableBridgeErrno(currentErrno) {
          Thread.sleep(forTimeInterval: bridgeRetryDelaySeconds)
          continue
        }
        return
      }
      if readCount == 0 {
        return
      }

      if writeAll(buffer, count: readCount, to: STDOUT_FILENO) == false {
        return
      }
    }
  }

  private func writeAll(_ buffer: [UInt8], count: Int, to fd: Int32) -> Bool {
    guard fd >= 0 else {
      return false
    }

    return buffer.withUnsafeBytes { ptr in
      guard let base = ptr.baseAddress else {
        return true
      }
      var offset = 0
      while offset < count {
        let written = bridgeWrite(fd, base.advanced(by: offset), count - offset)
        if written < 0 {
          let currentErrno = errno
          if isRetryableBridgeErrno(currentErrno) {
            Thread.sleep(forTimeInterval: bridgeRetryDelaySeconds)
            continue
          }
          return false
        }
        if written == 0 {
          return false
        }
        offset += written
      }
      return true
    }
  }

  private func currentSocketFD() -> Int32 {
    stateLock.lock()
    let fd = socketFD
    stateLock.unlock()
    return fd
  }
}

private final class UnixBridge {
  private let stateLock = NSLock()
  private let listener: UnixRuntimeListener
  private let host: String
  private let port: Int
  private var stopped = false
  private var started = false
  private var activeConnections: [ActiveUnixBridgeConnection] = []

  init(path: String, host: String, port: Int) throws {
    self.listener = try UnixRuntimeListener(path: path)
    self.host = host
    self.port = port
  }

  func start() {
    stateLock.lock()
    if started {
      stateLock.unlock()
      return
    }
    started = true
    stateLock.unlock()

    DispatchQueue.global(qos: .userInitiated).async {
      self.acceptLoop()
    }
  }

  func stop() {
    stateLock.lock()
    if stopped {
      stateLock.unlock()
      return
    }
    stopped = true
    let active = activeConnections
    activeConnections.removeAll()
    stateLock.unlock()

    for connection in active {
      connection.stop()
    }
    try? listener.close()
  }

  private func acceptLoop() {
    while !isStopped {
      do {
        let connection = try listener.accept()
        let socketFD = try connectLoopback(host: host, port: port)
        let active = ActiveUnixBridgeConnection(
          connection: connection,
          socketFD: socketFD
        ) { [weak self] closed in
          self?.removeConnection(closed)
        }
        appendConnection(active)
        active.start()
      } catch {
        if isStopped {
          return
        }
        Thread.sleep(forTimeInterval: bridgeRetryDelaySeconds)
      }
    }
  }

  private var isStopped: Bool {
    stateLock.lock()
    let value = stopped
    stateLock.unlock()
    return value
  }

  private func appendConnection(_ connection: ActiveUnixBridgeConnection) {
    stateLock.lock()
    if stopped {
      stateLock.unlock()
      connection.stop()
      return
    }
    activeConnections.append(connection)
    stateLock.unlock()
  }

  private func removeConnection(_ connection: ActiveUnixBridgeConnection) {
    stateLock.lock()
    activeConnections.removeAll { $0 === connection }
    stateLock.unlock()
  }
}

private final class ActiveUnixBridgeConnection {
  private let stateLock = NSLock()
  private let connection: RuntimeConnection
  private let socketFD: Int32
  private let onClose: (ActiveUnixBridgeConnection) -> Void
  private var stopped = false
  private let completion = DispatchGroup()

  init(
    connection: RuntimeConnection,
    socketFD: Int32,
    onClose: @escaping (ActiveUnixBridgeConnection) -> Void
  ) {
    self.connection = connection
    self.socketFD = socketFD
    self.onClose = onClose
  }

  func start() {
    completion.enter()
    DispatchQueue.global(qos: .userInitiated).async {
      self.pumpConnectionToSocket()
      self.completion.leave()
    }

    completion.enter()
    DispatchQueue.global(qos: .userInitiated).async {
      self.pumpSocketToConnection()
      self.completion.leave()
    }

    completion.notify(queue: .global(qos: .utility)) {
      self.stop()
    }
  }

  func stop() {
    stateLock.lock()
    if stopped {
      stateLock.unlock()
      return
    }
    stopped = true
    stateLock.unlock()

    try? connection.close()
    _ = bridgeShutdown(socketFD, bridgeShutdownReadWrite)
    _ = bridgeClose(socketFD)
    onClose(self)
  }

  private func pumpConnectionToSocket() {
    while !isStopped {
      do {
        let data = try connection.read(maxBytes: 16 * 1024)
        if data.isEmpty {
          _ = bridgeShutdown(socketFD, bridgeShutdownWrite)
          return
        }

        let bytes = [UInt8](data)
        if writeAll(bytes, count: bytes.count, to: socketFD) == false {
          return
        }
      } catch {
        return
      }
    }
  }

  private func pumpSocketToConnection() {
    var buffer = [UInt8](repeating: 0, count: 16 * 1024)
    while !isStopped {
      let readCount = buffer.withUnsafeMutableBytes { ptr in
        bridgeRead(socketFD, ptr.baseAddress, ptr.count)
      }
      if readCount < 0 {
        let currentErrno = errno
        if isRetryableBridgeErrno(currentErrno) {
          Thread.sleep(forTimeInterval: bridgeRetryDelaySeconds)
          continue
        }
        return
      }
      if readCount == 0 {
        return
      }

      do {
        try connection.write(Data(buffer.prefix(readCount)))
      } catch {
        return
      }
    }
  }

  private var isStopped: Bool {
    stateLock.lock()
    let value = stopped
    stateLock.unlock()
    return value
  }

  private func writeAll(_ buffer: [UInt8], count: Int, to fd: Int32) -> Bool {
    guard fd >= 0 else {
      return false
    }

    return buffer.withUnsafeBytes { ptr in
      guard let base = ptr.baseAddress else {
        return true
      }
      var offset = 0
      while offset < count {
        let written = bridgeWrite(fd, base.advanced(by: offset), count - offset)
        if written < 0 {
          let currentErrno = errno
          if isRetryableBridgeErrno(currentErrno) {
            Thread.sleep(forTimeInterval: bridgeRetryDelaySeconds)
            continue
          }
          return false
        }
        if written == 0 {
          return false
        }
        offset += written
      }
      return true
    }
  }
}

let bridgeRetryDelaySeconds: TimeInterval = 0.01

func isRetryableBridgeErrno(_ value: Int32) -> Bool {
  if value == EINTR || value == EAGAIN {
    return true
  }
  #if os(Linux)
    return value == EWOULDBLOCK
  #else
    return value == EWOULDBLOCK
  #endif
}

private func connectLoopback(host: String, port: Int) throws -> Int32 {
  let fd = bridgeSocket(AF_INET, bridgeSocketStreamType, 0)
  guard fd >= 0 else {
    throw TransportError.listenFailed(String(cString: strerror(errno)))
  }
  disableSIGPIPE(fd)

  var address = sockaddr_in()
  #if !os(Linux)
    address.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
  #endif
  address.sin_family = sa_family_t(AF_INET)
  address.sin_port = in_port_t(UInt16(port).bigEndian)
  address.sin_addr = in_addr(s_addr: inet_addr(host))

  let result = withUnsafePointer(to: &address) { ptr in
    ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
      bridgeConnect(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_in>.size))
    }
  }
  if result != 0 {
    let message = String(cString: strerror(errno))
    _ = bridgeClose(fd)
    throw TransportError.listenFailed(message)
  }

  return fd
}

private var bridgeSocketStreamType: Int32 {
  #if os(Linux)
    return Int32(SOCK_STREAM.rawValue)
  #else
    return SOCK_STREAM
  #endif
}

private var bridgeShutdownWrite: Int32 {
  #if os(Linux)
    return Int32(SHUT_WR)
  #else
    return SHUT_WR
  #endif
}

private var bridgeShutdownReadWrite: Int32 {
  #if os(Linux)
    return Int32(SHUT_RDWR)
  #else
    return SHUT_RDWR
  #endif
}

private func bridgeSocket(_ domain: Int32, _ type: Int32, _ proto: Int32) -> Int32 {
  #if os(Linux)
    return Glibc.socket(domain, type, proto)
  #else
    return Darwin.socket(domain, type, proto)
  #endif
}

private func bridgeConnect(_ fd: Int32, _ address: UnsafePointer<sockaddr>?, _ length: socklen_t)
  -> Int32
{
  #if os(Linux)
    return Glibc.connect(fd, address, length)
  #else
    return Darwin.connect(fd, address, length)
  #endif
}

private func bridgeRead(_ fd: Int32, _ buffer: UnsafeMutableRawPointer?, _ count: Int) -> Int {
  #if os(Linux)
    return Glibc.read(fd, buffer, count)
  #else
    return Darwin.read(fd, buffer, count)
  #endif
}

private func bridgeWrite(_ fd: Int32, _ buffer: UnsafeRawPointer?, _ count: Int) -> Int {
  #if os(Linux)
    return Glibc.write(fd, buffer, count)
  #else
    return Darwin.write(fd, buffer, count)
  #endif
}

private func bridgeShutdown(_ fd: Int32, _ how: Int32) -> Int32 {
  #if os(Linux)
    return Glibc.shutdown(fd, Int32(how))
  #else
    return Darwin.shutdown(fd, how)
  #endif
}

private func bridgeClose(_ fd: Int32) -> Int32 {
  #if os(Linux)
    return Glibc.close(fd)
  #else
    return Darwin.close(fd)
  #endif
}
