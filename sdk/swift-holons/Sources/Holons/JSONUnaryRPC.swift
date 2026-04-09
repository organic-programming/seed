import Foundation
import GRPC
import NIOCore
import SwiftProtobuf

public protocol JSONUnaryRPCMethodDescriptor: Sendable {
  var canonicalPath: String { get }
  func invoke(
    channel: GRPCChannel,
    payloadJSON: Data,
    callOptions: CallOptions?
  ) async throws -> Data
}

public struct UnaryJSONMethodDescriptor<
  Request: SwiftProtobuf.Message & Sendable,
  Response: SwiftProtobuf.Message & Sendable
>: JSONUnaryRPCMethodDescriptor {
  public let canonicalPath: String
  public let defaultCallOptions: CallOptions?

  public init(path: String, defaultCallOptions: CallOptions? = nil) {
    self.canonicalPath = canonicalGRPCMethodPath(path)
    self.defaultCallOptions = defaultCallOptions
  }

  public func invoke(
    channel: GRPCChannel,
    payloadJSON: Data,
    callOptions: CallOptions?
  ) async throws -> Data {
    let requestData = payloadJSON.isEmpty ? Data("{}".utf8) : payloadJSON
    let request = try Request(jsonUTF8Data: requestData)
    let call =
      channel.makeUnaryCall(
        path: canonicalPath,
        request: ProtobufJSONPayload(message: request),
        callOptions: callOptions ?? defaultCallOptions ?? CallOptions()
      )
      as UnaryCall<
        ProtobufJSONPayload<Request>,
        ProtobufJSONPayload<Response>
      >
    let response = try await eventLoopFutureValue(call.response)
    return try response.message.jsonUTF8Data()
  }
}

public struct UnaryJSONMethodRegistry: Sendable {
  public init(_ methods: [any JSONUnaryRPCMethodDescriptor]) {
    var indexed: [String: any JSONUnaryRPCMethodDescriptor] = [:]
    for method in methods {
      indexed[method.canonicalPath] = method
    }
    self.methodsByPath = indexed
  }

  private let methodsByPath: [String: any JSONUnaryRPCMethodDescriptor]

  public func resolve(_ method: String) throws -> any JSONUnaryRPCMethodDescriptor {
    let canonical = canonicalGRPCMethodPath(method)
    if let descriptor = methodsByPath[canonical] {
      return descriptor
    }

    let available = methodsByPath.keys.sorted().joined(separator: ", ")
    throw JSONUnaryRPCError.unknownMethod(method: method, available: available)
  }

  public func invoke(
    channel: GRPCChannel,
    method: String,
    payloadJSON: Data,
    callOptions: CallOptions? = nil
  ) async throws -> Data {
    try await resolve(method).invoke(
      channel: channel,
      payloadJSON: payloadJSON,
      callOptions: callOptions
    )
  }
}

public enum JSONUnaryRPCError: Error, LocalizedError {
  case unknownMethod(method: String, available: String)

  public var errorDescription: String? {
    switch self {
    case let .unknownMethod(method, available):
      if available.isEmpty {
        return "unknown unary gRPC method \(method)"
      }
      return "unknown unary gRPC method \(method). Available: \(available)"
    }
  }
}

public func canonicalGRPCMethodPath(_ method: String) -> String {
  let trimmed = method.trimmingCharacters(in: .whitespacesAndNewlines)
  if trimmed.hasPrefix("/") {
    return trimmed
  }
  return "/" + trimmed
}

public func invokeUnaryJSON(
  channel: GRPCChannel,
  method: String,
  payloadJSON: Data = Data("{}".utf8),
  registry: UnaryJSONMethodRegistry,
  callOptions: CallOptions? = nil
) async throws -> Data {
  try await registry.invoke(
    channel: channel,
    method: method,
    payloadJSON: payloadJSON,
    callOptions: callOptions
  )
}

private struct ProtobufJSONPayload<MessageType: SwiftProtobuf.Message & Sendable>: GRPCPayload, Sendable {
  let message: MessageType

  init(message: MessageType) {
    self.message = message
  }

  init(serializedByteBuffer: inout ByteBuffer) throws {
    let data = serializedByteBuffer.readData(length: serializedByteBuffer.readableBytes) ?? Data()
    self.message = try MessageType(serializedBytes: data)
  }

  func serialize(into buffer: inout ByteBuffer) throws {
    let bytes: [UInt8] = try message.serializedBytes()
    buffer.writeBytes(bytes)
  }
}

private func eventLoopFutureValue<T>(_ future: EventLoopFuture<T>) async throws -> T {
  try await withCheckedThrowingContinuation { continuation in
    future.whenComplete { result in
      continuation.resume(with: result)
    }
  }
}
