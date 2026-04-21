import Foundation
import GRPC
import NIOCore

@available(*, deprecated, message: "Use Holons_V1_DescribeResponse directly.")
public struct StaticDescribeResponse: Equatable, Sendable {
  public let payloadBase64: String

  public init(payloadBase64: String) {
    self.payloadBase64 = payloadBase64
  }
}

public enum DescribeRegistrationError: Error, CustomStringConvertible {
  case noIncodeDescription
  case invalidGeneratedResponse(String)

  public var description: String {
    switch self {
    case .noIncodeDescription:
      return Describe.noIncodeDescriptionMessage
    case .invalidGeneratedResponse(let detail):
      return "invalid generated Incode Description: \(detail)"
    }
  }
}

public enum Describe {
  public static let noIncodeDescriptionMessage = "no Incode Description registered — run op build"

  private static let staticResponseLock = NSLock()
  private static var registeredResponse: Holons_V1_DescribeResponse?

  public static func useStaticResponse(_ response: Holons_V1_DescribeResponse) throws {
    useStaticResponse(Optional(response))
  }

  @available(*, deprecated, message: "Use Describe.useStaticResponse(_ response: Holons_V1_DescribeResponse) instead.")
  public static func useStaticResponse(_ response: StaticDescribeResponse) throws {
    guard let data = Data(base64Encoded: response.payloadBase64) else {
      throw DescribeRegistrationError.invalidGeneratedResponse("payload is not valid base64")
    }

    do {
      try useStaticResponse(Holons_V1_DescribeResponse(serializedBytes: data))
    } catch {
      throw DescribeRegistrationError.invalidGeneratedResponse(String(describing: error))
    }
  }

  public static func clearStaticResponse() {
    useStaticResponse(nil)
  }

  static func useStaticResponse(_ response: Holons_V1_DescribeResponse?) {
    staticResponseLock.lock()
    registeredResponse = response
    staticResponseLock.unlock()
  }

  static func staticResponse() -> Holons_V1_DescribeResponse? {
    staticResponseLock.lock()
    let response = registeredResponse
    staticResponseLock.unlock()
    return response
  }

  static func register() throws -> Holons_V1_HolonMetaProvider {
    guard let response = staticResponse() else {
      throw DescribeRegistrationError.noIncodeDescription
    }
    return HolonMetaDescribeProvider(response: response)
  }

  // Build-time utility for op build and tests. Not used at runtime serve startup.
  static func buildResponse(protoDir: String) throws -> Holons_V1_DescribeResponse {
    let resolved = try Identity.resolveManifest(in: URL(fileURLWithPath: protoDir, isDirectory: true))
      .manifest
    let index = try parseProtoDirectory(URL(fileURLWithPath: protoDir))

    var response = Holons_V1_DescribeResponse()
    response.manifest = protoManifest(from: resolved)
    response.services = index.services
      .filter { $0.fullName != "holons.v1.HolonMeta" }
      .map { serviceDoc($0, index: index) }
    return response
  }
}

final class HolonMetaDescribeProvider: Holons_V1_HolonMetaProvider {
  private let response: Holons_V1_DescribeResponse

  init(response: Holons_V1_DescribeResponse) {
    self.response = response
  }

  func describe(_ request: Holons_V1_DescribeRequest = .init())
    -> Holons_V1_DescribeResponse
  {
    _ = request
    return response
  }

  func describe(
    request: Holons_V1_DescribeRequest,
    context: StatusOnlyCallContext
  ) -> EventLoopFuture<Holons_V1_DescribeResponse> {
    _ = request
    return context.eventLoop.makeSucceededFuture(response)
  }
}

