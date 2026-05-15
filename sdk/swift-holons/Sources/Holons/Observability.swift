// Swift reference implementation of the cross-SDK observability layer.
//
// Mirrors sdk/go-holons/pkg/observability. Same activation model
// (OP_OBS env + zero cost when disabled), same public surface
// (HolonLogger / Counter / Gauge / Histogram / EventBus / chain helpers),
// same proto types (holons.v1.HolonObservability). See OBSERVABILITY.md.

import Foundation
import GRPC
import NIOCore
import SwiftProtobuf

#if os(Linux)
import Glibc
#else
import Darwin
#endif

// MARK: - Families

public enum Family: String, CaseIterable, Sendable {
    case logs
    case metrics
    case events
    case prom
    case otel // reserved v2
}

private let v1Tokens: Set<String> = ["logs", "metrics", "events", "prom", "all"]

public struct InvalidTokenError: Error, CustomStringConvertible {
    public let variable: String
    public let token: String
    public let reason: String
    public init(token: String, reason: String, variable: String = "OP_OBS") {
        self.variable = variable
        self.token = token
        self.reason = reason
    }
    public var description: String { "\(variable): \(reason): \(token)" }
}

public func parseOpObs(_ raw: String) throws -> Set<Family> {
    var out = Set<Family>()
    let trimmed = raw.trimmingCharacters(in: .whitespaces)
    if trimmed.isEmpty { return out }
    for part in trimmed.split(separator: ",") {
        let tok = part.trimmingCharacters(in: .whitespaces)
        guard !tok.isEmpty else { continue }
        if tok == "otel" {
            throw InvalidTokenError(token: tok, reason: "otel export is reserved for v2; not implemented in v1")
        }
        if tok == "sessions" {
            throw InvalidTokenError(token: tok, reason: "sessions are reserved for v2; not implemented in v1")
        }
        guard v1Tokens.contains(tok) else {
            throw InvalidTokenError(token: tok, reason: "unknown OP_OBS token")
        }
        if tok == "all" {
            out.formUnion([.logs, .metrics, .events, .prom])
        } else if let fam = Family(rawValue: tok) {
            out.insert(fam)
        }
    }
    return out
}

public func checkEnv(_ env: [String: String] = ProcessInfo.processInfo.environment) throws {
    let sessions = (env["OP_SESSIONS"] ?? "").trimmingCharacters(in: .whitespaces)
    if !sessions.isEmpty {
        throw InvalidTokenError(token: sessions, reason: "sessions are reserved for v2; not implemented in v1", variable: "OP_SESSIONS")
    }
    let raw = (env["OP_OBS"] ?? "").trimmingCharacters(in: .whitespaces)
    if raw.isEmpty { return }
    for part in raw.split(separator: ",") {
        let tok = part.trimmingCharacters(in: .whitespaces)
        guard !tok.isEmpty else { continue }
        if tok == "otel" {
            throw InvalidTokenError(token: tok, reason: "otel export is reserved for v2; not implemented in v1")
        }
        if tok == "sessions" {
            throw InvalidTokenError(token: tok, reason: "sessions are reserved for v2; not implemented in v1")
        }
        if !v1Tokens.contains(tok) {
            throw InvalidTokenError(token: tok, reason: "unknown OP_OBS token")
        }
    }
}

// MARK: - Levels + events

public enum Level: Int32, Sendable, Comparable {
    case unset = 0
    case trace = 1
    case debug = 2
    case info = 3
    case warn = 4
    case error = 5
    case fatal = 6

    public static func < (lhs: Level, rhs: Level) -> Bool { lhs.rawValue < rhs.rawValue }

    public var name: String {
        switch self {
        case .trace: return "TRACE"
        case .debug: return "DEBUG"
        case .info: return "INFO"
        case .warn: return "WARN"
        case .error: return "ERROR"
        case .fatal: return "FATAL"
        case .unset: return "UNSPECIFIED"
        }
    }
}

public func parseLevel(_ s: String) -> Level {
    switch s.trimmingCharacters(in: .whitespaces).uppercased() {
    case "TRACE": return .trace
    case "DEBUG": return .debug
    case "INFO": return .info
    case "WARN", "WARNING": return .warn
    case "ERROR": return .error
    case "FATAL": return .fatal
    default: return .info
    }
}

public enum EventType: Int32, Sendable {
    case unspecified = 0
    case instanceSpawned = 1
    case instanceReady = 2
    case instanceExited = 3
    case instanceCrashed = 4
    case sessionStarted = 5
    case sessionEnded = 6
    case handlerPanic = 7
    case configReloaded = 8

    public var protoName: String {
        switch self {
        case .instanceSpawned: return "INSTANCE_SPAWNED"
        case .instanceReady: return "INSTANCE_READY"
        case .instanceExited: return "INSTANCE_EXITED"
        case .instanceCrashed: return "INSTANCE_CRASHED"
        case .sessionStarted: return "SESSION_STARTED"
        case .sessionEnded: return "SESSION_ENDED"
        case .handlerPanic: return "HANDLER_PANIC"
        case .configReloaded: return "CONFIG_RELOADED"
        case .unspecified: return "UNSPECIFIED"
        }
    }
}

// MARK: - Chain helpers

public struct Hop: Sendable {
    public let slug: String
    public let instanceUid: String
    public init(slug: String, instanceUid: String) {
        self.slug = slug
        self.instanceUid = instanceUid
    }
}

public func appendDirectChild(_ src: [Hop], childSlug: String, childUid: String) -> [Hop] {
    src + [Hop(slug: childSlug, instanceUid: childUid)]
}

public func enrichForMultilog(_ wire: [Hop], streamSourceSlug: String, streamSourceUid: String) -> [Hop] {
    appendDirectChild(wire, childSlug: streamSourceSlug, childUid: streamSourceUid)
}

// MARK: - Log entries + ring

public struct LogEntry: Sendable {
    public let timestamp: Date
    public let level: Level
    public let slug: String
    public let instanceUid: String
    public let sessionId: String
    public let rpcMethod: String
    public let message: String
    public let fields: [String: String]
    public let caller: String
    public let chain: [Hop]
    public let isPrivate: Bool

    public init(timestamp: Date, level: Level, slug: String, instanceUid: String,
                sessionId: String = "", rpcMethod: String = "",
                message: String, fields: [String: String] = [:], caller: String = "",
                chain: [Hop] = [], isPrivate: Bool = false) {
        self.timestamp = timestamp
        self.level = level
        self.slug = slug
        self.instanceUid = instanceUid
        self.sessionId = sessionId
        self.rpcMethod = rpcMethod
        self.message = message
        self.fields = fields
        self.caller = caller
        self.chain = chain
        self.isPrivate = isPrivate
    }
}

