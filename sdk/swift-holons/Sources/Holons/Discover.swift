import Foundation

public struct HolonBuild: Equatable {
  public var runner: String = ""
  public var main: String = ""

  public init() {}
}

public struct HolonArtifacts: Equatable {
  public var binary: String = ""
  public var primary: String = ""

  public init() {}
}

public struct HolonManifest: Equatable {
  public var kind: String = ""
  public var build = HolonBuild()
  public var artifacts = HolonArtifacts()

  public init() {}
}

public struct HolonEntry: Equatable {
  public var slug: String
  public var uuid: String
  public var dir: URL
  public var relativePath: String
  public var origin: String
  public var identity: HolonIdentity
  public var manifest: HolonManifest?
  public var sourceKind: String
  public var packageRoot: URL?
  public var runner: String
  public var transport: String
  public var entrypoint: String
  public var architectures: [String]
  public var hasDist: Bool
  public var hasSource: Bool

  public init(
    slug: String,
    uuid: String,
    dir: URL,
    relativePath: String,
    origin: String,
    identity: HolonIdentity,
    manifest: HolonManifest?,
    sourceKind: String = "source",
    packageRoot: URL? = nil,
    runner: String = "",
    transport: String = "",
    entrypoint: String = "",
    architectures: [String] = [],
    hasDist: Bool = false,
    hasSource: Bool = false
  ) {
    self.slug = slug
    self.uuid = uuid
    self.dir = dir
    self.relativePath = relativePath
    self.origin = origin
    self.identity = identity
    self.manifest = manifest
    self.sourceKind = sourceKind
    self.packageRoot = packageRoot
    self.runner = runner
    self.transport = transport
    self.entrypoint = entrypoint
    self.architectures = architectures
    self.hasDist = hasDist
    self.hasSource = hasSource
  }
}

public enum DiscoverError: Error, CustomStringConvertible {
  case ambiguousSlug(String)
  case ambiguousUUID(String)

  public var description: String {
    switch self {
    case .ambiguousSlug(let slug):
      return "ambiguous holon \"\(slug)\""
    case .ambiguousUUID(let prefix):
      return "ambiguous UUID prefix \"\(prefix)\""
    }
  }
}

private struct HolonPackageJSON: Decodable {
  var schema: String?
  var slug: String?
  var uuid: String?
  var identity: IdentityInfo?
  var lang: String?
  var runner: String?
  var status: String?
  var kind: String?
  var transport: String?
  var entrypoint: String?
  var architectures: [String]?
  var hasDist: Bool?
  var hasSource: Bool?

  private enum CodingKeys: String, CodingKey {
    case schema
    case slug
    case uuid
    case identity
    case lang
    case runner
    case status
    case kind
    case transport
    case entrypoint
    case architectures
    case hasDist = "has_dist"
    case hasSource = "has_source"
  }
}

private struct DiscoverLayer {
  let flag: Int
  let name: String
  let scan: (_ root: URL, _ timeout: Int) -> DiscoverResult
}

typealias DiscoverSourceBridge = (
  _ expression: String?,
  _ root: String?,
  _ specifiers: Int,
  _ limit: Int,
  _ timeout: Int
) -> DiscoverResult

