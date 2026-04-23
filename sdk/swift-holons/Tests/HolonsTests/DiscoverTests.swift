import XCTest

@testable import Holons

final class DiscoverContractTests: XCTestCase {
  func testDiscoverAllLayers() throws {
    let sandbox = try UniformSandbox(prefix: "discover-all")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let siblings = sandbox.bundleResources.appendingPathComponent("Holons/siblings.holon", isDirectory: true)
    let cwd = sandbox.cwdPackageRoot.appendingPathComponent("cwd.holon", isDirectory: true)
    let built = sandbox.builtRoot.appendingPathComponent("built.holon", isDirectory: true)
    let installed = sandbox.opBin.appendingPathComponent("installed.holon", isDirectory: true)
    let cached = sandbox.cache.appendingPathComponent("deps/cached.holon", isDirectory: true)
    let sourceDir = sandbox.cwdPackageRoot.appendingPathComponent("sources/source-offload", isDirectory: true)

    try writeUniformPackage(at: siblings, slug: "siblings", uuid: "uuid-siblings")
    try writeUniformPackage(at: cwd, slug: "cwd", uuid: "uuid-cwd")
    try writeUniformPackage(at: built, slug: "built", uuid: "uuid-built")
    try writeUniformPackage(at: installed, slug: "installed", uuid: "uuid-installed")
    try writeUniformPackage(at: cached, slug: "cached", uuid: "uuid-cached")
    try FileManager.default.createDirectory(at: sourceDir, withIntermediateDirectories: true)

    let context = makeContext(
      sandbox,
      bundleResources: sandbox.bundleResources,
      sourceBridge: { _, _, _, _, _ in
        DiscoverResult(found: [makeSourceRef(directory: sourceDir, slug: "source-offload", uuid: "uuid-source")])
      }
    )
    defer { context.restore() }

    let result = Discover(
      scope: LOCAL,
      expression: nil,
      root: sandbox.cwdPackageRoot.path,
      specifiers: ALL,
      limit: NO_LIMIT,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(Set(slugs(in: result)), Set(["siblings", "cwd", "source-offload", "built", "installed", "cached"]))
  }

  func testFilterBySpecifiers() throws {
    let sandbox = try UniformSandbox(prefix: "discover-filter")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(
      at: sandbox.cwdPackageRoot.appendingPathComponent("cwd-only.holon", isDirectory: true),
      slug: "cwd-only",
      uuid: "uuid-cwd-only"
    )
    try writeUniformPackage(
      at: sandbox.builtRoot.appendingPathComponent("built-only.holon", isDirectory: true),
      slug: "built-only",
      uuid: "uuid-built-only"
    )

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(
      scope: LOCAL,
      expression: nil,
      root: sandbox.cwdPackageRoot.path,
      specifiers: BUILT,
      limit: NO_LIMIT,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["built-only"])
  }

  func testMatchBySlug() throws {
    let sandbox = try UniformSandbox(prefix: "discover-slug")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(
      at: sandbox.cwdPackageRoot.appendingPathComponent("slug-target.holon", isDirectory: true),
      slug: "slug-target",
      uuid: "uuid-slug-target"
    )

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(
      scope: LOCAL,
      expression: "slug-target",
      root: sandbox.cwdPackageRoot.path,
      specifiers: CWD,
      limit: NO_LIMIT,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["slug-target"])
  }

  func testMatchByAlias() throws {
    let sandbox = try UniformSandbox(prefix: "discover-alias")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(
      at: sandbox.cwdPackageRoot.appendingPathComponent("alias-target.holon", isDirectory: true),
      slug: "alias-target",
      uuid: "uuid-alias-target",
      aliases: ["op"]
    )

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(
      scope: LOCAL,
      expression: "op",
      root: sandbox.cwdPackageRoot.path,
      specifiers: CWD,
      limit: NO_LIMIT,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["alias-target"])
  }

  func testMatchByUUIDPrefix() throws {
    let sandbox = try UniformSandbox(prefix: "discover-uuid")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(
      at: sandbox.cwdPackageRoot.appendingPathComponent("uuid-target.holon", isDirectory: true),
      slug: "uuid-target",
      uuid: "12345678-aaaa-bbbb-cccc-1234567890ab"
    )

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(
      scope: LOCAL,
      expression: "12345678",
      root: sandbox.cwdPackageRoot.path,
      specifiers: CWD,
      limit: NO_LIMIT,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["uuid-target"])
  }

  func testMatchByPath() throws {
    let sandbox = try UniformSandbox(prefix: "discover-path")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageDir = sandbox.cwdPackageRoot.appendingPathComponent("nested/path-target.holon", isDirectory: true)
    try writeUniformPackage(at: packageDir, slug: "path-target", uuid: "uuid-path-target")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(
      scope: LOCAL,
      expression: "nested/path-target.holon",
      root: sandbox.cwdPackageRoot.path,
      specifiers: ALL,
      limit: NO_LIMIT,
      timeout: NO_TIMEOUT
    )

    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["path-target"])
  }