public final class LogRing: @unchecked Sendable {
    private let capacity: Int
    private var buf: [LogEntry] = []
    private var subs: [(LogEntry) -> Void] = []
    private let lock = NSLock()

    public init(capacity: Int = 1024) {
        self.capacity = max(1, capacity)
    }

    public func push(_ e: LogEntry) {
        lock.lock()
        buf.append(e)
        if buf.count > capacity {
            buf.removeFirst(buf.count - capacity)
        }
        let snapshot = subs
        lock.unlock()
        for fn in snapshot {
            fn(e)
        }
    }

    public func drain() -> [LogEntry] {
        lock.lock(); defer { lock.unlock() }
        return buf
    }

    public func drainSince(_ cutoff: Date) -> [LogEntry] {
        lock.lock(); defer { lock.unlock() }
        return buf.filter { $0.timestamp >= cutoff }
    }

    @discardableResult
    public func subscribe(_ fn: @escaping (LogEntry) -> Void) -> () -> Void {
        lock.lock()
        subs.append(fn)
        let index = subs.count - 1
        lock.unlock()
        return { [weak self] in
            guard let self = self else { return }
            self.lock.lock()
            if index < self.subs.count {
                self.subs.remove(at: index)
            }
            self.lock.unlock()
        }
    }

    public func replayAndSubscribe(since cutoff: Date?, _ fn: @escaping (LogEntry) -> Void) -> ([LogEntry], () -> Void) {
        lock.lock()
        let snapshot = cutoff.map { cutoff in buf.filter { $0.timestamp >= cutoff } } ?? buf
        subs.append(fn)
        let token = ObjectIdentifier(SubscriptionBox(fn))
        let index = subs.count - 1
        lock.unlock()
        return (snapshot, { [weak self] in
            guard let self = self else { return }
            _ = token
            self.lock.lock()
            if index < self.subs.count {
                self.subs.remove(at: index)
            }
            self.lock.unlock()
        })
    }

    public var count: Int {
        lock.lock(); defer { lock.unlock() }
        return buf.count
    }
}

private final class SubscriptionBox {
    let fn: Any
    init(_ fn: Any) { self.fn = fn }
}

// MARK: - Events

public struct Event: Sendable {
    public let timestamp: Date
    public let type: EventType
    public let slug: String
    public let instanceUid: String
    public let sessionId: String
    public let payload: [String: String]
    public let chain: [Hop]
    public let isPrivate: Bool

    public init(timestamp: Date, type: EventType, slug: String, instanceUid: String,
                sessionId: String = "", payload: [String: String] = [:], chain: [Hop] = [], isPrivate: Bool = false) {
        self.timestamp = timestamp
        self.type = type
        self.slug = slug
        self.instanceUid = instanceUid
        self.sessionId = sessionId
        self.payload = payload
        self.chain = chain
        self.isPrivate = isPrivate
    }
}

public final class EventBus: @unchecked Sendable {
    private let capacity: Int
    private var buf: [Event] = []
    private var subs: [(Event) -> Void] = []
    private var closed = false
    private let lock = NSLock()

    public init(capacity: Int = 256) {
        self.capacity = max(1, capacity)
    }

    public func emit(_ e: Event) {
        lock.lock()
        if closed { lock.unlock(); return }
        buf.append(e)
        if buf.count > capacity {
            buf.removeFirst(buf.count - capacity)
        }
        let snapshot = subs
        lock.unlock()
        for fn in snapshot { fn(e) }
    }

    public func drain() -> [Event] {
        lock.lock(); defer { lock.unlock() }
        return buf
    }

    public func drainSince(_ cutoff: Date) -> [Event] {
        lock.lock(); defer { lock.unlock() }
        return buf.filter { $0.timestamp >= cutoff }
    }

    @discardableResult
    public func subscribe(_ fn: @escaping (Event) -> Void) -> () -> Void {
        lock.lock()
        subs.append(fn)
        let index = subs.count - 1
        lock.unlock()
        return { [weak self] in
            guard let self = self else { return }
            self.lock.lock()
            if index < self.subs.count {
                self.subs.remove(at: index)
            }
            self.lock.unlock()
        }
    }

    public func replayAndSubscribe(since cutoff: Date?, _ fn: @escaping (Event) -> Void) -> ([Event], () -> Void) {
        lock.lock()
        let snapshot = cutoff.map { cutoff in buf.filter { $0.timestamp >= cutoff } } ?? buf
        subs.append(fn)
        let token = ObjectIdentifier(SubscriptionBox(fn))
        let index = subs.count - 1
        lock.unlock()
        return (snapshot, { [weak self] in
            guard let self = self else { return }
            _ = token
            self.lock.lock()
            if index < self.subs.count {
                self.subs.remove(at: index)
            }
            self.lock.unlock()
        })
    }

    public func close() {
        lock.lock()
        closed = true
        subs.removeAll()
        lock.unlock()
    }
}

// MARK: - Metrics

public final class Counter: @unchecked Sendable {
    public let name: String
    public let help: String
    public let labels: [String: String]
    private var value: Int64 = 0
    private let lock = NSLock()

    fileprivate init(name: String, help: String, labels: [String: String]) {
        self.name = name
        self.help = help
        self.labels = labels
    }

    public func inc(_ n: Int64 = 1) {
        if n < 0 { return }
        lock.lock(); value &+= n; lock.unlock()
    }
    public func add(_ n: Int64) { inc(n) }
    public func read() -> Int64 {
        lock.lock(); defer { lock.unlock() }
        return value
    }
}

public final class Gauge: @unchecked Sendable {
    public let name: String
    public let help: String
    public let labels: [String: String]
    private var value: Double = 0
    private let lock = NSLock()

    fileprivate init(name: String, help: String, labels: [String: String]) {
        self.name = name
        self.help = help
        self.labels = labels
    }

    public func set(_ v: Double) { lock.lock(); value = v; lock.unlock() }
    public func add(_ d: Double) { lock.lock(); value += d; lock.unlock() }
    public func read() -> Double {
        lock.lock(); defer { lock.unlock() }
        return value
    }
}

public struct HistogramSnapshot: Sendable {
    public let bounds: [Double]
    public let counts: [Int64]
    public let total: Int64
    public let sum: Double

    public func quantile(_ q: Double) -> Double {
        if total == 0 { return .nan }
        let target = Double(total) * q
        for i in 0..<counts.count {
            if Double(counts[i]) >= target { return bounds[i] }
        }
        return .infinity
    }
}

public let defaultBuckets: [Double] = [
    50e-6, 100e-6, 250e-6, 500e-6,
    1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
    1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
]

public final class Histogram: @unchecked Sendable {
    public let name: String
    public let help: String
    public let labels: [String: String]
    private let bounds: [Double]
    private var counts: [Int64]
    private var total: Int64 = 0
    private var sum: Double = 0
    private let lock = NSLock()