private let packageSchemaVersion = "holon-package/v1"
private let defaultProbeTimeoutMilliseconds = 5_000
private let versionDirectoryPattern = try! NSRegularExpression(
  pattern: #"^v[0-9]+(?:[A-Za-z0-9._-]*)?$"#)

var discoverCurrentRootURLProvider: () -> URL = {
  normalizedDirectoryURL(
    URL(fileURLWithPath: FileManager.default.currentDirectoryPath, isDirectory: true))
}

var discoverEnvironmentProvider: () -> [String: String] = {
  ProcessInfo.processInfo.environment
}

var discoverBundleResourceURLProvider: () -> URL? = {
  Bundle.main.resourceURL?.standardizedFileURL
}

var discoverSourceBridge: DiscoverSourceBridge?

public func Discover(
  scope: Int,
  expression: String?,
  root: String?,
  specifiers: Int,
  limit: Int,
  timeout: Int
) -> DiscoverResult {
  if let error = discoverScopeError(scope) {
    return DiscoverResult(error: error)
  }

  if hasInvalidSpecifiers(specifiers) {
    return DiscoverResult(error: invalidSpecifiersError(specifiers))
  }

  if limit < 0 {
    return DiscoverResult(found: [])
  }

  let effectiveSpecifiers = specifiers == 0 ? ALL : specifiers
  let normalized = normalizedExpression(expression)

  if expression != nil, normalized == nil {
    return DiscoverResult(found: [])
  }

  if let normalized, isDeferredURLExpression(normalized) {
    return DiscoverResult(error: "direct URL expressions are not implemented")
  }

  var cachedRoot: URL?
  let resolveRoot: () throws -> URL = {
    if let cachedRoot {
      return cachedRoot
    }

    let resolved = try resolveDiscoverRoot(root)
    cachedRoot = resolved
    return resolved
  }

  if let normalized {
    let pathResult = discoverPathExpression(
      normalized,
      resolveRoot: resolveRoot,
      timeout: timeout
    )
    switch pathResult {
    case .handled(let result):
      return DiscoverResult(found: applyRefLimit(result.found, limit), error: result.error)
    case .unhandled:
      break
    }
  }

  let rootURL: URL
  do {
    rootURL = try resolveRoot()
  } catch {
    return DiscoverResult(error: String(describing: error))
  }

  return discoverLayers(
    root: rootURL,
    expression: normalized,
    specifiers: effectiveSpecifiers,
    limit: limit,
    timeout: timeout
  )
}

public func resolve(
  scope: Int,
  expression: String,
  root: String?,
  specifiers: Int,
  timeout: Int
) -> ResolveResult {
  let normalized = expression.trimmingCharacters(in: .whitespacesAndNewlines)
  if normalized.isEmpty {
    return ResolveResult(error: "expression is required")
  }

  let result = Discover(
    scope: scope,
    expression: normalized,
    root: root,
    specifiers: specifiers,
    limit: 1,
    timeout: timeout
  )
  if let error = result.error {
    return ResolveResult(error: error)
  }
  guard let first = result.found.first else {
    return ResolveResult(error: "holon \"\(normalized)\" not found")
  }
  if let error = first.error {
    return ResolveResult(ref: first, error: error)
  }
  return ResolveResult(ref: first)
}

public func discover(root: URL) throws -> [HolonEntry] {
  let normalizedRoot = normalizedDirectoryURL(root)
  var seen = Set<String>()
  var entries: [HolonEntry] = []
  appendEntries(
    &entries,
    seen: &seen,
    discovered: try discoverPackagesRecursive(normalizedRoot, origin: "local")
  )
  appendEntries(
    &entries,
    seen: &seen,
    discovered: try discoverSourceInRoot(normalizedRoot, origin: "local")
  )
  return entries
}

public func discoverLocal() throws -> [HolonEntry] {
  try discover(root: currentRootURL())
}

public func discoverAll() throws -> [HolonEntry] {
  var seen = Set<String>()
  var entries: [HolonEntry] = []

  if let bundleRoot = bundleHolonsRootURL() {
    appendEntries(
      &entries,
      seen: &seen,
      discovered: try discoverPackagesDirect(bundleRoot, origin: "bundle")
    )
    return entries
  }

  let discoverers: [() throws -> [HolonEntry]] = [
    {
      try discoverPackagesDirect(buildPackagesURL(), origin: "build")
    },
    {
      try discoverPackagesDirect(opbinURL(), origin: "$OPBIN")
    },
    {
      try discoverPackagesRecursive(cacheDirURL(), origin: "cache")
    },
    {
      try discoverSourceInRoot(currentRootURL(), origin: "local")
    },
  ]

  for discoverer in discoverers {
    appendEntries(&entries, seen: &seen, discovered: try discoverer())
  }

  return entries
}

public func findBySlug(_ slug: String) throws -> HolonEntry? {
  let needle = slug.trimmingCharacters(in: .whitespacesAndNewlines)
  if needle.isEmpty {
    return nil
  }

  var matched: HolonEntry?
  for entry in try discoverAll() where entry.slug == needle {
    if let matched, matched.uuid != entry.uuid {
      throw DiscoverError.ambiguousSlug(needle)
    }
    matched = entry
  }
  return matched
}

public func findByUUID(_ prefix: String) throws -> HolonEntry? {
  let needle = prefix.trimmingCharacters(in: .whitespacesAndNewlines)
  if needle.isEmpty {
    return nil
  }

  var matched: HolonEntry?
  for entry in try discoverAll() where entry.uuid.hasPrefix(needle) {
    if let matched, matched.uuid != entry.uuid {
      throw DiscoverError.ambiguousUUID(needle)
    }
    matched = entry
  }
  return matched
}

private enum PathExpressionResult {
  case handled(DiscoverResult)
  case unhandled
}

private func discoverLayers(
  root: URL,
  expression: String?,
  specifiers: Int,
  limit: Int,
  timeout: Int
) -> DiscoverResult {
  let layers: [DiscoverLayer] = [
    DiscoverLayer(
      flag: SIBLINGS,
      name: "siblings",
      scan: { _, timeout in
        discoverSiblingLayer(timeout: timeout)
      }
    ),
    DiscoverLayer(
      flag: CWD,
      name: "cwd",
      scan: { root, _ in
        discoverEntryLayer(
          try? discoverPackagesRecursive(root, origin: "cwd")
        )
      }
    ),
    DiscoverLayer(
      flag: SOURCE,
      name: "source",
      scan: { root, timeout in
        discoverSourceLayer(expression: expression, root: root, limit: limit, timeout: timeout)
      }
    ),
    DiscoverLayer(
      flag: BUILT,
      name: "built",
      scan: { root, _ in
        discoverEntryLayer(
          try? discoverPackagesDirect(
            root.appendingPathComponent(".op/build", isDirectory: true).standardizedFileURL,
            origin: "built"
          )
        )
      }
    ),
    DiscoverLayer(
      flag: INSTALLED,
      name: "installed",
      scan: { _, _ in
        discoverEntryLayer(try? discoverPackagesDirect(opbinURL(), origin: "installed"))
      }
    ),
    DiscoverLayer(
      flag: CACHED,
      name: "cached",
      scan: { _, _ in
        discoverEntryLayer(try? discoverPackagesRecursive(cacheDirURL(), origin: "cached"))
      }
    ),
  ]

  var seen = Set<String>()
  var found: [HolonRef] = []

  for layer in layers where specifiers & layer.flag != 0 {
    let result = layer.scan(root, timeout)
    if let error = result.error {
      return DiscoverResult(found: found, error: "scan \(layer.name) layer: \(error)")
    }

    for ref in result.found {
      if !matchesExpression(ref, expression: expression) {
        continue
      }

      let key = refKey(ref)
      if seen.insert(key).inserted {
        found.append(ref)
      }

      if limit > 0, found.count >= limit {
        return DiscoverResult(found: found)
      }
    }
  }

  return DiscoverResult(found: found)
}

private func discoverEntryLayer(_ entries: [HolonEntry]?) -> DiscoverResult {
  DiscoverResult(found: (entries ?? []).map(holonRefFromEntry))
}

private func discoverSiblingLayer(timeout: Int) -> DiscoverResult {
  guard let bundleRoot = bundleHolonsRootURL() else {
    return DiscoverResult(found: [])
  }
  do {
    return DiscoverResult(
      found: try discoverPackagesDirect(bundleRoot, origin: "siblings").map(holonRefFromEntry)
    )
  } catch {
    return DiscoverResult(error: String(describing: error))
  }
}

private func discoverSourceLayer(
  expression: String?,
  root: URL,
  limit: Int,
  timeout: Int
) -> DiscoverResult {
  guard let bridge = discoverSourceBridge else {
    return DiscoverResult(error: "SOURCE layer requires a local op discovery bridge")
  }
  return bridge(expression, root.path, SOURCE, limit, timeout)
}

private func discoverPathExpression(
  _ expression: String,
  resolveRoot: () throws -> URL,
  timeout: Int
) -> PathExpressionResult {
  let candidate: URL
  do {
    guard let resolved = try pathExpressionCandidate(expression, resolveRoot: resolveRoot) else {
      return .unhandled
    }
    candidate = resolved
  } catch {
    return .handled(DiscoverResult(error: String(describing: error)))
  }

  do {
    guard let ref = try discoverRef(at: candidate, timeout: timeout) else {
      return .handled(DiscoverResult(found: []))
    }
    return .handled(DiscoverResult(found: [ref]))
  } catch {
    return .handled(DiscoverResult(error: String(describing: error)))
  }
}

private func pathExpressionCandidate(
  _ expression: String,
  resolveRoot: () throws -> URL
) throws -> URL? {
  let trimmed = expression.trimmingCharacters(in: .whitespacesAndNewlines)
  if trimmed.isEmpty {
    return nil
  }

  if trimmed.lowercased().hasPrefix("file://") {
    return URL(fileURLWithPath: try pathFromFileURL(trimmed), isDirectory: false).standardizedFileURL
  }

  let isPathLike =
    trimmed.hasPrefix("/")
    || trimmed.hasPrefix(".")
    || trimmed.contains("/")
    || trimmed.contains("\\")
    || trimmed.lowercased().hasSuffix(".holon")

  if !isPathLike {
    return nil
  }

  if trimmed.hasPrefix("/") {
    return URL(fileURLWithPath: trimmed, isDirectory: false).standardizedFileURL
  }

  let root = try resolveRoot()

  return root.appendingPathComponent(trimmed).standardizedFileURL
}

private func discoverRef(at path: URL, timeout: Int) throws -> HolonRef? {
  let standardized = path.standardizedFileURL
  var isDirectoryFlag: ObjCBool = false
  guard FileManager.default.fileExists(atPath: standardized.path, isDirectory: &isDirectoryFlag) else {
    return nil
  }

  if isDirectoryFlag.boolValue {
    if standardized.lastPathComponent.hasSuffix(".holon")
      || hasHolonJSON(standardized)
    {
      let root = standardized.deletingLastPathComponent().standardizedFileURL
      if let loaded = try loadPackageEntry(root: root, directory: standardized, origin: "path") {
        return holonRefFromEntry(loaded)
      }
      do {
        return holonRefFromEntry(
          try probePackageEntry(root: root, directory: standardized, origin: "path", timeout: timeout)
        )
      } catch {
        return HolonRef(url: fileURL(standardized.path), error: String(describing: error))
      }
    }

    let entries = try discoverSourceInRoot(standardized, origin: "path")
    if entries.count == 1 {
      return holonRefFromEntry(entries[0])
    }
    for entry in entries where entry.dir.standardizedFileURL.path == standardized.path {
      return holonRefFromEntry(entry)
    }
    return nil
  }

  if standardized.lastPathComponent == Identity.protoManifestFileName {
    let entries = try discoverSourceInRoot(
      standardized.deletingLastPathComponent().standardizedFileURL,
      origin: "path"
    )
    if entries.count == 1 {
      return holonRefFromEntry(entries[0])
    }
    return nil
  }

  do {
    return holonRefFromEntry(try probeBinaryPath(standardized, timeout: timeout))
  } catch {
    return HolonRef(url: fileURL(standardized.path), error: String(describing: error))
  }
}

private func appendEntries(
  _ entries: inout [HolonEntry],
  seen: inout Set<String>,
  discovered: [HolonEntry]
) {
  for entry in discovered {
    let key = entryKey(entry)
    if seen.insert(key).inserted {
      entries.append(entry)
    }
  }
}

private func discoverPackagesDirect(_ root: URL, origin: String) throws -> [HolonEntry] {
  try discoverPackages(root, origin: origin, recursively: false)
}

private func discoverPackagesRecursive(_ root: URL, origin: String) throws -> [HolonEntry] {
  try discoverPackages(root, origin: origin, recursively: true)
}

private func discoverPackages(_ root: URL, origin: String, recursively: Bool) throws -> [HolonEntry] {
  let normalizedRoot = normalizedDirectoryURL(root)
  guard directoryExists(normalizedRoot) else {
    return []
  }

  let directories =
    try recursively
    ? packageDirectoriesRecursively(in: normalizedRoot)
    : packageDirectoriesDirectly(in: normalizedRoot)
  var entriesByKey: [String: HolonEntry] = [:]
  var orderedKeys: [String] = []

  for directory in directories {
    let entry: HolonEntry
    if let loaded = try loadPackageEntry(root: normalizedRoot, directory: directory, origin: origin) {
      entry = loaded
    } else {
      do {
        entry = try probePackageEntry(
          root: normalizedRoot,
          directory: directory,
          origin: origin,
          timeout: NO_TIMEOUT
        )
      } catch {
        continue
      }
    }

    let key = entryKey(entry)
    if let existing = entriesByKey[key] {
      if shouldReplaceEntry(existing: existing, next: entry) {
        entriesByKey[key] = entry
      }
      continue
    }

    entriesByKey[key] = entry
    orderedKeys.append(key)
  }

  return orderedKeys.compactMap { entriesByKey[$0] }.sorted(by: entrySort)
}

private func packageDirectoriesDirectly(in root: URL) throws -> [URL] {
  let children = try FileManager.default.contentsOfDirectory(
    at: root,
    includingPropertiesForKeys: [.isDirectoryKey],
    options: [.skipsHiddenFiles]
  )

  return children
    .filter {
      (try? $0.resourceValues(forKeys: [.isDirectoryKey]).isDirectory) == true
        && $0.lastPathComponent.hasSuffix(".holon")
    }
    .map(\.standardizedFileURL)
    .sorted { $0.path < $1.path }
}

private func packageDirectoriesRecursively(in root: URL) throws -> [URL] {
  guard
    let enumerator = FileManager.default.enumerator(
      at: root,
      includingPropertiesForKeys: [.isDirectoryKey],
      options: [],
      errorHandler: { _, _ in true }
    )
  else {
    return []
  }

  var directories: [URL] = []
  while let item = enumerator.nextObject() as? URL {
    let url = item.standardizedFileURL
    let values = try? url.resourceValues(forKeys: [.isDirectoryKey])
    guard values?.isDirectory == true else {
      continue
    }

    let name = url.lastPathComponent
    if url.path != root.path, shouldSkipDiscoveryDir(root: root, path: url, name: name) {
      enumerator.skipDescendants()
      continue
    }

    if name.hasSuffix(".holon") {
      directories.append(url)
      enumerator.skipDescendants()
    }
  }

  return directories.sorted { $0.path < $1.path }
}

private func loadPackageEntry(root: URL, directory: URL, origin: String) throws -> HolonEntry? {
  let manifestURL = directory.appendingPathComponent(".holon.json", isDirectory: false)
  guard let data = FileManager.default.contents(atPath: manifestURL.path) else {
    return nil
  }

  let payload = try JSONDecoder().decode(HolonPackageJSON.self, from: data)
  if let schema = payload.schema?.trimmingCharacters(in: .whitespacesAndNewlines),
    !schema.isEmpty,
    schema != packageSchemaVersion
  {
    return nil
  }

  var identity = HolonIdentity()
  identity.uuid = payload.uuid?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.givenName = payload.identity?.givenName.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.familyName = payload.identity?.familyName.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.motto = payload.identity?.motto.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.aliases = payload.identity?.aliases ?? []
  identity.status = payload.status?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.lang = payload.lang?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""

  let slug = payload.slug?.trimmingCharacters(in: .whitespacesAndNewlines).nonEmpty ?? identity.slug
  let entrypoint = payload.entrypoint?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  let runner = payload.runner?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  let transport = payload.transport?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  var manifest = HolonManifest()
  manifest.kind = payload.kind?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  manifest.build.runner = runner
  manifest.artifacts.binary = entrypoint

  return HolonEntry(
    slug: slug,
    uuid: identity.uuid,
    dir: directory.standardizedFileURL,
    relativePath: relativePath(root: root, dir: directory),
    origin: origin,
    identity: identity,
    manifest: manifest,
    sourceKind: "package",
    packageRoot: directory.standardizedFileURL,
    runner: runner,
    transport: transport,
    entrypoint: entrypoint,
    architectures: payload.architectures ?? [],
    hasDist: payload.hasDist ?? false,
    hasSource: payload.hasSource ?? false
  )
}

private func probePackageEntry(
  root: URL,
  directory: URL,
  origin: String,
  timeout: Int
) throws -> HolonEntry {
  let binaryPath = try packageBinaryPath(directory)
  var entry = try probeBinaryPath(binaryPath, timeout: timeout)
  entry.dir = directory.standardizedFileURL
  entry.relativePath = relativePath(root: root, dir: directory)
  entry.origin = origin
  entry.sourceKind = "package"
  entry.packageRoot = directory.standardizedFileURL
  entry.hasSource = false
  return entry
}

private func packageBinaryPath(_ directory: URL) throws -> URL {
  let archDirectory = directory
    .appendingPathComponent("bin", isDirectory: true)
    .appendingPathComponent(currentArchDirectory(), isDirectory: true)
    .standardizedFileURL

  let contents = try FileManager.default.contentsOfDirectory(
    at: archDirectory,
    includingPropertiesForKeys: [.isRegularFileKey],
    options: [.skipsHiddenFiles]
  )

  let candidate = contents
    .filter { (try? $0.resourceValues(forKeys: Set([URLResourceKey.isRegularFileKey])).isRegularFile) == true }
    .sorted(by: { $0.path < $1.path })
    .first

  guard let candidate
  else {
    throw NSError(
      domain: "Holons.Discover",
      code: 2,
      userInfo: [NSLocalizedDescriptionKey: "no package binary found in \(archDirectory.path)"]
    )
  }

  return candidate.standardizedFileURL
}

private func probeBinaryPath(_ binaryPath: URL, timeout: Int) throws -> HolonEntry {
  let standardized = binaryPath.standardizedFileURL
  var isDirectoryFlag: ObjCBool = false
  guard FileManager.default.fileExists(atPath: standardized.path, isDirectory: &isDirectoryFlag) else {
    throw NSError(
      domain: "Holons.Discover",
      code: 3,
      userInfo: [NSLocalizedDescriptionKey: "\(standardized.path) does not exist"]
    )
  }
  if isDirectoryFlag.boolValue {
    throw NSError(
      domain: "Holons.Discover",
      code: 4,
      userInfo: [NSLocalizedDescriptionKey: "\(standardized.path) is a directory"]
    )
  }

  let response = try describeLaunchTarget(
    LaunchTarget(
      kind: "probe-binary",
      executablePath: standardized.path,
      arguments: [],
      workingDirectory: standardized.deletingLastPathComponent().path
    ),
    timeout: TimeInterval(probeTimeoutMilliseconds(timeout)) / 1000.0
  )

  var entry = holonEntryFromDescribeResponse(
    response,
    dir: standardized.deletingLastPathComponent().standardizedFileURL,
    sourceKind: "binary",
    origin: "path",
    entrypointOverride: standardized.path
  )
  entry.manifest?.artifacts.binary = standardized.path
  entry.entrypoint = standardized.path
  return entry
}

private func discoverSourceInRoot(_ root: URL, origin: String) throws -> [HolonEntry] {
  let normalizedRoot = normalizedDirectoryURL(root)
  guard directoryExists(normalizedRoot) else {
    return []
  }

  guard
    let enumerator = FileManager.default.enumerator(
      at: normalizedRoot,
      includingPropertiesForKeys: [.isDirectoryKey],
      options: [],
      errorHandler: { _, _ in true }
    )
  else {
    return []
  }

  var entriesByKey: [String: HolonEntry] = [:]
  var orderedKeys: [String] = []

  while let item = enumerator.nextObject() as? URL {
    let url = item.standardizedFileURL
    let values = try? url.resourceValues(forKeys: [.isDirectoryKey])
    let isDirectory = values?.isDirectory ?? false
    let name = url.lastPathComponent

    if isDirectory {
      if shouldSkipDiscoveryDir(root: normalizedRoot, path: url, name: name) {
        enumerator.skipDescendants()
      }
      continue
    }

    guard name == Identity.protoManifestFileName else {
      continue
    }

    do {
      let resolved = try Identity.resolveProtoFile(url.path)
      let holonDir = sourceHolonDir(root: normalizedRoot, protoURL: url)
      var manifest = HolonManifest()
      manifest.kind = resolved.kind
      manifest.build.runner = resolved.buildRunner
      manifest.build.main = resolved.buildMain
      manifest.artifacts.binary = resolved.artifactBinary
      manifest.artifacts.primary = resolved.artifactPrimary

      let entry = HolonEntry(
        slug: resolved.identity.slug,
        uuid: resolved.identity.uuid,
        dir: holonDir,
        relativePath: relativePath(root: normalizedRoot, dir: holonDir),
        origin: origin,
        identity: resolved.identity,
        manifest: manifest,
        sourceKind: "source",
        packageRoot: nil,
        runner: resolved.buildRunner,
        transport: resolved.transport,
        entrypoint: resolved.artifactBinary,
        architectures: resolved.platforms,
        hasDist: false,
        hasSource: true
      )

      let key = entryKey(entry)
      if let existing = entriesByKey[key] {
        if shouldReplaceEntry(existing: existing, next: entry) {
          entriesByKey[key] = entry
        }
        continue
      }

      entriesByKey[key] = entry
      orderedKeys.append(key)
    } catch {
      continue
    }
  }

  return orderedKeys.compactMap { entriesByKey[$0] }.sorted(by: entrySort)
}

private func holonRefFromEntry(_ entry: HolonEntry) -> HolonRef {
  HolonRef(url: fileURL(entry.dir.path), info: holonInfoFromEntry(entry))
}

private func holonInfoFromEntry(_ entry: HolonEntry) -> HolonInfo {
  var runner = entry.runner
  var kind = ""
  if let manifest = entry.manifest {
    if runner.isEmpty {
      runner = manifest.build.runner
    }
    kind = manifest.kind
  }

  return HolonInfo(
    slug: entry.slug,
    uuid: entry.uuid,
    identity: IdentityInfo(
      givenName: entry.identity.givenName,
      familyName: entry.identity.familyName,
      motto: entry.identity.motto,
      aliases: entry.identity.aliases
    ),
    lang: entry.identity.lang,
    runner: runner,
    status: entry.identity.status,
    kind: kind,
    transport: entry.transport,
    entrypoint: entry.entrypoint,
    architectures: entry.architectures,
    hasDist: entry.hasDist,
    hasSource: entry.hasSource
  )
}

private func holonEntryFromDescribeResponse(
  _ response: Holons_V1_DescribeResponse,
  dir: URL,
  sourceKind: String,
  origin: String,
  entrypointOverride: String? = nil
) -> HolonEntry {
  let manifestResponse = response.manifest
  let identityResponse = manifestResponse.identity

  var identity = HolonIdentity()
  identity.uuid = identityResponse.uuid
  identity.givenName = identityResponse.givenName
  identity.familyName = identityResponse.familyName
  identity.motto = identityResponse.motto
  identity.aliases = identityResponse.aliases
  identity.lang = manifestResponse.lang
  identity.status = identityResponse.status

  var manifest = HolonManifest()
  manifest.kind = manifestResponse.kind
  manifest.build.runner = manifestResponse.build.runner
  manifest.build.main = manifestResponse.build.main
  manifest.artifacts.binary = manifestResponse.artifacts.binary
  manifest.artifacts.primary = manifestResponse.artifacts.primary

  let entrypoint =
    entrypointOverride?.trimmingCharacters(in: .whitespacesAndNewlines).nonEmpty
    ?? manifest.artifacts.binary

  return HolonEntry(
    slug: identity.slug,
    uuid: identity.uuid,
    dir: dir.standardizedFileURL,
    relativePath: ".",
    origin: origin,
    identity: identity,
    manifest: manifest,
    sourceKind: sourceKind,
    packageRoot: sourceKind == "package" ? dir.standardizedFileURL : nil,
    runner: manifest.build.runner,
    transport: manifestResponse.transport,
    entrypoint: entrypoint,
    architectures: manifestResponse.platforms,
    hasDist: false,
    hasSource: sourceKind == "source"
  )
}

private func entryKey(_ entry: HolonEntry) -> String {
  let uuid = entry.uuid.trimmingCharacters(in: .whitespacesAndNewlines)
  if !uuid.isEmpty {
    return uuid
  }
  if let packageRoot = entry.packageRoot {
    return packageRoot.path
  }
  return entry.dir.path
}

private func refKey(_ ref: HolonRef) -> String {
  if let uuid = ref.info?.uuid.trimmingCharacters(in: .whitespacesAndNewlines), !uuid.isEmpty {
    return uuid
  }
  return ref.url
}

private func shouldReplaceEntry(existing: HolonEntry, next: HolonEntry) -> Bool {
  pathDepth(next.relativePath) < pathDepth(existing.relativePath)
}

private func sourceHolonDir(root: URL, protoURL: URL) -> URL {
  let protoDir = protoURL.deletingLastPathComponent().standardizedFileURL
  let name = protoDir.lastPathComponent
  if isVersionDirectory(name) {
    let parent = protoDir.deletingLastPathComponent().standardizedFileURL
    if parent.lastPathComponent == "api" {
      return parent.deletingLastPathComponent().standardizedFileURL
    }
    if parent.path != root.path {
      return parent
    }
  }
  return protoDir
}

private func isVersionDirectory(_ name: String) -> Bool {
  versionDirectoryPattern.firstMatch(
    in: name,
    range: NSRange(location: 0, length: name.utf16.count)
  ) != nil
}

private func shouldSkipDiscoveryDir(root: URL, path: URL, name: String) -> Bool {
  if path.path == root.path {
    return false
  }
  if name.hasSuffix(".holon") {
    return false
  }
  if [".git", ".op", "node_modules", "vendor", "build", "testdata"].contains(name) {
    return true
  }
  return name.hasPrefix(".")
}

private func entrySort(_ left: HolonEntry, _ right: HolonEntry) -> Bool {
  if left.relativePath == right.relativePath {
    return left.uuid < right.uuid
  }
  return left.relativePath < right.relativePath
}

private func relativePath(root: URL, dir: URL) -> String {
  let rootPath = root.path
  let dirPath = dir.path
  if dirPath == rootPath {
    return "."
  }

  let prefix = rootPath.hasSuffix("/") ? rootPath : rootPath + "/"
  if dirPath.hasPrefix(prefix) {
    return String(dirPath.dropFirst(prefix.count))
  }
  return dirPath
}

private func pathDepth(_ relativePath: String) -> Int {
  let trimmed = relativePath.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
  if trimmed.isEmpty || trimmed == "." {
    return 0
  }
  return trimmed.split(separator: "/").count
}

private func currentRootURL() -> URL {
  discoverCurrentRootURLProvider().standardizedFileURL
}

private func resolveDiscoverRoot(_ root: String?) throws -> URL {
  if root == nil {
    return currentRootURL()
  }

  let trimmed = root?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  if trimmed.isEmpty {
    throw NSError(
      domain: "Holons.Discover",
      code: 5,
      userInfo: [NSLocalizedDescriptionKey: "root cannot be empty"]
    )
  }

  let resolved = URL(fileURLWithPath: trimmed, isDirectory: true).standardizedFileURL
  var isDirectoryFlag: ObjCBool = false
  guard FileManager.default.fileExists(atPath: resolved.path, isDirectory: &isDirectoryFlag) else {
    throw NSError(
      domain: "Holons.Discover",
      code: 6,
      userInfo: [NSLocalizedDescriptionKey: "root \"\(trimmed)\" does not exist"]
    )
  }
  if !isDirectoryFlag.boolValue {
    throw NSError(
      domain: "Holons.Discover",
      code: 7,
      userInfo: [NSLocalizedDescriptionKey: "root \"\(trimmed)\" is not a directory"]
    )
  }
  return resolved
}

private func opPathURL() -> URL {
  let environment = discoverEnvironmentProvider()
  if let path = environment["OPPATH"]?.trimmingCharacters(in: .whitespacesAndNewlines), !path.isEmpty {
    return normalizedDirectoryURL(URL(fileURLWithPath: path, isDirectory: true))
  }
  if let home = environment["HOME"]?.trimmingCharacters(in: .whitespacesAndNewlines), !home.isEmpty {
    return normalizedDirectoryURL(
      URL(fileURLWithPath: home, isDirectory: true).appendingPathComponent(".op", isDirectory: true))
  }
  return normalizedDirectoryURL(URL(fileURLWithPath: ".op", isDirectory: true))
}

private func buildPackagesURL() -> URL {
  currentRootURL().appendingPathComponent(".op/build", isDirectory: true).standardizedFileURL
}

private func opbinURL() -> URL {
  let environment = discoverEnvironmentProvider()
  if let path = environment["OPBIN"]?.trimmingCharacters(in: .whitespacesAndNewlines), !path.isEmpty {
    return normalizedDirectoryURL(URL(fileURLWithPath: path, isDirectory: true))
  }
  return normalizedDirectoryURL(opPathURL().appendingPathComponent("bin", isDirectory: true))
}

private func cacheDirURL() -> URL {
  normalizedDirectoryURL(opPathURL().appendingPathComponent("cache", isDirectory: true))
}

private func bundleHolonsRootURL() -> URL? {
  guard let resourceURL = discoverBundleResourceURLProvider()?.standardizedFileURL else {
    return nil
  }

  let candidate = resourceURL.appendingPathComponent("Holons", isDirectory: true).standardizedFileURL
  return directoryExists(candidate) ? candidate : nil
}

private func directoryExists(_ url: URL) -> Bool {
  var isDirectory: ObjCBool = false
  guard FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectory) else {
    return false
  }
  return isDirectory.boolValue
}