  func testLimitOne() throws {
    let sandbox = try UniformSandbox(prefix: "discover-limit-one")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("one.holon", isDirectory: true), slug: "one", uuid: "uuid-one")
    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("two.holon", isDirectory: true), slug: "two", uuid: "uuid-two")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CWD, limit: 1, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(result.found.count, 1)
  }

  func testLimitZeroMeansUnlimited() throws {
    let sandbox = try UniformSandbox(prefix: "discover-limit-zero")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("one.holon", isDirectory: true), slug: "one", uuid: "uuid-one")
    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("two.holon", isDirectory: true), slug: "two", uuid: "uuid-two")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CWD, limit: 0, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(Set(slugs(in: result)), Set(["one", "two"]))
  }

  func testNegativeLimitReturnsEmpty() throws {
    let sandbox = try UniformSandbox(prefix: "discover-limit-negative")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("one.holon", isDirectory: true), slug: "one", uuid: "uuid-one")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CWD, limit: -1, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertTrue(result.found.isEmpty)
  }

  func testInvalidSpecifiers() {
    let result = Discover(scope: LOCAL, expression: nil, root: nil, specifiers: 0x40, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertEqual(result.found.count, 0)
    XCTAssertNotNil(result.error)
  }

  func testSpecifiersZeroTreatedAsAll() throws {
    let sandbox = try UniformSandbox(prefix: "discover-specifiers-zero")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("cwd.holon", isDirectory: true), slug: "cwd", uuid: "uuid-cwd")
    try writeUniformPackage(at: sandbox.builtRoot.appendingPathComponent("built.holon", isDirectory: true), slug: "built", uuid: "uuid-built")

    let context = makeContext(
      sandbox,
      sourceBridge: { _, _, _, _, _ in DiscoverResult(found: []) }
    )
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: 0, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(Set(slugs(in: result)), Set(["cwd", "built"]))
  }

  func testNullExpressionReturnsAll() throws {
    let sandbox = try UniformSandbox(prefix: "discover-null-expression")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("one.holon", isDirectory: true), slug: "one", uuid: "uuid-one")
    try writeUniformPackage(at: sandbox.builtRoot.appendingPathComponent("two.holon", isDirectory: true), slug: "two", uuid: "uuid-two")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CWD | BUILT, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(Set(slugs(in: result)), Set(["one", "two"]))
  }

  func testMissingExpressionReturnsEmpty() throws {
    let sandbox = try UniformSandbox(prefix: "discover-missing-expression")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("one.holon", isDirectory: true), slug: "one", uuid: "uuid-one")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: "   ", root: sandbox.cwdPackageRoot.path, specifiers: CWD, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertTrue(result.found.isEmpty)
  }

  func testExcludedDirsSkipped() throws {
    let sandbox = try UniformSandbox(prefix: "discover-excluded")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("valid.holon", isDirectory: true), slug: "valid", uuid: "uuid-valid")
    for path in [".git/ignored.holon", ".op/ignored.holon", "node_modules/ignored.holon", "vendor/ignored.holon", "build/ignored.holon", "testdata/ignored.holon", ".cache/ignored.holon"] {
      try writeUniformPackage(
        at: sandbox.cwdPackageRoot.appendingPathComponent(path, isDirectory: true),
        slug: "ignored",
        uuid: "uuid-\(path.replacingOccurrences(of: "/", with: "-"))"
      )
    }

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CWD, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["valid"])
  }

  func testDeduplicateByUUID() throws {
    let sandbox = try UniformSandbox(prefix: "discover-dedupe")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let duplicateUUID = "uuid-duplicate"
    let cwdDir = sandbox.cwdPackageRoot.appendingPathComponent("duplicate-cwd.holon", isDirectory: true)
    let builtDir = sandbox.builtRoot.appendingPathComponent("duplicate-built.holon", isDirectory: true)
    try writeUniformPackage(at: cwdDir, slug: "duplicate-cwd", uuid: duplicateUUID)
    try writeUniformPackage(at: builtDir, slug: "duplicate-built", uuid: duplicateUUID)

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CWD | BUILT, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(result.found.count, 1)
    // `.holon` packages are directories on disk, but Discover emits their URL
    // with `isDirectory: false` (no trailing slash). `standardizedFileURL`
    // on newer macOS keeps the trailing slash, older versions strip it.
    // Compare trailing-slash-insensitively to be robust across hosts.
    XCTAssertEqual(
      trimmingTrailingSlash(result.found.first?.url),
      trimmingTrailingSlash(cwdDir.standardizedFileURL.absoluteString)
    )
  }

  func testHolonJSONFastPath() throws {
    let sandbox = try UniformSandbox(prefix: "discover-fast-path")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageDir = sandbox.builtRoot.appendingPathComponent("fast-path.holon", isDirectory: true)
    try writeUniformPackage(at: packageDir, slug: "fast-path", uuid: "uuid-fast-path", transport: "tcp")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: BUILT, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(result.found.count, 1)
    XCTAssertEqual(result.found.first?.info?.slug, "fast-path")
    XCTAssertEqual(result.found.first?.info?.transport, "tcp")
  }

  func testDescribeFallbackWhenHolonJSONMissing() throws {
    let sandbox = try UniformSandbox(prefix: "discover-describe-fallback")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let packageDir = sandbox.builtRoot.appendingPathComponent("describe-fallback.holon", isDirectory: true)
    try makeServablePackage(at: packageDir, slug: "describe-fallback", uuid: "uuid-describe-fallback", transport: "tcp", writeJSON: false)

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: BUILT, limit: NO_LIMIT, timeout: 5_000)
    XCTAssertNil(result.error)
    XCTAssertEqual(result.found.count, 1)
    XCTAssertEqual(result.found.first?.info?.slug, "describe-fallback")
    XCTAssertNil(result.found.first?.error)
  }

  func testSiblingsLayer() throws {
    let sandbox = try UniformSandbox(prefix: "discover-siblings")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(
      at: sandbox.bundleResources.appendingPathComponent("Holons/sibling-only.holon", isDirectory: true),
      slug: "sibling-only",
      uuid: "uuid-sibling-only"
    )

    let context = makeContext(sandbox, bundleResources: sandbox.bundleResources)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: SIBLINGS, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["sibling-only"])
  }

  func testSourceLayerOffloadsToLocalOp() throws {
    let sandbox = try UniformSandbox(prefix: "discover-source-offload")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    let sourceDir = sandbox.cwdPackageRoot.appendingPathComponent("sources/source-offload", isDirectory: true)
    try FileManager.default.createDirectory(at: sourceDir, withIntermediateDirectories: true)

    var capturedExpression: String?
    var capturedRoot: String?
    var capturedSpecifiers: Int?

    let context = makeContext(
      sandbox,
      sourceBridge: { expression, root, specifiers, _, _ in
        capturedExpression = expression
        capturedRoot = root
        capturedSpecifiers = specifiers
        return DiscoverResult(found: [makeSourceRef(directory: sourceDir, slug: "source-offload", uuid: "uuid-source")])
      }
    )
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: "source-offload", root: sandbox.cwdPackageRoot.path, specifiers: SOURCE, limit: NO_LIMIT, timeout: 5_000)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["source-offload"])
    XCTAssertEqual(capturedExpression, "source-offload")
    XCTAssertEqual(capturedRoot, sandbox.cwdPackageRoot.path)
    XCTAssertEqual(capturedSpecifiers, SOURCE)
  }

  func testBuiltLayer() throws {
    let sandbox = try UniformSandbox(prefix: "discover-built")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.builtRoot.appendingPathComponent("built-only.holon", isDirectory: true), slug: "built-only", uuid: "uuid-built-only")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: BUILT, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["built-only"])
  }

  func testInstalledLayer() throws {
    let sandbox = try UniformSandbox(prefix: "discover-installed")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.opBin.appendingPathComponent("installed-only.holon", isDirectory: true), slug: "installed-only", uuid: "uuid-installed-only")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: INSTALLED, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["installed-only"])
  }

  func testCachedLayer() throws {
    let sandbox = try UniformSandbox(prefix: "discover-cached")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cache.appendingPathComponent("deps/cached-only.holon", isDirectory: true), slug: "cached-only", uuid: "uuid-cached-only")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: sandbox.cwdPackageRoot.path, specifiers: CACHED, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["cached-only"])
  }

  func testNilRootDefaultsToCWD() throws {
    let sandbox = try UniformSandbox(prefix: "discover-nil-root")
    defer { try? FileManager.default.removeItem(at: sandbox.root) }

    try writeUniformPackage(at: sandbox.cwdPackageRoot.appendingPathComponent("cwd-default.holon", isDirectory: true), slug: "cwd-default", uuid: "uuid-cwd-default")

    let context = makeContext(sandbox)
    defer { context.restore() }

    let result = Discover(scope: LOCAL, expression: nil, root: nil, specifiers: CWD, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNil(result.error)
    XCTAssertEqual(slugs(in: result), ["cwd-default"])
  }

  func testEmptyRootReturnsError() {
    let result = Discover(scope: LOCAL, expression: nil, root: "", specifiers: CWD, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNotNil(result.error)
  }

  func testUnsupportedScopeReturnsError() {
    let result = Discover(scope: PROXY, expression: nil, root: nil, specifiers: CWD, limit: NO_LIMIT, timeout: NO_TIMEOUT)
    XCTAssertNotNil(result.error)
  }
}

private func makeContext(
  _ sandbox: UniformSandbox,
  bundleResources: URL? = nil,
  sourceBridge: DiscoverSourceBridge? = nil
) -> UniformDiscoveryContext {
  try? FileManager.default.createDirectory(at: sandbox.cwdPackageRoot, withIntermediateDirectories: true)
  return UniformDiscoveryContext(
    root: sandbox.cwdPackageRoot,
    opHome: sandbox.opHome,
    opBin: sandbox.opBin,
    bundleResources: bundleResources,
    sourceBridge: sourceBridge
  )
}

private func slugs(in result: DiscoverResult) -> [String] {
  result.found.compactMap { $0.info?.slug }
}

private func trimmingTrailingSlash(_ value: String?) -> String? {
  guard let value else { return nil }
  return value.hasSuffix("/") ? String(value.dropLast()) : value
}