    fileprivate init(name: String, help: String, labels: [String: String], bounds: [Double]?) {
        self.name = name
        self.help = help
        self.labels = labels
        var b = bounds ?? defaultBuckets
        b.sort()
        self.bounds = b
        self.counts = [Int64](repeating: 0, count: b.count)
    }

    public func observe(_ v: Double) {
        lock.lock()
        total &+= 1
        sum += v
        for i in 0..<bounds.count {
            if v <= bounds[i] { counts[i] &+= 1 }
        }
        lock.unlock()
    }

    public func observeDuration(_ d: TimeInterval) { observe(d) }

    public func snapshot() -> HistogramSnapshot {
        lock.lock(); defer { lock.unlock() }
        return HistogramSnapshot(bounds: bounds, counts: counts, total: total, sum: sum)
    }
}

private func metricKey(_ name: String, _ labels: [String: String]) -> String {
    if labels.isEmpty { return name }
    let keys = labels.keys.sorted()
    var s = name
    for k in keys {
        s += "|\(k)=\(labels[k] ?? "")"
    }
    return s
}

public final class Registry: @unchecked Sendable {
    private var counters: [String: Counter] = [:]
    private var gauges: [String: Gauge] = [:]
    private var histograms: [String: Histogram] = [:]
    private let lock = NSLock()

    public func counter(_ name: String, help: String = "", labels: [String: String] = [:]) -> Counter {
        let k = metricKey(name, labels)
        lock.lock(); defer { lock.unlock() }
        if let c = counters[k] { return c }
        let c = Counter(name: name, help: help, labels: labels)
        counters[k] = c
        return c
    }

    public func gauge(_ name: String, help: String = "", labels: [String: String] = [:]) -> Gauge {
        let k = metricKey(name, labels)
        lock.lock(); defer { lock.unlock() }
        if let g = gauges[k] { return g }
        let g = Gauge(name: name, help: help, labels: labels)
        gauges[k] = g
        return g
    }

    public func histogram(_ name: String, help: String = "", labels: [String: String] = [:],
                          bounds: [Double]? = nil) -> Histogram {
        let k = metricKey(name, labels)
        lock.lock(); defer { lock.unlock() }
        if let h = histograms[k] { return h }
        let h = Histogram(name: name, help: help, labels: labels, bounds: bounds)
        histograms[k] = h
        return h
    }

    public func listCounters() -> [Counter] {
        lock.lock(); defer { lock.unlock() }
        return counters.values.sorted(by: { $0.name < $1.name })
    }

    public func listGauges() -> [Gauge] {
        lock.lock(); defer { lock.unlock() }
        return gauges.values.sorted(by: { $0.name < $1.name })
    }

    public func listHistograms() -> [Histogram] {
        lock.lock(); defer { lock.unlock() }
        return histograms.values.sorted(by: { $0.name < $1.name })
    }
}

// MARK: - Config + Observability

public struct ObsConfig: Sendable {
    public var slug: String
    public var defaultLogLevel: Level
    public var promAddr: String
    public var redactedFields: [String]
    public var logsRingSize: Int
    public var eventsRingSize: Int
    public var runDir: String
    public var instanceUid: String
    public var organismUid: String
    public var organismSlug: String

    public init(slug: String = "",
                defaultLogLevel: Level = .info,
                promAddr: String = "",
                redactedFields: [String] = [],
                logsRingSize: Int = 1024,
                eventsRingSize: Int = 256,
                runDir: String = "",
                instanceUid: String = "",
                organismUid: String = "",
                organismSlug: String = "") {
        self.slug = slug
        self.defaultLogLevel = defaultLogLevel
        self.promAddr = promAddr
        self.redactedFields = redactedFields
        self.logsRingSize = logsRingSize
        self.eventsRingSize = eventsRingSize
        self.runDir = runDir
        self.instanceUid = instanceUid
        self.organismUid = organismUid
        self.organismSlug = organismSlug
    }
}

public final class HolonLogger: @unchecked Sendable {
    fileprivate weak var obs: Observability?
    public let name: String
    private var level: Level
    private let lock = NSLock()

    fileprivate init(obs: Observability?, name: String) {
        self.obs = obs
        self.name = name
        self.level = obs?.cfg.defaultLogLevel ?? .fatal
    }

    public func setLevel(_ l: Level) { lock.lock(); level = l; lock.unlock() }

    public func enabled(_ l: Level) -> Bool {
        lock.lock(); defer { lock.unlock() }
        return obs != nil && l >= level
    }

    public func log(_ lvl: Level, _ message: String, fields: [String: String] = [:], isPrivate: Bool = false,
                    file: String = #fileID, line: Int = #line) {
        guard enabled(lvl), let obs = obs else { return }
        let redact = Set(obs.cfg.redactedFields)
        var out: [String: String] = [:]
        for (k, v) in fields {
            if k.isEmpty { continue }
            out[k] = redact.contains(k) ? "<redacted>" : v
        }
        let entry = LogEntry(
            timestamp: Date(),
            level: lvl,
            slug: obs.cfg.slug,
            instanceUid: obs.cfg.instanceUid,
            message: message,
            fields: out,
            caller: "\(file):\(line)",
            isPrivate: isPrivate
        )
        obs.logRing?.push(entry)
    }

