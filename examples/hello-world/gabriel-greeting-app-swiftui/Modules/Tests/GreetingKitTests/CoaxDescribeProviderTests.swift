import XCTest
@testable import GreetingKit

final class CoaxDescribeProviderTests: XCTestCase {
    func testGreetingDescribeIncludesSelectTransport() {
        let response = makeCoaxDescribeResponse()
        let greetingService = response.services.first { $0.name == "greeting.v1.GreetingAppService" }

        XCTAssertNotNil(greetingService)
        let method = greetingService?.methods.first { $0.name == "SelectTransport" }
        XCTAssertNotNil(method)
        XCTAssertEqual(method?.inputType, "greeting.v1.SelectTransportRequest")
        XCTAssertEqual(method?.outputType, "greeting.v1.SelectTransportResponse")
        XCTAssertEqual(method?.inputFields.first?.name, "transport")
    }
}
