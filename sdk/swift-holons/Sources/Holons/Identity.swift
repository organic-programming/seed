import Foundation

public struct HolonIdentity: Equatable {
  public var uuid: String = ""
  public var givenName: String = ""
  public var familyName: String = ""
  public var motto: String = ""
  public var composer: String = ""
  public var clade: String = ""
  public var status: String = ""
  public var born: String = ""
  public var version: String = ""
  public var lang: String = ""
  public var parents: [String] = []
  public var reproduction: String = ""
  public var generatedBy: String = ""
  public var protoStatus: String = ""
  public var aliases: [String] = []

  public init() {}

  public var slug: String {
    let given = givenName.trimmingCharacters(in: .whitespacesAndNewlines)
    let family =
      familyName
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .replacingOccurrences(of: #"\?$"#, with: "", options: .regularExpression)
    if given.isEmpty && family.isEmpty {
      return ""
    }

    let joined = "\(given)-\(family)"
      .trimmingCharacters(in: .whitespacesAndNewlines)
      .lowercased()
      .replacingOccurrences(of: " ", with: "-")
    return joined.trimmingCharacters(in: CharacterSet(charactersIn: "-"))
  }
}

public struct HolonResolvedSkill: Equatable {
  public var name: String = ""
  public var description: String = ""
  public var when: String = ""
  public var steps: [String] = []

  public init() {}
}

public struct HolonResolvedSequenceParam: Equatable {
  public var name: String = ""
  public var description: String = ""
  public var required: Bool = false
  public var defaultValue: String = ""

  public init() {}
}

public struct HolonResolvedSequence: Equatable {
  public var name: String = ""
  public var description: String = ""
  public var params: [HolonResolvedSequenceParam] = []
  public var steps: [String] = []

  public init() {}
}

public struct HolonResolvedManifest: Equatable {
  public var identity = HolonIdentity()
  public var sourcePath: String = ""
  public var description: String = ""
  public var kind: String = ""
  public var transport: String = ""
  public var platforms: [String] = []
  public var buildRunner: String = ""
  public var buildMain: String = ""
  public var artifactBinary: String = ""
  public var artifactPrimary: String = ""
  public var requiredCommands: [String] = []
  public var requiredFiles: [String] = []
  public var requiredPlatforms: [String] = []
  public var skills: [HolonResolvedSkill] = []
  public var sequences: [HolonResolvedSequence] = []
  public var guide: String = ""

  public init() {}
}

public enum IdentityError: Error, CustomStringConvertible {
  case invalidManifest(String)
  case invalidProtoFile(String)
  case manifestNotFound(String)

  public var description: String {
    switch self {
    case .invalidManifest(let path):
      return "\(path): missing holons.v1.manifest option in holon.proto"
    case .invalidProtoFile(let path):
      return "\(path) is not a holon.proto file"
    case .manifestNotFound(let path):
      return "no holon.proto found in \(path)"
    }
  }
}

public enum Identity {
  public static let protoManifestFileName = "holon.proto"

  public static func parseHolon(_ path: String) throws -> HolonIdentity {
    try resolveProtoFile(path).identity
  }

  public static func resolve(_ root: URL) throws -> HolonResolvedManifest {
    try resolveManifest(in: root).manifest
  }

  public static func resolveProtoFile(_ path: String) throws -> HolonResolvedManifest {
    let normalizedPath = URL(fileURLWithPath: path).standardizedFileURL.path
    guard URL(fileURLWithPath: normalizedPath).lastPathComponent == protoManifestFileName else {
      throw IdentityError.invalidProtoFile(normalizedPath)
    }
    return try parseManifest(normalizedPath)
  }