    public func trace(_ m: String, _ f: [String: String] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.trace, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func debug(_ m: String, _ f: [String: String] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.debug, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func info(_ m: String, _ f: [String: String] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.info, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func warn(_ m: String, _ f: [String: String] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.warn, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func error(_ m: String, _ f: [String: String] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.error, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func fatal(_ m: String, _ f: [String: String] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.fatal, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
}

public func Private() -> Bool { true }

public final class Observability: @unchecked Sendable {
    public let cfg: ObsConfig
    public let families: Set<Family>
    public let logRing: LogRing?
    public let eventBus: EventBus?
    public let registry: Registry?
    private var loggers: [String: HolonLogger] = [:]
    private let lock = NSLock()

    fileprivate init(cfg: ObsConfig, families: Set<Family>) {
        self.cfg = cfg
        self.families = families
        self.logRing = families.contains(.logs) ? LogRing(capacity: cfg.logsRingSize) : nil
        self.eventBus = families.contains(.events) ? EventBus(capacity: cfg.eventsRingSize) : nil
        self.registry = families.contains(.metrics) ? Registry() : nil
    }

    public func enabled(_ f: Family) -> Bool { families.contains(f) }

    public var isOrganismRoot: Bool {
        !cfg.organismUid.isEmpty && cfg.organismUid == cfg.instanceUid
    }

    public func logger(_ name: String) -> HolonLogger {
        if !families.contains(.logs) { return disabledLogger }
        lock.lock(); defer { lock.unlock() }
        if let l = loggers[name] { return l }
        let l = HolonLogger(obs: self, name: name)
        loggers[name] = l
        return l
    }

    public func counter(_ name: String, help: String = "", labels: [String: String] = [:]) -> Counter? {
        registry?.counter(name, help: help, labels: labels)
    }

    public func gauge(_ name: String, help: String = "", labels: [String: String] = [:]) -> Gauge? {
        registry?.gauge(name, help: help, labels: labels)
    }

    public func histogram(_ name: String, help: String = "", labels: [String: String] = [:],
                          bounds: [Double]? = nil) -> Histogram? {
        registry?.histogram(name, help: help, labels: labels, bounds: bounds)
    }

    public func emit(_ type: EventType, payload: [String: String] = [:], isPrivate: Bool = false) {
        guard let bus = eventBus else { return }
        let redact = Set(cfg.redactedFields)
        var p: [String: String] = [:]
        for (k, v) in payload {
            p[k] = redact.contains(k) ? "<redacted>" : v
        }
        bus.emit(Event(
            timestamp: Date(),
            type: type,
            slug: cfg.slug,
            instanceUid: cfg.instanceUid,
            payload: p,
            isPrivate: isPrivate
        ))
    }

    public func close() {
        eventBus?.close()
    }
}

private let disabledLogger: HolonLogger = {
    let cfg = ObsConfig(defaultLogLevel: .fatal)
    let obs = Observability(cfg: cfg, families: [])
    return HolonLogger(obs: obs, name: "")
}()

// MARK: - Package-scope singleton

private let obsLock = NSLock()
private nonisolated(unsafe) var _current: Observability?

@discardableResult
public func configure(_ cfg: ObsConfig, env: [String: String] = ProcessInfo.processInfo.environment) throws -> Observability {
    let families = try parseOpObs(env["OP_OBS"] ?? "")
    var normalized = cfg
    if normalized.slug.isEmpty {
        normalized.slug = CommandLine.arguments.first.map { (($0 as NSString).lastPathComponent) } ?? ""
    }
    if normalized.instanceUid.isEmpty {
        normalized.instanceUid = UUID().uuidString
    }
    if !normalized.runDir.isEmpty {
        normalized.runDir = deriveRunDir(root: normalized.runDir, slug: normalized.slug, uid: normalized.instanceUid)
    }
    let obs = Observability(cfg: normalized, families: families)
    obsLock.lock()
    _current = obs
    obsLock.unlock()
    return obs
}

@discardableResult
public func fromEnv(_ base: ObsConfig = ObsConfig(), env: [String: String] = ProcessInfo.processInfo.environment) throws -> Observability {
    var cfg = base
    if cfg.instanceUid.isEmpty { cfg.instanceUid = env["OP_INSTANCE_UID"] ?? "" }
    if cfg.organismUid.isEmpty { cfg.organismUid = env["OP_ORGANISM_UID"] ?? "" }
    if cfg.organismSlug.isEmpty { cfg.organismSlug = env["OP_ORGANISM_SLUG"] ?? "" }
    if cfg.promAddr.isEmpty { cfg.promAddr = env["OP_PROM_ADDR"] ?? "" }
    if cfg.runDir.isEmpty { cfg.runDir = env["OP_RUN_DIR"] ?? "" }
    return try configure(cfg, env: env)
}

public func current() -> Observability {
    obsLock.lock(); defer { obsLock.unlock() }
    return _current ?? Observability(cfg: ObsConfig(defaultLogLevel: .fatal), families: [])
}

public func reset() {
    obsLock.lock()
    _current?.close()
    _current = nil
    obsLock.unlock()
}

public func deriveRunDir(root: String, slug: String, uid: String) -> String {
    if root.isEmpty || slug.isEmpty || uid.isEmpty { return root }
    return (root as NSString).appendingPathComponent((slug as NSString).appendingPathComponent(uid))
}

// MARK: - Proto conversion + gRPC service

private func timestamp(_ date: Date) -> Google_Protobuf_Timestamp {
    Google_Protobuf_Timestamp(date: date)
}

private func hopToProto(_ hop: Hop) -> Holons_V1_ChainHop {
    var out = Holons_V1_ChainHop()
    out.slug = hop.slug
    out.instanceUid = hop.instanceUid
    return out
}

private func levelToProto(_ level: Level) -> Holons_V1_LogLevel {
    Holons_V1_LogLevel(rawValue: Int(level.rawValue)) ?? .unspecified
}

private func eventTypeToProto(_ type: EventType) -> Holons_V1_EventType {
    Holons_V1_EventType(rawValue: Int(type.rawValue)) ?? .unspecified
}

public func toProtoLogEntry(_ entry: LogEntry) -> Holons_V1_LogEntry {
    var out = Holons_V1_LogEntry()
    out.ts = timestamp(entry.timestamp)
    out.level = levelToProto(entry.level)
    out.slug = entry.slug
    out.instanceUid = entry.instanceUid
    out.sessionID = entry.sessionId
    out.rpcMethod = entry.rpcMethod
    out.message = entry.message
    out.fields = entry.fields
    out.caller = entry.caller
    out.chain = entry.chain.map(hopToProto)
    return out
}

private func histogramToProto(_ snapshot: HistogramSnapshot) -> Holons_V1_HistogramSample {
    var out = Holons_V1_HistogramSample()
    out.buckets = zip(snapshot.bounds, snapshot.counts).map { upper, count in
        var bucket = Holons_V1_Bucket()
        bucket.upperBound = upper
        bucket.count = count
        return bucket
    }
    out.count = snapshot.total
    out.sum = snapshot.sum
    return out
}

public func toProtoMetricSamples(_ registry: Registry) -> [Holons_V1_MetricSample] {
    var samples: [Holons_V1_MetricSample] = []
    for counter in registry.listCounters() {
        var sample = Holons_V1_MetricSample()
        sample.name = counter.name
        sample.help = counter.help
        sample.labels = counter.labels
        sample.counter = counter.read()
        samples.append(sample)
    }
    for gauge in registry.listGauges() {
        var sample = Holons_V1_MetricSample()
        sample.name = gauge.name
        sample.help = gauge.help
        sample.labels = gauge.labels
        sample.gauge = gauge.read()
        samples.append(sample)
    }
    for histogram in registry.listHistograms() {
        var sample = Holons_V1_MetricSample()
        sample.name = histogram.name
        sample.help = histogram.help
        sample.labels = histogram.labels
        sample.histogram = histogramToProto(histogram.snapshot())
        samples.append(sample)
    }
    return samples
}

public func toProtoEvent(_ event: Event) -> Holons_V1_EventInfo {
    var out = Holons_V1_EventInfo()
    out.ts = timestamp(event.timestamp)
    out.type = eventTypeToProto(event.type)
    out.slug = event.slug
    out.instanceUid = event.instanceUid
    out.sessionID = event.sessionId
    out.payload = event.payload
    out.chain = event.chain.map(hopToProto)
    return out
}

public final class HolonObservabilityService: Holons_V1_HolonObservabilityProvider {
    private let obs: Observability

    public init(_ obs: Observability = current()) {
        self.obs = obs
    }

    public func logs(
        request: Holons_V1_LogsRequest,
        context: StreamingResponseCallContext<Holons_V1_LogEntry>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.logs), let ring = obs.logRing else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "logs family is not enabled (OP_OBS)")
            )
        }
        let minLevel = request.minLevel.rawValue == 0 ? Int(Level.info.rawValue) : request.minLevel.rawValue
        let cutoff = request.hasSince ? Date().addingTimeInterval(-durationSeconds(request.since)) : nil
        var pendingLive: [LogEntry] = []
        var bufferingLive = request.follow
        let pendingLock = NSLock()
        let entries: [LogEntry]
        if request.follow {
            let replay = ring.replayAndSubscribe(since: cutoff) { entry in
                guard !entry.isPrivate,
                      matchLog(entry, minLevel: minLevel, sessionIds: request.sessionIds, rpcMethods: request.rpcMethods) else {
                    return
                }
                pendingLock.lock()
                if bufferingLive {
                    pendingLive.append(entry)
                    pendingLock.unlock()
                    return
                }
                pendingLock.unlock()
                context.eventLoop.execute {
                    context.sendResponse(toProtoLogEntry(entry), promise: nil)
                }
            }
            entries = replay.0
            _ = replay.1
        } else {
            entries = cutoff.map { ring.drainSince($0) } ?? ring.drain()
        }
        var future = context.eventLoop.makeSucceededFuture(())
        for entry in entries where !entry.isPrivate && matchLog(entry, minLevel: minLevel, sessionIds: request.sessionIds, rpcMethods: request.rpcMethods) {
            future = future.flatMap { context.sendResponse(toProtoLogEntry(entry)) }
        }
        guard request.follow else {
            return future.map { .ok }
        }
        let followPromise = context.eventLoop.makePromise(of: GRPCStatus.self)
        future = future.flatMap {
            pendingLock.lock()
            bufferingLive = false
            let buffered = pendingLive
            pendingLive.removeAll()
            pendingLock.unlock()
            var flush = context.eventLoop.makeSucceededFuture(())
            for entry in buffered {
                flush = flush.flatMap { context.sendResponse(toProtoLogEntry(entry)) }
            }
            return flush
        }
        return future.flatMap { followPromise.futureResult }
    }

    public func metrics(
        request: Holons_V1_MetricsRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_MetricsSnapshot> {
        guard obs.enabled(.metrics), let registry = obs.registry else {
            return context.eventLoop.makeFailedFuture(
                GRPCStatus(code: .failedPrecondition, message: "metrics family is not enabled (OP_OBS)")
            )
        }
        var samples = toProtoMetricSamples(registry)
        if !request.namePrefixes.isEmpty {
            samples = samples.filter { sample in
                request.namePrefixes.contains { prefix in sample.name.hasPrefix(prefix) }
            }
        }
        var snapshot = Holons_V1_MetricsSnapshot()
        snapshot.capturedAt = timestamp(Date())
        snapshot.slug = obs.cfg.slug
        snapshot.instanceUid = obs.cfg.instanceUid
        snapshot.samples = samples
        return context.eventLoop.makeSucceededFuture(snapshot)
    }

    public func events(
        request: Holons_V1_EventsRequest,
        context: StreamingResponseCallContext<Holons_V1_EventInfo>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.events), let bus = obs.eventBus else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "events family is not enabled (OP_OBS)")
            )
        }
        let wanted = Set(request.types.map { $0.rawValue })
        let cutoff = request.hasSince ? Date().addingTimeInterval(-durationSeconds(request.since)) : nil
        var pendingLive: [Event] = []
        var bufferingLive = request.follow
        let pendingLock = NSLock()
        let events: [Event]
        if request.follow {
            let replay = bus.replayAndSubscribe(since: cutoff) { event in
                guard !event.isPrivate, matchEvent(event, wanted: wanted) else {
                    return
                }
                pendingLock.lock()
                if bufferingLive {
                    pendingLive.append(event)
                    pendingLock.unlock()
                    return
                }
                pendingLock.unlock()
                context.eventLoop.execute {
                    context.sendResponse(toProtoEvent(event), promise: nil)
                }
            }
            events = replay.0
            _ = replay.1
        } else {
            events = cutoff.map { bus.drainSince($0) } ?? bus.drain()
        }
        var future = context.eventLoop.makeSucceededFuture(())
        for event in events where !event.isPrivate && matchEvent(event, wanted: wanted) {
            future = future.flatMap { context.sendResponse(toProtoEvent(event)) }
        }
        guard request.follow else {
            return future.map { .ok }
        }
        let followPromise = context.eventLoop.makePromise(of: GRPCStatus.self)
        future = future.flatMap {
            pendingLock.lock()
            bufferingLive = false
            let buffered = pendingLive
            pendingLive.removeAll()
            pendingLock.unlock()
            var flush = context.eventLoop.makeSucceededFuture(())
            for event in buffered {
                flush = flush.flatMap { context.sendResponse(toProtoEvent(event)) }
            }
            return flush
        }
        return future.flatMap { followPromise.futureResult }
    }
}

