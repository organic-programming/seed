import XCTest
@testable import SwiftHelloWorldCore

final class SwiftHelloWorldTests: XCTestCase {
    func testGreetName() {
        XCTAssertEqual(HelloService.greet(name: "Bob"), "Hello, Bob!")
    }

    func testGreetDefault() {
        XCTAssertEqual(HelloService.greet(name: nil), "Hello, World!")
        XCTAssertEqual(HelloService.greet(name: "   "), "Hello, World!")
    }

    func testListenURI() {
        XCTAssertEqual(HelloService.listenURI(from: ["--listen", "tcp://:8080"]), "tcp://:8080")
        XCTAssertEqual(HelloService.listenURI(from: ["--port", "9090"]), "tcp://:9090")
    }
}
