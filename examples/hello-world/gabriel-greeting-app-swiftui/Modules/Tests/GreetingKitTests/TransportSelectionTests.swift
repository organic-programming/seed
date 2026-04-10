import HolonsApp
import XCTest

@testable import GreetingKit

final class TransportSelectionTests: XCTestCase {
  func testValidatedRPCNameAcceptsCanonicalTransportNames() {
    XCTAssertEqual(HolonTransportName.parseCanonical("stdio"), .stdio)
    XCTAssertEqual(HolonTransportName.parseCanonical("tcp"), .tcp)
    XCTAssertEqual(HolonTransportName.parseCanonical("unix"), .unix)
  }

  func testValidatedRPCNameRejectsUnknownTransportNames() {
    XCTAssertNil(HolonTransportName.parseCanonical(""))
    XCTAssertNil(HolonTransportName.parseCanonical("2"))
    XCTAssertNil(HolonTransportName.parseCanonical("tcp://127.0.0.1:60000"))
  }

  func testNormalizedSelectionPreservesLegacyAliases() {
    XCTAssertEqual(HolonTransportName.normalize("auto"), .stdio)
    XCTAssertEqual(HolonTransportName.normalize("stdio://"), .stdio)
    XCTAssertEqual(HolonTransportName.normalize("tcp://"), .tcp)
    XCTAssertEqual(HolonTransportName.normalize("unix://"), .unix)
    XCTAssertEqual(HolonTransportName.normalize("bogus"), .stdio)
  }
}
