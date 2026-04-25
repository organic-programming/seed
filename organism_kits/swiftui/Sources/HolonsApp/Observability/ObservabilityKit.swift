import Foundation
import Network
import Combine
import Holons

public enum GateOverride: String, CaseIterable, Identifiable, Sendable {
    case defaultValue
    case on
    case off

    public var id: String { rawValue }
}

public struct ObservabilityMemberRef: Identifiable, Hashable, Sendable {
    public let slug: String
    public let uid: String
    public let address: String

    public var id: String { uid }

    public init(slug: String, uid: String, address: String = "") {
        self.slug = slug
        self.uid = uid
        self.address = address
    }
}

@MainActor
public final class RuntimeGate: ObservableObject {
    private static let masterKey = "observability.master.enabled"
    private static let logsKey = "observability.family.logs"
    private static let metricsKey = "observability.family.metrics"
    private static let eventsKey = "observability.family.events"
    private static let promKey = "observability.family.prom"
    private static let promAddrKey = "observability.prom.addr"
    private static let memberPrefix = "observability.member."

    public let settings: SettingsStore
    public let members: [ObservabilityMemberRef]

    @Published public var masterEnabled: Bool
    @Published public var logsEnabled: Bool
    @Published public var metricsEnabled: Bool
    @Published public var eventsEnabled: Bool
    @Published public var promEnabled: Bool
    @Published public var promAddress: String
    @Published private var memberOverrides: [String: GateOverride]

    public init(settings: SettingsStore, members: [ObservabilityMemberRef] = []) {
        self.settings = settings
        self.members = members
        self.masterEnabled = settings.readBool(Self.masterKey, defaultValue: true)
        self.logsEnabled = settings.readBool(Self.logsKey, defaultValue: true)
        self.metricsEnabled = settings.readBool(Self.metricsKey, defaultValue: true)
        self.eventsEnabled = settings.readBool(Self.eventsKey, defaultValue: true)
        self.promEnabled = settings.readBool(Self.promKey, defaultValue: false)
        self.promAddress = settings.readString(Self.promAddrKey, defaultValue: "")
        var overrides: [String: GateOverride] = [:]
        for member in members {
            overrides[member.uid] = GateOverride(rawValue: settings.readString(Self.memberPrefix + member.uid, defaultValue: "")) ?? .defaultValue
        }
        self.memberOverrides = overrides
    }

    public func familyEnabled(_ family: Family) -> Bool {
        guard masterEnabled else { return false }
        switch family {
        case .logs: return logsEnabled
        case .metrics: return metricsEnabled
        case .events: return eventsEnabled
        case .prom: return promEnabled
        case .otel: return false
        }
    }

    public func memberOverride(_ uid: String) -> GateOverride {
        memberOverrides[uid] ?? .defaultValue
    }

    public func memberEnabled(_ uid: String) -> Bool {
        guard masterEnabled else { return false }
        switch memberOverride(uid) {
        case .defaultValue, .on: return true
        case .off: return false
        }
    }

    public func setMaster(_ value: Bool) {
        masterEnabled = value
        settings.writeBool(Self.masterKey, value)
    }

    public func setFamily(_ family: Family, _ value: Bool) {
        switch family {
        case .logs:
            logsEnabled = value
            settings.writeBool(Self.logsKey, value)
        case .metrics:
            metricsEnabled = value
            settings.writeBool(Self.metricsKey, value)
        case .events:
            eventsEnabled = value
            settings.writeBool(Self.eventsKey, value)
        case .prom:
            promEnabled = value
            settings.writeBool(Self.promKey, value)
        case .otel:
            return
        }
    }

    public func setPromAddress(_ value: String) {
        promAddress = value
        settings.writeString(Self.promAddrKey, value)
    }

    public func setMemberOverride(_ uid: String, _ value: GateOverride) {
        memberOverrides[uid] = value
        settings.writeString(Self.memberPrefix + uid, value == .defaultValue ? "" : value.rawValue)
    }
}

@MainActor
public final class ConsoleController: ObservableObject {
    public let obs: Observability
    public let gate: RuntimeGate
    @Published public private(set) var entries: [LogEntry]
    @Published public var minLevel: Level = .trace
    @Published public var query: String = ""
    private nonisolated(unsafe) var unsubscribe: (() -> Void)?

    public init(obs: Observability, gate: RuntimeGate) {
        self.obs = obs
        self.gate = gate
        self.entries = obs.logRing?.drain() ?? []
        self.unsubscribe = obs.logRing?.subscribe { [weak self] entry in
            DispatchQueue.main.async { self?.entries.append(entry) }
        }
    }

