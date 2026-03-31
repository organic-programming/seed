import XCTest

@testable import Holons

final class ConnectContractTests: XCTestCase {
  func testUnresolvableTarget() async throws {
    let sandbox = try UniformSandbox(prefix: "connect-contract-missing")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let context = makeConnectContext(sandbox)
    defer { context.restore() }

    let result = await connect(
      scope: LOCAL,
      expression: "missing-target",
      root: sandbox.cwdPackageRoot.path,
      specifiers: BUILT,
      timeout: 5_000
    )

    XCTAssertNil(result.channel)
    XCTAssertNotNil(result.error)
  }

  func testReturnsConnectResult() async throws {
    let sandbox = try UniformSandbox(prefix: "connect-contract-result")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageDir = sandbox.builtRoot.appendingPathComponent("connect-result.holon", isDirectory: true)
    try makeServablePackage(
      at: packageDir,
      slug: "connect-result",
      uuid: "uuid-connect-result",
      transport: "tcp",
      writeJSON: true
    )

    let context = makeConnectContext(sandbox)
    defer { context.restore() }

    let result = await connect(
      scope: LOCAL,
      expression: "connect-result",
      root: sandbox.cwdPackageRoot.path,
      specifiers: BUILT,
      timeout: 5_000
    )
    defer { disconnect(result) }

    XCTAssertNil(result.error)
    XCTAssertNotNil(result.channel)

    let channel = try XCTUnwrap(result.channel)
    let response = try describeResponse(channel: channel, timeout: 2.0)
    XCTAssertEqual(response.manifest.identity.givenName, "connect-result")
  }

  func testPopulatesOrigin() async throws {
    let sandbox = try UniformSandbox(prefix: "connect-contract-origin")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageDir = sandbox.builtRoot.appendingPathComponent("connect-origin.holon", isDirectory: true)
    try makeServablePackage(
      at: packageDir,
      slug: "connect-origin",
      uuid: "uuid-connect-origin",
      transport: "tcp",
      writeJSON: true
    )

    let context = makeConnectContext(sandbox)
    defer { context.restore() }

    let result = await connect(
      scope: LOCAL,
      expression: "connect-origin",
      root: sandbox.cwdPackageRoot.path,
      specifiers: BUILT,
      timeout: 5_000
    )
    defer { disconnect(result) }

    XCTAssertNil(result.error)
    XCTAssertEqual(result.origin?.info?.slug, "connect-origin")
    XCTAssertTrue(result.origin?.url.hasPrefix("tcp://") == true)
  }

  func testDisconnectAcceptsConnectResult() async throws {
    let sandbox = try UniformSandbox(prefix: "connect-contract-disconnect")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageDir = sandbox.builtRoot.appendingPathComponent("connect-disconnect.holon", isDirectory: true)
    try makeServablePackage(
      at: packageDir,
      slug: "connect-disconnect",
      uuid: "uuid-connect-disconnect",
      transport: "tcp",
      writeJSON: true
    )

    let context = makeConnectContext(sandbox)
    defer { context.restore() }

    let result = await connect(
      scope: LOCAL,
      expression: "connect-disconnect",
      root: sandbox.cwdPackageRoot.path,
      specifiers: BUILT,
      timeout: 5_000
    )

    XCTAssertNotNil(result.channel)
    disconnect(result)
  }
}

private func makeConnectContext(_ sandbox: UniformSandbox) -> UniformDiscoveryContext {
  try? FileManager.default.createDirectory(at: sandbox.cwdPackageRoot, withIntermediateDirectories: true)
  return UniformDiscoveryContext(
    root: sandbox.cwdPackageRoot,
    opHome: sandbox.opHome,
    opBin: sandbox.opBin
  )
}
