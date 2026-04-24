import GRPC
import NIOPosix
import XCTest
@testable import Holons

final class ObservabilityTests: XCTestCase {
    override func setUp() {
        super.setUp()
        reset()
        Describe.useStaticResponse(nil)
    }

    override func tearDown() {
        Describe.useStaticResponse(nil)
        reset()
        super.tearDown()
    }

    func testParseOpObs() {
        XCTAssertEqual(parseOpObs(""), [])
        XCTAssertEqual(parseOpObs("logs"), [.logs])
        XCTAssertEqual(parseOpObs("logs,metrics"), [.logs, .metrics])
        XCTAssertEqual(parseOpObs("all"), [.logs, .metrics, .events, .prom])
        XCTAssertEqual(parseOpObs("all,otel"), [.logs, .metrics, .events, .prom])
        XCTAssertEqual(parseOpObs("all,sessions"), [.logs, .metrics, .events, .prom])
        XCTAssertEqual(parseOpObs("unknown"), [])
    }

    func testCheckEnvRejectsOtelAndUnknown() {
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "logs,otel"]))
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "logs,sessions"]))
        XCTAssertThrowsError(try checkEnv(["OP_SESSIONS": "metrics"]))
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

    func testRunDirDerivesFromRegistryRoot() {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("swift-obs-root-\(UUID().uuidString)", isDirectory: true)
        defer { try? FileManager.default.removeItem(at: root) }

        let o = configure(
            ObsConfig(slug: "gabriel", runDir: root.path, instanceUid: "uid-1"),
            env: ["OP_OBS": "logs"]
        )

        XCTAssertEqual(
            o.cfg.runDir,
            root.appendingPathComponent("gabriel").appendingPathComponent("uid-1").path
        )
    }

    func testDiskWritersAndMetaJson() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("swift-obs-disk-\(UUID().uuidString)", isDirectory: true)
        defer { try? FileManager.default.removeItem(at: root) }

        let o = configure(
            ObsConfig(slug: "gabriel", runDir: root.path, instanceUid: "uid-1"),
            env: ["OP_OBS": "logs,events"]
        )
        enableDiskWriters(o.cfg.runDir)
        o.logger("test").info("ready", ["port": "123"])
        o.emit(.instanceReady, payload: ["listener": "tcp://127.0.0.1:123"])
        try writeMetaJson(
            o.cfg.runDir,
            MetaJson(
                slug: "gabriel",
                uid: "uid-1",
                pid: 42,
                startedAt: Date(timeIntervalSince1970: 1),
                transport: "tcp",
                address: "tcp://127.0.0.1:123",
                logPath: (o.cfg.runDir as NSString).appendingPathComponent("stdout.log")
            )
        )

        let stdout = try String(contentsOfFile: (o.cfg.runDir as NSString).appendingPathComponent("stdout.log"))
        let events = try String(contentsOfFile: (o.cfg.runDir as NSString).appendingPathComponent("events.jsonl"))
        let meta = try String(contentsOfFile: (o.cfg.runDir as NSString).appendingPathComponent("meta.json"))
        XCTAssertTrue(stdout.contains("ready"))
        XCTAssertTrue(events.contains("INSTANCE_READY"))
        XCTAssertTrue(meta.contains(#""slug" : "gabriel""#))
        XCTAssertTrue(meta.contains(#""uid" : "uid-1""#))
    }

    func testServeAutoRegistersHolonObservability() throws {
        try Describe.useStaticResponse(Self.sampleDescribeResponse())

        let runRoot = FileManager.default.temporaryDirectory
            .appendingPathComponent("swift-obs-serve-\(UUID().uuidString)", isDirectory: true)
        defer { try? FileManager.default.removeItem(at: runRoot) }

        let env = [
            "OP_OBS": "logs,metrics,events",
            "OP_RUN_DIR": runRoot.path,
            "OP_INSTANCE_UID": "uid-1",
        ]
        let running = try Serve.startWithOptions(
            "tcp://127.0.0.1:0",
            serviceProviders: [],
            options: Serve.Options(logger: { _ in }, environment: env)
        )
        let parsed = try Transport.parse(running.publicURI)
        let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)
        let channel = ClientConnection.insecure(group: group)
            .connect(host: parsed.host ?? "127.0.0.1", port: parsed.port ?? 0)
        defer {
            _ = try? channel.close().wait()
            running.stop()
            try? group.syncShutdownGracefully()
        }

        let o = current()
        o.logger("test").info("served")
        o.counter("requests_total", help: "requests")?.inc()
        o.emit(.configReloaded, payload: ["source": "test"])

        let client = Holons_V1_HolonObservabilityClient(channel: channel)
        var logs: [Holons_V1_LogEntry] = []
        var logRequest = Holons_V1_LogsRequest()
        logRequest.follow = false
        let logCall = client.logs(logRequest) { logs.append($0) }
        XCTAssertEqual(try logCall.status.wait().code, .ok)
        XCTAssertTrue(logs.contains { $0.message == "served" })

        let metrics = try client.metrics(Holons_V1_MetricsRequest()).response.wait()
        XCTAssertTrue(metrics.samples.contains { $0.name == "requests_total" })

        var events: [Holons_V1_EventInfo] = []
        var eventRequest = Holons_V1_EventsRequest()
        eventRequest.follow = false
        let eventCall = client.events(eventRequest) { events.append($0) }
        XCTAssertEqual(try eventCall.status.wait().code, .ok)
        XCTAssertTrue(events.contains { $0.type == .instanceReady })
        XCTAssertTrue(events.contains { $0.type == .configReloaded })

        let meta = runRoot
            .appendingPathComponent(o.cfg.slug)
            .appendingPathComponent("uid-1")
            .appendingPathComponent("meta.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: meta.path))
        let metaJSON = try JSONSerialization.jsonObject(with: Data(contentsOf: meta)) as? [String: Any]
        XCTAssertEqual(metaJSON?["address"] as? String, running.publicURI)
    }

    private static func sampleDescribeResponse() -> Holons_V1_DescribeResponse {
        var manifest = Holons_V1_HolonManifest()
        manifest.identity.schema = "holon/v1"
        manifest.identity.uuid = "swift-observability-0000"
        manifest.identity.givenName = "Swift"
        manifest.identity.familyName = "Observability"
        manifest.identity.motto = "Static describe fixture."
        manifest.identity.composer = "swift-observability-test"
        manifest.identity.status = "draft"
        manifest.identity.born = "2026-03-23"
        manifest.lang = "swift"

        var method = Holons_V1_MethodDoc()
        method.name = "Ping"
        method.description_p = "No-op fixture method."

        var service = Holons_V1_ServiceDoc()
        service.name = "fixture.v1.Empty"
        service.description_p = "Static fixture service."
        service.methods = [method]

        var response = Holons_V1_DescribeResponse()
        response.manifest = manifest
        response.services = [service]
        return response
    }
}
