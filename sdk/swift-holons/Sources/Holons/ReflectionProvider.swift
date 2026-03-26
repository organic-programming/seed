import Foundation
import GRPC
import NIOCore
import SwiftProtobuf

final class ReflectionProvider: Grpc_Reflection_V1alpha_ServerReflectionProvider {
  private let catalog: ReflectionCatalog

  init(protoDir: String) throws {
    self.catalog = try ReflectionCatalog(protoDir: protoDir)
  }

  func serverReflectionInfo(
    context: StreamingResponseCallContext<Grpc_Reflection_V1alpha_ServerReflectionResponse>
  ) -> EventLoopFuture<(StreamEvent<Grpc_Reflection_V1alpha_ServerReflectionRequest>) -> Void> {
    context.eventLoop.makeSucceededFuture { [catalog] event in
      switch event {
      case .message(let request):
        context.sendResponse(catalog.response(for: request), promise: nil)
      case .end:
        context.statusPromise.succeed(.ok)
      }
    }
  }
}

private struct ReflectionCatalog {
  private let services: [String]
  private let filesByName: [String: Google_Protobuf_FileDescriptorProto]
  private let fileDataByName: [String: Data]
  private let symbolsToFileNames: [String: String]

  init(protoDir: String) throws {
    let descriptorSet = try Self.loadDescriptorSet(protoDir: URL(fileURLWithPath: protoDir, isDirectory: true))
    var filesByName: [String: Google_Protobuf_FileDescriptorProto] = [:]
    var fileDataByName: [String: Data] = [:]
    var symbolsToFileNames: [String: String] = [:]
    var services: [String] = []

    for file in descriptorSet.file where !file.name.isEmpty {
      filesByName[file.name] = file
      fileDataByName[file.name] = try file.serializedData()

      for service in file.service {
        let serviceName = Self.qualify(file.package, service.name)
        if !serviceName.isEmpty {
          services.append(serviceName)
          symbolsToFileNames[serviceName] = file.name
          for method in service.method where !method.name.isEmpty {
            symbolsToFileNames["\(serviceName).\(method.name)"] = file.name
          }
        }
      }

      for message in file.messageType {
        Self.index(message: message, prefix: file.package, fileName: file.name, symbols: &symbolsToFileNames)
      }
      for enumType in file.enumType {
        let enumName = Self.qualify(file.package, enumType.name)
        if !enumName.isEmpty {
          symbolsToFileNames[enumName] = file.name
        }
      }
    }

    self.services = Array(Set(services)).sorted()
    self.filesByName = filesByName
    self.fileDataByName = fileDataByName
    self.symbolsToFileNames = symbolsToFileNames
  }

  func response(
    for request: Grpc_Reflection_V1alpha_ServerReflectionRequest
  ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
    switch request.messageRequest {
    case .listServices:
      var payload = Grpc_Reflection_V1alpha_ListServiceResponse()
      payload.service = services.map { name in
        var response = Grpc_Reflection_V1alpha_ServiceResponse()
        response.name = name
        return response
      }

      var response = baseResponse(for: request)
      response.listServicesResponse = payload
      return response

    case .fileContainingSymbol(let symbol):
      guard let fileName = symbolsToFileNames[symbol] else {
        return errorResponse(for: request, code: 5, message: "symbol not found: \(symbol)")
      }
      return descriptorResponse(for: request, startingAt: fileName)

    case .fileByFilename(let filename):
      guard filesByName[filename] != nil else {
        return errorResponse(for: request, code: 5, message: "file not found: \(filename)")
      }
      return descriptorResponse(for: request, startingAt: filename)

    case .fileContainingExtension:
      return errorResponse(
        for: request,
        code: 12,
        message: "file_containing_extension is not implemented"
      )

    case .allExtensionNumbersOfType:
      return errorResponse(
        for: request,
        code: 12,
        message: "all_extension_numbers_of_type is not implemented"
      )

    case .none:
      return errorResponse(for: request, code: 3, message: "empty reflection request")
    }
  }

  private func descriptorResponse(
    for request: Grpc_Reflection_V1alpha_ServerReflectionRequest,
    startingAt fileName: String
  ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
    var payload = Grpc_Reflection_V1alpha_FileDescriptorResponse()
    payload.fileDescriptorProto = descriptorClosure(startingAt: fileName)

    var response = baseResponse(for: request)
    response.fileDescriptorResponse = payload
    return response
  }

  private func descriptorClosure(startingAt fileName: String) -> [Data] {
    var visited: Set<String> = []
    var ordered: [Data] = []

    func visit(_ current: String) {
      guard !visited.contains(current), let file = filesByName[current] else {
        return
      }
      visited.insert(current)
      for dependency in file.dependency {
        visit(dependency)
      }
      if let data = fileDataByName[current] {
        ordered.append(data)
      }
    }

    visit(fileName)
    return ordered
  }

