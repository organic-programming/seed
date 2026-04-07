import Foundation
import GRPC
import NIOCore
import XCTest

@testable import Holons

final class ConnectTests: XCTestCase {
  func testConnectDialsDirectTarget() throws {
    let server = try startConnectHelperServer(slug: "direct-connect", listen: "tcp://127.0.0.1:0")
    defer { server.stop() }

    let channel = try connect(server.uri)
    defer { try? disconnect(channel) }

    let slug = try describeSlug(channel, timeout: 2.0)
    XCTAssertEqual(slug, "direct-connect")
  }

  func testConnectStartsSlugEphemerallyAndStopsOnDisconnect() throws {
    let sandbox = try makeSandbox(prefix: "connect-slug")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let fixture = try sandbox.makeHolonFixture(slug: "connect-ephemeral")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let channel = try connect(fixture.slug, options: ConnectOptions(lifecycle: "ephemeral"))
    let pid = try waitForPID(at: fixture.pidFile)
    let slug = try describeSlug(channel, timeout: 2.0)
    XCTAssertEqual(slug, fixture.slug)

    try disconnect(channel)

    try waitForProcessExit(pid)
    XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.portFile.path))
  }

  func testConnectStartsSlugFromHolonDirectory() throws {
    let sandbox = try makeSandbox(prefix: "connect-cwd")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let fixture = try sandbox.makeHolonFixture(slug: "connect-cwd")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let channel = try connect(fixture.slug)
    defer { try? disconnect(channel) }

    _ = try waitForPID(at: fixture.pidFile)
    let childDirectory = URL(
      fileURLWithPath: try waitForFileContents(at: fixture.cwdFile)
        .trimmingCharacters(in: .whitespacesAndNewlines),
      isDirectory: true
    ).resolvingSymlinksInPath().path
    XCTAssertEqual(childDirectory, fixture.holonDir.resolvingSymlinksInPath().path)
  }

  func testConnectStartsPackageBinaryViaStdio() throws {
    let sandbox = try makeSandbox(prefix: "connect-package-bin")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let fixture = try sandbox.makePackageBinaryFixture(slug: "connect-package-bin")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let channel = try connect(fixture.slug, options: ConnectOptions(transport: "stdio", lifecycle: "ephemeral"))
    let pid = try waitForPID(at: fixture.pidFile)
    let slug = try describeSlug(channel, timeout: 2.0)
    XCTAssertEqual(slug, fixture.slug)

    let childDirectory = URL(
      fileURLWithPath: try waitForFileContents(at: fixture.cwdFile)
        .trimmingCharacters(in: .whitespacesAndNewlines),
      isDirectory: true
    ).resolvingSymlinksInPath().path
    XCTAssertEqual(childDirectory, fixture.holonDir.resolvingSymlinksInPath().path)

    try disconnect(channel)
    try waitForProcessExit(pid)
  }

  func testResolveLaunchTargetUsesPackageDistInterpreter() throws {
    let sandbox = try makeSandbox(prefix: "connect-package-dist")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageRoot = try sandbox.makePackageDistFixture(
      slug: "connect-package-dist",
      runner: "python",
      entrypoint: "main.py"
    )
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let entry = try XCTUnwrap(findBySlug("connect-package-dist"))
    let launchTarget = try resolveLaunchTarget(entry)
    XCTAssertTrue(launchTarget.executablePath.hasSuffix("/python3"))
    XCTAssertEqual(launchTarget.arguments, [packageRoot.appendingPathComponent("dist/main.py").path])
    XCTAssertEqual(launchTarget.workingDirectory, packageRoot.path)
  }

  func testResolveLaunchTargetFallsBackToPackageGitSource() throws {
    let sandbox = try makeSandbox(prefix: "connect-package-git")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageRoot = try sandbox.makePackageGitFixture(slug: "connect-package-git")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let entry = try XCTUnwrap(findBySlug("connect-package-git"))
    let launchTarget = try resolveLaunchTarget(entry)
    XCTAssertTrue(launchTarget.executablePath.hasSuffix("/go"))
    XCTAssertEqual(launchTarget.arguments, ["run", "./cmd/daemon"])
    XCTAssertEqual(launchTarget.workingDirectory, packageRoot.appendingPathComponent("git").path)
  }

  func testResolveLaunchTargetReportsMissingCurrentArchBinary() throws {
    let sandbox = try makeSandbox(prefix: "connect-package-missing")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    _ = try sandbox.makePackageManifestOnlyFixture(
      slug: "connect-package-missing",
      runner: "go-module",
      entrypoint: "connect-package-missing",
      architectures: ["linux_amd64"],
      hasDist: false,
      hasSource: false
    )
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let entry = try XCTUnwrap(findBySlug("connect-package-missing"))
    XCTAssertThrowsError(try resolveLaunchTarget(entry)) { error in
      XCTAssertTrue(String(describing: error).contains(testCurrentArchDirectory()))
    }
  }

  func testConnectWithTCPOptionsWritesPortFileAndReusesServer() throws {
    let sandbox = try makeSandbox(prefix: "connect-port")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let fixture = try sandbox.makeHolonFixture(slug: "connect-persistent")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let initial = try connect(
      fixture.slug,
      options: ConnectOptions(timeout: 5.0, transport: "tcp", start: true)
    )
    let pid = try waitForPID(at: fixture.pidFile)
    XCTAssertEqual(try describeSlug(initial, timeout: 2.0), fixture.slug)
    try disconnect(initial)

    XCTAssertTrue(pidExists(pid))

    let portTarget = try String(contentsOf: fixture.portFile, encoding: .utf8)
      .trimmingCharacters(in: .whitespacesAndNewlines)
    XCTAssertTrue(portTarget.hasPrefix("tcp://127.0.0.1:"))

    let reused = try connect(fixture.slug)
    XCTAssertEqual(try describeSlug(reused, timeout: 2.0), fixture.slug)
    try disconnect(reused)

    XCTAssertTrue(pidExists(pid))
    terminateProcess(pid)
    try waitForProcessExit(pid)
  }

  func testConnectWithUnixOptionsWritesPortFileAndReusesServer() throws {
    let sandbox = try makeSandbox(prefix: "connect-unix")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let fixture = try sandbox.makeHolonFixture(slug: "connect-unix")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let initial = try connect(
      fixture.slug,
      options: ConnectOptions(timeout: 5.0, transport: "unix", start: true)
    )
    let pid = try waitForPID(at: fixture.pidFile)
    XCTAssertEqual(try describeSlug(initial, timeout: 2.0), fixture.slug)
    try disconnect(initial)

    XCTAssertTrue(pidExists(pid))

    let portTarget = try String(contentsOf: fixture.portFile, encoding: .utf8)
      .trimmingCharacters(in: .whitespacesAndNewlines)
    XCTAssertEqual(
      portTarget,
      defaultUnixSocketURI(slug: fixture.slug, portFile: normalizedPortFilePath(nil, slug: fixture.slug))
    )

    let reused = try connect(fixture.slug)
    XCTAssertEqual(try describeSlug(reused, timeout: 2.0), fixture.slug)
    try disconnect(reused)

    XCTAssertTrue(pidExists(pid))
    terminateProcess(pid)
    try waitForProcessExit(pid)
  }

  func testNormalizedPortFilePathUsesCachesWhenBundleHolonsArePresent() throws {
    let sandbox = try makeSandbox(prefix: "connect-bundle-port")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let resourceRoot = sandbox.root.appendingPathComponent("Resources", isDirectory: true)
    let holonsRoot = resourceRoot.appendingPathComponent("Holons", isDirectory: true)
    try FileManager.default.createDirectory(at: holonsRoot, withIntermediateDirectories: true)

    let previousProvider = discoverBundleResourceURLProvider
    defer { discoverBundleResourceURLProvider = previousProvider }
    discoverBundleResourceURLProvider = { resourceRoot }

    let expected = try XCTUnwrap(FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask).first)
      .appendingPathComponent("holons")
      .appendingPathComponent("run")
      .appendingPathComponent("bundle-slug.port")
      .path

    XCTAssertEqual(normalizedPortFilePath(nil, slug: "bundle-slug"), expected)
  }

  func testLaunchWorkingDirectoryURLFallsBackToTemporaryDirectoryWhenNotWritable() throws {
    let sandbox = try makeSandbox(prefix: "connect-readonly-cwd")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let readOnly = sandbox.root.appendingPathComponent("readonly", isDirectory: true)
    try FileManager.default.createDirectory(at: readOnly, withIntermediateDirectories: true)
    try FileManager.default.setAttributes([.posixPermissions: 0o555], ofItemAtPath: readOnly.path)
    defer {
      try? FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: readOnly.path)
    }

    let resolved = try XCTUnwrap(launchWorkingDirectoryURL(readOnly.path))
    XCTAssertEqual(resolved.standardizedFileURL.path, URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true).standardizedFileURL.path)
  }

  func testConnectRemovesStalePortFileAndStartsFresh() throws {
    let sandbox = try makeSandbox(prefix: "connect-stale")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let fixture = try sandbox.makeHolonFixture(slug: "connect-stale")
    let previousDirectory = FileManager.default.currentDirectoryPath
    defer {
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
    }
    XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

    let stalePort = try reserveLoopbackPort()
    try FileManager.default.createDirectory(
      at: fixture.portFile.deletingLastPathComponent(), withIntermediateDirectories: true)
    try "tcp://127.0.0.1:\(stalePort)\n".write(
      to: fixture.portFile, atomically: true, encoding: .utf8)

    let channel = try connect(fixture.slug, options: ConnectOptions(lifecycle: "ephemeral"))
    let pid = try waitForPID(at: fixture.pidFile)
    XCTAssertEqual(try describeSlug(channel, timeout: 2.0), fixture.slug)
    XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.portFile.path))

    try disconnect(channel)
    try waitForProcessExit(pid)
  }

  func testConnectWithUnixTransportRemovesStaleSocketFileBeforeLaunch() throws {
    #if os(Windows)
      throw XCTSkip("unix transport is not supported on Windows")
    #else
      let sandbox = try makeSandbox(prefix: "connect-unix-stale")
      defer { try? FileManager.default.removeItem(at: sandbox.root) }

      let fixture = try sandbox.makeHolonFixture(
        slug: "connect-unix-stale",
        launchDelaySeconds: 0.4
      )
      let previousDirectory = FileManager.default.currentDirectoryPath
      defer {
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(previousDirectory))
      }
      XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(sandbox.root.path))

      let socketPath = expectedUnixSocketPath(slug: fixture.slug, portFile: fixture.portFile.path)
      let socketURL = URL(fileURLWithPath: socketPath)
      try FileManager.default.createDirectory(
        at: socketURL.deletingLastPathComponent(),
        withIntermediateDirectories: true
      )
      try Data("stale".utf8).write(to: socketURL)

      let channel = try connect(
        fixture.slug,
        options: ConnectOptions(timeout: 5.0, transport: "unix", lifecycle: "ephemeral")
      )
      let pid = try waitForPID(at: fixture.pidFile)
      XCTAssertEqual(try describeSlug(channel, timeout: 2.0), fixture.slug)

      try disconnect(channel)
      try waitForProcessExit(pid)
    #endif
  }
}

