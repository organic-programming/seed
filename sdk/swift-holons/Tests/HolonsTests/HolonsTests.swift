import XCTest

@testable import Holons

#if os(Linux)
  import Glibc
#else
  import Darwin
#endif

final class HolonsTests: XCTestCase {
  func testSchemeExtraction() {
    XCTAssertEqual(Transport.scheme("tcp://:9090"), "tcp")
    XCTAssertEqual(Transport.scheme("unix:///tmp/x.sock"), "unix")
    XCTAssertEqual(Transport.scheme("stdio://"), "stdio")
    XCTAssertEqual(Transport.scheme("ws://localhost:8080"), "ws")
    XCTAssertEqual(Transport.scheme("wss://localhost:8443"), "wss")
  }

  func testTransportParse() throws {
    let tcp = try Transport.parse("tcp://127.0.0.1:9000")
    XCTAssertEqual(tcp.scheme, "tcp")
    XCTAssertEqual(tcp.host, "127.0.0.1")
    XCTAssertEqual(tcp.port, 9000)

    let ws = try Transport.parse("ws://127.0.0.1:8080")
    XCTAssertEqual(ws.path, "/grpc")

    let wss = try Transport.parse("wss://example.com:8443/holon")
    XCTAssertEqual(wss.scheme, "wss")
    XCTAssertEqual(wss.path, "/holon")
  }

  func testListenVariants() throws {
    XCTAssertEqual(try Transport.listen("stdio://"), .stdio)
    XCTAssertEqual(try Transport.listen("unix:///tmp/test.sock"), .unix(path: "/tmp/test.sock"))
    XCTAssertEqual(try Transport.listen("tcp://:9090"), .tcp(host: "0.0.0.0", port: 9090))
  }

  func testParseFlags() {
    XCTAssertEqual(Serve.parseFlags(["--listen", "tcp://:8080"]), "tcp://:8080")
    XCTAssertEqual(Serve.parseFlags(["--port", "3000"]), "tcp://:3000")
    XCTAssertEqual(Serve.parseFlags([]), Transport.defaultURI)
  }

  func testParseOptionsReflect() {
    let parsed = Serve.parseOptions(["--listen", "tcp://:8080", "--reflect"])
    XCTAssertEqual(parsed.listenURI, "tcp://:8080")
    XCTAssertTrue(parsed.reflect)
  }

  func testIdentityParse() throws {
    let tmp = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons_test_\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
    let protoFile = tmp.appendingPathComponent(Identity.protoManifestFileName)
    try writeHolonProto(
      to: protoFile,
      uuid: "abc-123",
      givenName: "swift-holon",
      familyName: "",
      lang: "swift",
      parents: ["a", "b"],
      aliases: ["s1"]
    )
    defer { try? FileManager.default.removeItem(at: tmp) }

    let id = try Identity.parseHolon(protoFile.path)
    XCTAssertEqual(id.uuid, "abc-123")
    XCTAssertEqual(id.givenName, "swift-holon")
    XCTAssertEqual(id.lang, "swift")
    XCTAssertEqual(id.parents, ["a", "b"])
    XCTAssertEqual(id.aliases, ["s1"])
    XCTAssertEqual(id.slug, "swift-holon")
  }

  func testDiscoverRecursesSkipsAndDedups() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons_discover_\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    try writeHolon(
      at: root, relativeDir: "holons/alpha",
      seed: HolonSeed(
        uuid: "uuid-alpha",
        givenName: "Alpha",
        familyName: "Go",
        binary: "alpha-go"
      ))
    try writeHolon(
      at: root, relativeDir: "nested/beta",
      seed: HolonSeed(
        uuid: "uuid-beta",
        givenName: "Beta",
        familyName: "Rust",
        binary: "beta-rust"
      ))
    try writeHolon(
      at: root, relativeDir: "nested/dup/alpha",
      seed: HolonSeed(
        uuid: "uuid-alpha",
        givenName: "Alpha",
        familyName: "Go",
        binary: "alpha-go"
      ))
    for skipped in [
      ".git/hidden", ".op/hidden", "node_modules/hidden", "vendor/hidden", "build/hidden",
      ".cache/hidden",
    ] {
      try writeHolon(
        at: root, relativeDir: skipped,
        seed: HolonSeed(
          uuid: "ignored-uuid",
          givenName: "Ignored",
          familyName: "Holon",
          binary: "ignored-holon"
        ))
    }