  private func baseResponse(
    for request: Grpc_Reflection_V1alpha_ServerReflectionRequest
  ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
    var response = Grpc_Reflection_V1alpha_ServerReflectionResponse()
    response.validHost = request.host
    response.originalRequest = request
    return response
  }

  private func errorResponse(
    for request: Grpc_Reflection_V1alpha_ServerReflectionRequest,
    code: Int32,
    message: String
  ) -> Grpc_Reflection_V1alpha_ServerReflectionResponse {
    var payload = Grpc_Reflection_V1alpha_ErrorResponse()
    payload.errorCode = code
    payload.errorMessage = message

    var response = baseResponse(for: request)
    response.errorResponse = payload
    return response
  }

  private static func loadDescriptorSet(protoDir: URL) throws -> Google_Protobuf_FileDescriptorSet {
    let includeRoots = includeRoots(for: protoDir)
    let inputs = try protoInputs(includeRoots: includeRoots)
    guard !inputs.isEmpty else {
      throw NSError(
        domain: "Holons.Reflection",
        code: 1,
        userInfo: [NSLocalizedDescriptionKey: "no .proto files found under \(protoDir.path)"]
      )
    }

    let tempDir = FileManager.default.temporaryDirectory
      .appendingPathComponent("holons-reflection-\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: tempDir) }

    let output = tempDir.appendingPathComponent("reflection.pb")
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")

    var args = ["protoc"]
    for root in includeRoots {
      args.append("-I")
      args.append(root.path)
    }
    args.append("--include_imports")
    args.append("--descriptor_set_out=\(output.path)")
    args.append(contentsOf: inputs)
    process.arguments = args

    let stderr = Pipe()
    process.standardError = stderr
    process.standardOutput = Pipe()

    try process.run()
    process.waitUntilExit()

    if process.terminationStatus != 0 {
      let message = String(data: stderr.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8)?
        .trimmingCharacters(in: .whitespacesAndNewlines)
      throw NSError(
        domain: "Holons.Reflection",
        code: Int(process.terminationStatus),
        userInfo: [
          NSLocalizedDescriptionKey: message?.isEmpty == false
            ? message!
            : "protoc failed while building reflection descriptors"
        ]
      )
    }

    let data = try Data(contentsOf: output)
    return try Google_Protobuf_FileDescriptorSet(serializedBytes: data)
  }

  private static func includeRoots(for protoDir: URL) -> [URL] {
    var roots: [URL] = []
    for child in ["api", "protos"] {
      let candidate = protoDir.appendingPathComponent(child, isDirectory: true).standardizedFileURL
      if FileManager.default.fileExists(atPath: candidate.path) {
        roots.append(candidate)
      }
    }
    roots.append(protoDir.standardizedFileURL)

    var current = protoDir.standardizedFileURL
    while true {
      let shared = current.appendingPathComponent("_protos", isDirectory: true).standardizedFileURL
      if FileManager.default.fileExists(atPath: shared.path) {
        roots.append(shared)
      }

      let parent = current.deletingLastPathComponent().standardizedFileURL
      if parent.path == current.path {
        break
      }
      current = parent
    }

    var seen: Set<String> = []
    return roots.filter { seen.insert($0.path).inserted }
  }

  private static func protoInputs(includeRoots: [URL]) throws -> [String] {
    var inputs: [String] = []
    var seenPaths: Set<String> = []

    for root in includeRoots where FileManager.default.fileExists(atPath: root.path) {
      let files = try FileManager.default.subpathsOfDirectory(atPath: root.path)
        .filter { $0.hasSuffix(".proto") && URL(fileURLWithPath: $0).lastPathComponent != "holon.proto" }
        .sorted()
      for file in files {
        let absolutePath = root.appendingPathComponent(file).standardizedFileURL.path
        guard seenPaths.insert(absolutePath).inserted else {
          continue
        }
        let normalized = file.replacingOccurrences(of: "\\", with: "/")
        inputs.append(normalized)
      }
    }

    return inputs
  }

  private static func index(
    message: Google_Protobuf_DescriptorProto,
    prefix: String,
    fileName: String,
    symbols: inout [String: String]
  ) {
    let messageName = qualify(prefix, message.name)
    if !messageName.isEmpty {
      symbols[messageName] = fileName
    }

    for enumType in message.enumType {
      let enumName = qualify(messageName, enumType.name)
      if !enumName.isEmpty {
        symbols[enumName] = fileName
      }
    }
    for nested in message.nestedType {
      index(message: nested, prefix: messageName, fileName: fileName, symbols: &symbols)
    }
  }

  private static func qualify(_ prefix: String, _ name: String) -> String {
    guard !name.isEmpty else {
      return prefix
    }
    guard !prefix.isEmpty else {
      return name
    }
    return "\(prefix).\(name)"
  }
}
