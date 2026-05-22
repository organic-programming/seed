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

    func testParseOpObs() throws {
        XCTAssertEqual(try parseOpObs(""), [])
        XCTAssertEqual(try parseOpObs("logs"), [.logs])
        XCTAssertEqual(try parseOpObs("logs,metrics"), [.logs, .metrics])
        XCTAssertEqual(try parseOpObs("all"), [.logs, .metrics, .events, .prom])
        XCTAssertThrowsError(try parseOpObs("all,otel"))
        XCTAssertThrowsError(try parseOpObs("all,sessions"))
        XCTAssertThrowsError(try parseOpObs("unknown"))
    }

    func testCheckEnvRejectsOtelAndUnknown() {
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "logs,otel"]))
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "logs,sessions"]))
        XCTAssertThrowsError(try checkEnv(["OP_SESSIONS": "metrics"]))
        XCTAssertThrowsError(try checkEnv(["OP_OBS": "bogus"]))
        XCTAssertNoThrow(try checkEnv(["OP_OBS": "logs,metrics,events,prom,all"]))
    }

    func testDisabledIsNoOp() throws {
        let o = try configure(ObsConfig(slug: "t"))
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

    func testOTLPWireEncodingUsesTypedAttributesAndMetricOneofs() throws {
        let o = try configure(
            ObsConfig(slug: "wire-test", instanceUid: "uid-1"),
            env: ["OP_OBS": "logs,metrics"]
        )
        o.logger("wire").info("typed", [
            "transport": "stdio",
            "duration_ns": .int64(42),
            "ok": true,
            "ratio": 1.5,
        ])
        let record = try XCTUnwrap(o.logRing?.drain().first?.record)
        XCTAssertEqual(record.body.stringValue, "typed")
        XCTAssertEqual(record.severityNumber, .info)
        XCTAssertEqual(record.attributes.first { $0.key == AttrHolonsSlug }?.value.stringValue, "wire-test")
        XCTAssertEqual(record.attributes.first { $0.key == AttrServiceName }?.value.stringValue, "wire-test")
        XCTAssertEqual(record.attributes.first { $0.key == AttrHolonsInstanceUID }?.value.stringValue, "uid-1")
        XCTAssertEqual(record.attributes.first { $0.key == AttrServiceInstanceID }?.value.stringValue, "uid-1")
        XCTAssertEqual(record.attributes.first { $0.key == AttrHolonsSessionID }?.value.stringValue, "")
        XCTAssertEqual(record.attributes.first { $0.key == "transport" }?.value.stringValue, "stdio")
        XCTAssertEqual(record.attributes.first { $0.key == "duration_ns" }?.value.intValue, 42)
        XCTAssertEqual(record.attributes.first { $0.key == "ok" }?.value.boolValue, true)
        XCTAssertEqual(record.attributes.first { $0.key == "ratio" }?.value.doubleValue, 1.5)

        o.counter("requests_total")?.inc(2)
        o.gauge("queue_depth")?.set(3.5)
        o.histogram("latency_s", bounds: [0.1, 1.0])?.observe(0.25)
        let metrics = toProtoMetrics(try XCTUnwrap(o.registry), slug: o.cfg.slug, uid: o.cfg.instanceUid, startTime: o.startTime)
        XCTAssertEqual(metrics.first { $0.name == "requests_total" }?.sum.isMonotonic, true)
        XCTAssertEqual(metrics.first { $0.name == "requests_total" }?.sum.aggregationTemporality, .cumulative)
        XCTAssertEqual(metrics.first { $0.name == "requests_total" }?.sum.dataPoints.first?.asInt, 2)
        XCTAssertEqual(metrics.first { $0.name == "queue_depth" }?.gauge.dataPoints.first?.asDouble, 3.5)
        XCTAssertEqual(metrics.first { $0.name == "latency_s" }?.histogram.aggregationTemporality, .cumulative)
        XCTAssertEqual(metrics.first { $0.name == "latency_s" }?.histogram.dataPoints.first?.bucketCounts, [0, 1, 0])
    }

    func testEventEncodingUsesCanonicalEventName() throws {
        let o = try configure(
            ObsConfig(slug: "event-test", instanceUid: "uid-1"),
            env: ["OP_OBS": "events"]
        )
        o.emit(EventConfigReloaded, payload: ["source": "test"])
        let event = try XCTUnwrap(o.eventBus?.drain().first?.record)
        XCTAssertEqual(event.eventName, EventConfigReloaded)
        XCTAssertEqual(event.body.stringValue, EventConfigReloaded)
        XCTAssertEqual(event.attributes.first { $0.key == "source" }?.value.stringValue, "test")
    }

    func testLogRingRetention() {
        let r = LogRing(capacity: 3)
        for ch in ["a", "b", "c", "d", "e"] {
            r.push(Self.logRecord(ch))
        }
        let entries = r.drain()
        XCTAssertEqual(entries.count, 3)
        XCTAssertEqual(entries.first?.bodyString, "c")
        XCTAssertEqual(entries.last?.bodyString, "e")
    }

    func testEventBusFanOut() {
        let bus = EventBus(capacity: 16)
        let exp = expectation(description: "event delivered")
        let _ = bus.subscribe { e in
            XCTAssertEqual(e.record.eventName, EventInstanceReady)
            exp.fulfill()
        }
        bus.emit(Self.eventRecord(EventInstanceReady))
        wait(for: [exp], timeout: 0.5)
    }

    func testLogsFollowReplaysRingOnSubscribe() {
        let ring = LogRing(capacity: 8)
        ring.push(Self.logRecord("before"))
        let exp = expectation(description: "live log delivered")
        var live: [LogRecord] = []
        let replay = ring.replayAndSubscribe(since: nil) { entry in
            live.append(entry)
            exp.fulfill()
        }
        defer { replay.1() }

        XCTAssertEqual(replay.0.map(\.bodyString), ["before"])
        ring.push(Self.logRecord("after"))
        wait(for: [exp], timeout: 0.5)
        XCTAssertEqual(live.map(\.bodyString), ["after"])
    }

    func testEventsFollowReplaysRingOnSubscribe() {
        let bus = EventBus(capacity: 8)
        bus.emit(Self.eventRecord(EventInstanceReady, payload: ["phase": "before"]))
        let exp = expectation(description: "live event delivered")
        var live: [Event] = []
        let replay = bus.replayAndSubscribe(since: nil) { event in
            live.append(event)
            exp.fulfill()
        }
        defer { replay.1() }

        XCTAssertEqual(replay.0.map { $0.attribute("phase") }, ["before"])
        bus.emit(Self.eventRecord(EventInstanceReady, payload: ["phase": "after"]))
        wait(for: [exp], timeout: 0.5)
        XCTAssertEqual(live.map { $0.attribute("phase") }, ["after"])
    }

    func testChainAppendAndEnrichment() {
        let c1 = appendDirectChild([], childSlug: "gabriel-greeting-rust", childUid: "1c2d")
        XCTAssertEqual(c1.count, 1)
        XCTAssertEqual(c1.first, "gabriel-greeting-rust")
        let c2 = enrichForMultilog(c1, streamSourceSlug: "gabriel-greeting-go", streamSourceUid: "ea34")
        XCTAssertEqual(c2.count, 2)
        XCTAssertEqual(c2.last, "gabriel-greeting-go")
        // original unchanged
        XCTAssertEqual(c1.count, 1)
    }

    func testIsOrganismRoot() throws {
        let o1 = try configure(ObsConfig(slug: "g", instanceUid: "x", organismUid: "x"))
        XCTAssertTrue(o1.isOrganismRoot)
        reset()
        let o2 = try configure(ObsConfig(slug: "g", instanceUid: "x", organismUid: "y"))
        XCTAssertFalse(o2.isOrganismRoot)
    }

    func testRunDirDerivesFromRegistryRoot() throws {
        let root = FileManager.default.temporaryDirectory
            .appendingPathComponent("swift-obs-root-\(UUID().uuidString)", isDirectory: true)
        defer { try? FileManager.default.removeItem(at: root) }

        let o = try configure(
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

        let o = try configure(
            ObsConfig(slug: "gabriel", runDir: root.path, instanceUid: "uid-1"),
            env: ["OP_OBS": "logs,events"]
        )
        enableDiskWriters(o.cfg.runDir)
        o.logger("test").info("ready", ["port": "123"])
        o.emit(EventInstanceReady, payload: ["listener": "tcp://127.0.0.1:123"])
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
        XCTAssertTrue(events.contains("instance.ready"))
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
            options: Serve.Options(slug: "swift-observability-test", logger: { _ in }, environment: env)
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
        o.emit(EventConfigReloaded, payload: ["source": "test"])

        let client = Holons_V1_HolonObservabilityClient(channel: channel)
        var logs: [Holons_V1_LogRecord] = []
        var logRequest = Holons_V1_LogsRequest()
        logRequest.follow = false
        let logCall = client.logs(logRequest) { logs.append($0) }
        XCTAssertEqual(try logCall.status.wait().code, .ok)
        XCTAssertTrue(logs.contains { $0.body.stringValue == "served" })

        var metrics: [Holons_V1_Metric] = []
        let metricsCall = client.metrics(Holons_V1_MetricsRequest()) { metrics.append($0) }
        XCTAssertEqual(try metricsCall.status.wait().code, .ok)
        XCTAssertTrue(metrics.contains { $0.name == "requests_total" && $0.sum.isMonotonic })

        var events: [Holons_V1_LogRecord] = []
        var eventRequest = Holons_V1_EventsRequest()
        eventRequest.follow = false
        let eventCall = client.events(eventRequest) { events.append($0) }
        XCTAssertEqual(try eventCall.status.wait().code, .ok)
        XCTAssertTrue(events.contains { $0.eventName == EventInstanceReady })
        XCTAssertTrue(events.contains { $0.eventName == EventConfigReloaded })

        let meta = runRoot
            .appendingPathComponent(o.cfg.slug)
            .appendingPathComponent("uid-1")
            .appendingPathComponent("meta.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: meta.path))
        let metaJSON = try JSONSerialization.jsonObject(with: Data(contentsOf: meta)) as? [String: Any]
        XCTAssertEqual(metaJSON?["address"] as? String, running.publicURI)
    }

    func testMetaJsonDecodesGoOmittedDefaults() throws {
        let data = Data("""
        {
          "slug": "observability-cascade-go-node",
          "uid": "uid-1",
          "pid": 42,
          "started_at": "2026-05-16T01:03:52.008127+02:00",
          "mode": "persistent",
          "transport": "tcp",
          "address": "tcp://127.0.0.1:60360",
          "metrics_addr": "http://127.0.0.1:60359/metrics",
          "log_path": "/tmp/stdout.log"
        }
        """.utf8)
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        let meta = try decoder.decode(MetaJson.self, from: data)
        XCTAssertEqual(meta.slug, "observability-cascade-go-node")
        XCTAssertEqual(meta.uid, "uid-1")
        XCTAssertEqual(meta.address, "tcp://127.0.0.1:60360")
        XCTAssertEqual(meta.logBytesRotated, 0)
        XCTAssertEqual(meta.organismUid, "")
        XCTAssertFalse(meta.isDefault)
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

    private static func logRecord(_ body: String) -> LogRecord {
        var record = Holons_V1_LogRecord()
        record.timeUnixNano = UInt64(Date().timeIntervalSince1970 * 1_000_000_000)
        record.observedTimeUnixNano = record.timeUnixNano
        record.severityNumber = .info
        record.severityText = "INFO"
        record.body = toAnyValue(.string(body))
        record.attributes = [
            keyValue(AttrHolonsSlug, .string("g")),
            keyValue(AttrServiceName, .string("g")),
            keyValue(AttrHolonsInstanceUID, .string("uid")),
            keyValue(AttrServiceInstanceID, .string("uid")),
            keyValue(AttrHolonsSessionID, .string("")),
        ]
        return LogRecord(record: record)
    }

    private static func eventRecord(_ eventName: String, payload: [String: String] = [:]) -> Event {
        var record = logRecord(eventName).record
        record.eventName = eventName
        record.attributes += payload.map { keyValue($0.key, .string($0.value)) }
        return Event(record: record)
    }
}
