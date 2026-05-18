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
    case debug = 5
    case info = 9
    case warn = 13
    case error = 17
    case fatal = 21

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

public let EventInstanceSpawned = "instance.spawned"
public let EventInstanceReady = "instance.ready"
public let EventInstanceExited = "instance.exited"
public let EventInstanceCrashed = "instance.crashed"
public let EventSessionStarted = "session.started"
public let EventSessionEnded = "session.ended"
public let EventHandlerPanic = "handler.panic"
public let EventConfigReloaded = "config.reloaded"

public let AttrHolonsSlug = "holons.slug"
public let AttrServiceName = "service.name"
public let AttrHolonsInstanceUID = "holons.instance_uid"
public let AttrServiceInstanceID = "service.instance.id"
public let AttrHolonsSessionID = "holons.session_id"
public let AttrHolonsTransport = "holons.transport"
public let AttrLoggerName = "logger.name"
public let AttrCodeCaller = "code.caller"
public let AttrRPCMethod = "rpc.method"

public enum LogField: Sendable, Equatable,
    ExpressibleByStringLiteral, ExpressibleByBooleanLiteral,
    ExpressibleByIntegerLiteral, ExpressibleByFloatLiteral {
    case string(String)
    case bool(Bool)
    case int64(Int64)
    case float64(Double)

    public init(stringLiteral value: String) { self = .string(value) }
    public init(booleanLiteral value: Bool) { self = .bool(value) }
    public init(integerLiteral value: Int64) { self = .int64(value) }
    public init(floatLiteral value: Double) { self = .float64(value) }
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

public func appendDirectChild(_ src: [String], childSlug: String, childUid: String = "") -> [String] {
    src + [childSlug]
}

public func enrichForMultilog(_ wire: [String], streamSourceSlug: String, streamSourceUid: String = "") -> [String] {
    appendDirectChild(wire, childSlug: streamSourceSlug, childUid: streamSourceUid)
}

// MARK: - Log records + ring

public struct LogRecord: Sendable {
    public var record: Holons_V1_LogRecord
    public var isPrivate: Bool

    public init(record: Holons_V1_LogRecord = Holons_V1_LogRecord(), isPrivate: Bool = false) {
        self.record = record
        self.isPrivate = isPrivate
    }

    public var timestamp: Date {
        guard record.timeUnixNano > 0 else { return Date(timeIntervalSince1970: 0) }
        return Date(timeIntervalSince1970: TimeInterval(record.timeUnixNano) / 1_000_000_000)
    }

    public var bodyString: String {
        anyValueString(record.body)
    }

    public func attribute(_ key: String) -> String {
        stringAttribute(record.attributes, key: key)
    }
}

public final class LogRing: @unchecked Sendable {
    private let capacity: Int
    private var buf: [LogRecord] = []
    private var subs: [(LogRecord) -> Void] = []
    private let lock = NSLock()

    public init(capacity: Int = 1024) {
        self.capacity = max(1, capacity)
    }

    public func push(_ e: LogRecord) {
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

    public func drain() -> [LogRecord] {
        lock.lock(); defer { lock.unlock() }
        return buf
    }

    public func drainSince(_ cutoff: Date) -> [LogRecord] {
        lock.lock(); defer { lock.unlock() }
        return buf.filter { $0.timestamp >= cutoff }
    }

    @discardableResult
    public func subscribe(_ fn: @escaping (LogRecord) -> Void) -> () -> Void {
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

    public func replayAndSubscribe(since cutoff: Date?, _ fn: @escaping (LogRecord) -> Void) -> ([LogRecord], () -> Void) {
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

public typealias Event = LogRecord

public final class EventBus: @unchecked Sendable {
    private let capacity: Int
    private var buf: [LogRecord] = []
    private var subs: [(LogRecord) -> Void] = []
    private var closed = false
    private let lock = NSLock()

    public init(capacity: Int = 256) {
        self.capacity = max(1, capacity)
    }

    public func emit(_ e: LogRecord) {
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

    public func drain() -> [LogRecord] {
        lock.lock(); defer { lock.unlock() }
        return buf
    }

    public func drainSince(_ cutoff: Date) -> [LogRecord] {
        lock.lock(); defer { lock.unlock() }
        return buf.filter { $0.timestamp >= cutoff }
    }

    @discardableResult
    public func subscribe(_ fn: @escaping (LogRecord) -> Void) -> () -> Void {
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

    public func replayAndSubscribe(since cutoff: Date?, _ fn: @escaping (LogRecord) -> Void) -> ([LogRecord], () -> Void) {
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

    public func log(_ lvl: Level, _ message: String, fields: [String: LogField] = [:], isPrivate: Bool = false,
                    file: String = #fileID, line: Int = #line) {
        guard enabled(lvl), let obs = obs else { return }
        let redact = Set(obs.cfg.redactedFields)
        var attrs = resourceAttributes(slug: obs.cfg.slug, uid: obs.cfg.instanceUid, sessionId: "")
        if !name.isEmpty {
            attrs.append(keyValue(AttrLoggerName, .string(name)))
        }
        for (k, v) in fields {
            if k.isEmpty { continue }
            attrs.append(keyValue(k, redact.contains(k) ? .string("<redacted>") : v))
        }
        attrs.append(keyValue(AttrCodeCaller, .string("\(file):\(line)")))
        let now = unixNano(Date())
        var record = Holons_V1_LogRecord()
        record.timeUnixNano = now
        record.observedTimeUnixNano = now
        record.severityNumber = severityToProto(lvl)
        record.severityText = lvl.name
        record.body = toAnyValue(.string(message))
        record.attributes = attrs
        obs.logRing?.push(LogRecord(record: record, isPrivate: isPrivate))
    }

    public func trace(_ m: String, _ f: [String: LogField] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.trace, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func debug(_ m: String, _ f: [String: LogField] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.debug, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func info(_ m: String, _ f: [String: LogField] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.info, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func warn(_ m: String, _ f: [String: LogField] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.warn, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func error(_ m: String, _ f: [String: LogField] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.error, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
    public func fatal(_ m: String, _ f: [String: LogField] = [:], isPrivate: Bool = false, file: String = #fileID, line: Int = #line) { log(.fatal, m, fields: f, isPrivate: isPrivate, file: file, line: line) }
}

public func Private() -> Bool { true }

public final class Observability: @unchecked Sendable {
    public let cfg: ObsConfig
    public let families: Set<Family>
    public let logRing: LogRing?
    public let eventBus: EventBus?
    public let registry: Registry?
    let startTime: Date
    private var loggers: [String: HolonLogger] = [:]
    private let lock = NSLock()

    fileprivate init(cfg: ObsConfig, families: Set<Family>) {
        self.cfg = cfg
        self.families = families
        self.startTime = Date()
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

    public func emit(_ eventName: String, payload: [String: LogField] = [:], isPrivate: Bool = false) {
        guard let bus = eventBus else { return }
        let redact = Set(cfg.redactedFields)
        var attrs = resourceAttributes(slug: cfg.slug, uid: cfg.instanceUid, sessionId: "")
        for (k, v) in payload {
            if k.isEmpty { continue }
            attrs.append(keyValue(k, redact.contains(k) ? .string("<redacted>") : v))
        }
        let now = unixNano(Date())
        var record = Holons_V1_LogRecord()
        record.timeUnixNano = now
        record.observedTimeUnixNano = now
        record.severityNumber = .info
        record.severityText = "INFO"
        record.body = toAnyValue(.string(eventName))
        record.attributes = attrs
        record.eventName = eventName
        bus.emit(LogRecord(record: record, isPrivate: isPrivate))
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

private func unixNano(_ date: Date) -> UInt64 {
    UInt64(max(0, date.timeIntervalSince1970 * 1_000_000_000))
}

private func severityToProto(_ level: Level) -> Holons_V1_SeverityNumber {
    Holons_V1_SeverityNumber(rawValue: Int(level.rawValue)) ?? .unspecified
}

public func toAnyValue(_ field: LogField) -> Holons_V1_AnyValue {
    var out = Holons_V1_AnyValue()
    switch field {
    case .string(let value):
        out.stringValue = value
    case .bool(let value):
        out.boolValue = value
    case .int64(let value):
        out.intValue = value
    case .float64(let value):
        out.doubleValue = value
    }
    return out
}

public func anyValueString(_ value: Holons_V1_AnyValue) -> String {
    switch value.value {
    case .stringValue(let value)?:
        return value
    case .boolValue(let value)?:
        return value ? "true" : "false"
    case .intValue(let value)?:
        return String(value)
    case .doubleValue(let value)?:
        return String(value)
    case nil:
        return ""
    }
}

public func keyValue(_ key: String, _ value: LogField) -> Holons_V1_KeyValue {
    var out = Holons_V1_KeyValue()
    out.key = key
    out.value = toAnyValue(value)
    return out
}

public func stringAttribute(_ attrs: [Holons_V1_KeyValue], key: String) -> String {
    for attr in attrs where attr.key == key {
        return anyValueString(attr.value)
    }
    return ""
}

private func resourceAttributes(slug: String, uid: String, sessionId: String) -> [Holons_V1_KeyValue] {
    [
        keyValue(AttrHolonsSlug, .string(slug)),
        keyValue(AttrServiceName, .string(slug)),
        keyValue(AttrHolonsInstanceUID, .string(uid)),
        keyValue(AttrServiceInstanceID, .string(uid)),
        keyValue(AttrHolonsSessionID, .string(sessionId)),
    ]
}

private func sortedMapAttributes(_ labels: [String: String]) -> [Holons_V1_KeyValue] {
    labels.keys.sorted().map { keyValue($0, .string(labels[$0] ?? "")) }
}

public func toProtoLogRecord(_ entry: LogRecord) -> Holons_V1_LogRecord {
    entry.record
}

public func fromProtoLogRecord(_ proto: Holons_V1_LogRecord) -> LogRecord {
    LogRecord(record: proto)
}

private func histogramBucketCounts(_ snapshot: HistogramSnapshot) -> [UInt64] {
    var out: [UInt64] = []
    var previous: Int64 = 0
    for cumulative in snapshot.counts {
        out.append(UInt64(max(0, cumulative - previous)))
        previous = cumulative
    }
    out.append(UInt64(max(0, snapshot.total - previous)))
    return out
}

public func toProtoMetrics(_ registry: Registry, slug: String = "", uid: String = "", startTime: Date = Date()) -> [Holons_V1_Metric] {
    var samples: [Holons_V1_Metric] = []
    let startNano = unixNano(startTime)
    let timeNano = unixNano(Date())
    for counter in registry.listCounters() {
        var point = Holons_V1_NumberDataPoint()
        point.startTimeUnixNano = startNano
        point.timeUnixNano = timeNano
        point.asInt = counter.read()
        point.attributes = resourceAttributes(slug: slug, uid: uid, sessionId: "") + sortedMapAttributes(counter.labels)
        var sum = Holons_V1_Sum()
        sum.aggregationTemporality = .cumulative
        sum.isMonotonic = true
        sum.dataPoints = [point]
        var sample = Holons_V1_Metric()
        sample.name = counter.name
        sample.description_p = counter.help
        sample.sum = sum
        samples.append(sample)
    }
    for gauge in registry.listGauges() {
        var point = Holons_V1_NumberDataPoint()
        point.startTimeUnixNano = startNano
        point.timeUnixNano = timeNano
        point.asDouble = gauge.read()
        point.attributes = resourceAttributes(slug: slug, uid: uid, sessionId: "") + sortedMapAttributes(gauge.labels)
        var gaugeData = Holons_V1_Gauge()
        gaugeData.dataPoints = [point]
        var sample = Holons_V1_Metric()
        sample.name = gauge.name
        sample.description_p = gauge.help
        sample.gauge = gaugeData
        samples.append(sample)
    }
    for histogram in registry.listHistograms() {
        let snap = histogram.snapshot()
        var point = Holons_V1_HistogramDataPoint()
        point.startTimeUnixNano = startNano
        point.timeUnixNano = timeNano
        point.count = UInt64(max(0, snap.total))
        point.sum = snap.sum
        point.bucketCounts = histogramBucketCounts(snap)
        point.explicitBounds = snap.bounds
        point.attributes = resourceAttributes(slug: slug, uid: uid, sessionId: "") + sortedMapAttributes(histogram.labels)
        var histogramData = Holons_V1_Histogram()
        histogramData.aggregationTemporality = .cumulative
        histogramData.dataPoints = [point]
        var sample = Holons_V1_Metric()
        sample.name = histogram.name
        sample.description_p = histogram.help
        sample.histogram = histogramData
        samples.append(sample)
    }
    return samples
}

public final class HolonObservabilityService: Holons_V1_HolonObservabilityProvider {
    public let interceptors: Holons_V1_HolonObservabilityServerInterceptorFactoryProtocol? = nil
    private let obs: Observability

    public init(_ obs: Observability = current()) {
        self.obs = obs
    }

    public func logs(
        request: Holons_V1_LogsRequest,
        context: StreamingResponseCallContext<Holons_V1_LogRecord>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.logs), let ring = obs.logRing else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "logs family is not enabled (OP_OBS)")
            )
        }
        let minLevel = request.minSeverityNumber.rawValue == 0 ? Int(Level.info.rawValue) : request.minSeverityNumber.rawValue
        let cutoff = request.hasSince ? Date().addingTimeInterval(-durationSeconds(request.since)) : nil
        var pendingLive: [LogRecord] = []
        var bufferingLive = request.follow
        let pendingLock = NSLock()
        let entries: [LogRecord]
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
                    context.sendResponse(toProtoLogRecord(entry), promise: nil)
                }
            }
            entries = replay.0
            _ = replay.1
        } else {
            entries = cutoff.map { ring.drainSince($0) } ?? ring.drain()
        }
        var future = context.eventLoop.makeSucceededFuture(())
        for entry in entries where !entry.isPrivate && matchLog(entry, minLevel: minLevel, sessionIds: request.sessionIds, rpcMethods: request.rpcMethods) {
            future = future.flatMap { context.sendResponse(toProtoLogRecord(entry)) }
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
                flush = flush.flatMap { context.sendResponse(toProtoLogRecord(entry)) }
            }
            return flush
        }
        return future.flatMap { followPromise.futureResult }
    }

    public func metrics(
        request: Holons_V1_MetricsRequest,
        context: StreamingResponseCallContext<Holons_V1_Metric>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.metrics), let registry = obs.registry else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "metrics family is not enabled (OP_OBS)")
            )
        }
        var samples = toProtoMetrics(registry, slug: obs.cfg.slug, uid: obs.cfg.instanceUid, startTime: obs.startTime)
        if !request.namePrefixes.isEmpty {
            samples = samples.filter { sample in
                request.namePrefixes.contains { prefix in sample.name.hasPrefix(prefix) }
            }
        }
        var future = context.eventLoop.makeSucceededFuture(())
        for sample in samples {
            future = future.flatMap { context.sendResponse(sample) }
        }
        return future.map { .ok }
    }

    public func events(
        request: Holons_V1_EventsRequest,
        context: StreamingResponseCallContext<Holons_V1_LogRecord>
    ) -> EventLoopFuture<GRPCStatus> {
        guard obs.enabled(.events), let bus = obs.eventBus else {
            return context.eventLoop.makeSucceededFuture(
                GRPCStatus(code: .failedPrecondition, message: "events family is not enabled (OP_OBS)")
            )
        }
        let wanted = Set(request.eventNames)
        let cutoff = request.hasSince ? Date().addingTimeInterval(-durationSeconds(request.since)) : nil
        var pendingLive: [LogRecord] = []
        var bufferingLive = request.follow
        let pendingLock = NSLock()
        let events: [LogRecord]
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
                    context.sendResponse(toProtoLogRecord(event), promise: nil)
                }
            }
            events = replay.0
            _ = replay.1
        } else {
            events = cutoff.map { bus.drainSince($0) } ?? bus.drain()
        }
        var future = context.eventLoop.makeSucceededFuture(())
        for event in events where !event.isPrivate && matchEvent(event, wanted: wanted) {
            future = future.flatMap { context.sendResponse(toProtoLogRecord(event)) }
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
                flush = flush.flatMap { context.sendResponse(toProtoLogRecord(event)) }
            }
            return flush
        }
        return future.flatMap { followPromise.futureResult }
    }
}

public func registerObservabilityService(_ obs: Observability = current()) -> CallHandlerProvider {
    HolonObservabilityService(obs)
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
                request.minSeverityNumber = .info
                request.follow = true
                let call = client.logs(request) { [weak self] proto in
                    guard let self = self else { return }
                    var entry = fromProtoLogRecord(proto)
                    entry.record.chain = enrichForMultilog(entry.record.chain, streamSourceSlug: identity.slug, streamSourceUid: identity.instanceUid)
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
                    var event = fromProtoLogRecord(proto)
                    event.record.chain = enrichForMultilog(event.record.chain, streamSourceSlug: identity.slug, streamSourceUid: identity.instanceUid)
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
            let uid = stringAttribute(event.attributes, key: AttrHolonsInstanceUID)
            let slug = stringAttribute(event.attributes, key: AttrHolonsSlug)
            guard event.eventName == EventInstanceReady, !uid.isEmpty else { return }
            let identity = MemberIdentity(slug: slug.isEmpty ? memberSlug : slug, instanceUid: uid)
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

private func matchLog(_ entry: LogRecord, minLevel: Int, sessionIds: [String], rpcMethods: [String]) -> Bool {
    if entry.record.severityNumber.rawValue < minLevel { return false }
    if !sessionIds.isEmpty && !sessionIds.contains(entry.attribute(AttrHolonsSessionID)) { return false }
    if !rpcMethods.isEmpty && !rpcMethods.contains(entry.attribute(AttrRPCMethod)) { return false }
    return true
}

private func matchEvent(_ event: LogRecord, wanted: Set<String>) -> Bool {
    wanted.isEmpty || wanted.contains(event.record.eventName)
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
                "level": entry.record.severityText,
                "slug": entry.attribute(AttrHolonsSlug),
                "instance_uid": entry.attribute(AttrHolonsInstanceUID),
                "message": entry.bodyString,
            ]
            let sessionId = entry.attribute(AttrHolonsSessionID)
            let rpcMethod = entry.attribute(AttrRPCMethod)
            let caller = entry.attribute(AttrCodeCaller)
            if !sessionId.isEmpty { rec["session_id"] = sessionId }
            if !rpcMethod.isEmpty { rec["rpc_method"] = rpcMethod }
            if !caller.isEmpty { rec["caller"] = caller }
            if !entry.record.chain.isEmpty {
                rec["chain"] = entry.record.chain
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
                "event_name": e.record.eventName,
                "slug": e.attribute(AttrHolonsSlug),
                "instance_uid": e.attribute(AttrHolonsInstanceUID),
            ]
            let sessionId = e.attribute(AttrHolonsSessionID)
            if !sessionId.isEmpty { rec["session_id"] = sessionId }
            if !e.record.chain.isEmpty {
                rec["chain"] = e.record.chain
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