public func registerObservabilityService(_ obs: Observability = current()) -> CallHandlerProvider {
    HolonObservabilityService(obs)
}

public func fromProtoLogEntry(_ proto: Holons_V1_LogEntry) -> LogEntry {
    LogEntry(
        timestamp: proto.ts.date,
        level: Level(rawValue: Int32(proto.level.rawValue)) ?? .unset,
        slug: proto.slug,
        instanceUid: proto.instanceUid,
        sessionId: proto.sessionID,
        rpcMethod: proto.rpcMethod,
        message: proto.message,
        fields: proto.fields,
        caller: proto.caller,
        chain: proto.chain.map { Hop(slug: $0.slug, instanceUid: $0.instanceUid) }
    )
}

public func fromProtoEvent(_ proto: Holons_V1_EventInfo) -> Event {
    Event(
        timestamp: proto.ts.date,
        type: EventType(rawValue: Int32(proto.type.rawValue)) ?? .unspecified,
        slug: proto.slug,
        instanceUid: proto.instanceUid,
        sessionId: proto.sessionID,
        payload: proto.payload,
        chain: proto.chain.map { Hop(slug: $0.slug, instanceUid: $0.instanceUid) }
    )
}

private func prometheusText(_ obs: Observability) -> String {
    guard let registry = obs.registry else { return "" }
    var lines: [String] = []
    for counter in registry.listCounters() {
        if !counter.help.isEmpty { lines.append("# HELP \(counter.name) \(escapePromHelp(counter.help))") }
        lines.append("# TYPE \(counter.name) counter")
        lines.append("\(counter.name)\(promLabels(counter.labels, obs: obs)) \(counter.read())")
    }
    for gauge in registry.listGauges() {
        if !gauge.help.isEmpty { lines.append("# HELP \(gauge.name) \(escapePromHelp(gauge.help))") }
        lines.append("# TYPE \(gauge.name) gauge")
        lines.append("\(gauge.name)\(promLabels(gauge.labels, obs: obs)) \(gauge.read())")
    }
    for histogram in registry.listHistograms() {
        if !histogram.help.isEmpty { lines.append("# HELP \(histogram.name) \(escapePromHelp(histogram.help))") }
        lines.append("# TYPE \(histogram.name) histogram")
        let snapshot = histogram.snapshot()
        let baseLabels = identityLabels(histogram.labels, obs: obs)
        for (bound, count) in zip(snapshot.bounds, snapshot.counts) {
            var labels = baseLabels
            labels["le"] = String(bound)
            lines.append("\(histogram.name)_bucket\(formatPromLabels(labels)) \(count)")
        }
        var infLabels = baseLabels
        infLabels["le"] = "+Inf"
        lines.append("\(histogram.name)_bucket\(formatPromLabels(infLabels)) \(snapshot.total)")
        lines.append("\(histogram.name)_sum\(formatPromLabels(baseLabels)) \(snapshot.sum)")
        lines.append("\(histogram.name)_count\(formatPromLabels(baseLabels)) \(snapshot.total)")
    }
    return lines.joined(separator: "\n") + "\n"
}