    public var filteredEntries: [LogEntry] {
        guard gate.familyEnabled(.logs) else { return [] }
        let q = query.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        return entries.filter { entry in
            guard entry.level >= minLevel else { return false }
            return q.isEmpty || entry.message.lowercased().contains(q) || entry.slug.lowercased().contains(q)
        }
    }

    deinit { unsubscribe?() }
}

public struct MetricSnapshot: Sendable {
    public let capturedAt: Date
    public let counters: [Counter]
    public let gauges: [Gauge]
    public let histograms: [Histogram]
}

@MainActor
public final class MetricsController: ObservableObject {
    public let obs: Observability
    public let gate: RuntimeGate
    @Published public private(set) var history: [MetricSnapshot] = []
    private nonisolated(unsafe) var timer: Timer?

    public init(obs: Observability, gate: RuntimeGate) {
        self.obs = obs
        self.gate = gate
        refresh()
        timer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: true) { [weak self] _ in
            Task { @MainActor in self?.refresh() }
        }
    }

    public var latest: MetricSnapshot? { history.last }

    public func refresh() {
        guard let registry = obs.registry else { return }
        history.append(MetricSnapshot(
            capturedAt: Date(),
            counters: registry.listCounters(),
            gauges: registry.listGauges(),
            histograms: registry.listHistograms()
        ))
        if history.count > 30 {
            history.removeFirst(history.count - 30)
        }
    }

    deinit { timer?.invalidate() }
}

@MainActor
public final class EventsController: ObservableObject {
    public let obs: Observability
    public let gate: RuntimeGate
    @Published public private(set) var allEvents: [Event]
    private nonisolated(unsafe) var unsubscribe: (() -> Void)?

    public init(obs: Observability, gate: RuntimeGate) {
        self.obs = obs
        self.gate = gate
        self.allEvents = obs.eventBus?.drain() ?? []
        self.unsubscribe = obs.eventBus?.subscribe { [weak self] event in
            DispatchQueue.main.async { self?.allEvents.append(event) }
        }
    }

    public var events: [Event] {
        gate.familyEnabled(.events) ? allEvents : []
    }

    deinit { unsubscribe?() }
}

@MainActor
public final class RelayController: ObservableObject {
    public let gate: RuntimeGate

    public init(gate: RuntimeGate) {
        self.gate = gate
    }

    public var activeMembers: [ObservabilityMemberRef] {
        gate.members.filter { gate.memberEnabled($0.uid) }
    }
}

@MainActor
public final class PrometheusController: ObservableObject {
    public let obs: Observability
    public let gate: RuntimeGate
    @Published public private(set) var boundAddress = ""
    private var listener: NWListener?
    private let queue = DispatchQueue(label: "holonsapp.prometheus")

    public init(obs: Observability, gate: RuntimeGate) {
        self.obs = obs
        self.gate = gate
    }

    public func sync() {
        if gate.familyEnabled(.prom) {
            try? start()
        } else {
            stop()
        }
    }

    public func start() throws {
        guard listener == nil else { return }
        let listener = try NWListener(using: .tcp, on: .any)
        listener.newConnectionHandler = { [weak self] connection in
            connection.start(queue: self?.queue ?? .main)
            self?.serve(connection)
        }
        listener.stateUpdateHandler = { [weak self] state in
            if case .ready = state, let port = listener.port {
                DispatchQueue.main.async {
                    self?.boundAddress = "http://127.0.0.1:\(port.rawValue)/metrics"
                    self?.gate.setPromAddress(self?.boundAddress ?? "")
                }
            }
        }
        self.listener = listener
        listener.start(queue: queue)
    }

    public func stop() {
        listener?.cancel()
        listener = nil
        boundAddress = ""
    }

    private nonisolated func serve(_ connection: NWConnection) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 4096) { [weak self] _, _, _, _ in
            Task { @MainActor in
                let text = prometheusText(self?.obs)
                let response = "HTTP/1.1 200 OK\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: \(text.utf8.count)\r\n\r\n\(text)"
                connection.send(content: response.data(using: .utf8), completion: .contentProcessed { _ in
                    connection.cancel()
                })
            }
        }
    }

    deinit { listener?.cancel() }
}

@MainActor
public struct ExportController {
    public let kit: ObservabilityKit

    public func export(to parent: URL) throws -> URL {
        let stamp = ISO8601DateFormatter().string(from: Date()).replacingOccurrences(of: ":", with: "")
        let dir = parent.appendingPathComponent("observability-\(kit.slug)-\(stamp)", isDirectory: true)
        return try exportBundle(to: dir)
    }