  static func parseManifest(_ path: String) throws -> HolonResolvedManifest {
    let text = try String(contentsOfFile: path, encoding: .utf8)
    guard let manifestBlock = extractManifestBlock(from: text) else {
      throw IdentityError.invalidManifest(path)
    }

    var manifest = HolonResolvedManifest()
    manifest.sourcePath = path
    if let identityBlock = extractBlock(named: "identity", in: manifestBlock) {
      manifest.identity.uuid = scalar(named: "uuid", in: identityBlock) ?? ""
      manifest.identity.givenName = scalar(named: "given_name", in: identityBlock) ?? ""
      manifest.identity.familyName = scalar(named: "family_name", in: identityBlock) ?? ""
      manifest.identity.motto = scalar(named: "motto", in: identityBlock) ?? ""
      manifest.identity.composer = scalar(named: "composer", in: identityBlock) ?? ""
      manifest.identity.clade = scalar(named: "clade", in: identityBlock) ?? ""
      manifest.identity.status = scalar(named: "status", in: identityBlock) ?? ""
      manifest.identity.born = scalar(named: "born", in: identityBlock) ?? ""
      manifest.identity.version = scalar(named: "version", in: identityBlock) ?? ""
      manifest.identity.protoStatus = scalar(named: "proto_status", in: identityBlock) ?? ""
      manifest.identity.aliases = stringList(named: "aliases", in: identityBlock)
    }
    if let lineageBlock = extractBlock(named: "lineage", in: manifestBlock) {
      manifest.identity.parents = stringList(named: "parents", in: lineageBlock)
      manifest.identity.reproduction = scalar(named: "reproduction", in: lineageBlock) ?? ""
      manifest.identity.generatedBy = scalar(named: "generated_by", in: lineageBlock) ?? ""
    }
    manifest.description = scalar(named: "description", in: manifestBlock) ?? ""
    manifest.identity.lang = scalar(named: "lang", in: manifestBlock) ?? ""
    manifest.kind = scalar(named: "kind", in: manifestBlock) ?? ""
    manifest.transport = scalar(named: "transport", in: manifestBlock) ?? ""
    manifest.platforms = stringList(named: "platforms", in: manifestBlock)
    if let buildBlock = extractBlock(named: "build", in: manifestBlock) {
      manifest.buildRunner = scalar(named: "runner", in: buildBlock) ?? ""
      manifest.buildMain = scalar(named: "main", in: buildBlock) ?? ""
    }
    if let requiresBlock = extractBlock(named: "requires", in: manifestBlock) {
      manifest.requiredCommands = stringList(named: "commands", in: requiresBlock)
      manifest.requiredFiles = stringList(named: "files", in: requiresBlock)
      manifest.requiredPlatforms = stringList(named: "platforms", in: requiresBlock)
    }
    if let artifactsBlock = extractBlock(named: "artifacts", in: manifestBlock) {
      manifest.artifactBinary = scalar(named: "binary", in: artifactsBlock) ?? ""
      manifest.artifactPrimary = scalar(named: "primary", in: artifactsBlock) ?? ""
    }
    manifest.skills = inlineObjectList(named: "skills", in: manifestBlock).map(parseSkill)
    manifest.sequences = inlineObjectList(named: "sequences", in: manifestBlock).map(parseSequence)
    manifest.guide = scalar(named: "guide", in: manifestBlock) ?? ""

    return manifest
  }

  static func findHolonProto(in root: URL) -> URL? {
    var isDirectory: ObjCBool = false
    let normalizedRoot = root.standardizedFileURL
    if !FileManager.default.fileExists(atPath: normalizedRoot.path, isDirectory: &isDirectory) {
      return nil
    }
    if !isDirectory.boolValue {
      return normalizedRoot.lastPathComponent == protoManifestFileName ? normalizedRoot : nil
    }

    guard
      let enumerator = FileManager.default.enumerator(
        at: normalizedRoot,
        includingPropertiesForKeys: [.isDirectoryKey],
        options: [],
        errorHandler: { _, _ in true }
      )
    else {
      return nil
    }

    var candidates: [URL] = []
    while let item = enumerator.nextObject() as? URL {
      let item = item.standardizedFileURL
      let values = try? item.resourceValues(forKeys: [.isDirectoryKey])
      if values?.isDirectory == true {
        continue
      }
      if item.lastPathComponent == protoManifestFileName {
        candidates.append(item)
      }
    }

    return candidates.sorted { $0.path < $1.path }.first
  }

