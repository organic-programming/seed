import Holons
import HolonsApp
import XCTest
@testable import GreetingKit

final class ObservabilityPanelTests: XCTestCase {
    @MainActor
    func testObservabilityPanelReadsGreetingAppKitState() throws {
        let kit = try ObservabilityKit.standalone(
            slug: "gabriel-greeting-app-swiftui-test",
            declaredFamilies: [.logs, .metrics, .events, .prom],
            settings: MemorySettingsStore(),
            bundledHolons: [
                ObservabilityMemberRef(slug: "gabriel-greeting-swift", uid: "gabriel-greeting-swift")
            ]
        )
        let manager = GreetingHolonManager()
        manager.attachObservability(kit.obs)

        kit.obs.logger("app").info("panel test log")
        kit.obs.emit(EventInstanceReady, payload: ["runtime": "swiftui-test"])
        kit.obs.counter("panel_test_total")?.inc()
        kit.metrics.refresh()
        drainMainQueue()

        XCTAssertTrue(kit.logs.filteredEntries.contains { $0.bodyString == "panel test log" })
        XCTAssertTrue(kit.events.events.contains { $0.record.eventName == EventInstanceReady })
        XCTAssertTrue(kit.metrics.latest?.counters.contains { $0.name == "panel_test_total" } ?? false)
        XCTAssertEqual(kit.relay.activeMembers.map(\.slug), ["gabriel-greeting-swift"])
        XCTAssertNotNil(ObservabilityPanel(kit: kit).body)
    }
}

@MainActor
private func drainMainQueue() {
    RunLoop.main.run(until: Date().addingTimeInterval(0.05))
}