private func protoManifest(from resolved: HolonResolvedManifest) -> Holons_V1_HolonManifest {
  var manifest = Holons_V1_HolonManifest()
  manifest.identity = .with {
    $0.schema = "holon/v1"
    $0.uuid = resolved.identity.uuid
    $0.givenName = resolved.identity.givenName
    $0.familyName = resolved.identity.familyName
    $0.motto = resolved.identity.motto
    $0.composer = resolved.identity.composer
    $0.status = resolved.identity.status
    $0.born = resolved.identity.born
    $0.version = resolved.identity.version
    $0.aliases = resolved.identity.aliases
  }
  manifest.description_p = resolved.description
  manifest.lang = resolved.identity.lang
  manifest.skills = resolved.skills.map { skill in
    .with {
      $0.name = skill.name
      $0.description_p = skill.description
      $0.when = skill.when
      $0.steps = skill.steps
    }
  }
  manifest.kind = resolved.kind
  manifest.platforms = resolved.platforms
  manifest.transport = resolved.transport
  manifest.build = .with {
    $0.runner = resolved.buildRunner
    $0.main = resolved.buildMain
  }
  if !resolved.requiredCommands.isEmpty || !resolved.requiredFiles.isEmpty || !resolved.requiredPlatforms.isEmpty {
    manifest.requires = .with {
      $0.commands = resolved.requiredCommands
      $0.files = resolved.requiredFiles
      $0.platforms = resolved.requiredPlatforms
    }
  }
  manifest.artifacts = .with {
    $0.binary = resolved.artifactBinary
    $0.primary = resolved.artifactPrimary
  }
  manifest.sequences = resolved.sequences.map { sequence in
    .with {
      $0.name = sequence.name
      $0.description_p = sequence.description
      $0.params = sequence.params.map { param in
        .with {
          $0.name = param.name
          $0.description_p = param.description
          $0.required = param.required
          $0.default = param.defaultValue
        }
      }
      $0.steps = sequence.steps
    }
  }
  manifest.guide = resolved.guide
  return manifest
}

