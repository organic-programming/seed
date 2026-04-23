import XCTest
@testable import Holons

final class ObservabilityTests: XCTestCase {
    override func setUp() {
        super.setUp()
        reset()
    }

    func testParseOpObs() {
        XCTAssertEqual(parseOpObs(""), [])
        XCTAssertEqual(parseOpObs("logs"), [.logs])
        XCTAssertEqual(parseOpObs("logs,metrics"), [.logs, .metrics])
        XCTAssertEqual(parseOpObs("all"), [.logs, .metrics, .events, .prom])
        XCTAssertEqual(parseOpObs("all,otel"), [.logs, .metrics, .events, .prom])
        XCTAssertEqual(parseOpObs("unknown"), [])
    }

    func testCheckEnvRejectsOtelAndUnknown() {
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "logs,otel"]))
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "bogus"]))
        XCTAssertNoThrow(try checkEnv(["OP_OBS": "logs,metrics,events,prom,all"]))
    }

    func testDisabledIsNoOp() {
        let o = configure(ObsConfig(slug: "t"))
        XCTAssertFalse(o.enabled(.logs))
        o.logger("x").info("drop", ["k": "v"])
        XCTAssertNil(o.counter("t_total"))
    }

    func testRegistryCounterHistogram() {
        let reg = Registry()
        let c = reg.counter("t_total")
        for _ in 0..<1000 { c.inc() }
        XCTAssertEqual(c.read(), 1000)

        let h = reg.histogram("lat_s", bounds: [1e-3, 1e-2, 1e-1, 1.0])
        for _ in 0..<900 { h.observe(0.5e-3) }
        for _ in 0..<100 { h.observe(0.5) }
        let snap = h.snapshot()
        XCTAssertEqual(snap.quantile(0.5), 1e-3)
        XCTAssertEqual(snap.quantile(0.99), 1.0)
    }

    func testLogRingRetention() {
        let r = LogRing(capacity: 3)
        for ch in ["a", "b", "c", "d", "e"] {
            r.push(LogEntry(timestamp: Date(), level: .info, slug: "g", instanceUid: "", message: ch))
        }
        let entries = r.drain()
        XCTAssertEqual(entries.count, 3)
        XCTAssertEqual(entries.first?.message, "c")
        XCTAssertEqual(entries.last?.message, "e")
    }

    func testEventBusFanOut() {
        let bus = EventBus(capacity: 16)
        let exp = expectation(description: "event delivered")
        let _ = bus.subscribe { e in
            XCTAssertEqual(e.type, .instanceReady)
            exp.fulfill()
        }
        bus.emit(Event(timestamp: Date(), type: .instanceReady, slug: "g", instanceUid: "uid"))
        wait(for: [exp], timeout: 0.5)
    }

    func testChainAppendAndEnrichment() {
        let c1 = appendDirectChild([], childSlug: "gabriel-greeting-rust", childUid: "1c2d")
        XCTAssertEqual(c1.count, 1)
        XCTAssertEqual(c1.first?.slug, "gabriel-greeting-rust")
        let c2 = enrichForMultilog(c1, streamSourceSlug: "gabriel-greeting-go", streamSourceUid: "ea34")
        XCTAssertEqual(c2.count, 2)
        XCTAssertEqual(c2.last?.slug, "gabriel-greeting-go")
        // original unchanged
        XCTAssertEqual(c1.count, 1)
    }

    func testIsOrganismRoot() {
        let o1 = configure(ObsConfig(slug: "g", instanceUid: "x", organismUid: "x"))
        XCTAssertTrue(o1.isOrganismRoot)
        reset()
        let o2 = configure(ObsConfig(slug: "g", instanceUid: "x", organismUid: "y"))
        XCTAssertFalse(o2.isOrganismRoot)
    }
}