private func identityLabels(_ labels: [String: String], obs: Observability) -> [String: String] {
    var out = labels
    if !obs.cfg.slug.isEmpty, out["slug"] == nil { out["slug"] = obs.cfg.slug }
    if !obs.cfg.instanceUid.isEmpty, out["instance_uid"] == nil { out["instance_uid"] = obs.cfg.instanceUid }
    return out
}

private func promLabels(_ labels: [String: String], obs: Observability) -> String {
    formatPromLabels(identityLabels(labels, obs: obs))
}

private func formatPromLabels(_ labels: [String: String]) -> String {
    guard !labels.isEmpty else { return "" }
    let rendered = labels.keys.sorted().map { key in
        "\(key)=\"\(escapePromLabel(labels[key] ?? ""))\""
    }
    return "{\(rendered.joined(separator: ","))}"
}

private func escapePromHelp(_ value: String) -> String {
    value.replacingOccurrences(of: "\\", with: "\\\\")
        .replacingOccurrences(of: "\n", with: "\\n")
}

private func escapePromLabel(_ value: String) -> String {
    value.replacingOccurrences(of: "\\", with: "\\\\")
        .replacingOccurrences(of: "\n", with: "\\n")
        .replacingOccurrences(of: "\"", with: "\\\"")
}

final class PrometheusServer {
    private let obs: Observability
    private let fd: Int32
    private let queue = DispatchQueue(label: "holons.prometheus.server")
    private var stopped = false
    private let lock = NSLock()
    private(set) var address: String = ""

    init(obs: Observability, bind: String) throws {
        self.obs = obs
        let parsed = parsePromBind(bind)
        fd = obsSocket(AF_INET, obsSocketType, 0)
        guard fd >= 0 else { throw NSError(domain: "prometheus", code: 1, userInfo: [NSLocalizedDescriptionKey: obsErrnoMessage()]) }
        var one: Int32 = 1
        _ = setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, socklen_t(MemoryLayout<Int32>.size))
        var addr = sockaddr_in()
        #if os(Linux)
        #else
        addr.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
        #endif
        addr.sin_family = sa_family_t(AF_INET)
        addr.sin_port = in_port_t(parsed.port).bigEndian
        addr.sin_addr = in_addr(s_addr: inet_addr(parsed.host))
        let bindResult = withUnsafePointer(to: &addr) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                obsBind(fd, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
            }
        }
        guard bindResult == 0 else {
            obsClose(fd)
            throw NSError(domain: "prometheus", code: 2, userInfo: [NSLocalizedDescriptionKey: obsErrnoMessage()])
        }
        guard obsListen(fd, 16) == 0 else {
            obsClose(fd)
            throw NSError(domain: "prometheus", code: 3, userInfo: [NSLocalizedDescriptionKey: obsErrnoMessage()])
        }
        var actual = sockaddr_in()
        var len = socklen_t(MemoryLayout<sockaddr_in>.size)
        withUnsafeMutablePointer(to: &actual) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
                _ = getsockname(fd, $0, &len)
            }
        }
        let port = Int(UInt16(bigEndian: actual.sin_port))
        address = "http://\(parsed.advertisedHost):\(port)/metrics"
        queue.async { [weak self] in self?.acceptLoop() }
    }

    func stop() {
        lock.lock()
        if stopped {
            lock.unlock()
            return
        }
        stopped = true
        lock.unlock()
        obsClose(fd)
    }

    private func acceptLoop() {
        while true {
            lock.lock()
            let isStopped = stopped
            lock.unlock()
            if isStopped { return }
            let client = obsAccept(fd, nil, nil)
            if client < 0 { continue }
            handle(client)
        }
    }

    private func handle(_ client: Int32) {
        var buffer = [UInt8](repeating: 0, count: 1024)
        _ = obsRead(client, &buffer, buffer.count)
        let body = prometheusText(obs)
        let response = """
HTTP/1.1 200 OK\r
Content-Type: text/plain; version=0.0.4; charset=utf-8\r
Content-Length: \(body.utf8.count)\r
Connection: close\r
\r
\(body)
"""
        response.withCString { pointer in
            _ = obsWrite(client, pointer, strlen(pointer))
        }
        obsClose(client)
    }
}

private func parsePromBind(_ raw: String) -> (host: String, advertisedHost: String, port: Int) {
    var value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    if value.isEmpty { value = "127.0.0.1:0" }
    if value.hasPrefix("http://"), let url = URL(string: value) {
        value = "\(url.host ?? "127.0.0.1"):\(url.port ?? 0)"
    }
    let idx = value.lastIndex(of: ":")
    let rawHost = idx.map { String(value[..<$0]) } ?? ""
    let host = rawHost.isEmpty ? "127.0.0.1" : rawHost
    let portText = idx.map { String(value[value.index(after: $0)...]) } ?? "0"
    let advertised = (host == "0.0.0.0" || host == "*") ? "127.0.0.1" : host
    return (host == "*" ? "0.0.0.0" : host, advertised, Int(portText) ?? 0)
}