private let packageRegex = try! NSRegularExpression(pattern: #"^package\s+([A-Za-z0-9_.]+)\s*;"#)
private let serviceRegex = try! NSRegularExpression(
  pattern: #"^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?"#)
private let messageRegex = try! NSRegularExpression(
  pattern: #"^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?"#)
private let enumRegex = try! NSRegularExpression(
  pattern: #"^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?"#)
private let rpcRegex = try! NSRegularExpression(
  pattern:
    #"^rpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*;?"#
)
private let mapFieldRegex = try! NSRegularExpression(
  pattern:
    #"^(repeated\s+)?map\s*<\s*([.A-Za-z0-9_]+)\s*,\s*([.A-Za-z0-9_]+)\s*>\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;"#
)
private let fieldRegex = try! NSRegularExpression(
  pattern:
    #"^(optional\s+|repeated\s+)?([.A-Za-z0-9_]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;"#)
private let enumValueRegex = try! NSRegularExpression(
  pattern: #"^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(-?\d+)\s*;"#)
private let scalarTypes: Set<String> = [
  "double", "float", "int64", "uint64", "int32", "fixed64", "fixed32",
  "bool", "string", "bytes", "uint32", "sfixed32", "sfixed64", "sint32", "sint64",
]

private func serviceDoc(_ service: ServiceDef, index: ProtoIndex) -> Holons_V1_ServiceDoc {
  var doc = Holons_V1_ServiceDoc()
  doc.name = service.fullName
  doc.description_p = service.comment.description
  doc.methods = service.methods.map { methodDoc($0, index: index) }
  return doc
}

private func methodDoc(_ method: MethodDef, index: ProtoIndex) -> Holons_V1_MethodDoc {
  var doc = Holons_V1_MethodDoc()
  doc.name = method.name
  doc.description_p = method.comment.description
  doc.inputType = method.inputType
  doc.outputType = method.outputType
  doc.clientStreaming = method.clientStreaming
  doc.serverStreaming = method.serverStreaming
  doc.exampleInput = method.comment.example
  if let input = index.messages[method.inputType] {
    doc.inputFields = input.fields.map { fieldDoc($0, index: index, seen: []) }
  }
  if let output = index.messages[method.outputType] {
    doc.outputFields = output.fields.map { fieldDoc($0, index: index, seen: []) }
  }
  return doc
}

private func fieldDoc(_ field: FieldDef, index: ProtoIndex, seen: Set<String>)
  -> Holons_V1_FieldDoc
{
  var doc = Holons_V1_FieldDoc()
  doc.name = field.name
  doc.type = field.typeName
  doc.number = Int32(field.number)
  doc.description_p = field.comment.description
  doc.label = field.label
  doc.required = field.comment.required
  doc.example = field.comment.example
  doc.mapKeyType = field.mapKeyType ?? ""
  doc.mapValueType = field.mapValueType ?? ""

  if field.cardinality == .map {
    let mapValueType = field.resolvedMapValueType(index: index)
    if let nested = index.messages[mapValueType], !seen.contains(nested.fullName) {
      let nextSeen = seen.union([nested.fullName])
      doc.nestedFields = nested.fields.map { fieldDoc($0, index: index, seen: nextSeen) }
    }
    if let enumDef = index.enums[mapValueType] {
      doc.enumValues = enumDef.values.map(enumValueDoc)
    }
    return doc
  }

  let resolvedType = field.resolvedType(index: index)
  if let nested = index.messages[resolvedType], !seen.contains(nested.fullName) {
    let nextSeen = seen.union([nested.fullName])
    doc.nestedFields = nested.fields.map { fieldDoc($0, index: index, seen: nextSeen) }
  }
  if let enumDef = index.enums[resolvedType] {
    doc.enumValues = enumDef.values.map(enumValueDoc)
  }
  return doc
}

private func enumValueDoc(_ value: EnumValueDef) -> Holons_V1_EnumValueDoc {
  var doc = Holons_V1_EnumValueDoc()
  doc.name = value.name
  doc.number = Int32(value.number)
  doc.description_p = value.comment.description
  return doc
}

private func parseProtoDirectory(_ dir: URL) throws -> ProtoIndex {
  var index = ProtoIndex()
  guard FileManager.default.fileExists(atPath: dir.path) else {
    return index
  }

  let files = try FileManager.default.subpathsOfDirectory(atPath: dir.path)
    .filter { $0.hasSuffix(".proto") }
    .sorted()

  for relative in files {
    try parseProtoFile(dir.appendingPathComponent(relative), index: &index)
  }

  return index
}

private func parseProtoFile(_ path: URL, index: inout ProtoIndex) throws {
  var packageName = ""
  var stack: [Block] = []
  var pendingComments: [String] = []

  let text = try String(contentsOf: path, encoding: .utf8)
  for raw in text.split(separator: "\n", omittingEmptySubsequences: false) {
    let line = raw.trimmingCharacters(in: .whitespaces)
    if line.hasPrefix("//") {
      pendingComments.append(String(line.dropFirst(2)).trimmingCharacters(in: .whitespaces))
      continue
    }
    if line.isEmpty {
      continue
    }

    if let groups = match(packageRegex, line) {
      packageName = groups[1]
      pendingComments.removeAll()
      continue
    }
    if let groups = match(serviceRegex, line) {
      index.services.append(
        ServiceDef(
          fullName: qualify(packageName, groups[1]), comment: CommentMeta.parse(pendingComments)))
      pendingComments.removeAll()
      stack.append(.service(groups[1]))
      trimClosedBlocks(line, stack: &stack)
      continue
    }
    if let groups = match(messageRegex, line) {
      let scope = messageScope(stack)
      let message = MessageDef(
        fullName: qualify(packageName, qualifyScope(scope, groups[1])), scope: scope)
      index.messages[message.fullName] = message
      index.simpleTypes[message.simpleKey] =
        index.simpleTypes[message.simpleKey] ?? message.fullName
      pendingComments.removeAll()
      stack.append(.message(groups[1]))
      trimClosedBlocks(line, stack: &stack)
      continue
    }
    if let groups = match(enumRegex, line) {
      let scope = messageScope(stack)
      let enumDef = EnumDef(
        fullName: qualify(packageName, qualifyScope(scope, groups[1])), scope: scope)
      index.enums[enumDef.fullName] = enumDef
      index.simpleTypes[enumDef.simpleKey] =
        index.simpleTypes[enumDef.simpleKey] ?? enumDef.fullName
      pendingComments.removeAll()
      stack.append(.enumType(groups[1]))
      trimClosedBlocks(line, stack: &stack)
      continue
    }

    switch stack.last {
    case .service:
      if let groups = match(rpcRegex, line), !index.services.isEmpty {
        index.services[index.services.count - 1].methods.append(
          MethodDef(
            name: groups[1],
            inputType: resolveTypeName(
              groups[3], packageName: packageName, scope: [], index: index),
            outputType: resolveTypeName(
              groups[5], packageName: packageName, scope: [], index: index),
            clientStreaming: !groups[2].isEmpty,
            serverStreaming: !groups[4].isEmpty,
            comment: CommentMeta.parse(pendingComments)
          )
        )
        pendingComments.removeAll()
        trimClosedBlocks(line, stack: &stack)
        continue
      }
    case .message:
      let scope = messageScope(stack)
      let key = qualify(packageName, scope.joined(separator: "."))
      if let groups = match(mapFieldRegex, line) {
        let mapKeyType = resolveTypeName(
          groups[2], packageName: packageName, scope: scope, index: index)
        let mapValueType = resolveTypeName(
          groups[3], packageName: packageName, scope: scope, index: index)
        let field = FieldDef(
          name: groups[4],
          number: Int(groups[5]) ?? 0,
          comment: CommentMeta.parse(pendingComments),
          cardinality: .map,
          type: nil,
          mapKeyType: mapKeyType,
          mapValueType: mapValueType,
          packageName: packageName,
          scope: scope
        )
        if var message = index.messages[key] {
          message.fields.append(field)
          index.messages[key] = message
        }
        pendingComments.removeAll()
        trimClosedBlocks(line, stack: &stack)
        continue
      }
      if let groups = match(fieldRegex, line) {
        let resolvedType = resolveTypeName(
          groups[2], packageName: packageName, scope: scope, index: index)
        let field = FieldDef(
          name: groups[3],
          number: Int(groups[4]) ?? 0,
          comment: CommentMeta.parse(pendingComments),
          cardinality: groups[1].trimmingCharacters(in: .whitespaces) == "repeated"
            ? .repeated : .optional,
          type: resolvedType,
          mapKeyType: nil,
          mapValueType: nil,
          packageName: packageName,
          scope: scope
        )
        if var message = index.messages[key] {
          message.fields.append(field)
          index.messages[key] = message
        }
        pendingComments.removeAll()
        trimClosedBlocks(line, stack: &stack)
        continue
      }
    case .enumType(let name):
      let key = qualify(packageName, qualifyScope(messageScope(stack), name))
      if let groups = match(enumValueRegex, line) {
        index.enums[key]?.values.append(
          EnumValueDef(
            name: groups[1],
            number: Int(groups[2]) ?? 0,
            comment: CommentMeta.parse(pendingComments)
          )
        )
        pendingComments.removeAll()
        trimClosedBlocks(line, stack: &stack)
        continue
      }
    case nil:
      break
    }

    pendingComments.removeAll()
    trimClosedBlocks(line, stack: &stack)
  }
}

private func trimClosedBlocks(_ line: String, stack: inout [Block]) {
  let closers = line.filter { $0 == "}" }.count
  for _ in 0..<closers where !stack.isEmpty {
    stack.removeLast()
  }
}

private func messageScope(_ stack: [Block]) -> [String] {
  stack.compactMap {
    if case .message(let name) = $0 { return name }
    return nil
  }
}

private func qualify(_ packageName: String, _ name: String) -> String {
  if name.isEmpty { return "" }
  let cleaned = name.hasPrefix(".") ? String(name.dropFirst()) : name
  if cleaned.contains(".") || packageName.isEmpty { return cleaned }
  return "\(packageName).\(cleaned)"
}

private func qualifyScope(_ scope: [String], _ name: String) -> String {
  scope.isEmpty ? name : "\(scope.joined(separator: ".")).\(name)"
}

private func resolveTypeName(
  _ typeName: String, packageName: String, scope: [String], index: ProtoIndex
) -> String {
  let cleaned = typeName.trimmingCharacters(in: .whitespaces)
  if cleaned.isEmpty { return "" }
  if cleaned.hasPrefix(".") { return String(cleaned.dropFirst()) }
  if scalarTypes.contains(cleaned) { return cleaned }
  if cleaned.contains(".") {
    let qualified = qualify(packageName, cleaned)
    if index.messages[qualified] != nil || index.enums[qualified] != nil { return qualified }
    return cleaned
  }
  for i in stride(from: scope.count, through: 0, by: -1) {
    let candidate = qualify(packageName, qualifyScope(Array(scope.prefix(i)), cleaned))
    if index.messages[candidate] != nil || index.enums[candidate] != nil { return candidate }
  }
  if let nested = index.simpleTypes[qualifyScope(scope, cleaned)] { return nested }
  if let direct = index.simpleTypes[cleaned] { return direct }
  return qualify(packageName, cleaned)
}

private func match(_ regex: NSRegularExpression, _ line: String) -> [String]? {
  let range = NSRange(line.startIndex..<line.endIndex, in: line)
  guard let match = regex.firstMatch(in: line, range: range) else { return nil }
  return (0..<match.numberOfRanges).map { index in
    let nsRange = match.range(at: index)
    guard let swiftRange = Range(nsRange, in: line) else { return "" }
    return String(line[swiftRange])
  }
}

private struct ProtoIndex {
  var services: [ServiceDef] = []
  var messages: [String: MessageDef] = [:]
  var enums: [String: EnumDef] = [:]
  var simpleTypes: [String: String] = [:]
}

private struct ServiceDef {
  let fullName: String
  let comment: CommentMeta
  var methods: [MethodDef] = []
}

private struct MethodDef {
  let name: String
  let inputType: String
  let outputType: String
  let clientStreaming: Bool
  let serverStreaming: Bool
  let comment: CommentMeta
}

private struct MessageDef {
  let fullName: String
  let scope: [String]
  var fields: [FieldDef] = []

  var simpleKey: String {
    qualifyScope(scope, fullName.split(separator: ".").last.map(String.init) ?? "")
  }
}

private struct EnumDef {
  let fullName: String
  let scope: [String]
  var values: [EnumValueDef] = []

  var simpleKey: String {
    qualifyScope(scope, fullName.split(separator: ".").last.map(String.init) ?? "")
  }
}

private struct EnumValueDef {
  let name: String
  let number: Int
  let comment: CommentMeta
}

private enum FieldCardinality { case optional, repeated, map }

private struct FieldDef {
  let name: String
  let number: Int
  let comment: CommentMeta
  let cardinality: FieldCardinality
  let type: String?
  let mapKeyType: String?
  let mapValueType: String?
  let packageName: String
  let scope: [String]

  var typeName: String {
    if cardinality == .map { return "map<\(mapKeyType ?? ""), \(mapValueType ?? "")>" }
    return type ?? ""
  }

  var label: Holons_V1_FieldLabel {
    switch cardinality {
    case .optional: return .optional
    case .repeated: return .repeated
    case .map: return .map
    }
  }

  func resolvedType(index: ProtoIndex) -> String {
    resolveTypeName(type ?? "", packageName: packageName, scope: scope, index: index)
  }

  func resolvedMapValueType(index: ProtoIndex) -> String {
    resolveTypeName(mapValueType ?? "", packageName: packageName, scope: scope, index: index)
  }
}

private struct CommentMeta {
  let description: String
  let required: Bool
  let example: String

  static func parse(_ lines: [String]) -> CommentMeta {
    var description: [String] = []
    var examples: [String] = []
    var required = false

    for raw in lines {
      let line = raw.trimmingCharacters(in: .whitespaces)
      if line.isEmpty { continue }
      if line == "@required" {
        required = true
        continue
      }
      if line.hasPrefix("@example") {
        let example = String(line.dropFirst("@example".count)).trimmingCharacters(in: .whitespaces)
        if !example.isEmpty { examples.append(example) }
        continue
      }
      description.append(line)
    }

    return CommentMeta(
      description: description.joined(separator: " "),
      required: required,
      example: examples.joined(separator: "\n")
    )
  }
}

private enum Block {
  case service(String)
  case message(String)
  case enumType(String)
}