    let entries = try discover(root: root)
    XCTAssertEqual(entries.count, 2)

    let alpha = try XCTUnwrap(entries.first(where: { $0.uuid == "uuid-alpha" }))
    XCTAssertEqual(alpha.slug, "alpha-go")
    XCTAssertEqual(alpha.relativePath, "holons/alpha")
    XCTAssertEqual(alpha.manifest?.build.runner, "go-module")

    let beta = try XCTUnwrap(entries.first(where: { $0.uuid == "uuid-beta" }))
    XCTAssertEqual(beta.relativePath, "nested/beta")
  }

  func testDiscoverLocalAndFindHelpers() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons_find_\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    try writeHolon(
      at: root, relativeDir: "rob-go",
      seed: HolonSeed(
        uuid: "c7f3a1b2-1111-1111-1111-111111111111",
        givenName: "Rob",
        familyName: "Go",
        binary: "rob-go"
      ))

    let original = FileManager.default.currentDirectoryPath
    let originalCurrentRoot = discoverCurrentRootURLProvider
    let originalEnvironment = discoverEnvironmentProvider
    let originalBundle = discoverBundleResourceURLProvider
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(original))
      discoverCurrentRootURLProvider = originalCurrentRoot
      discoverEnvironmentProvider = originalEnvironment
      discoverBundleResourceURLProvider = originalBundle
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(root.path))

    discoverCurrentRootURLProvider = { root }
    discoverEnvironmentProvider = { ["HOME": root.path] }
    discoverBundleResourceURLProvider = { nil }

    let local = try discoverLocal()
    XCTAssertEqual(local.count, 1)
    XCTAssertEqual(local.first?.slug, "rob-go")

    let bySlug = try findBySlug("rob-go")
    XCTAssertEqual(bySlug?.uuid, "c7f3a1b2-1111-1111-1111-111111111111")

    let byUUID = try findByUUID("c7f3a1b2")
    XCTAssertEqual(byUUID?.slug, "rob-go")

    XCTAssertNil(try findBySlug("missing"))
  }

  func testDiscoverAllPrefersBuildPackageOverInstalledCacheAndSource() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons-package-build-\(UUID().uuidString)", isDirectory: true)
    let localRoot = root.appendingPathComponent("local", isDirectory: true)
    let opHome = root.appendingPathComponent("runtime", isDirectory: true)
    let opBin = opHome.appendingPathComponent("bin", isDirectory: true)
    let cache = opHome.appendingPathComponent("cache", isDirectory: true)
    let buildRoot = localRoot.appendingPathComponent(".op/build", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    try writeHolon(
      at: localRoot, relativeDir: "rob-go",
      seed: HolonSeed(
        uuid: "same-uuid",
        givenName: "Rob",
        familyName: "Go",
        binary: "rob-go"
      ))
    try writePackageHolon(
      at: buildRoot.appendingPathComponent("rob-go.holon", isDirectory: true),
      seed: PackageSeed(
        uuid: "same-uuid",
        givenName: "Rob",
        familyName: "Go",
        runner: "go-module",
        entrypoint: "rob-go",
        architectures: [testCurrentArchDirectory()]
      ))
    try writePackageHolon(
      at: opBin.appendingPathComponent("rob-go.holon", isDirectory: true),
      seed: PackageSeed(
        uuid: "same-uuid",
        givenName: "Rob",
        familyName: "Go",
        runner: "go-module",
        entrypoint: "rob-go",
        architectures: [testCurrentArchDirectory()]
      ))
    try writePackageHolon(
      at: cache.appendingPathComponent("deps/rob-go.holon", isDirectory: true),
      seed: PackageSeed(
        uuid: "same-uuid",
        givenName: "Rob",
        familyName: "Go",
        runner: "go-module",
        entrypoint: "rob-go",
        architectures: [testCurrentArchDirectory()]
      ))

    let originalCurrentRoot = discoverCurrentRootURLProvider
    let originalEnvironment = discoverEnvironmentProvider
    let originalBundle = discoverBundleResourceURLProvider
    defer {
      discoverCurrentRootURLProvider = originalCurrentRoot
      discoverEnvironmentProvider = originalEnvironment
      discoverBundleResourceURLProvider = originalBundle
    }

    discoverCurrentRootURLProvider = { localRoot }
    discoverEnvironmentProvider = {
      ["OPPATH": opHome.path, "OPBIN": opBin.path, "HOME": root.path]
    }
    discoverBundleResourceURLProvider = { nil }

    let entries = try discoverAll()
    XCTAssertEqual(entries.count, 1)
    XCTAssertEqual(entries.first?.origin, "build")
    XCTAssertEqual(entries.first?.sourceKind, "package")
  }

  func testDiscoverAllPrefersBundlePackage() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons-package-bundle-\(UUID().uuidString)", isDirectory: true)
    let localRoot = root.appendingPathComponent("local", isDirectory: true)
    let bundleResources = root.appendingPathComponent("MyApp.app/Contents/Resources", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    try writeHolon(
      at: localRoot, relativeDir: "rob-go",
      seed: HolonSeed(
        uuid: "bundle-uuid",
        givenName: "Rob",
        familyName: "Go",
        binary: "rob-go"
      ))
    try writePackageHolon(
      at: bundleResources.appendingPathComponent("Holons/rob-go.holon", isDirectory: true),
      seed: PackageSeed(
        uuid: "bundle-uuid",
        givenName: "Rob",
        familyName: "Go",
        runner: "go-module",
        entrypoint: "rob-go",
        architectures: [testCurrentArchDirectory()]
      ))

    let originalCurrentRoot = discoverCurrentRootURLProvider
    let originalEnvironment = discoverEnvironmentProvider
    let originalBundle = discoverBundleResourceURLProvider
    defer {
      discoverCurrentRootURLProvider = originalCurrentRoot
      discoverEnvironmentProvider = originalEnvironment
      discoverBundleResourceURLProvider = originalBundle
    }

    discoverCurrentRootURLProvider = { localRoot }
    discoverEnvironmentProvider = { ["HOME": root.path] }
    discoverBundleResourceURLProvider = { bundleResources }

    let entries = try discoverAll()
    XCTAssertEqual(entries.count, 1)
    XCTAssertEqual(entries.first?.origin, "bundle")
    XCTAssertEqual(entries.first?.sourceKind, "package")
  }

  func testDiscoverAllBundleExcludesNonBundleSources() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons-bundle-exclusive-\(UUID().uuidString)", isDirectory: true)
    let localRoot = root.appendingPathComponent("local", isDirectory: true)
    let opHome = root.appendingPathComponent("runtime", isDirectory: true)
    let bundleResources = root.appendingPathComponent("App.app/Contents/Resources", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    // Holon in bundle — should be discovered.
    try writePackageHolon(
      at: bundleResources.appendingPathComponent("Holons/alpha.holon", isDirectory: true),
      seed: PackageSeed(
        uuid: "uuid-alpha",
        givenName: "Alpha",
        familyName: "Go",
        runner: "go-module",
        entrypoint: "alpha-go",
        architectures: [testCurrentArchDirectory()]
      ))

    // Holon in local source — should NOT be discovered when bundle exists.
    try writeHolon(
      at: localRoot, relativeDir: "beta-rust",
      seed: HolonSeed(
        uuid: "uuid-beta",
        givenName: "Beta",
        familyName: "Rust",
        binary: "beta-rust"
      ))

    let originalCurrentRoot = discoverCurrentRootURLProvider
    let originalEnvironment = discoverEnvironmentProvider
    let originalBundle = discoverBundleResourceURLProvider
    defer {
      discoverCurrentRootURLProvider = originalCurrentRoot
      discoverEnvironmentProvider = originalEnvironment
      discoverBundleResourceURLProvider = originalBundle
    }

    discoverCurrentRootURLProvider = { localRoot }
    discoverEnvironmentProvider = { ["OPPATH": opHome.path, "HOME": root.path] }
    discoverBundleResourceURLProvider = { bundleResources }

    let entries = try discoverAll()
    XCTAssertEqual(entries.count, 1, "only bundle holons should be discovered")
    XCTAssertEqual(entries.first?.slug, "alpha-go")
    XCTAssertEqual(entries.first?.origin, "bundle")
  }

  func testRuntimeTCPRoundTrip() throws {
    do {
      let runtime = try Transport.listenRuntime("tcp://127.0.0.1:0")
      guard case .tcp(let listener) = runtime else {
        XCTFail("expected tcp runtime listener")
        return
      }
      defer { try? listener.close() }

      let clientFD = try connectTCP(host: "127.0.0.1", port: listener.boundPort)
      let client = POSIXRuntimeConnection(
        readFD: clientFD,
        writeFD: clientFD,
        ownsReadFD: true,
        ownsWriteFD: true
      )
      defer { try? client.close() }

      let server = try listener.accept()
      defer { try? server.close() }

      try client.write(Data("ping".utf8))
      let received = try server.read(maxBytes: 4)
      XCTAssertEqual(String(data: received, encoding: .utf8), "ping")
    } catch {
      if isPermissionDeniedError(error) {
        throw XCTSkip("tcp runtime sockets unavailable in this environment: \(error)")
      }
      throw error
    }
  }

  func testRuntimeUnixRoundTrip() throws {
    let socketPath = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons_test_\(UUID().uuidString).sock")
      .path

    do {
      let runtime = try Transport.listenRuntime("unix://\(socketPath)")
      guard case .unix(let listener) = runtime else {
        XCTFail("expected unix runtime listener")
        return
      }
      defer { try? listener.close() }

      let clientFD = try connectUnix(path: socketPath)
      let client = POSIXRuntimeConnection(
        readFD: clientFD,
        writeFD: clientFD,
        ownsReadFD: true,
        ownsWriteFD: true
      )
      defer { try? client.close() }

      let server = try listener.accept()
      defer { try? server.close() }

      try client.write(Data("unix".utf8))
      let received = try server.read(maxBytes: 4)
      XCTAssertEqual(String(data: received, encoding: .utf8), "unix")
    } catch {
      if isPermissionDeniedError(error) {
        throw XCTSkip("unix runtime sockets unavailable in this environment: \(error)")
      }
      throw error
    }
  }

  func testRuntimeStdioSingleAccept() throws {
    let runtime = try Transport.listenRuntime("stdio://")
    guard case .stdio(let listener) = runtime else {
      XCTFail("expected stdio runtime listener")
      return
    }

    _ = try listener.accept()
    XCTAssertThrowsError(try listener.accept())
    try listener.close()
  }

  func testRuntimeWebSocketUnsupported() {
    XCTAssertThrowsError(try Transport.listenRuntime("ws://127.0.0.1:8080/grpc")) { error in
      guard case TransportError.runtimeUnsupported(let uri, let reason) = error else {
        XCTFail("unexpected error: \(error)")
        return
      }
      XCTAssertEqual(uri, "ws://127.0.0.1:8080/grpc")
      XCTAssertFalse(reason.isEmpty)
    }
  }
}