private struct MemberIdentity {
    let slug: String
    let instanceUid: String
}

public final class MemberRelay {
    private let obs: Observability
    private let memberSlug: String
    private let address: String
    private let logger: (String) -> Void
    private var stopped = false
    private let lock = NSLock()

    public init(obs: Observability, memberSlug: String, address: String, logger: @escaping (String) -> Void = { _ in }) {
        self.obs = obs
        self.memberSlug = memberSlug
        self.address = address
        self.logger = logger
    }

    public func start() {
        if obs.enabled(.logs), obs.logRing != nil {
            DispatchQueue.global().async { [weak self] in self?.pumpLogs() }
        }
        if obs.enabled(.events), obs.eventBus != nil {
            DispatchQueue.global().async { [weak self] in self?.pumpEvents() }
        }
    }

    public func stop() {
        lock.lock()
        stopped = true
        lock.unlock()
    }

    private var isStopped: Bool {
        lock.lock(); defer { lock.unlock() }
        return stopped
    }

    private func pumpLogs() {
        while !isStopped {
            var channel: GRPCChannel?
            do {
                channel = try connect(address, options: ConnectOptions(timeout: 5, transport: "tcp", start: false))
                let identity = resolveIdentity(channel!)
                let client = Holons_V1_HolonObservabilityClient(channel: channel!)
                var request = Holons_V1_LogsRequest()
                request.minLevel = .info
                request.follow = true
                let call = client.logs(request) { [weak self] proto in
                    guard let self = self else { return }
                    var entry = fromProtoLogEntry(proto)
                    entry = LogEntry(
                        timestamp: entry.timestamp,
                        level: entry.level,
                        slug: entry.slug,
                        instanceUid: entry.instanceUid,
                        sessionId: entry.sessionId,
                        rpcMethod: entry.rpcMethod,
                        message: entry.message,
                        fields: entry.fields,
                        caller: entry.caller,
                        chain: enrichForMultilog(entry.chain, streamSourceSlug: identity.slug, streamSourceUid: identity.instanceUid)
                    )
                    self.obs.logRing?.push(entry)
                }
                _ = try? call.status.wait()
            } catch {
                logger("member relay logs \(memberSlug): \(error)")
            }
            if let channel { try? disconnect(channel) }
            if !isStopped { Thread.sleep(forTimeInterval: 2) }
        }
    }

    private func pumpEvents() {
        while !isStopped {
            var channel: GRPCChannel?
            do {
                channel = try connect(address, options: ConnectOptions(timeout: 5, transport: "tcp", start: false))
                let identity = resolveIdentity(channel!)
                let client = Holons_V1_HolonObservabilityClient(channel: channel!)
                var request = Holons_V1_EventsRequest()
                request.follow = true
                let call = client.events(request) { [weak self] proto in
                    guard let self = self else { return }
                    var event = fromProtoEvent(proto)
                    event = Event(
                        timestamp: event.timestamp,
                        type: event.type,
                        slug: event.slug,
                        instanceUid: event.instanceUid,
                        sessionId: event.sessionId,
                        payload: event.payload,
                        chain: enrichForMultilog(event.chain, streamSourceSlug: identity.slug, streamSourceUid: identity.instanceUid)
                    )
                    self.obs.eventBus?.emit(event)
                }
                _ = try? call.status.wait()
            } catch {
                logger("member relay events \(memberSlug): \(error)")
            }
            if let channel { try? disconnect(channel) }
            if !isStopped { Thread.sleep(forTimeInterval: 2) }
        }
    }

    private func resolveIdentity(_ channel: GRPCChannel) -> MemberIdentity {
        let client = Holons_V1_HolonObservabilityClient(channel: channel)
        var selected: MemberIdentity?
        let call = client.events(Holons_V1_EventsRequest()) { [memberSlug] event in
            guard event.type == .instanceReady, !event.instanceUid.isEmpty else { return }
            let identity = MemberIdentity(slug: event.slug.isEmpty ? memberSlug : event.slug, instanceUid: event.instanceUid)
            if event.chain.isEmpty || selected == nil {
                selected = identity
            }
        }
        _ = try? call.status.wait()
        return selected ?? MemberIdentity(slug: memberSlug, instanceUid: "")
    }
}

private var obsSocketType: Int32 {
    #if os(Linux)
    return Int32(SOCK_STREAM.rawValue)
    #else
    return SOCK_STREAM
    #endif
}

private func obsErrnoMessage() -> String { String(cString: strerror(errno)) }

private func obsSocket(_ domain: Int32, _ type: Int32, _ proto: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.socket(domain, type, proto)
    #else
    return Darwin.socket(domain, type, proto)
    #endif
}

private func obsBind(_ fd: Int32, _ addr: UnsafePointer<sockaddr>?, _ len: socklen_t) -> Int32 {
    #if os(Linux)
    return Glibc.bind(fd, addr, len)
    #else
    return Darwin.bind(fd, addr, len)
    #endif
}

private func obsListen(_ fd: Int32, _ backlog: Int32) -> Int32 {
    #if os(Linux)
    return Glibc.listen(fd, backlog)
    #else
    return Darwin.listen(fd, backlog)
    #endif
}

private func obsAccept(_ fd: Int32, _ addr: UnsafeMutablePointer<sockaddr>?, _ len: UnsafeMutablePointer<socklen_t>?) -> Int32 {
    #if os(Linux)
    return Glibc.accept(fd, addr, len)
    #else
    return Darwin.accept(fd, addr, len)
    #endif
}

private func obsRead(_ fd: Int32, _ buf: UnsafeMutableRawPointer?, _ count: Int) -> Int {
    #if os(Linux)
    return Glibc.read(fd, buf, count)
    #else
    return Darwin.read(fd, buf, count)
    #endif
}

private func obsWrite(_ fd: Int32, _ buf: UnsafeRawPointer?, _ count: Int) -> Int {
    #if os(Linux)
    return Glibc.write(fd, buf, count)
    #else
    return Darwin.write(fd, buf, count)
    #endif
}

private func obsClose(_ fd: Int32) {
    #if os(Linux)
    _ = Glibc.close(fd)
    #else
    _ = Darwin.close(fd)
    #endif
}

private func durationSeconds(_ duration: Google_Protobuf_Duration) -> TimeInterval {
    TimeInterval(duration.seconds) + TimeInterval(duration.nanos) / 1_000_000_000
}

private func matchLog(_ entry: LogEntry, minLevel: Int, sessionIds: [String], rpcMethods: [String]) -> Bool {
    if Int(entry.level.rawValue) < minLevel { return false }
    if !sessionIds.isEmpty && !sessionIds.contains(entry.sessionId) { return false }
    if !rpcMethods.isEmpty && !rpcMethods.contains(entry.rpcMethod) { return false }
    return true
}

