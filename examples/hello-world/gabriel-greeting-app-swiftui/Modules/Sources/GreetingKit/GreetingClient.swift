import Foundation
import GRPC
import NIOCore
import SwiftProtobuf
#if os(macOS)
import Holons
#endif

final class GreetingClient: GRPCClient, @unchecked Sendable {
    let channel: GRPCChannel
    var defaultCallOptions = CallOptions(timeLimit: .timeout(.seconds(2)))
    private let closeAction: () throws -> Void

    init(channel: GRPCChannel, closeAction: @escaping () throws -> Void) {
        self.channel = channel
        self.closeAction = closeAction
    }

    static func connected(to target: String, options: ConnectOptions = ConnectOptions()) throws -> GreetingClient {
#if os(macOS)
        let channel = try connect(target, options: options)
        return GreetingClient(channel: channel) {
            try disconnect(channel)
        }
#else
        throw GreetingClientError.unsupportedPlatform
#endif
    }

    func listLanguages() async throws -> [Greeting_V1_Language] {
        let response: ProtobufPayload<Greeting_V1_ListLanguagesResponse> = try await performAsyncUnaryCall(
            path: "/greeting.v1.GreetingService/ListLanguages",
            request: ProtobufPayload(message: Greeting_V1_ListLanguagesRequest()),
            responseType: ProtobufPayload<Greeting_V1_ListLanguagesResponse>.self
        )
        return Array(response.message.languages)
    }

    func sayHello(name: String, langCode: String) async throws -> String {
        var request = Greeting_V1_SayHelloRequest()
        request.name = name
        request.langCode = langCode

        let response: ProtobufPayload<Greeting_V1_SayHelloResponse> = try await performAsyncUnaryCall(
            path: "/greeting.v1.GreetingService/SayHello",
            request: ProtobufPayload(message: request),
            responseType: ProtobufPayload<Greeting_V1_SayHelloResponse>.self
        )
        return response.message.greeting
    }

    func close() throws {
        try closeAction()
    }
}

enum GreetingClientError: LocalizedError {
    case unsupportedPlatform

    var errorDescription: String? {
        switch self {
        case .unsupportedPlatform:
            return "swift-holons connect is only available for macOS in this example"
        }
    }
}

private struct ProtobufPayload<MessageType: SwiftProtobuf.Message & Sendable>: GRPCPayload, Sendable {
    let message: MessageType

    init(message: MessageType) {
        self.message = message
    }

    init(serializedByteBuffer: inout ByteBuffer) throws {
        let data = serializedByteBuffer.readData(length: serializedByteBuffer.readableBytes) ?? Data()
        message = try MessageType(serializedBytes: data)
    }

    func serialize(into buffer: inout ByteBuffer) throws {
        let bytes: [UInt8] = try message.serializedBytes()
        buffer.writeBytes(bytes)
    }
}