private struct ConnectSandbox {
  let root: URL
  let goBinary: String
  let helperSource: URL
  let goModuleRoot: URL
  let helperExecutable: URL

  struct Fixture {
    let slug: String
    let pidFile: URL
    let portFile: URL
    let cwdFile: URL
    let holonDir: URL
  }

  func makeHolonFixture(slug: String, launchDelaySeconds: Double = 0) throws -> Fixture {
    let holonDir =
      root
      .appendingPathComponent("holons")
      .appendingPathComponent(slug)
    let binaryDir =
      holonDir
      .appendingPathComponent(".op")
      .appendingPathComponent("build")
      .appendingPathComponent("bin")
    try FileManager.default.createDirectory(at: binaryDir, withIntermediateDirectories: true)

    let pidFile = root.appendingPathComponent("\(slug).pid")
    let cwdFile = root.appendingPathComponent("\(slug).cwd")
    let wrapper = binaryDir.appendingPathComponent("holon-helper")
    let delayLine =
      launchDelaySeconds > 0
      ? "sleep \(String(format: "%.3f", launchDelaySeconds))\n"
      : ""
    let script = """
      #!/bin/sh
      printf '%s\n' "$$" > \(shellQuote(pidFile.path))
      pwd > \(shellQuote(cwdFile.path))
      \(delayLine)exec \(shellQuote(helperExecutable.path)) --slug \(shellQuote(slug)) "$@"
      """
    try script.write(to: wrapper, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: wrapper.path)

    try writeHolonProto(
      to: holonDir.appendingPathComponent("holon.proto"),
      packageName: "connect.v1",
      uuid: "\(slug)-uuid",
      givenName: slug,
      familyName: "",
      composer: "connect-tests",
      kind: "service",
      buildRunner: "go",
      buildMain: "Tests/HolonsTests/Fixtures/connect-helper-go/main.go",
      artifactBinary: "holon-helper"
    )

    return Fixture(
      slug: slug,
      pidFile: pidFile,
      portFile:
        root
        .appendingPathComponent(".op")
        .appendingPathComponent("run")
        .appendingPathComponent("\(slug).port"),
      cwdFile: cwdFile,
      holonDir: holonDir
    )
  }