    public func exportBundle(to dir: URL) throws -> URL {
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try (kit.obs.logRing?.drain() ?? []).map(logJSON).joined(separator: "\n").appending("\n")
            .write(to: dir.appendingPathComponent("logs.jsonl"), atomically: true, encoding: .utf8)
        try (kit.obs.eventBus?.drain() ?? []).map(eventJSON).joined(separator: "\n").appending("\n")
            .write(to: dir.appendingPathComponent("events.jsonl"), atomically: true, encoding: .utf8)
        try prometheusText(kit.obs).write(to: dir.appendingPathComponent("metrics.prom"), atomically: true, encoding: .utf8)
        let metadata = [
            "slug": kit.slug,
            "instance_uid": kit.obs.cfg.instanceUid,
            "exported_at": ISO8601DateFormatter().string(from: Date()),
        ]
        let data = try JSONSerialization.data(withJSONObject: metadata, options: [.prettyPrinted, .sortedKeys])
        try data.write(to: dir.appendingPathComponent("metadata.json"))
        return dir
    }
}

@MainActor
public final class ObservabilityKit: ObservableObject {
    public let slug: String
    public let obs: Observability
    public let gate: RuntimeGate
    public let logs: ConsoleController
    public let metrics: MetricsController
    public let events: EventsController
    public let relay: RelayController
    public let prometheus: PrometheusController
    public lazy var export = ExportController(kit: self)

    private init(slug: String, obs: Observability, gate: RuntimeGate) {
        self.slug = slug
        self.obs = obs
        self.gate = gate
        self.logs = ConsoleController(obs: obs, gate: gate)
        self.metrics = MetricsController(obs: obs, gate: gate)
        self.events = EventsController(obs: obs, gate: gate)
        self.relay = RelayController(gate: gate)
        self.prometheus = PrometheusController(obs: obs, gate: gate)
    }

    public static func standalone(
        slug: String,
        declaredFamilies: [Family],
        settings: SettingsStore,
        bundledHolons: [ObservabilityMemberRef] = []
    ) throws -> ObservabilityKit {
        var env = ProcessInfo.processInfo.environment
        env["OP_OBS"] = declaredFamilies.map(\.rawValue).joined(separator: ",")
        let obs = try fromEnv(ObsConfig(
            slug: slug,
            instanceUid: "kit-\(Date().timeIntervalSince1970)"
        ), env: env)
        let gate = RuntimeGate(settings: settings, members: bundledHolons)
        return ObservabilityKit(slug: slug, obs: obs, gate: gate)
    }
}

public func prometheusText(_ obs: Observability?) -> String {
    guard let registry = obs?.registry else { return "" }
    var lines: [String] = []
    for counter in registry.listCounters() {
        lines.append("# TYPE \(counter.name) counter")
        lines.append("\(counter.name)\(labels(counter.labels)) \(counter.read())")
    }
    for gauge in registry.listGauges() {
        lines.append("# TYPE \(gauge.name) gauge")
        lines.append("\(gauge.name)\(labels(gauge.labels)) \(gauge.read())")
    }
    for histogram in registry.listHistograms() {
        let snap = histogram.snapshot()
        lines.append("# TYPE \(histogram.name) histogram")
        for index in snap.bounds.indices {
            var bucketLabels = histogram.labels
            bucketLabels["le"] = String(snap.bounds[index])
            lines.append("\(histogram.name)_bucket\(labels(bucketLabels)) \(snap.counts[index])")
        }
        lines.append("\(histogram.name)_count\(labels(histogram.labels)) \(snap.total)")
        lines.append("\(histogram.name)_sum\(labels(histogram.labels)) \(snap.sum)")
    }
    return lines.joined(separator: "\n") + "\n"
}

private func labels(_ labels: [String: String]) -> String {
    guard !labels.isEmpty else { return "" }
    return "{" + labels.keys.sorted().map { "\($0)=\"\(labels[$0] ?? "")\"" }.joined(separator: ",") + "}"
}

private func logJSON(_ entry: LogEntry) -> String {
    let object: [String: Any] = [
        "ts": ISO8601DateFormatter().string(from: entry.timestamp),
        "level": entry.level.name,
        "slug": entry.slug,
        "instance_uid": entry.instanceUid,
        "message": entry.message,
        "fields": entry.fields,
    ]
    let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
    return String(data: data ?? Data(), encoding: .utf8) ?? "{}"
}

private func eventJSON(_ event: Event) -> String {
    let object: [String: Any] = [
        "ts": ISO8601DateFormatter().string(from: event.timestamp),
        "type": event.type.protoName,
        "slug": event.slug,
        "instance_uid": event.instanceUid,
        "payload": event.payload,
    ]
    let data = try? JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
    return String(data: data ?? Data(), encoding: .utf8) ?? "{}"
}
