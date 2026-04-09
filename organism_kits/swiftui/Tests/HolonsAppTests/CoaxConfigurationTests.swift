import XCTest
@testable import HolonsApp

final class CoaxConfigurationTests: XCTestCase {
    func testCoaxLaunchOverridesRespectCustomSocketDefaults() {
        let defaults = CoaxSettingsDefaults.standard(socketName: "henri-nobody.sock")
        let overrides = coaxLaunchOverrides(
            environment: ["OP_COAX_SERVER_LISTEN_URI": "unix:///tmp/henri-nobody.sock"],
            defaults: defaults
        )

        XCTAssertNil(overrides.isEnabled)
        XCTAssertEqual(overrides.snapshot?.serverTransport, .unix)
        XCTAssertEqual(overrides.snapshot?.serverUnixPath, "/tmp/henri-nobody.sock")
        XCTAssertEqual(defaults.serverUnixPath, defaultCoaxUnixPath(socketName: "henri-nobody.sock"))
    }

    @MainActor
    func testCoaxServerExposesDefaultPreviewEndpoint() {
        let server = CoaxServer(
            providers: { [] },
            registerDescribe: {},
            coaxDefaults: .standard(socketName: "henri-nobody.sock")
        )

        XCTAssertEqual(server.serverPreviewEndpoint, "tcp://127.0.0.1:60000")
        XCTAssertEqual(server.defaultUnixPath, defaultCoaxUnixPath(socketName: "henri-nobody.sock"))
    }

    func testTransportSelectionNormalizesAutoToStdio() {
        XCTAssertEqual(GreetingTransportName.normalizedSelection("auto"), .stdio)
        XCTAssertEqual(GreetingTransportName.normalizedSelection("tcp://"), .tcp)
        XCTAssertEqual(GreetingTransportName.normalizedSelection("unix"), .unix)
    }
}
