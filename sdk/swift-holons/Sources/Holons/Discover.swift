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
  struct IdentityJSON: Decodable {
    var givenName: String?
    var familyName: String?
    var motto: String?

    private enum CodingKeys: String, CodingKey {
      case givenName = "given_name"
      case familyName = "family_name"
      case motto
    }
  }

  var schema: String?
  var slug: String?
  var uuid: String?
  var identity: IdentityJSON?
  var lang: String?
  var runner: String?
  var status: String?
  var kind: String?
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
    case entrypoint
    case architectures
    case hasDist = "has_dist"
    case hasSource = "has_source"
  }
}

private let packageSchemaVersion = "holon-package/v1"
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

public func discover(root: URL) throws -> [HolonEntry] {
  let normalizedRoot = normalizedDirectoryURL(root)
  var seen = Set<String>()
  var entries: [HolonEntry] = []
  appendEntries(&entries, seen: &seen, discovered: try discoverPackagesRecursive(normalizedRoot, origin: "local"))
  appendEntries(&entries, seen: &seen, discovered: try discoverSourceInRoot(normalizedRoot, origin: "local"))
  return entries
}

public func discoverLocal() throws -> [HolonEntry] {
  try discover(root: currentRootURL())
}

public func discoverAll() throws -> [HolonEntry] {
  var seen = Set<String>()
  var entries: [HolonEntry] = []

  // Bundle apps discover exclusively from their embedded Holons directory.
  // No filesystem scanning outside the bundle — avoids macOS TCC prompts
  // and keeps the discovery scope well-defined.
  if let bundleRoot = bundleHolonsRootURL() {
    appendEntries(&entries, seen: &seen, discovered: try discoverPackagesDirect(bundleRoot, origin: "bundle"))
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

private func appendEntries(_ entries: inout [HolonEntry], seen: inout Set<String>, discovered: [HolonEntry]) {
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

  let directories = try recursively ? packageDirectoriesRecursively(in: normalizedRoot) : packageDirectoriesDirectly(in: normalizedRoot)
  var entriesByKey: [String: HolonEntry] = [:]
  var orderedKeys: [String] = []

  for directory in directories {
    guard let entry = try loadPackageEntry(root: normalizedRoot, directory: directory, origin: origin) else {
      continue
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
  identity.givenName = payload.identity?.givenName?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.familyName = payload.identity?.familyName?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.motto = payload.identity?.motto?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.status = payload.status?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  identity.lang = payload.lang?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""

  let slug = payload.slug?.trimmingCharacters(in: .whitespacesAndNewlines).nonEmpty ?? identity.slug
  let entrypoint = payload.entrypoint?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
  let runner = payload.runner?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
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
    entrypoint: entrypoint,
    architectures: payload.architectures ?? [],
    hasDist: payload.hasDist ?? false,
    hasSource: payload.hasSource ?? false
  )
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
        entrypoint: resolved.artifactBinary,
        architectures: [],
        hasDist: false,
        hasSource: false
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
  if [".git", ".op", "node_modules", "vendor", "build", "testdata"].contains(name) {
    return true
  }
  // Avoid walking into macOS/Linux user-space directories that trigger TCC
  // permission prompts when the working directory is $HOME or similar.
  if userSpaceDirectories.contains(name) {
    return true
  }
  return name.hasPrefix(".")
}

private let userSpaceDirectories: Set<String> = [
  "Music", "Movies", "Pictures", "Photos",
  "Downloads", "Desktop", "Documents",
  "Library", "Applications",
  "Google Drive", "Dropbox", "OneDrive", "iCloud Drive",
  "Public", "Sites",
]

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

private extension String {
  var nonEmpty: String? {
    isEmpty ? nil : self
  }
}
