import Foundation
import HolonsApp
import XCTest

@testable import GreetingKit

final class GreetingZigMemberTests: XCTestCase {
  @MainActor
  func testSelectingZigOrganDrivesGreetingRPC() async throws {
    let fakeClient = FakeGreetingClient()
    let recorder = ConnectRecorder()

    let manager = GreetingHolonManager(
      clientFactory: { slug, options in
        recorder.append(slug: slug, transport: options.transport)
        return fakeClient
      },
      autoRefresh: false
    )
    manager.availableHolons = [
      identity(slug: "gabriel-greeting-swift", displayName: "Gabriel (Swift)", sortRank: 0),
      identity(slug: "gabriel-greeting-zig", displayName: "Gabriel (Zig)", sortRank: 3),
    ]
    manager.transport = "stdio"

    let selected = try await manager.selectHolon(slug: "gabriel-greeting-zig")
    XCTAssertEqual(selected.slug, "gabriel-greeting-zig")
    XCTAssertEqual(selected.displayName, "Gabriel (Zig)")
    XCTAssertEqual(recorder.calls.map(\.slug), ["gabriel-greeting-zig"])
    XCTAssertEqual(recorder.calls.map(\.transport), ["stdio"])

    let payload = Data(#"{"name":"Bob","langCode":"fr"}"#.utf8)
    let responseData = try await manager.tellMember(
      slug: "gabriel-greeting-zig",
      method: "greeting.v1.GreetingService/SayHello",
      payloadJSON: payload
    )
    let response = try Greeting_V1_SayHelloResponse(jsonUTF8Data: responseData)

    XCTAssertEqual(response.greeting, "Bonjour Bob from Zig")
    XCTAssertEqual(manager.greeting, "Bonjour Bob from Zig")
    XCTAssertEqual(fakeClient.sayHelloCalls, [SayHelloCall(name: "Bob", langCode: "fr")])
  }

  @MainActor
  func testSelectLanguageAndGreetRefreshesGreeting() async throws {
    let fakeClient = FakeGreetingClient()
    let recorder = ConnectRecorder()

    let manager = GreetingHolonManager(
      clientFactory: { slug, options in
        recorder.append(slug: slug, transport: options.transport)
        return fakeClient
      },
      autoRefresh: false
    )
    manager.availableHolons = [
      identity(slug: "gabriel-greeting-zig", displayName: "Gabriel (Zig)", sortRank: 3),
    ]
    manager.transport = "stdio"
    manager.userName = "Bob"

    let code = try await manager.selectLanguageAndGreet("fr")

    XCTAssertEqual(code, "fr")
    XCTAssertEqual(manager.selectedLanguageCode, "fr")
    XCTAssertEqual(manager.greeting, "Bonjour Bob from Zig")
    XCTAssertEqual(recorder.calls.map(\.slug), ["gabriel-greeting-zig"])
    XCTAssertEqual(fakeClient.sayHelloCalls, [SayHelloCall(name: "Bob", langCode: "fr")])
  }

  @MainActor
  func testConnectMemberReloadsLanguagesAndGreetsCurrentSelection() async throws {
    let fakeClient = FakeGreetingClient()
    let recorder = ConnectRecorder()

    let manager = GreetingHolonManager(
      clientFactory: { slug, options in
        recorder.append(slug: slug, transport: options.transport)
        return fakeClient
      },
      autoRefresh: false
    )
    manager.availableHolons = [
      identity(slug: "gabriel-greeting-swift", displayName: "Gabriel (Swift)", sortRank: 0),
      identity(slug: "gabriel-greeting-zig", displayName: "Gabriel (Zig)", sortRank: 3),
    ]
    manager.userName = "Ada"

    let member = try await manager.connectMember(slug: "gabriel-greeting-zig", transport: "tcp")

    XCTAssertEqual(member.slug, "gabriel-greeting-zig")
    XCTAssertEqual(member.state, .connected)
    XCTAssertEqual(manager.transport, "tcp")
    XCTAssertEqual(manager.selectedHolon?.slug, "gabriel-greeting-zig")
    XCTAssertEqual(manager.selectedLanguageCode, "en")
    XCTAssertEqual(manager.greeting, "Hello Ada from Zig")
    XCTAssertEqual(recorder.calls.map(\.slug), ["gabriel-greeting-zig"])
    XCTAssertEqual(recorder.calls.map(\.transport), ["tcp"])
    XCTAssertEqual(fakeClient.sayHelloCalls, [SayHelloCall(name: "Ada", langCode: "en")])
  }
}

private final class ConnectRecorder: @unchecked Sendable {
  private let lock = NSLock()
  private var storedCalls: [(slug: String, transport: String)] = []

  var calls: [(slug: String, transport: String)] {
    lock.lock()
    defer { lock.unlock() }
    return storedCalls
  }

  func append(slug: String, transport: String) {
    lock.lock()
    storedCalls.append((slug: slug, transport: transport))
    lock.unlock()
  }
}

private struct SayHelloCall: Equatable {
  let name: String
  let langCode: String
}

private final class FakeGreetingClient: GreetingClientProtocol, @unchecked Sendable {
  private(set) var sayHelloCalls: [SayHelloCall] = []

  func listLanguages() async throws -> [Greeting_V1_Language] {
    [
      language(code: "en", name: "English", native: "English"),
      language(code: "fr", name: "French", native: "Francais"),
    ]
  }

  func sayHello(name: String, langCode: String) async throws -> String {
    sayHelloCalls.append(SayHelloCall(name: name, langCode: langCode))
    return langCode == "fr" ? "Bonjour \(name) from Zig" : "Hello \(name) from Zig"
  }

  func tell(method: String, payloadJSON: Data) async throws -> Data {
    throw HolonError.notConnected
  }

  func close() throws {}
}

private func identity(
  slug: String,
  displayName: String,
  sortRank: Int
) -> GabrielHolonIdentity {
  GabrielHolonIdentity(
    slug: slug,
    familyName: displayName,
    binaryName: slug,
    buildRunner: slug == "gabriel-greeting-zig" ? "zig" : "swift-package",
    displayName: displayName,
    sortRank: sortRank,
    holonUUID: "\(slug)-uuid",
    born: "2026-01-01",
    sourceKind: "source",
    discoveryPath: "/tmp/\(slug)",
    hasSource: true
  )
}

private func language(code: String, name: String, native: String) -> Greeting_V1_Language {
  var value = Greeting_V1_Language()
  value.code = code
  value.name = name
  value.native = native
  return value
}
