import Foundation
import SwiftProtobuf
import XCTest

@testable import Holons

final class DescribeTests: XCTestCase {
  func testBuildDescribeResponseParsesEchoProto() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }

    let response = try Describe.buildResponse(protoDir: root.path)

    XCTAssertEqual(response.manifest.identity.givenName, "Echo")
    XCTAssertEqual(response.manifest.identity.familyName, "Server")
    XCTAssertEqual(response.manifest.identity.motto, "Reply precisely.")
    XCTAssertEqual(response.services.count, 1)

    let service = try XCTUnwrap(response.services.first)
    XCTAssertEqual(service.name, "echo.v1.Echo")
    XCTAssertEqual(service.description_p, "Echo echoes request payloads for documentation tests.")
    XCTAssertEqual(service.methods.count, 1)

    let method = try XCTUnwrap(service.methods.first)
    XCTAssertEqual(method.name, "Ping")
    XCTAssertEqual(method.inputType, "echo.v1.PingRequest")
    XCTAssertEqual(method.outputType, "echo.v1.PingResponse")
    XCTAssertEqual(method.exampleInput, #"{"message":"hello","sdk":"go-holons"}"#)

    let field = try XCTUnwrap(method.inputFields.first)
    XCTAssertEqual(field.name, "message")
    XCTAssertEqual(field.type, "string")
    XCTAssertEqual(field.number, 1)
    XCTAssertEqual(field.description_p, "Message to echo back.")
    XCTAssertEqual(field.label, .optional)
    XCTAssertTrue(field.required)
    XCTAssertEqual(field.example, #""hello""#)
  }

  func testProviderReturnsDescribeResponse() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }

    let provider = HolonMetaDescribeProvider(response: try Describe.buildResponse(protoDir: root.path))
    let response = provider.describe()

    XCTAssertEqual(response.manifest.identity.givenName, "Echo")
    XCTAssertEqual(response.services.count, 1)
    XCTAssertEqual(response.services.first?.name, "echo.v1.Echo")
    XCTAssertEqual(response.services.first?.methods.first?.name, "Ping")
  }

  func testBuildDescribeResponseHandlesManifestOnlyHolon() throws {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("swift_holons_empty_\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: root) }

    try writeHolonProto(
      to: root.appendingPathComponent("holon.proto"),
      uuid: "silent-holon-uuid",
      givenName: "Silent",
      familyName: "Holon",
      motto: "Quietly available."
    )

    let response = try Describe.buildResponse(protoDir: root.path)

    XCTAssertEqual(response.manifest.identity.givenName, "Silent")
    XCTAssertEqual(response.manifest.identity.familyName, "Holon")
    XCTAssertEqual(response.manifest.identity.motto, "Quietly available.")
    XCTAssertTrue(response.services.isEmpty)
  }

  func testIdentityResolveAndBuildResponseExposeResolvedManifestMetadata() throws {
    let root = try writeDetailedHolon()
    defer { try? FileManager.default.removeItem(at: root) }

    let resolved = try Identity.resolve(root)
    XCTAssertEqual(resolved.sourcePath, root.appendingPathComponent("holon.proto").path)
    XCTAssertEqual(resolved.identity.version, "0.6.0")
    XCTAssertEqual(resolved.description, "Resolved manifest test.")
    XCTAssertEqual(resolved.transport, "stdio")
    XCTAssertEqual(resolved.platforms, ["macos", "linux"])
    XCTAssertEqual(resolved.requiredCommands, ["swift"])
    XCTAssertEqual(resolved.requiredFiles, ["Package.swift"])
    XCTAssertEqual(resolved.requiredPlatforms, ["macos"])
    XCTAssertEqual(resolved.skills.first?.name, "greet")
    XCTAssertEqual(resolved.skills.first?.steps, ["connect", "invoke"])
    XCTAssertEqual(resolved.sequences.first?.name, "demo")
    XCTAssertEqual(resolved.sequences.first?.params.first?.defaultValue, "World")
    XCTAssertEqual(resolved.guide, "Use the demo sequence.")

    let response = try Describe.buildResponse(protoDir: root.path)
    XCTAssertEqual(response.manifest.description_p, "Resolved manifest test.")
    XCTAssertEqual(response.manifest.identity.version, "0.6.0")
    XCTAssertEqual(response.manifest.transport, "stdio")
    XCTAssertEqual(response.manifest.platforms, ["macos", "linux"])
    XCTAssertEqual(response.manifest.requires.files, ["Package.swift"])
    XCTAssertEqual(response.manifest.skills.first?.name, "greet")
    XCTAssertEqual(response.manifest.sequences.first?.params.first?.default, "World")
    XCTAssertEqual(response.manifest.guide, "Use the demo sequence.")
  }

  func testPublicStaticDescribeRegistrationAcceptsGeneratedPayload() throws {
    let root = try writeEchoHolon()
    defer { try? FileManager.default.removeItem(at: root) }
    defer { Describe.clearStaticResponse() }

    let response = try Describe.buildResponse(protoDir: root.path)
    let payload = try response.serializedData().base64EncodedString()

    try Describe.useStaticResponse(StaticDescribeResponse(payloadBase64: payload))

    let provider = try Describe.register()
    let describeProvider = try XCTUnwrap(provider as? HolonMetaDescribeProvider)
    XCTAssertEqual(describeProvider.describe().manifest.identity.givenName, "Echo")
  }

  private func writeEchoHolon() throws -> URL {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("swift_holons_describe_\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    let protoDir = root.appendingPathComponent("protos/echo/v1", isDirectory: true)
    try FileManager.default.createDirectory(at: protoDir, withIntermediateDirectories: true)

    try writeHolonProto(
      to: root.appendingPathComponent("holon.proto"),
      packageName: "describe.v1",
      uuid: "echo-server-uuid",
      givenName: "Echo",
      familyName: "Server",
      motto: "Reply precisely."
    )

    try """
    syntax = "proto3";
    package echo.v1;

    // Echo echoes request payloads for documentation tests.
    service Echo {
      // Ping echoes the inbound message.
      // @example {"message":"hello","sdk":"go-holons"}
      rpc Ping(PingRequest) returns (PingResponse);
    }

    message PingRequest {
      // Message to echo back.
      // @required
      // @example "hello"
      string message = 1;

      // SDK marker included in the response.
      // @example "go-holons"
      string sdk = 2;
    }

    message PingResponse {
      // Echoed message.
      string message = 1;

      // SDK marker from the server.
      string sdk = 2;
    }
    """.write(to: protoDir.appendingPathComponent("echo.proto"), atomically: true, encoding: .utf8)

    return root
  }

  private func writeDetailedHolon() throws -> URL {
    let root = FileManager.default.temporaryDirectory
      .appendingPathComponent("swift_holons_identity_\(UUID().uuidString)", isDirectory: true)
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)

    try """
    syntax = "proto3";

    package identity.v1;

    import "holons/v1/manifest.proto";

    option (holons.v1.manifest) = {
      identity: {
        uuid: "identity-holon-uuid"
        given_name: "Meta"
        family_name: "Holon"
        motto: "Explain yourself."
        composer: "tester"
        status: "draft"
        born: "2026-03-23"
        version: "0.6.0"
        aliases: ["meta-holon"]
      }
      description: "Resolved manifest test."
      lang: "swift"
      kind: "native"
      platforms: ["macos", "linux"]
      transport: "stdio"
      build: {
        runner: "swift-package"
        main: "./Sources/App"
      }
      requires: {
        commands: ["swift"]
        files: ["Package.swift"]
        platforms: ["macos"]
      }
      artifacts: {
        binary: "meta-holon"
        primary: "meta-holon"
      }
      skills: [{
        name: "greet"
        description: "Say hello"
        when: "when asked"
        steps: ["connect", "invoke"]
      }]
      sequences: [{
        name: "demo"
        description: "Run the demo"
        params: [{name: "name", description: "Person", required: true, default: "World"}]
        steps: ["list-languages", "say-hello"]
      }]
      guide: "Use the demo sequence."
    };
    """.write(to: root.appendingPathComponent("holon.proto"), atomically: true, encoding: .utf8)

    return root
  }
}