  func makePackageBinaryFixture(slug: String) throws -> Fixture {
    let packageRoot = try makePackageManifestOnlyFixture(
      slug: slug,
      runner: "go-module",
      entrypoint: slug,
      architectures: [testCurrentArchDirectory()],
      hasDist: false,
      hasSource: false
    )
    let binary = packageRoot
      .appendingPathComponent("bin", isDirectory: true)
      .appendingPathComponent(testCurrentArchDirectory(), isDirectory: true)
      .appendingPathComponent(slug)
    try FileManager.default.createDirectory(at: binary.deletingLastPathComponent(), withIntermediateDirectories: true)

    let pidFile = root.appendingPathComponent("\(slug).pid")
    let cwdFile = root.appendingPathComponent("\(slug).cwd")
    let script = """
      #!/bin/sh
      printf '%s\n' "$$" > \(shellQuote(pidFile.path))
      pwd > \(shellQuote(cwdFile.path))
      exec \(shellQuote(helperExecutable.path)) --slug \(shellQuote(slug)) "$@"
      """
    try script.write(to: binary, atomically: true, encoding: .utf8)
    try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: binary.path)

    return Fixture(
      slug: slug,
      pidFile: pidFile,
      portFile:
        root
        .appendingPathComponent(".op")
        .appendingPathComponent("run")
        .appendingPathComponent("\(slug).port"),
      cwdFile: cwdFile,
      holonDir: packageRoot
    )
  }

  func makePackageDistFixture(slug: String, runner: String, entrypoint: String) throws -> URL {
    let packageRoot = try makePackageManifestOnlyFixture(
      slug: slug,
      runner: runner,
      entrypoint: entrypoint,
      architectures: [],
      hasDist: true,
      hasSource: false
    )
    let distEntry = packageRoot.appendingPathComponent("dist/\(entrypoint)")
    try FileManager.default.createDirectory(at: distEntry.deletingLastPathComponent(), withIntermediateDirectories: true)
    try "print('hello')\n".write(to: distEntry, atomically: true, encoding: .utf8)
    return packageRoot
  }

  func makePackageGitFixture(slug: String) throws -> URL {
    let packageRoot = try makePackageManifestOnlyFixture(
      slug: slug,
      runner: "go-module",
      entrypoint: slug,
      architectures: [],
      hasDist: false,
      hasSource: true
    )
    let gitRoot = packageRoot.appendingPathComponent("git", isDirectory: true)
    try FileManager.default.createDirectory(
      at: gitRoot.appendingPathComponent("api/v1", isDirectory: true),
      withIntermediateDirectories: true
    )
    try FileManager.default.createDirectory(
      at: gitRoot.appendingPathComponent("cmd/daemon", isDirectory: true),
      withIntermediateDirectories: true
    )
    try "module example.com/package-git\n\ngo 1.25.1\n".write(
      to: gitRoot.appendingPathComponent("go.mod"),
      atomically: true,
      encoding: .utf8
    )
    try "package main\nfunc main() {}\n".write(
      to: gitRoot.appendingPathComponent("cmd/daemon/main.go"),
      atomically: true,
      encoding: .utf8
    )
    try writeHolonProto(
      to: gitRoot.appendingPathComponent("api/v1/holon.proto"),
      packageName: "connect.v1",
      uuid: "\(slug)-uuid",
      givenName: slug,
      familyName: "",
      composer: "connect-tests",
      kind: "service",
      buildRunner: "go-module",
      buildMain: "./cmd/daemon",
      artifactBinary: slug
    )
    return packageRoot
  }

  func makePackageManifestOnlyFixture(
    slug: String,
    runner: String,
    entrypoint: String,
    architectures: [String],
    hasDist: Bool,
    hasSource: Bool
  ) throws -> URL {
    let packageRoot = root
      .appendingPathComponent(".op", isDirectory: true)
      .appendingPathComponent("build", isDirectory: true)
      .appendingPathComponent("\(slug).holon", isDirectory: true)
    try FileManager.default.createDirectory(at: packageRoot, withIntermediateDirectories: true)

    let architectureList = architectures.map { "\"\($0)\"" }.joined(separator: ", ")
    let data = """
      {
        "schema": "holon-package/v1",
        "slug": "\(slug)",
        "uuid": "\(slug)-uuid",
        "identity": {
          "given_name": "\(slug)",
          "family_name": ""
        },
        "lang": "go",
        "runner": "\(runner)",
        "status": "draft",
        "kind": "native",
        "entrypoint": "\(entrypoint)",
        "architectures": [\(architectureList)],
        "has_dist": \(hasDist ? "true" : "false"),
        "has_source": \(hasSource ? "true" : "false")
      }
      """
    try data.write(to: packageRoot.appendingPathComponent(".holon.json"), atomically: true, encoding: .utf8)
    return packageRoot
  }
}