  public static func resolveManifest(in root: URL) throws -> (path: URL, manifest: HolonResolvedManifest) {
    guard let manifestURL = findHolonProto(in: root) else {
      throw IdentityError.manifestNotFound(root.path)
    }
    return (manifestURL, try resolveProtoFile(manifestURL.path))
  }
}

private func parseSkill(_ source: String) -> HolonResolvedSkill {
  var skill = HolonResolvedSkill()
  skill.name = scalar(named: "name", in: source) ?? ""
  skill.description = scalar(named: "description", in: source) ?? ""
  skill.when = scalar(named: "when", in: source) ?? ""
  skill.steps = stringList(named: "steps", in: source)
  return skill
}

private func parseSequence(_ source: String) -> HolonResolvedSequence {
  var sequence = HolonResolvedSequence()
  sequence.name = scalar(named: "name", in: source) ?? ""
  sequence.description = scalar(named: "description", in: source) ?? ""
  sequence.params = inlineObjectList(named: "params", in: source).map(parseSequenceParam)
  sequence.steps = stringList(named: "steps", in: source)
  return sequence
}

private func parseSequenceParam(_ source: String) -> HolonResolvedSequenceParam {
  var param = HolonResolvedSequenceParam()
  param.name = scalar(named: "name", in: source) ?? ""
  param.description = scalar(named: "description", in: source) ?? ""
  param.required = boolValue(named: "required", in: source) ?? false
  param.defaultValue = scalar(named: "default", in: source) ?? ""
  return param
}

private func extractManifestBlock(from source: String) -> String? {
  guard
    let start = firstMatch(
      of: #"option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{"#,
      in: source
    )
  else {
    return nil
  }

  guard let brace = source[start.lowerBound...].firstIndex(of: "{") else {
    return nil
  }
  return balancedDelimitedContents(in: source, openingDelimiter: brace, open: "{", close: "}")
}

private func extractBlock(named name: String, in source: String) -> String? {
  guard
    let start = firstMatch(
      of: #"\b\#(NSRegularExpression.escapedPattern(for: name))\s*:\s*\{"#,
      in: source
    )
  else {
    return nil
  }

  guard let brace = source[start.lowerBound...].firstIndex(of: "{") else {
    return nil
  }
  return balancedDelimitedContents(in: source, openingDelimiter: brace, open: "{", close: "}")
}

private func inlineObjectList(named name: String, in source: String) -> [String] {
  guard
    let start = firstMatch(
      of: #"\b\#(NSRegularExpression.escapedPattern(for: name))\s*:\s*\["#,
      in: source
    ),
    let bracket = source[start.lowerBound...].firstIndex(of: "["),
    let body = balancedDelimitedContents(in: source, openingDelimiter: bracket, open: "[", close: "]")
  else {
    return []
  }

  var objects: [String] = []
  var index = body.startIndex
  while index < body.endIndex {
    let character = body[index]
    if character == "{",
      let range = balancedDelimitedRange(in: body, openingDelimiter: index, open: "{", close: "}")
    {
      let contentStart = body.index(after: index)
      let contentEnd = body.index(before: range.upperBound)
      objects.append(String(body[contentStart..<contentEnd]))
      index = range.upperBound
      continue
    }
    index = body.index(after: index)
  }

  return objects
}

private func scalar(named name: String, in source: String) -> String? {
  let quotedPattern =
    #"\b\#(NSRegularExpression.escapedPattern(for: name))\s*:\s*"((?:[^"\\]|\\.)*)""#
  if let groups = captureGroups(matching: quotedPattern, in: source), groups.count > 1 {
    return unescapeProtoString(groups[1])
  }

  let barePattern = #"\b\#(NSRegularExpression.escapedPattern(for: name))\s*:\s*([^\s,\]}]+)"#
  if let groups = captureGroups(matching: barePattern, in: source), groups.count > 1 {
    return groups[1]
  }
  return nil
}

private func boolValue(named name: String, in source: String) -> Bool? {
  guard let value = scalar(named: name, in: source)?.trimmingCharacters(in: .whitespacesAndNewlines)
    .lowercased()
  else {
    return nil
  }

  switch value {
  case "true":
    return true
  case "false":
    return false
  default:
    return nil
  }
}

private func stringList(named name: String, in source: String) -> [String] {
  let pattern = #"\b\#(NSRegularExpression.escapedPattern(for: name))\s*:\s*\[(.*?)\]"#
  guard let regex = try? NSRegularExpression(pattern: pattern, options: [.dotMatchesLineSeparators])
  else {
    return []
  }
  guard let match = regex.firstMatch(in: source, range: nsRange(for: source)) else {
    return []
  }
  guard let range = Range(match.range(at: 1), in: source) else {
    return []
  }

  let body = String(source[range])
  guard let valueRegex = try? NSRegularExpression(pattern: #""((?:[^"\\]|\\.)*)"|([^\s,\]]+)"#)
  else {
    return []
  }
  let matches = valueRegex.matches(in: body, range: nsRange(for: body))
  return matches.compactMap { match in
    if let quotedRange = Range(match.range(at: 1), in: body) {
      return unescapeProtoString(String(body[quotedRange]))
    }
    if let bareRange = Range(match.range(at: 2), in: body) {
      return String(body[bareRange])
    }
    return nil
  }
}

private func balancedDelimitedContents(
  in source: String,
  openingDelimiter: String.Index,
  open: Character,
  close: Character
) -> String? {
  guard let range = balancedDelimitedRange(
    in: source,
    openingDelimiter: openingDelimiter,
    open: open,
    close: close
  ) else {
    return nil
  }
  let contentStart = source.index(after: openingDelimiter)
  let contentEnd = source.index(before: range.upperBound)
  return String(source[contentStart..<contentEnd])
}

private func balancedDelimitedRange(
  in source: String,
  openingDelimiter: String.Index,
  open: Character,
  close: Character
) -> Range<String.Index>? {
  var depth = 0
  var isInsideString = false
  var escapeNext = false
  var index = openingDelimiter

  while index < source.endIndex {
    let character = source[index]
    if isInsideString {
      if escapeNext {
        escapeNext = false
      } else if character == "\\" {
        escapeNext = true
      } else if character == "\"" {
        isInsideString = false
      }
    } else {
      if character == "\"" {
        isInsideString = true
      } else if character == open {
        depth += 1
      } else if character == close {
        depth -= 1
        if depth == 0 {
          return openingDelimiter..<source.index(after: index)
        }
      }
    }

    index = source.index(after: index)
  }

  return nil
}

private func firstMatch(of pattern: String, in source: String) -> Range<String.Index>? {
  guard let regex = try? NSRegularExpression(pattern: pattern, options: [.dotMatchesLineSeparators])
  else {
    return nil
  }
  guard let match = regex.firstMatch(in: source, range: nsRange(for: source)) else {
    return nil
  }
  return Range(match.range, in: source)
}

private func captureGroups(matching pattern: String, in source: String) -> [String]? {
  guard let regex = try? NSRegularExpression(pattern: pattern, options: [.dotMatchesLineSeparators])
  else {
    return nil
  }
  guard let match = regex.firstMatch(in: source, range: nsRange(for: source)) else {
    return nil
  }

  return (0..<match.numberOfRanges).compactMap { index in
    guard let range = Range(match.range(at: index), in: source) else {
      return nil
    }
    return String(source[range])
  }
}

private func nsRange(for source: String) -> NSRange {
  NSRange(source.startIndex..<source.endIndex, in: source)
}

private func unescapeProtoString(_ value: String) -> String {
  value
    .replacingOccurrences(of: #"\""#, with: "\"")
    .replacingOccurrences(of: #"\\\\"#, with: "\\")
}
