import XCTest

@testable import Holons

final class ResolveContractTests: XCTestCase {
  func testKnownSlug() throws {
    let sandbox = try UniformSandbox(prefix: "resolve-known")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(
      at: sandbox.cwdPackageRoot.appendingPathComponent("resolve-known.holon", isDirectory: true),
      slug: "resolve-known",
      uuid: "uuid-resolve-known"
    )

    let context = makeResolveContext(sandbox)
    defer { context.restore() }

    let result = resolve(
      scope: LOCAL,
      expression: "resolve-known",
      root: sandbox.cwdPackageRoot.path,
      specifiers: CWD,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(result.ref?.info?.slug, "resolve-known")
  }

  func testMissingTarget() throws {
    let sandbox = try UniformSandbox(prefix: "resolve-missing")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let context = makeResolveContext(sandbox)
    defer { context.restore() }

    let result = resolve(
      scope: LOCAL,
      expression: "missing-target",
      root: sandbox.cwdPackageRoot.path,
      specifiers: CWD,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.ref)
    XCTAssertNotNil(result.error)
  }

  func testInvalidSpecifiers() throws {
    let sandbox = try UniformSandbox(prefix: "resolve-invalid-specifiers")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let context = makeResolveContext(sandbox)
    defer { context.restore() }

    let result = resolve(
      scope: LOCAL,
      expression: "anything",
      root: sandbox.cwdPackageRoot.path,
      specifiers: 0x40,
      timeout: NO_TIMEOUT
    )

    XCTAssertNotNil(result.error)
  }
}

private func makeResolveContext(_ sandbox: UniformSandbox) -> UniformDiscoveryContext {
  try? FileManager.default.createDirectory(at: sandbox.cwdPackageRoot, withIntermediateDirectories: true)
  return UniformDiscoveryContext(
    root: sandbox.cwdPackageRoot,
    opHome: sandbox.opHome,
    opBin: sandbox.opBin
  )
}
