import XCTest
import SwiftUI
import Holons
@testable import HolonsApp

final class ObservabilityKitTests: XCTestCase {
    @MainActor
    func testRuntimeGatePersistsFamilyAndMemberOverrides() {
        let settings = MemorySettingsStore()
        let member = ObservabilityMemberRef(slug: "child", uid: "child-1", address: "tcp://127.0.0.1:50051")
        let gate = RuntimeGate(settings: settings, members: [member])

        gate.setMaster(false)
        gate.setFamily(.logs, false)
        gate.setFamily(.events, false)
        gate.setMemberOverride(member.uid, .off)

        let restored = RuntimeGate(settings: settings, members: [member])
        XCTAssertFalse(restored.masterEnabled)
        XCTAssertFalse(restored.logsEnabled)
        XCTAssertFalse(restored.eventsEnabled)
        XCTAssertEqual(restored.memberOverride(member.uid), .off)
    }

    @MainActor
    func testControllersExposeLogsEventsMetricsAndExportBundle() throws {
        reset()
        let settings = MemorySettingsStore()
        let kit = try ObservabilityKit.standalone(
            slug: "swift-kit-test",
            declaredFamilies: [.logs, .metrics, .events],
            settings: settings,
            bundledHolons: [ObservabilityMemberRef(slug: "child", uid: "child-1")]
        )

        kit.obs.logger("test").info("hello from kit", ["route": "say"])
        kit.obs.counter("calls_total")?.inc()
        kit.obs.gauge("queue_depth")?.set(4)
        kit.obs.histogram("latency_seconds")?.observe(0.025)
        kit.obs.emit(.instanceReady, payload: ["phase": "ready"])
        kit.metrics.refresh()
        drainMainQueue()

        XCTAssertTrue(kit.logs.filteredEntries.contains { $0.message == "hello from kit" })
        XCTAssertTrue(kit.events.events.contains { $0.type == .instanceReady })
        XCTAssertTrue(kit.metrics.latest?.counters.contains { $0.name == "calls_total" } ?? false)
        XCTAssertTrue(kit.metrics.latest?.gauges.contains { $0.name == "queue_depth" } ?? false)
        XCTAssertTrue(kit.metrics.latest?.histograms.contains { $0.name == "latency_seconds" } ?? false)
        XCTAssertEqual(kit.relay.activeMembers.map(\.uid), ["child-1"])

        let parent = FileManager.default.temporaryDirectory
            .appendingPathComponent("holonsapp-observability-tests-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: parent, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: parent) }

        let bundle = try kit.export.export(to: parent)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("logs.jsonl").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("events.jsonl").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("metrics.prom").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("metadata.json").path))
    }

    @MainActor
    func testObservabilityViewsCanBeConstructed() throws {
        reset()
        let kit = try ObservabilityKit.standalone(
            slug: "swift-kit-view-test",
            declaredFamilies: [.logs, .metrics, .events],
            settings: MemorySettingsStore()
        )

        XCTAssertNotNil(ObservabilityPanel(kit: kit).body)
        XCTAssertNotNil(LogConsoleView(controller: kit.logs).body)
        XCTAssertNotNil(MetricsView(controller: kit.metrics).body)
        XCTAssertNotNil(EventsView(controller: kit.events).body)
        XCTAssertNotNil(RelaySettingsView(kit: kit).body)
        XCTAssertNotNil(SparklineView(values: [1, 2, 3]).body)
        let histogram = try XCTUnwrap(kit.obs.histogram("view_latency_seconds"))
        histogram.observe(0.1)
        XCTAssertNotNil(HistogramChart(snapshot: histogram.snapshot()).body)
    }
}

@MainActor
private func drainMainQueue() {
    RunLoop.main.run(until: Date().addingTimeInterval(0.05))
}
