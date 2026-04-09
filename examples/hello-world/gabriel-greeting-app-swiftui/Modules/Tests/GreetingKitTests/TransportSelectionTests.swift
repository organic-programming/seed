import XCTest
import HolonsApp
@testable import GreetingKit

final class TransportSelectionTests: XCTestCase {
    func testValidatedRPCNameAcceptsCanonicalTransportNames() {
        XCTAssertEqual(GreetingTransportName.validatedRPCName("stdio"), .stdio)
        XCTAssertEqual(GreetingTransportName.validatedRPCName("tcp"), .tcp)
        XCTAssertEqual(GreetingTransportName.validatedRPCName("unix"), .unix)
    }

    func testValidatedRPCNameRejectsUnknownTransportNames() {
        XCTAssertNil(GreetingTransportName.validatedRPCName(""))
        XCTAssertNil(GreetingTransportName.validatedRPCName("2"))
        XCTAssertNil(GreetingTransportName.validatedRPCName("tcp://127.0.0.1:60000"))
    }

    func testNormalizedSelectionPreservesLegacyAliases() {
        XCTAssertEqual(GreetingTransportName.normalizedSelection("auto"), .stdio)
        XCTAssertEqual(GreetingTransportName.normalizedSelection("stdio://"), .stdio)
        XCTAssertEqual(GreetingTransportName.normalizedSelection("tcp://"), .tcp)
        XCTAssertEqual(GreetingTransportName.normalizedSelection("unix://"), .unix)
        XCTAssertEqual(GreetingTransportName.normalizedSelection("bogus"), .stdio)
    }
}