private struct HolonSeed {
  let uuid: String
  let givenName: String
  let familyName: String
  let binary: String
}

private struct PackageSeed {
  let uuid: String
  let givenName: String
  let familyName: String
  let runner: String
  let entrypoint: String
  let architectures: [String]
  var hasDist: Bool = false
  var hasSource: Bool = false
}

private func writeHolon(at root: URL, relativeDir: String, seed: HolonSeed) throws {
  let dir =
    relativeDir
    .split(separator: "/")
    .reduce(root) { partial, component in
      partial.appendingPathComponent(String(component), isDirectory: true)
    }
  try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)

  try writeHolonProto(
    to: dir.appendingPathComponent("holon.proto"),
    uuid: seed.uuid,
    givenName: seed.givenName,
    familyName: seed.familyName,
    motto: "Test",
    composer: "test",
    clade: "deterministic/pure",
    status: "draft",
    born: "2026-03-07",
    kind: "native",
    buildRunner: "go-module",
    artifactBinary: seed.binary,
    generatedBy: "test"
  )
}

private func writePackageHolon(at root: URL, seed: PackageSeed) throws {
  try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
  let architectures = seed.architectures.map { "\"\($0)\"" }.joined(separator: ", ")
  let data = """
    {
      "schema": "holon-package/v1",
      "slug": "\(seed.givenName.lowercased())-\(seed.familyName.lowercased())",
      "uuid": "\(seed.uuid)",
      "identity": {
        "given_name": "\(seed.givenName)",
        "family_name": "\(seed.familyName)"
      },
      "lang": "go",
      "runner": "\(seed.runner)",
      "status": "draft",
      "kind": "native",
      "entrypoint": "\(seed.entrypoint)",
      "architectures": [\(architectures)],
      "has_dist": \(seed.hasDist ? "true" : "false"),
      "has_source": \(seed.hasSource ? "true" : "false")
    }
    """
  try data.write(to: root.appendingPathComponent(".holon.json"), atomically: true, encoding: .utf8)
}