private func normalizedDirectoryURL(_ url: URL) -> URL {
  url.standardizedFileURL
}

private func fileURL(_ path: String) -> String {
  URL(fileURLWithPath: path, isDirectory: false).standardizedFileURL.absoluteString
}

private func hasHolonJSON(_ directory: URL) -> Bool {
  let manifestURL = directory.appendingPathComponent(".holon.json", isDirectory: false)
  var isDirectoryFlag: ObjCBool = false
  guard FileManager.default.fileExists(atPath: manifestURL.path, isDirectory: &isDirectoryFlag) else {
    return false
  }
  return !isDirectoryFlag.boolValue
}

private func pathFromFileURL(_ raw: String) throws -> String {
  guard let parsed = URL(string: raw), parsed.scheme?.lowercased() == "file" else {
    throw NSError(
      domain: "Holons.Discover",
      code: 8,
      userInfo: [NSLocalizedDescriptionKey: "holon URL \"\(raw)\" is not a local file target"]
    )
  }

  let path = parsed.path
  if path.isEmpty {
    throw NSError(
      domain: "Holons.Discover",
      code: 9,
      userInfo: [NSLocalizedDescriptionKey: "holon URL \"\(raw)\" has no path"]
    )
  }
  return URL(fileURLWithPath: path).standardizedFileURL.path
}

