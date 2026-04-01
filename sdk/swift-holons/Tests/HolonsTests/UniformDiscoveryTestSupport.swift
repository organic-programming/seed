import Foundation
import XCTest

@testable import Holons

struct UniformSandbox {
  let root: URL
  let bundleResources: URL
  let opHome: URL
  let opBin: URL
  let cache: URL

  init(prefix: String) throws {
    self.root = FileManager.default.temporaryDirectory
      .appendingPathComponent("\(prefix)-\(UUID().uuidString)", isDirectory: true)
    self.bundleResources = root.appendingPathComponent("App.app/Contents/Resources", isDirectory: true)
    self.opHome = root.appendingPathComponent("runtime", isDirectory: true)
    self.opBin = opHome.appendingPathComponent("bin", isDirectory: true)
    self.cache = opHome.appendingPathComponent("cache", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
  }

  var cwdPackageRoot: URL {
    root.appendingPathComponent("workspace", isDirectory: true)
  }

  var builtRoot: URL {
    cwdPackageRoot.appendingPathComponent(".op/build", isDirectory: true)
  }
}

final class UniformDiscoveryContext {
  private let previousDirectory = FileManager.default.currentDirectoryPath
  private let previousCurrentRoot = discoverCurrentRootURLProvider
  private let previousEnvironment = discoverEnvironmentProvider
  private let previousBundle = discoverBundleResourceURLProvider
  private let previousBridge = discoverSourceBridge

  init(
    root: URL,
    opHome: URL,
    opBin: URL,
    bundleResources: URL? = nil,
    sourceBridge: DiscoverSourceBridge? = nil
  ) {
    _ = FileManager.default.changeCurrentDirectoryPath(root.path)
    discoverCurrentRootURLProvider = { root }
    discoverEnvironmentProvider = {
      [
        "HOME": root.path,
        "OPPATH": opHome.path,
        "OPBIN": opBin.path,
      ]
    }
    discoverBundleResourceURLProvider = { bundleResources }
    discoverSourceBridge = sourceBridge
  }

  func restore() {
    _ = FileManager.default.changeCurrentDirectoryPath(previousDirectory)
    discoverCurrentRootURLProvider = previousCurrentRoot
    discoverEnvironmentProvider = previousEnvironment
    discoverBundleResourceURLProvider = previousBundle
    discoverSourceBridge = previousBridge
  }
}

func writeUniformPackage(
  at root: URL,
  slug: String,
  uuid: String,
  givenName: String? = nil,
  familyName: String = "",
  aliases: [String] = [],
  runner: String = "go-module",
  transport: String = "",
  entrypoint: String? = nil,
  architectures: [String]? = nil,
  hasDist: Bool = false,
  hasSource: Bool = false
) throws {
  try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
  let architectureList = (architectures ?? [uniformCurrentArchDirectory()])
    .map { "\"\($0)\"" }
    .joined(separator: ", ")
  let aliasList = aliases.map { "\"\($0)\"" }.joined(separator: ", ")
  let data = """
    {
      "schema": "holon-package/v1",
      "slug": "\(slug)",
      "uuid": "\(uuid)",
      "identity": {
        "given_name": "\(givenName ?? slug)",
        "family_name": "\(familyName)",
        "aliases": [\(aliasList)]
      },
      "lang": "swift",
      "runner": "\(runner)",
      "status": "draft",
      "kind": "native",
      "transport": "\(transport)",
      "entrypoint": "\(entrypoint ?? slug)",
      "architectures": [\(architectureList)],
      "has_dist": \(hasDist ? "true" : "false"),
      "has_source": \(hasSource ? "true" : "false")
    }
    """
  try data.write(to: root.appendingPathComponent(".holon.json"), atomically: true, encoding: .utf8)
}

func makeSourceRef(
  directory: URL,
  slug: String,
  uuid: String,
  aliases: [String] = []
) -> HolonRef {
  HolonRef(
    url: directory.standardizedFileURL.absoluteString,
    info: HolonInfo(
      slug: slug,
      uuid: uuid,
      identity: IdentityInfo(givenName: slug, aliases: aliases),
      lang: "swift",
      runner: "swift-package",
      status: "draft",
      kind: "native",
      transport: "stdio",
      entrypoint: slug,
      architectures: [],
      hasDist: false,
      hasSource: true
    )
  )
}

func makeServablePackage(
  at packageRoot: URL,
  slug: String,
  uuid: String,
  transport: String,
  writeJSON: Bool
) throws {
  try FileManager.default.createDirectory(at: packageRoot, withIntermediateDirectories: true)

  let sourceRoot = packageRoot.appendingPathComponent("describe-source", isDirectory: true)
  try writeHolonProto(
    to: sourceRoot.appendingPathComponent("holon.proto"),
    packageName: "uniform.v1",
    uuid: uuid,
    givenName: slug,
    familyName: "",
    lang: "swift",
    kind: "native",
    buildRunner: "swift-package",
    artifactBinary: slug
  )

  let response = try Describe.buildResponse(protoDir: sourceRoot.path)
  let payload = try response.serializedData().base64EncodedString()
  let executable = try uniformFindBuiltExecutable(
    named: "protoless-describe-fixture",
    under: holonsSwiftBuildRoot()
  )

  let binary = packageRoot
    .appendingPathComponent("bin", isDirectory: true)
    .appendingPathComponent(uniformCurrentArchDirectory(), isDirectory: true)
    .appendingPathComponent(slug)
  try FileManager.default.createDirectory(at: binary.deletingLastPathComponent(), withIntermediateDirectories: true)

  let script = """
    #!/bin/sh
    LISTEN="tcp://127.0.0.1:0"
    if [ "$1" = "serve" ] && [ "$2" = "--listen" ] && [ -n "$3" ]; then
      LISTEN="$3"
      shift 3
    fi
    export PROTOLESS_STATIC_DESCRIBE_BASE64=\(uniformShellQuote(payload))
    exec \(uniformShellQuote(executable.path)) "$LISTEN" "$@"
    """
  try script.write(to: binary, atomically: true, encoding: .utf8)
  try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: binary.path)

  if writeJSON {
    try writeUniformPackage(
      at: packageRoot,
      slug: slug,
      uuid: uuid,
      runner: "swift-package",
      transport: transport,
      entrypoint: slug
    )
  }
}

func uniformFindBuiltExecutable(named name: String, under root: URL) throws -> URL {
  guard
    let enumerator = FileManager.default.enumerator(
      at: root,
      includingPropertiesForKeys: [.isRegularFileKey],
      options: [],
      errorHandler: { _, _ in true }
    )
  else {
    throw XCTSkip("failed to enumerate built executables under \(root.path)")
  }

  while let candidate = enumerator.nextObject() as? URL {
    if candidate.lastPathComponent == name, FileManager.default.isExecutableFile(atPath: candidate.path) {
      return candidate
    }
  }

  throw XCTSkip("failed to locate built executable \(name)")
}

func holonsSwiftBuildRoot() -> URL {
  if let configured = ProcessInfo.processInfo.environment["HOLONS_SWIFT_BUILD_ROOT"],
     !configured.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
  {
    return URL(fileURLWithPath: configured, isDirectory: true)
  }
  return CertificationCLI.packageRoot().appendingPathComponent(".build", isDirectory: true)
}

func uniformCurrentArchDirectory() -> String {
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

func uniformShellQuote(_ value: String) -> String {
  "'" + value.replacingOccurrences(of: "'", with: "'\"'\"'") + "'"
}