private func testCurrentArchDirectory() -> String {
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

private enum TestConnectionError: Error {
  case connectFailed(String)
}

private func isPermissionDeniedError(_ error: Error) -> Bool {
  let message = String(describing: error).lowercased()
  return message.contains("operation not permitted")
    || message.contains("permission denied")
}

private func connectTCP(host: String, port: Int) throws -> Int32 {
  var hints = addrinfo(
    ai_flags: 0,
    ai_family: AF_UNSPEC,
    ai_socktype: testSocketStreamType,
    ai_protocol: 0,
    ai_addrlen: 0,
    ai_canonname: nil,
    ai_addr: nil,
    ai_next: nil
  )

  let hostCString = strdup(host)
  let portCString = strdup(String(port))
  defer {
    if let hostCString {
      free(hostCString)
    }
    if let portCString {
      free(portCString)
    }
  }

  var infos: UnsafeMutablePointer<addrinfo>?
  let gai = getaddrinfo(hostCString, portCString, &hints, &infos)
  guard gai == 0 else {
    throw TestConnectionError.connectFailed(String(cString: gai_strerror(gai)))
  }
  defer {
    if let infos {
      freeaddrinfo(infos)
    }
  }

  var current = infos
  var lastError = "unable to connect"
  while let infoPtr = current {
    let info = infoPtr.pointee
    let fd = testSocket(info.ai_family, info.ai_socktype, info.ai_protocol)
    if fd < 0 {
      lastError = testErrno()
      current = info.ai_next
      continue
    }

    if testConnect(fd, info.ai_addr, info.ai_addrlen) == 0 {
      return fd
    }

    lastError = testErrno()
    _ = testClose(fd)
    current = info.ai_next
  }

  throw TestConnectionError.connectFailed(lastError)
}

private func connectUnix(path: String) throws -> Int32 {
  let fd = testSocket(AF_UNIX, testSocketStreamType, 0)
  if fd < 0 {
    throw TestConnectionError.connectFailed(testErrno())
  }

  var addr = sockaddr_un()
  addr.sun_family = sa_family_t(AF_UNIX)

  let maxPathLength = MemoryLayout.size(ofValue: addr.sun_path)
  if path.utf8.count >= maxPathLength {
    _ = testClose(fd)
    throw TestConnectionError.connectFailed("unix path too long")
  }

  _ = path.withCString { cString in
    withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
      ptr.withMemoryRebound(to: CChar.self, capacity: maxPathLength) { dest in
        strncpy(dest, cString, maxPathLength - 1)
      }
    }
  }

  let connectResult = withUnsafePointer(to: &addr) { ptr in
    ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockaddrPtr in
      testConnect(fd, sockaddrPtr, socklen_t(MemoryLayout<sockaddr_un>.size))
    }
  }
  if connectResult != 0 {
    let message = testErrno()
    _ = testClose(fd)
    throw TestConnectionError.connectFailed(message)
  }

  return fd
}

private var testSocketStreamType: Int32 {
  #if os(Linux)
    return Int32(SOCK_STREAM.rawValue)
  #else
    return SOCK_STREAM
  #endif
}

private func testErrno() -> String {
  String(cString: strerror(errno))
}

private func testSocket(_ domain: Int32, _ type: Int32, _ proto: Int32) -> Int32 {
  #if os(Linux)
    return Glibc.socket(domain, type, proto)
  #else
    return Darwin.socket(domain, type, proto)
  #endif
}

private func testConnect(_ fd: Int32, _ addr: UnsafePointer<sockaddr>?, _ len: socklen_t) -> Int32 {
  #if os(Linux)
    return Glibc.connect(fd, addr, len)
  #else
    return Darwin.connect(fd, addr, len)
  #endif
}

private func testClose(_ fd: Int32) -> Int32 {
  #if os(Linux)
    return Glibc.close(fd)
  #else
    return Darwin.close(fd)
  #endif
}