private func matchEvent(_ event: Event, wanted: Set<Int>) -> Bool {
    wanted.isEmpty || wanted.contains(Int(event.type.rawValue))
}

// MARK: - Disk writers

public func enableDiskWriters(_ runDir: String) {
    obsLock.lock()
    let obs = _current
    obsLock.unlock()
    guard let obs = obs, !runDir.isEmpty else { return }
    try? FileManager.default.createDirectory(atPath: runDir, withIntermediateDirectories: true)

    if obs.enabled(.logs), let ring = obs.logRing {
        let fp = (runDir as NSString).appendingPathComponent("stdout.log")
        ring.subscribe { entry in
            var rec: [String: Any] = [
                "kind": "log",
                "ts": ISO8601DateFormatter().string(from: entry.timestamp),
                "level": entry.level.name,
                "slug": entry.slug,
                "instance_uid": entry.instanceUid,
                "message": entry.message,
            ]
            if !entry.sessionId.isEmpty { rec["session_id"] = entry.sessionId }
            if !entry.rpcMethod.isEmpty { rec["rpc_method"] = entry.rpcMethod }
            if !entry.fields.isEmpty { rec["fields"] = entry.fields }
            if !entry.caller.isEmpty { rec["caller"] = entry.caller }
            if !entry.chain.isEmpty {
                rec["chain"] = entry.chain.map { ["slug": $0.slug, "instance_uid": $0.instanceUid] }
            }
            appendJSONL(fp, rec)
        }
    }

    if obs.enabled(.events), let bus = obs.eventBus {
        let fp = (runDir as NSString).appendingPathComponent("events.jsonl")
        bus.subscribe { e in
            var rec: [String: Any] = [
                "kind": "event",
                "ts": ISO8601DateFormatter().string(from: e.timestamp),
                "type": e.type.protoName,
                "slug": e.slug,
                "instance_uid": e.instanceUid,
            ]
            if !e.sessionId.isEmpty { rec["session_id"] = e.sessionId }
            if !e.payload.isEmpty { rec["payload"] = e.payload }
            if !e.chain.isEmpty {
                rec["chain"] = e.chain.map { ["slug": $0.slug, "instance_uid": $0.instanceUid] }
            }
            appendJSONL(fp, rec)
        }
    }
}

private func appendJSONL(_ path: String, _ rec: [String: Any]) {
    guard let data = try? JSONSerialization.data(withJSONObject: rec, options: []) else { return }
    var payload = data
    payload.append(0x0a) // newline
    if FileManager.default.fileExists(atPath: path) {
        if let h = try? FileHandle(forWritingTo: URL(fileURLWithPath: path)) {
            defer { try? h.close() }
            _ = try? h.seekToEnd()
            try? h.write(contentsOf: payload)
        }
    } else {
        try? payload.write(to: URL(fileURLWithPath: path))
    }
}

public struct MetaJson: Codable {
    public var slug: String
    public var uid: String
    public var pid: Int
    public var startedAt: Date
    public var mode: String
    public var transport: String
    public var address: String
    public var metricsAddr: String
    public var logPath: String
    public var logBytesRotated: Int64
    public var organismUid: String
    public var organismSlug: String
    public var isDefault: Bool

    enum CodingKeys: String, CodingKey {
        case slug, uid, pid
        case startedAt = "started_at"
        case mode, transport, address
        case metricsAddr = "metrics_addr"
        case logPath = "log_path"
        case logBytesRotated = "log_bytes_rotated"
        case organismUid = "organism_uid"
        case organismSlug = "organism_slug"
        case isDefault = "default"
    }

    public init(slug: String, uid: String, pid: Int, startedAt: Date,
                mode: String = "persistent", transport: String = "",
                address: String = "", metricsAddr: String = "",
                logPath: String = "", logBytesRotated: Int64 = 0,
                organismUid: String = "", organismSlug: String = "",
                isDefault: Bool = false) {
        self.slug = slug
        self.uid = uid
        self.pid = pid
        self.startedAt = startedAt
        self.mode = mode
        self.transport = transport
        self.address = address
        self.metricsAddr = metricsAddr
        self.logPath = logPath
        self.logBytesRotated = logBytesRotated
        self.organismUid = organismUid
        self.organismSlug = organismSlug
        self.isDefault = isDefault
    }

    public init(from decoder: Swift.Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.slug = try container.decode(String.self, forKey: .slug)
        self.uid = try container.decode(String.self, forKey: .uid)
        self.pid = try container.decode(Int.self, forKey: .pid)
        if let decoded = try? container.decode(Date.self, forKey: .startedAt) {
            self.startedAt = decoded
        } else {
            let raw = try container.decode(String.self, forKey: .startedAt)
            self.startedAt = parseMetaDate(raw) ?? Date(timeIntervalSince1970: 0)
        }
        self.mode = try container.decodeIfPresent(String.self, forKey: .mode) ?? "persistent"
        self.transport = try container.decodeIfPresent(String.self, forKey: .transport) ?? ""
        self.address = try container.decodeIfPresent(String.self, forKey: .address) ?? ""
        self.metricsAddr = try container.decodeIfPresent(String.self, forKey: .metricsAddr) ?? ""
        self.logPath = try container.decodeIfPresent(String.self, forKey: .logPath) ?? ""
        self.logBytesRotated = try container.decodeIfPresent(Int64.self, forKey: .logBytesRotated) ?? 0
        self.organismUid = try container.decodeIfPresent(String.self, forKey: .organismUid) ?? ""
        self.organismSlug = try container.decodeIfPresent(String.self, forKey: .organismSlug) ?? ""
        self.isDefault = try container.decodeIfPresent(Bool.self, forKey: .isDefault) ?? false
    }
}

public func writeMetaJson(_ runDir: String, _ meta: MetaJson) throws {
    try FileManager.default.createDirectory(atPath: runDir, withIntermediateDirectories: true)
    let url = URL(fileURLWithPath: (runDir as NSString).appendingPathComponent("meta.json"))
    let tmp = url.appendingPathExtension("tmp")
    let enc = JSONEncoder()
    enc.outputFormatting = [.prettyPrinted, .sortedKeys]
    enc.dateEncodingStrategy = .iso8601
    let data = try enc.encode(meta)
    try data.write(to: tmp)
    if FileManager.default.fileExists(atPath: url.path) {
        _ = try? FileManager.default.removeItem(at: url)
    }
    try FileManager.default.moveItem(at: tmp, to: url)
}

private func parseMetaDate(_ raw: String) -> Date? {
    let fractional = ISO8601DateFormatter()
    fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let date = fractional.date(from: raw) {
        return date
    }
    return ISO8601DateFormatter().date(from: raw)
}
