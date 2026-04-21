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
    func testCoaxManagerExposesDefaultPreviewEndpoint() {
        let manager = CoaxManager(
            providers: { [] },
            registerDescribe: {},
            settingsStore: MemorySettingsStore(),
            coaxDefaults: .standard(socketName: "henri-nobody.sock")
        )

        XCTAssertEqual(manager.serverPreviewEndpoint, "tcp://127.0.0.1:60000")
        XCTAssertEqual(manager.defaultUnixPath, defaultCoaxUnixPath(socketName: "henri-nobody.sock"))
    }

    func testTransportSelectionNormalizesAutoToStdio() {
        XCTAssertEqual(HolonTransportName.normalize("auto"), .stdio)
        XCTAssertEqual(HolonTransportName.normalize("tcp://"), .tcp)
        XCTAssertEqual(HolonTransportName.normalize("unix"), .unix)
    }
}