private func normalizedExpression(_ expression: String?) -> String? {
  guard let expression else {
    return nil
  }
  let trimmed = expression.trimmingCharacters(in: .whitespacesAndNewlines)
  return trimmed.isEmpty ? nil : trimmed
}

private func matchesExpression(_ ref: HolonRef, expression: String?) -> Bool {
  guard let expression else {
    return true
  }
  guard let info = ref.info else {
    return false
  }

  if info.slug == expression {
    return true
  }
  if info.uuid.hasPrefix(expression) {
    return true
  }
  if info.identity.aliases.contains(expression) {
    return true
  }

  let base: String
  if let parsed = URL(string: ref.url), parsed.scheme?.lowercased() == "file" {
    base = parsed.deletingPathExtension().lastPathComponent
  } else {
    base = URL(fileURLWithPath: ref.url).deletingPathExtension().lastPathComponent
  }
  return base == expression
}

private func applyRefLimit(_ refs: [HolonRef], _ limit: Int) -> [HolonRef] {
  if limit <= 0 || refs.count <= limit {
    return refs
  }
  return Array(refs.prefix(limit))
}

private func hasInvalidSpecifiers(_ specifiers: Int) -> Bool {
  specifiers < 0 || (specifiers & ~ALL) != 0
}

private func invalidSpecifiersError(_ specifiers: Int) -> String {
  "invalid specifiers 0x\(String(specifiers, radix: 16).uppercased()): valid range is 0x00-0x3F"
}

private func discoverScopeError(_ scope: Int) -> String? {
  switch scope {
  case LOCAL:
    return nil
  case PROXY:
    return "PROXY scope is not implemented"
  case DELEGATED:
    return "DELEGATED scope is not implemented"
  default:
    return "scope \(scope) not supported"
  }
}

private func isDeferredURLExpression(_ expression: String) -> Bool {
  guard let scheme = URL(string: expression)?.scheme?.lowercased() else {
    return false
  }
  if scheme == "file" {
    return false
  }
  return expression.contains("://")
}

private func probeTimeoutMilliseconds(_ timeout: Int) -> Int {
  timeout > 0 ? timeout : defaultProbeTimeoutMilliseconds
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

private extension String {
  var nonEmpty: String? {
    isEmpty ? nil : self
  }
}
