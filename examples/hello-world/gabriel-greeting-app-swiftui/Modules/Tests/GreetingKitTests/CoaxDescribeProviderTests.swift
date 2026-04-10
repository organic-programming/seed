import XCTest

@testable import GreetingKit

final class CoaxDescribeProviderTests: XCTestCase {
  func testGeneratedDescribeIncludesSelectTransport() throws {
    let fileURL = URL(fileURLWithPath: #filePath)
    let exampleRoot =
      fileURL
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .deletingLastPathComponent()
      .deletingLastPathComponent()
    let source = try String(
      contentsOf: exampleRoot.appendingPathComponent("gen/describe_generated.swift"),
      encoding: .utf8
    )

    XCTAssertTrue(source.contains(#"$0.name = "greeting.v1.GreetingAppService""#))
    XCTAssertTrue(source.contains(#"$0.name = "SelectTransport""#))
    XCTAssertTrue(source.contains(#"$0.inputType = "greeting.v1.SelectTransportRequest""#))
    XCTAssertTrue(source.contains(#"$0.outputType = "greeting.v1.SelectTransportResponse""#))
    XCTAssertTrue(source.contains(#"$0.name = "transport""#))
  }
}