private struct RunningConnectHelperServer {
  let process: Process
  let stdout: Pipe
  let stderr: Pipe
  let uri: String

  func stop() {
    if process.isRunning {
      process.terminate()
      let deadline = Date().addingTimeInterval(2.0)
      while process.isRunning && Date() < deadline {
        Thread.sleep(forTimeInterval: 0.05)
      }
      if process.isRunning {
        process.interrupt()
      }
    }
  }
}

private func makeSandbox(prefix: String) throws -> ConnectSandbox {
  let root = FileManager.default.temporaryDirectory
    .appendingPathComponent("\(prefix)-\(UUID().uuidString)", isDirectory: true)
  try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)

  let packageRoot = URL(fileURLWithPath: #filePath)
    .deletingLastPathComponent()
    .deletingLastPathComponent()
    .deletingLastPathComponent()
  let goBinary = try resolveGoBinary()
  let helperExecutable = root.appendingPathComponent("connect-helper")

  let build = Process()
  build.executableURL = URL(fileURLWithPath: goBinary)
  build.arguments = [
    "build",
    "-o",
    helperExecutable.path,
    packageRoot
      .appendingPathComponent("Tests")
      .appendingPathComponent("HolonsTests")
      .appendingPathComponent("Fixtures")
      .appendingPathComponent("connect-helper-go")
      .appendingPathComponent("main.go")
      .path,
  ]
  build.currentDirectoryURL =
    packageRoot
    .deletingLastPathComponent()
    .appendingPathComponent("go-holons")
  let buildStdout = Pipe()
  let buildStderr = Pipe()
  build.standardOutput = buildStdout
  build.standardError = buildStderr
  try build.run()
  build.waitUntilExit()
  guard build.terminationStatus == 0 else {
    let stderr =
      String(data: buildStderr.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
    throw XCTSkip("failed to build connect helper: \(stderr)")
  }

  return ConnectSandbox(
    root: root,
    goBinary: goBinary,
    helperSource:
      packageRoot
      .appendingPathComponent("Tests")
      .appendingPathComponent("HolonsTests")
      .appendingPathComponent("Fixtures")
      .appendingPathComponent("connect-helper-go")
      .appendingPathComponent("main.go"),
    goModuleRoot:
      packageRoot
      .deletingLastPathComponent()
      .appendingPathComponent("go-holons"),
    helperExecutable: helperExecutable
  )
}

private func writeEchoProto(into holonDir: URL) throws {
  let protoDir = holonDir.appendingPathComponent("protos/echo/v1", isDirectory: true)
  try FileManager.default.createDirectory(at: protoDir, withIntermediateDirectories: true)
  try """
  syntax = "proto3";
  package echo.v1;

  service Echo {
    rpc Ping(PingRequest) returns (PingResponse);
  }

  message PingRequest {
    string message = 1;
  }

  message PingResponse {
    string message = 1;
  }
  """.write(to: protoDir.appendingPathComponent("echo.proto"), atomically: true, encoding: .utf8)
}

private func startConnectHelperServer(slug: String, listen: String) throws
  -> RunningConnectHelperServer
{
  let sandbox = try makeSandbox(prefix: "connect-direct")
  let process = Process()
  process.executableURL = sandbox.helperExecutable
  process.arguments = [
    "--slug",
    slug,
    "--listen",
    listen,
  ]

  let stdout = Pipe()
  let stderr = Pipe()
  process.standardOutput = stdout
  process.standardError = stderr

  try process.run()

  let uri = try readStartupLine(
    from: stdout.fileHandleForReading,
    stderr: stderr.fileHandleForReading,
    process: process,
    timeout: 5.0
  )

  return RunningConnectHelperServer(process: process, stdout: stdout, stderr: stderr, uri: uri)
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

private func expectedUnixSocketPath(slug: String, portFile: String) -> String {
  String(expectedUnixSocketURI(slug: slug, portFile: portFile).dropFirst("unix://".count))
}

private func expectedUnixSocketURI(slug: String, portFile: String) -> String {
  let hash = testFNV1a64(Array(portFile.utf8))
  _ = slug
  return "unix://\(testTemporaryDirectoryPath())/h\(String(format: "%08llx", hash & 0xffffffff)).s"
}

private func testTemporaryDirectoryPath() -> String {
  var tempDir = NSTemporaryDirectory()
  if tempDir.hasSuffix("/") {
    tempDir.removeLast()
  }
  return tempDir
}

private func testFNV1a64(_ bytes: [UInt8]) -> UInt64 {
  var hash: UInt64 = 0xcbf29ce484222325
  for byte in bytes {
    hash ^= UInt64(byte)
    hash &*= 0x100000001b3
  }
  return hash
}

private func readStartupLine(
  from stdout: FileHandle,
  stderr: FileHandle,
  process: Process,
  timeout: TimeInterval
) throws -> String {
  let deadline = Date().addingTimeInterval(timeout)
  var stdoutBuffer = Data()
  var stderrBuffer = Data()

  while Date() < deadline {
    if process.isRunning == false {
      let stderrText = String(data: stderr.availableData + stderrBuffer, encoding: .utf8) ?? ""
      throw XCTSkip("connect helper exited before startup: \(stderrText)")
    }

    let chunk = stdout.availableData
    if !chunk.isEmpty {
      stdoutBuffer.append(chunk)
      if let newline = stdoutBuffer.firstIndex(of: 0x0A) {
        let lineData = stdoutBuffer.prefix(upTo: newline)
        if let line = String(data: lineData, encoding: .utf8), !line.isEmpty {
          return line
        }
      }
    }

    let stderrChunk = stderr.availableData
    if !stderrChunk.isEmpty {
      stderrBuffer.append(stderrChunk)
    }

    Thread.sleep(forTimeInterval: 0.05)
  }

  let stderrText = String(data: stderrBuffer, encoding: .utf8) ?? ""
  throw XCTSkip("timed out waiting for connect helper startup: \(stderrText)")
}

private func resolveGoBinary() throws -> String {
  let environment = ProcessInfo.processInfo.environment
  if let configured = environment["GO_BIN"]?.trimmingCharacters(in: .whitespacesAndNewlines),
    !configured.isEmpty
  {
    return configured
  }

  let process = Process()
  process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
  process.arguments = ["which", "go"]
  let stdout = Pipe()
  process.standardOutput = stdout
  process.standardError = Pipe()

  do {
    try process.run()
  } catch {
    throw XCTSkip("go is required for connect tests")
  }

  process.waitUntilExit()
  guard process.terminationStatus == 0,
    let path = String(data: stdout.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8)?
      .trimmingCharacters(in: .whitespacesAndNewlines),
    !path.isEmpty
  else {
    throw XCTSkip("go is required for connect tests")
  }
  return path
}

private func describeSlug(_ channel: GRPCChannel, timeout: TimeInterval) throws -> String {
  let call =
    channel.makeUnaryCall(
      path: "/holons.v1.HolonMeta/Describe",
      request: ConnectTestRawBytesPayload(),
      callOptions: CallOptions(
        timeLimit: .timeout(.nanoseconds(Int64(timeout * 1_000_000_000)))
      )
    ) as UnaryCall<ConnectTestRawBytesPayload, ConnectTestRawBytesPayload>
  let response = try call.response.wait()
  let describe = try Holons_V1_DescribeResponse(serializedBytes: response.data)
  let identity = describe.manifest.identity
  var holonIdentity = HolonIdentity()
  holonIdentity.givenName = identity.givenName
  holonIdentity.familyName = identity.familyName
  return holonIdentity.slug
}

private struct ConnectTestRawBytesPayload: GRPCPayload {
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

private func parseTopLevelStringField(_ data: Data, fieldNumber: UInt64) throws -> String {
  var index = data.startIndex

  while index < data.endIndex {
    let key = try decodeVarint(data, index: &index)
    let wireType = key & 0x07
    let number = key >> 3

    if wireType == 2 {
      let length = try decodeVarint(data, index: &index)
      guard let end = data.index(index, offsetBy: Int(length), limitedBy: data.endIndex) else {
        throw ConnectError.ioFailure("invalid protobuf payload")
      }
      let slice = data[index..<end]
      if number == fieldNumber, let value = String(data: slice, encoding: .utf8) {
        return value
      }
      index = end
      continue
    }

    try skipField(wireType: wireType, data: data, index: &index)
  }

  throw ConnectError.ioFailure("missing field \(fieldNumber)")
}

private func decodeVarint(_ data: Data, index: inout Data.Index) throws -> UInt64 {
  var value: UInt64 = 0
  var shift: UInt64 = 0

  while index < data.endIndex {
    let byte = UInt64(data[index])
    data.formIndex(after: &index)

    value |= (byte & 0x7f) << shift
    if byte & 0x80 == 0 {
      return value
    }

    shift += 7
    if shift >= 64 {
      break
    }
  }

  throw ConnectError.ioFailure("invalid varint")
}

private func skipField(wireType: UInt64, data: Data, index: inout Data.Index) throws {
  switch wireType {
  case 0:
    _ = try decodeVarint(data, index: &index)
  case 1:
    guard let end = data.index(index, offsetBy: 8, limitedBy: data.endIndex) else {
      throw ConnectError.ioFailure("invalid fixed64 field")
    }
    index = end
  case 2:
    let length = try decodeVarint(data, index: &index)
    guard let end = data.index(index, offsetBy: Int(length), limitedBy: data.endIndex) else {
      throw ConnectError.ioFailure("invalid length-delimited field")
    }
    index = end
  case 5:
    guard let end = data.index(index, offsetBy: 4, limitedBy: data.endIndex) else {
      throw ConnectError.ioFailure("invalid fixed32 field")
    }
    index = end
  default:
    throw ConnectError.ioFailure("unsupported wire type \(wireType)")
  }
}

private func waitForPID(at path: URL, timeout: TimeInterval = 5.0) throws -> Int32 {
  let deadline = Date().addingTimeInterval(timeout)
  while Date() < deadline {
    if let raw = try? String(contentsOf: path, encoding: .utf8).trimmingCharacters(
      in: .whitespacesAndNewlines),
      let pid = Int32(raw),
      pid > 0
    {
      return pid
    }
    Thread.sleep(forTimeInterval: 0.025)
  }
  throw ConnectError.ioFailure("timed out waiting for pid file \(path.path)")
}

private func waitForFileContents(at path: URL, timeout: TimeInterval = 5.0) throws -> String {
  let deadline = Date().addingTimeInterval(timeout)
  while Date() < deadline {
    if let raw = try? String(contentsOf: path, encoding: .utf8),
      !raw.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    {
      return raw
    }
    Thread.sleep(forTimeInterval: 0.025)
  }
  throw ConnectError.ioFailure("timed out waiting for file \(path.path)")
}

private func pidExists(_ pid: Int32) -> Bool {
  kill(pid, 0) == 0
}

private func waitForProcessExit(_ pid: Int32, timeout: TimeInterval = 2.0) throws {
  let deadline = Date().addingTimeInterval(timeout)
  while Date() < deadline {
    if !pidExists(pid) {
      return
    }
    Thread.sleep(forTimeInterval: 0.025)
  }
  throw ConnectError.ioFailure("process \(pid) did not exit")
}

private func terminateProcess(_ pid: Int32) {
  guard pidExists(pid) else {
    return
  }
  _ = kill(pid, SIGTERM)
  let deadline = Date().addingTimeInterval(2.0)
  while pidExists(pid) && Date() < deadline {
    Thread.sleep(forTimeInterval: 0.025)
  }
  if pidExists(pid) {
    _ = kill(pid, SIGKILL)
  }
}

private func reserveLoopbackPort() throws -> Int {
  let listener = try TCPRuntimeListener(host: "127.0.0.1", port: 0)
  defer { try? listener.close() }
  return listener.boundPort
}

private func shellQuote(_ value: String) -> String {
  "'" + value.replacingOccurrences(of: "'", with: "'\"'\"'") + "'"
}
