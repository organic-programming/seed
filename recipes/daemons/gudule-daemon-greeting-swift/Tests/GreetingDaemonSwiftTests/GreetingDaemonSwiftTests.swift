import GRPC
import GreetingDaemonSwiftSupport
import GreetingGenerated
import Holons
import NIOCore
import NIOPosix
import SwiftProtobuf
import XCTest
@testable import GreetingDaemonSwiftSupport

final class GreetingDaemonSwiftTests: XCTestCase {
    func testGreetingTableExposes56Languages() {
        XCTAssertEqual(greetings.count, 56)
    }

    func testLookupFallsBackToEnglish() {
        XCTAssertEqual(lookupGreeting("??").code, "en")
    }

    func testServeRoundTripReturnsBonjourForFrench() throws {
        let recipeRoot = try findGreetingDaemonSwiftRecipeRoot()
        let running = try Serve.startWithOptions(
            "tcp://127.0.0.1:0",
            serviceProviders: GreetingDaemonSwiftSupport.makeServiceProviders(),
            options: Serve.Options(
                logger: { _ in },
                protoDir: recipeRoot.appendingPathComponent("protos").path,
                holonYAMLPath: recipeRoot.appendingPathComponent("holon.yaml").path
            )
        )
        let parsed = try Transport.parse(running.publicURI)
        let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
        let channel = ClientConnection.insecure(group: group)
            .connect(host: parsed.host ?? "127.0.0.1", port: parsed.port ?? 0)

        defer {
            _ = try? channel.close().wait()
            running.stop()
            try? group.syncShutdownGracefully()
        }

        let client = GreetingClient(channel: channel)
        let languages = try client.listLanguages()
        let greeting = try client.sayHello(name: "Ada", langCode: "fr")

        XCTAssertEqual(languages.languages.count, 56)
        XCTAssertEqual(greeting.greeting, "Bonjour, Ada")
    }
}

private final class GreetingClient: GRPCClient, @unchecked Sendable {
    let channel: GRPCChannel
    var defaultCallOptions = CallOptions(timeLimit: .timeout(.seconds(2)))

    init(channel: GRPCChannel) {
        self.channel = channel
    }

    func listLanguages() throws -> Greeting_V1_ListLanguagesResponse {
        try performUnaryCall(
            path: "/greeting.v1.GreetingService/ListLanguages",
            request: TestProtobufPayload(message: Greeting_V1_ListLanguagesRequest()),
            responseType: TestProtobufPayload<Greeting_V1_ListLanguagesResponse>.self
        ).message
    }

    func sayHello(name: String, langCode: String) throws -> Greeting_V1_SayHelloResponse {
        var request = Greeting_V1_SayHelloRequest()
        request.name = name
        request.langCode = langCode

        return try performUnaryCall(
            path: "/greeting.v1.GreetingService/SayHello",
            request: TestProtobufPayload(message: request),
            responseType: TestProtobufPayload<Greeting_V1_SayHelloResponse>.self
        ).message
    }

    private func performUnaryCall<RequestPayload: GRPCPayload, ResponsePayload: GRPCPayload>(
        path: String,
        request: RequestPayload,
        responseType: ResponsePayload.Type
    ) throws -> ResponsePayload {
        let call = channel.makeUnaryCall(
            path: path,
            request: request,
            callOptions: defaultCallOptions
        ) as UnaryCall<RequestPayload, ResponsePayload>
        return try call.response.wait()
    }
}

private struct TestProtobufPayload<MessageType: SwiftProtobuf.Message & Sendable>: GRPCPayload, Sendable {
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
