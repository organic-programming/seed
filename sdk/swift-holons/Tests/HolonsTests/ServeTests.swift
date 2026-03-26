import GRPC
import NIOCore
import NIOPosix
import SwiftProtobuf
import XCTest

@testable import Holons

final class ServeTests: XCTestCase {
  func testRetryableBridgeErrnosCoverTransientReadWriteFailures() {
    XCTAssertTrue(isRetryableBridgeErrno(EINTR))
    XCTAssertTrue(isRetryableBridgeErrno(EAGAIN))
    XCTAssertTrue(isRetryableBridgeErrno(EWOULDBLOCK))
    XCTAssertFalse(isRetryableBridgeErrno(EBADF))
  }

  func testStartWithOptionsRegistersDescribeService() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }
    Describe.useStaticResponse(try Describe.buildResponse(protoDir: root.path))
    defer { Describe.useStaticResponse(nil) }

    let running = try Serve.startWithOptions(
      "tcp://127.0.0.1:0",
      serviceProviders: [],
      options: Serve.Options(logger: { _ in })
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

    let call =
      channel.makeUnaryCall(
        path: "/holons.v1.HolonMeta/Describe",
        request: TestProtobufPayload(message: Holons_V1_DescribeRequest()),
        callOptions: CallOptions()
      )
      as UnaryCall<
        TestProtobufPayload<Holons_V1_DescribeRequest>,
        TestProtobufPayload<Holons_V1_DescribeResponse>
      >
    let response = try call.response.wait().message

    XCTAssertEqual(response.manifest.identity.givenName, "Echo")
    XCTAssertEqual(response.services.count, 1)
    XCTAssertEqual(response.services.first?.name, "echo.v1.Echo")
  }

  func testStartWithOptionsRegistersDescribeServiceOverUnix() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }
    Describe.useStaticResponse(try Describe.buildResponse(protoDir: root.path))
    defer { Describe.useStaticResponse(nil) }

    let socketPath = root.appendingPathComponent("serve.sock").path
    let running = try Serve.startWithOptions(
      "unix://\(socketPath)",
      serviceProviders: [],
      options: Serve.Options(logger: { _ in })
    )

    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
    var configuration = ClientConnection.Configuration.default(
      target: .unixDomainSocket(socketPath),
      eventLoopGroup: group
    )
    configuration.connectionBackoff = nil
    let channel = ClientConnection(configuration: configuration)

    defer {
      _ = try? channel.close().wait()
      running.stop()
      try? group.syncShutdownGracefully()
    }

    let call =
      channel.makeUnaryCall(
        path: "/holons.v1.HolonMeta/Describe",
        request: TestProtobufPayload(message: Holons_V1_DescribeRequest()),
        callOptions: CallOptions()
      )
      as UnaryCall<
        TestProtobufPayload<Holons_V1_DescribeRequest>,
        TestProtobufPayload<Holons_V1_DescribeResponse>
      >
    let response = try call.response.wait().message

    XCTAssertEqual(running.publicURI, "unix://\(socketPath)")
    XCTAssertEqual(response.manifest.identity.givenName, "Echo")
    XCTAssertEqual(response.services.count, 1)
    XCTAssertEqual(response.services.first?.name, "echo.v1.Echo")
  }

  func testStartWithOptionsRegistersReflectionWhenEnabled() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }
    Describe.useStaticResponse(try Describe.buildResponse(protoDir: root.path))
    defer { Describe.useStaticResponse(nil) }

    let running = try Serve.startWithOptions(
      "tcp://127.0.0.1:0",
      serviceProviders: [],
      options: Serve.Options(
        reflect: true,
        logger: { _ in },
        protoDir: root.path
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

    let client = GRPCAnyServiceClient(channel: channel)
    var responses: [Grpc_Reflection_V1alpha_ServerReflectionResponse] = []
    let statusExpectation = expectation(description: "reflection completed")

    let rpc: BidirectionalStreamingCall<
      Grpc_Reflection_V1alpha_ServerReflectionRequest,
      Grpc_Reflection_V1alpha_ServerReflectionResponse
    > = client.makeBidirectionalStreamingCall(
      path: "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
      handler: { responses.append($0) }
    )

    var request = Grpc_Reflection_V1alpha_ServerReflectionRequest()
    request.listServices = "*"
    rpc.sendMessage(request, promise: nil)
    rpc.sendEnd(promise: nil)
    rpc.status.whenComplete { result in
      switch result {
      case .success(let status):
        XCTAssertEqual(status.code, .ok)
      case .failure(let error):
        XCTFail("reflection failed: \(error)")
      }
      statusExpectation.fulfill()
    }

    wait(for: [statusExpectation], timeout: 2.0)

    let services = responses.flatMap { $0.listServicesResponse.service }.map(\.name)
    XCTAssertTrue(services.contains("echo.v1.Echo"))
  }

  func testStartWithOptionsFailsWithoutRegisteredIncodeDescription() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }
    Describe.useStaticResponse(nil)

    var logs: [String] = []
    XCTAssertThrowsError(
      try Serve.startWithOptions(
        "tcp://127.0.0.1:0",
        serviceProviders: [],
        options: Serve.Options(
          logger: { logs.append($0) },
          protoDir: root.path
        )
      )
    ) { error in
      XCTAssertEqual(
        String(describing: error),
        Describe.noIncodeDescriptionMessage
      )
    }

    XCTAssertTrue(
      logs.contains { $0.contains("HolonMeta registration failed: \(Describe.noIncodeDescriptionMessage)") }
    )
  }

  private func writeEchoHolon() throws -> URL {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("shs_\(UUID().uuidString.prefix(8))", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    let protoDir = root.appendingPathComponent("protos/echo/v1", isDirectory: true)
    try FileManager.default.createDirectory(at: protoDir, withIntermediateDirectories: true)

    try writeHolonProto(
      to: root.appendingPathComponent("holon.proto"),
      packageName: "serve.v1",
      uuid: "echo-server-uuid",
      givenName: "Echo",
      familyName: "Server",
      motto: "Reply precisely."
    )

    try """
    syntax = "proto3";
    package echo.v1;

    service Echo {
      rpc Ping(PingRequest) returns (PingResponse);
    }

    message PingRequest {
      string message = 1;
    }

    message PingResponse {
      string message = 1;
    }
    """.write(to: protoDir.appendingPathComponent("echo.proto"), atomically: true, encoding: .utf8)

    return root
  }
}

private struct TestProtobufPayload<MessageType: SwiftProtobuf.Message & Sendable>: GRPCPayload,
  Sendable
{
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
