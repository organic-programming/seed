import XCTest
import HolonsApp
@testable import GreetingKit

final class CoaxConfigurationTests: XCTestCase {
    func testDefaultUnixPathUsesTemporaryDirectory() {
        let defaults = CoaxSettingsDefaults.standard(socketName: "gabriel-greeting-coax.sock")
        XCTAssertEqual(
            defaults.serverUnixPath,
            NSTemporaryDirectory() + "gabriel-greeting-coax.sock"
        )
    }

    func testTCPLaunchOverridesHonorEnvironment() {
        let defaults = CoaxSettingsDefaults.standard(socketName: "gabriel-greeting-coax.sock")
        let overrides = coaxLaunchOverrides(environment: [
            "OP_COAX_SERVER_ENABLED": "1",
            "OP_COAX_SERVER_LISTEN_URI": "tcp://127.0.0.1:61234",
        ], defaults: defaults)

        XCTAssertEqual(overrides.isEnabled, true)
        XCTAssertEqual(overrides.snapshot?.serverTransport, .tcp)
        XCTAssertEqual(overrides.snapshot?.serverHost, "127.0.0.1")
        XCTAssertEqual(overrides.snapshot?.serverPortText, "61234")
        XCTAssertEqual(resolvedCoaxEnabled(storedValue: false, overrides: overrides), true)
    }

    func testUnixLaunchOverridesHonorEnvironment() {
        let defaults = CoaxSettingsDefaults.standard(socketName: "gabriel-greeting-coax.sock")
        let overrides = coaxLaunchOverrides(environment: [
            "OP_COAX_SERVER_LISTEN_URI": "unix:///tmp/gabriel-greeting-coax-test.sock",
        ], defaults: defaults)

        XCTAssertNil(overrides.isEnabled)
        XCTAssertEqual(overrides.snapshot?.serverTransport, .unix)
        XCTAssertEqual(overrides.snapshot?.serverUnixPath, "/tmp/gabriel-greeting-coax-test.sock")
        XCTAssertEqual(resolvedCoaxEnabled(storedValue: false, overrides: overrides), true)
    }

    func testExplicitDisableWinsOverListenURI() {
        let defaults = CoaxSettingsDefaults.standard(socketName: "gabriel-greeting-coax.sock")
        let overrides = coaxLaunchOverrides(environment: [
            "OP_COAX_SERVER_ENABLED": "false",
            "OP_COAX_SERVER_LISTEN_URI": "tcp://127.0.0.1:61234",
        ], defaults: defaults)

        XCTAssertEqual(overrides.isEnabled, false)
        XCTAssertEqual(resolvedCoaxEnabled(storedValue: true, overrides: overrides), false)
    }
}
