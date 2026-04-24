//! Rust reference implementation of the cross-SDK observability layer.
//!
//! Mirrors sdk/go-holons/pkg/observability. Same activation model
//! (OP_OBS env + zero cost when disabled), same public surface, same
//! on-disk JSONL shape. See OBSERVABILITY.md.

use std::collections::{BTreeMap, HashMap, HashSet, VecDeque};
use std::fs::{File, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicI64, Ordering};
use std::sync::{Arc, Mutex, OnceLock, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

// ---------------------------------------------------------------------------
// Families & environment
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Family {
    Logs,
    Metrics,
    Events,
    Prom,
    Otel, // reserved v2
}

impl Family {
    pub fn as_str(&self) -> &'static str {
        match self {
            Family::Logs => "logs",
            Family::Metrics => "metrics",
            Family::Events => "events",
            Family::Prom => "prom",
            Family::Otel => "otel",
        }
    }
}

#[derive(Debug)]
pub struct InvalidTokenError {
    pub variable: &'static str,
    pub token: String,
    pub reason: &'static str,
}

impl std::fmt::Display for InvalidTokenError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}: {}: {}", self.variable, self.reason, self.token)
    }
}

impl std::error::Error for InvalidTokenError {}

const V1_TOKENS: &[&str] = &["logs", "metrics", "events", "prom", "all"];

pub fn parse_op_obs(raw: &str) -> HashSet<Family> {
    let mut out = HashSet::new();
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return out;
    }
    for part in trimmed.split(',') {
        let tok = part.trim();
        if tok.is_empty() {
            continue;
        }
        if tok == "otel" || tok == "sessions" {
            continue;
        }
        if !V1_TOKENS.contains(&tok) {
            continue;
        }
        if tok == "all" {
            out.insert(Family::Logs);
            out.insert(Family::Metrics);
            out.insert(Family::Events);
            out.insert(Family::Prom);
        } else if let Some(f) = family_from_str(tok) {
            out.insert(f);
        }
    }
    out
}

fn family_from_str(s: &str) -> Option<Family> {
    Some(match s {
        "logs" => Family::Logs,
        "metrics" => Family::Metrics,
        "events" => Family::Events,
        "prom" => Family::Prom,
        "otel" => Family::Otel,
        _ => return None,
    })
}

pub fn check_env_from(env: &HashMap<String, String>) -> Result<(), InvalidTokenError> {
    let default = String::new();
    let sessions = env.get("OP_SESSIONS").unwrap_or(&default).trim();
    if !sessions.is_empty() {
        return Err(InvalidTokenError {
            variable: "OP_SESSIONS",
            token: sessions.to_string(),
            reason: "sessions are reserved for v2; not implemented in v1",
        });
    }
    let raw = env.get("OP_OBS").unwrap_or(&default).trim();
    if raw.is_empty() {
        return Ok(());
    }
    for part in raw.split(',') {
        let tok = part.trim();
        if tok.is_empty() {
            continue;
        }
        if tok == "otel" {
            return Err(InvalidTokenError {
                variable: "OP_OBS",
                token: tok.to_string(),
                reason: "otel export is reserved for v2; not implemented in v1",
            });
        }
        if tok == "sessions" {
            return Err(InvalidTokenError {
                variable: "OP_OBS",
                token: tok.to_string(),
                reason: "sessions are reserved for v2; not implemented in v1",
            });
        }
        if !V1_TOKENS.contains(&tok) {
            return Err(InvalidTokenError {
                variable: "OP_OBS",
                token: tok.to_string(),
                reason: "unknown OP_OBS token",
            });
        }
    }
    Ok(())
}

pub fn check_env() -> Result<(), InvalidTokenError> {
    let env: HashMap<String, String> = std::env::vars().collect();
    check_env_from(&env)
}

// ---------------------------------------------------------------------------
// Levels & events
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
#[repr(i32)]
pub enum Level {
    Unset = 0,
    Trace = 1,
    Debug = 2,
    Info = 3,
    Warn = 4,
    Error = 5,
    Fatal = 6,
}

impl Level {
    pub fn name(&self) -> &'static str {
        match self {
            Level::Trace => "TRACE",
            Level::Debug => "DEBUG",
            Level::Info => "INFO",
            Level::Warn => "WARN",
            Level::Error => "ERROR",
            Level::Fatal => "FATAL",
            Level::Unset => "UNSPECIFIED",
        }
    }
}

pub fn parse_level(s: &str) -> Level {
    match s.trim().to_ascii_uppercase().as_str() {
        "TRACE" => Level::Trace,
        "DEBUG" => Level::Debug,
        "INFO" => Level::Info,
        "WARN" | "WARNING" => Level::Warn,
        "ERROR" => Level::Error,
        "FATAL" => Level::Fatal,
        _ => Level::Info,
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(i32)]
pub enum EventType {
    Unspecified = 0,
    InstanceSpawned = 1,
    InstanceReady = 2,
    InstanceExited = 3,
    InstanceCrashed = 4,
    SessionStarted = 5,
    SessionEnded = 6,
    HandlerPanic = 7,
    ConfigReloaded = 8,
}

impl EventType {
    pub fn name(&self) -> &'static str {
        match self {
            EventType::InstanceSpawned => "INSTANCE_SPAWNED",
            EventType::InstanceReady => "INSTANCE_READY",
            EventType::InstanceExited => "INSTANCE_EXITED",
            EventType::InstanceCrashed => "INSTANCE_CRASHED",
            EventType::SessionStarted => "SESSION_STARTED",
            EventType::SessionEnded => "SESSION_ENDED",
            EventType::HandlerPanic => "HANDLER_PANIC",
            EventType::ConfigReloaded => "CONFIG_RELOADED",
            EventType::Unspecified => "UNSPECIFIED",
        }
    }
}

// ---------------------------------------------------------------------------
// Chain helpers
// ---------------------------------------------------------------------------

#[derive(Debug, Clone)]
pub struct Hop {
    pub slug: String,
    pub instance_uid: String,
}

pub fn append_direct_child(src: &[Hop], child_slug: &str, child_uid: &str) -> Vec<Hop> {
    let mut out = src.to_vec();
    out.push(Hop {
        slug: child_slug.to_string(),
        instance_uid: child_uid.to_string(),
    });
    out
}

pub fn enrich_for_multilog(wire: &[Hop], source_slug: &str, source_uid: &str) -> Vec<Hop> {
    append_direct_child(wire, source_slug, source_uid)
}

// ---------------------------------------------------------------------------
// Log entry & ring
// ---------------------------------------------------------------------------

#[derive(Debug, Clone)]
pub struct LogEntry {
    pub timestamp: SystemTime,
    pub level: Level,
    pub slug: String,
    pub instance_uid: String,
    pub session_id: String,
    pub rpc_method: String,
    pub message: String,
    pub fields: BTreeMap<String, String>,
    pub caller: String,
    pub chain: Vec<Hop>,
}

pub struct LogRing {
    capacity: usize,
    inner: Mutex<VecDeque<LogEntry>>,
    subs: Mutex<Vec<Box<dyn Fn(&LogEntry) + Send + Sync>>>,
}

impl LogRing {
    pub fn new(capacity: usize) -> Self {
        let cap = capacity.max(1);
        Self {
            capacity: cap,
            inner: Mutex::new(VecDeque::with_capacity(cap)),
            subs: Mutex::new(Vec::new()),
        }
    }

    pub fn push(&self, e: LogEntry) {
        let cap = self.capacity;
        let snapshot: Vec<Box<dyn Fn(&LogEntry) + Send + Sync>> = {
            let mut subs_guard = self.subs.lock().unwrap();
            // Copy the raw function pointer references by cloning is not
            // possible for Box<dyn Fn>; we fire under the lock instead.
            {
                let mut buf = self.inner.lock().unwrap();
                if buf.len() == cap {
                    buf.pop_front();
                }
                buf.push_back(e.clone());
            }
            for fn_ in subs_guard.iter() {
                fn_(&e);
            }
            std::mem::take(&mut *subs_guard)
        };
        // Restore subs to let future emits use them.
        let mut subs_guard = self.subs.lock().unwrap();
        subs_guard.extend(snapshot);
    }

    pub fn drain(&self) -> Vec<LogEntry> {
        let buf = self.inner.lock().unwrap();
        buf.iter().cloned().collect()
    }

    pub fn drain_since(&self, cutoff: SystemTime) -> Vec<LogEntry> {
        let buf = self.inner.lock().unwrap();
        buf.iter()
            .filter(|e| e.timestamp >= cutoff)
            .cloned()
            .collect()
    }

    pub fn subscribe<F>(&self, f: F)
    where
        F: Fn(&LogEntry) + Send + Sync + 'static,
    {
        self.subs.lock().unwrap().push(Box::new(f));
    }

    pub fn len(&self) -> usize {
        self.inner.lock().unwrap().len()
    }

    pub fn is_empty(&self) -> bool {
        self.inner.lock().unwrap().is_empty()
    }
}

// ---------------------------------------------------------------------------
// Event bus
// ---------------------------------------------------------------------------

#[derive(Debug, Clone)]
pub struct Event {
    pub timestamp: SystemTime,
    pub event_type: EventType,
    pub slug: String,
    pub instance_uid: String,
    pub session_id: String,
    pub payload: BTreeMap<String, String>,
    pub chain: Vec<Hop>,
}

pub struct EventBus {
    capacity: usize,
    inner: Mutex<VecDeque<Event>>,
    subs: Mutex<Vec<Box<dyn Fn(&Event) + Send + Sync>>>,
    closed: Mutex<bool>,
}

impl EventBus {
    pub fn new(capacity: usize) -> Self {
        let cap = capacity.max(1);
        Self {
            capacity: cap,
            inner: Mutex::new(VecDeque::with_capacity(cap)),
            subs: Mutex::new(Vec::new()),
            closed: Mutex::new(false),
        }
    }

    pub fn emit(&self, e: Event) {
        if *self.closed.lock().unwrap() {
            return;
        }
        {
            let mut buf = self.inner.lock().unwrap();
            if buf.len() == self.capacity {
                buf.pop_front();
            }
            buf.push_back(e.clone());
        }
        let subs = self.subs.lock().unwrap();
        for fn_ in subs.iter() {
            fn_(&e);
        }
    }

    pub fn drain(&self) -> Vec<Event> {
        let buf = self.inner.lock().unwrap();
        buf.iter().cloned().collect()
    }

    pub fn drain_since(&self, cutoff: SystemTime) -> Vec<Event> {
        let buf = self.inner.lock().unwrap();
        buf.iter()
            .filter(|e| e.timestamp >= cutoff)
            .cloned()
            .collect()
    }

    pub fn subscribe<F>(&self, f: F)
    where
        F: Fn(&Event) + Send + Sync + 'static,
    {
        self.subs.lock().unwrap().push(Box::new(f));
    }

    pub fn close(&self) {
        *self.closed.lock().unwrap() = true;
        self.subs.lock().unwrap().clear();
    }
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

pub struct Counter {
    pub name: String,
    pub help: String,
    pub labels: BTreeMap<String, String>,
    value: AtomicI64,
}

impl Counter {
    fn new(name: String, help: String, labels: BTreeMap<String, String>) -> Self {
        Self {
            name,
            help,
            labels,
            value: AtomicI64::new(0),
        }
    }
    pub fn inc(&self) {
        self.value.fetch_add(1, Ordering::Relaxed);
    }
    pub fn add(&self, n: i64) {
        if n >= 0 {
            self.value.fetch_add(n, Ordering::Relaxed);
        }
    }
    pub fn value(&self) -> i64 {
        self.value.load(Ordering::Relaxed)
    }
}

pub struct Gauge {
    pub name: String,
    pub help: String,
    pub labels: BTreeMap<String, String>,
    value: Mutex<f64>,
}

impl Gauge {
    fn new(name: String, help: String, labels: BTreeMap<String, String>) -> Self {
        Self {
            name,
            help,
            labels,
            value: Mutex::new(0.0),
        }
    }
    pub fn set(&self, v: f64) {
        *self.value.lock().unwrap() = v;
    }
    pub fn add(&self, d: f64) {
        *self.value.lock().unwrap() += d;
    }
    pub fn value(&self) -> f64 {
        *self.value.lock().unwrap()
    }
}

pub struct HistogramSnapshot {
    pub bounds: Vec<f64>,
    pub counts: Vec<i64>,
    pub total: i64,
    pub sum: f64,
}

impl HistogramSnapshot {
    pub fn quantile(&self, q: f64) -> f64 {
        if self.total == 0 {
            return f64::NAN;
        }
        let target = self.total as f64 * q;
        for i in 0..self.counts.len() {
            if self.counts[i] as f64 >= target {
                return self.bounds[i];
            }
        }
        f64::INFINITY
    }
}

pub const DEFAULT_BUCKETS: [f64; 19] = [
    50e-6, 100e-6, 250e-6, 500e-6, 1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
    1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
];

pub struct Histogram {
    pub name: String,
    pub help: String,
    pub labels: BTreeMap<String, String>,
    bounds: Vec<f64>,
    inner: Mutex<HistogramInner>,
}

struct HistogramInner {
    counts: Vec<i64>,
    total: i64,
    sum: f64,
}

impl Histogram {
    fn new(
        name: String,
        help: String,
        labels: BTreeMap<String, String>,
        bounds: Option<Vec<f64>>,
    ) -> Self {
        let mut b = bounds.unwrap_or_else(|| DEFAULT_BUCKETS.to_vec());
        b.sort_by(|a, b| a.partial_cmp(b).unwrap_or(std::cmp::Ordering::Equal));
        let len = b.len();
        Self {
            name,
            help,
            labels,
            bounds: b,
            inner: Mutex::new(HistogramInner {
                counts: vec![0; len],
                total: 0,
                sum: 0.0,
            }),
        }
    }
    pub fn observe(&self, v: f64) {
        let mut g = self.inner.lock().unwrap();
        g.total += 1;
        g.sum += v;
        for i in 0..self.bounds.len() {
            if v <= self.bounds[i] {
                g.counts[i] += 1;
            }
        }
    }
    pub fn snapshot(&self) -> HistogramSnapshot {
        let g = self.inner.lock().unwrap();
        HistogramSnapshot {
            bounds: self.bounds.clone(),
            counts: g.counts.clone(),
            total: g.total,
            sum: g.sum,
        }
    }
}

fn metric_key(name: &str, labels: &BTreeMap<String, String>) -> String {
    if labels.is_empty() {
        return name.to_string();
    }
    let mut s = String::from(name);
    for (k, v) in labels {
        s.push('|');
        s.push_str(k);
        s.push('=');
        s.push_str(v);
    }
    s
}

pub struct Registry {
    counters: RwLock<HashMap<String, Arc<Counter>>>,
    gauges: RwLock<HashMap<String, Arc<Gauge>>>,
    histograms: RwLock<HashMap<String, Arc<Histogram>>>,
}

impl Registry {
    pub fn new() -> Self {
        Self {
            counters: RwLock::new(HashMap::new()),
            gauges: RwLock::new(HashMap::new()),
            histograms: RwLock::new(HashMap::new()),
        }
    }

    pub fn counter(
        &self,
        name: &str,
        help: &str,
        labels: BTreeMap<String, String>,
    ) -> Arc<Counter> {
        let key = metric_key(name, &labels);
        {
            let r = self.counters.read().unwrap();
            if let Some(c) = r.get(&key) {
                return c.clone();
            }
        }
        let mut w = self.counters.write().unwrap();
        if let Some(c) = w.get(&key) {
            return c.clone();
        }
        let c = Arc::new(Counter::new(name.to_string(), help.to_string(), labels));
        w.insert(key, c.clone());
        c
    }

    pub fn gauge(&self, name: &str, help: &str, labels: BTreeMap<String, String>) -> Arc<Gauge> {
        let key = metric_key(name, &labels);
        {
            let r = self.gauges.read().unwrap();
            if let Some(g) = r.get(&key) {
                return g.clone();
            }
        }
        let mut w = self.gauges.write().unwrap();
        if let Some(g) = w.get(&key) {
            return g.clone();
        }
        let g = Arc::new(Gauge::new(name.to_string(), help.to_string(), labels));
        w.insert(key, g.clone());
        g
    }

    pub fn histogram(
        &self,
        name: &str,
        help: &str,
        labels: BTreeMap<String, String>,
        bounds: Option<Vec<f64>>,
    ) -> Arc<Histogram> {
        let key = metric_key(name, &labels);
        {
            let r = self.histograms.read().unwrap();
            if let Some(h) = r.get(&key) {
                return h.clone();
            }
        }
        let mut w = self.histograms.write().unwrap();
        if let Some(h) = w.get(&key) {
            return h.clone();
        }
        let h = Arc::new(Histogram::new(
            name.to_string(),
            help.to_string(),
            labels,
            bounds,
        ));
        w.insert(key, h.clone());
        h
    }
}

// ---------------------------------------------------------------------------
// Observability root
// ---------------------------------------------------------------------------

#[derive(Clone, Default)]
pub struct Config {
    pub slug: String,
    pub default_log_level: Option<Level>,
    pub prom_addr: String,
    pub redacted_fields: Vec<String>,
    pub logs_ring_size: usize,
    pub events_ring_size: usize,
    pub run_dir: String,
    pub instance_uid: String,
    pub organism_uid: String,
    pub organism_slug: String,
}

pub struct Observability {
    pub cfg: Config,
    pub families: HashSet<Family>,
    pub log_ring: Option<Arc<LogRing>>,
    pub event_bus: Option<Arc<EventBus>>,
    pub registry: Option<Arc<Registry>>,
    loggers: Mutex<HashMap<String, Arc<Logger>>>,
}

pub struct Logger {
    name: String,
    obs: Arc<Observability>,
    level: Mutex<Level>,
}

impl Logger {
    pub fn name(&self) -> &str {
        &self.name
    }
    pub fn set_level(&self, l: Level) {
        *self.level.lock().unwrap() = l;
    }
    pub fn enabled(&self, l: Level) -> bool {
        l >= *self.level.lock().unwrap()
    }

    fn log(&self, lvl: Level, message: &str, fields: &[(&str, &str)]) {
        if !self.enabled(lvl) {
            return;
        }
        let obs = &self.obs;
        let redact: HashSet<&str> = obs.cfg.redacted_fields.iter().map(|s| s.as_str()).collect();
        let mut out = BTreeMap::new();
        for (k, v) in fields {
            if k.is_empty() {
                continue;
            }
            let value = if redact.contains(*k) { "<redacted>" } else { *v };
            out.insert((*k).to_string(), value.to_string());
        }
        let entry = LogEntry {
            timestamp: SystemTime::now(),
            level: lvl,
            slug: obs.cfg.slug.clone(),
            instance_uid: obs.cfg.instance_uid.clone(),
            session_id: String::new(),
            rpc_method: String::new(),
            message: message.to_string(),
            fields: out,
            caller: String::new(),
            chain: Vec::new(),
        };
        if let Some(ref ring) = obs.log_ring {
            ring.push(entry);
        }
    }

    pub fn trace(&self, m: &str, f: &[(&str, &str)]) {
        self.log(Level::Trace, m, f);
    }
    pub fn debug(&self, m: &str, f: &[(&str, &str)]) {
        self.log(Level::Debug, m, f);
    }
    pub fn info(&self, m: &str, f: &[(&str, &str)]) {
        self.log(Level::Info, m, f);
    }
    pub fn warn(&self, m: &str, f: &[(&str, &str)]) {
        self.log(Level::Warn, m, f);
    }
    pub fn error(&self, m: &str, f: &[(&str, &str)]) {
        self.log(Level::Error, m, f);
    }
    pub fn fatal(&self, m: &str, f: &[(&str, &str)]) {
        self.log(Level::Fatal, m, f);
    }
}

impl Observability {
    pub fn enabled(&self, f: Family) -> bool {
        self.families.contains(&f)
    }

    pub fn is_organism_root(&self) -> bool {
        !self.cfg.organism_uid.is_empty() && self.cfg.organism_uid == self.cfg.instance_uid
    }

    pub fn logger(self: &Arc<Self>, name: &str) -> Arc<Logger> {
        if !self.families.contains(&Family::Logs) {
            return Arc::new(Logger {
                name: name.to_string(),
                obs: self.clone(),
                level: Mutex::new(Level::Fatal),
            });
        }
        let mut m = self.loggers.lock().unwrap();
        if let Some(l) = m.get(name) {
            return l.clone();
        }
        let default = self.cfg.default_log_level.unwrap_or(Level::Info);
        let l = Arc::new(Logger {
            name: name.to_string(),
            obs: self.clone(),
            level: Mutex::new(default),
        });
        m.insert(name.to_string(), l.clone());
        l
    }

    pub fn counter(
        &self,
        name: &str,
        help: &str,
        labels: BTreeMap<String, String>,
    ) -> Option<Arc<Counter>> {
        self.registry
            .as_ref()
            .map(|r| r.counter(name, help, labels))
    }

    pub fn gauge(
        &self,
        name: &str,
        help: &str,
        labels: BTreeMap<String, String>,
    ) -> Option<Arc<Gauge>> {
        self.registry.as_ref().map(|r| r.gauge(name, help, labels))
    }

    pub fn histogram(
        &self,
        name: &str,
        help: &str,
        labels: BTreeMap<String, String>,
        bounds: Option<Vec<f64>>,
    ) -> Option<Arc<Histogram>> {
        self.registry
            .as_ref()
            .map(|r| r.histogram(name, help, labels, bounds))
    }

    pub fn emit(&self, event_type: EventType, payload: BTreeMap<String, String>) {
        let Some(ref bus) = self.event_bus else {
            return;
        };
        let redact: HashSet<&str> = self.cfg.redacted_fields.iter().map(|s| s.as_str()).collect();
        let mut p = BTreeMap::new();
        for (k, v) in payload {
            let value = if redact.contains(k.as_str()) {
                "<redacted>".to_string()
            } else {
                v
            };
            p.insert(k, value);
        }
        bus.emit(Event {
            timestamp: SystemTime::now(),
            event_type,
            slug: self.cfg.slug.clone(),
            instance_uid: self.cfg.instance_uid.clone(),
            session_id: String::new(),
            payload: p,
            chain: Vec::new(),
        });
    }

    pub fn close(&self) {
        if let Some(ref bus) = self.event_bus {
            bus.close();
        }
    }
}

static CURRENT: OnceLock<RwLock<Option<Arc<Observability>>>> = OnceLock::new();

fn slot() -> &'static RwLock<Option<Arc<Observability>>> {
    CURRENT.get_or_init(|| RwLock::new(None))
}

pub fn configure(mut cfg: Config) -> Arc<Observability> {
    let families = parse_op_obs(std::env::var("OP_OBS").as_deref().unwrap_or(""));
    if cfg.slug.is_empty() {
        cfg.slug = std::env::args()
            .next()
            .unwrap_or_default()
            .rsplit('/')
            .next()
            .unwrap_or_default()
            .to_string();
    }
    let log_ring = if families.contains(&Family::Logs) {
        Some(Arc::new(LogRing::new(if cfg.logs_ring_size == 0 {
            1024
        } else {
            cfg.logs_ring_size
        })))
    } else {
        None
    };
    let event_bus = if families.contains(&Family::Events) {
        Some(Arc::new(EventBus::new(if cfg.events_ring_size == 0 {
            256
        } else {
            cfg.events_ring_size
        })))
    } else {
        None
    };
    let registry = if families.contains(&Family::Metrics) {
        Some(Arc::new(Registry::new()))
    } else {
        None
    };
    let obs = Arc::new(Observability {
        cfg,
        families,
        log_ring,
        event_bus,
        registry,
        loggers: Mutex::new(HashMap::new()),
    });
    *slot().write().unwrap() = Some(obs.clone());
    obs
}

pub fn from_env(base: Config) -> Arc<Observability> {
    let mut cfg = base;
    let env = std::env::vars().collect::<HashMap<_, _>>();
    if cfg.instance_uid.is_empty() {
        cfg.instance_uid = env.get("OP_INSTANCE_UID").cloned().unwrap_or_default();
    }
    if cfg.organism_uid.is_empty() {
        cfg.organism_uid = env.get("OP_ORGANISM_UID").cloned().unwrap_or_default();
    }
    if cfg.organism_slug.is_empty() {
        cfg.organism_slug = env.get("OP_ORGANISM_SLUG").cloned().unwrap_or_default();
    }
    if cfg.prom_addr.is_empty() {
        cfg.prom_addr = env.get("OP_PROM_ADDR").cloned().unwrap_or_default();
    }
    if cfg.run_dir.is_empty() {
        cfg.run_dir = env.get("OP_RUN_DIR").cloned().unwrap_or_default();
    }
    configure(cfg)
}

pub fn current() -> Arc<Observability> {
    if let Some(ref o) = *slot().read().unwrap() {
        return o.clone();
    }
    // disabled stub
    Arc::new(Observability {
        cfg: Config::default(),
        families: HashSet::new(),
        log_ring: None,
        event_bus: None,
        registry: None,
        loggers: Mutex::new(HashMap::new()),
    })
}

pub fn reset() {
    let mut guard = slot().write().unwrap();
    if let Some(obs) = guard.take() {
        obs.close();
    }
}

// ---------------------------------------------------------------------------
// Disk writers + meta.json
// ---------------------------------------------------------------------------

pub fn enable_disk_writers(run_dir: &str) {
    let obs = current();
    if run_dir.is_empty() {
        return;
    }
    let _ = std::fs::create_dir_all(run_dir);

    if obs.enabled(Family::Logs) {
        if let Some(ref ring) = obs.log_ring {
            let path: PathBuf = Path::new(run_dir).join("stdout.log");
            ring.subscribe(move |e| {
                let rec = log_entry_to_json(e);
                append_jsonl(&path, &rec);
            });
        }
    }
    if obs.enabled(Family::Events) {
        if let Some(ref bus) = obs.event_bus {
            let path: PathBuf = Path::new(run_dir).join("events.jsonl");
            bus.subscribe(move |e| {
                let rec = event_to_json(e);
                append_jsonl(&path, &rec);
            });
        }
    }
}

fn log_entry_to_json(e: &LogEntry) -> String {
    let mut parts = vec![
        ("kind", json_string("log")),
        ("ts", json_string(&rfc3339(e.timestamp))),
        ("level", json_string(e.level.name())),
        ("slug", json_string(&e.slug)),
        ("instance_uid", json_string(&e.instance_uid)),
        ("message", json_string(&e.message)),
    ];
    if !e.session_id.is_empty() {
        parts.push(("session_id", json_string(&e.session_id)));
    }
    if !e.rpc_method.is_empty() {
        parts.push(("rpc_method", json_string(&e.rpc_method)));
    }
    if !e.fields.is_empty() {
        parts.push(("fields", json_map(&e.fields)));
    }
    if !e.caller.is_empty() {
        parts.push(("caller", json_string(&e.caller)));
    }
    if !e.chain.is_empty() {
        parts.push(("chain", json_chain(&e.chain)));
    }
    object(parts)
}

fn event_to_json(e: &Event) -> String {
    let mut parts = vec![
        ("kind", json_string("event")),
        ("ts", json_string(&rfc3339(e.timestamp))),
        ("type", json_string(e.event_type.name())),
        ("slug", json_string(&e.slug)),
        ("instance_uid", json_string(&e.instance_uid)),
    ];
    if !e.session_id.is_empty() {
        parts.push(("session_id", json_string(&e.session_id)));
    }
    if !e.payload.is_empty() {
        parts.push(("payload", json_map(&e.payload)));
    }
    if !e.chain.is_empty() {
        parts.push(("chain", json_chain(&e.chain)));
    }
    object(parts)
}

fn append_jsonl(path: &PathBuf, body: &str) {
    if let Ok(mut f) = OpenOptions::new().create(true).append(true).open(path) {
        let _ = writeln!(f, "{}", body);
    }
}

// --- Tiny, dependency-free JSON encoder ---

fn object(parts: Vec<(&str, String)>) -> String {
    let mut s = String::from("{");
    let mut first = true;
    for (k, v) in parts {
        if !first {
            s.push(',');
        }
        first = false;
        s.push('"');
        s.push_str(k);
        s.push_str("\":");
        s.push_str(&v);
    }
    s.push('}');
    s
}

fn json_string(s: &str) -> String {
    let mut out = String::from("\"");
    for c in s.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            '"' => out.push_str("\\\""),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            _ if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            _ => out.push(c),
        }
    }
    out.push('"');
    out
}

fn json_map(m: &BTreeMap<String, String>) -> String {
    let mut s = String::from("{");
    let mut first = true;
    for (k, v) in m {
        if !first {
            s.push(',');
        }
        first = false;
        s.push_str(&json_string(k));
        s.push(':');
        s.push_str(&json_string(v));
    }
    s.push('}');
    s
}

fn json_chain(chain: &[Hop]) -> String {
    let mut s = String::from("[");
    let mut first = true;
    for h in chain {
        if !first {
            s.push(',');
        }
        first = false;
        let mut parts = Vec::new();
        parts.push(("slug", json_string(&h.slug)));
        parts.push(("instance_uid", json_string(&h.instance_uid)));
        s.push_str(&object(parts));
    }
    s.push(']');
    s
}

fn rfc3339(t: SystemTime) -> String {
    let d = t.duration_since(UNIX_EPOCH).unwrap_or_default();
    let secs = d.as_secs() as i64;
    let nanos = d.subsec_nanos();
    // Round-trip through chrono-less format: manual assembly.
    let tm = humantime::format_rfc3339_nanos(t);
    let s = tm.to_string();
    if !s.is_empty() {
        return s;
    }
    // Fallback (should never trigger).
    format!("epoch+{}.{:09}", secs, nanos)
}

// Manually implement the RFC3339 formatter to avoid pulling humantime:
// this one is only used if humantime isn't a dependency. We rewrite the
// rfc3339 function to avoid that crate below.

mod humantime {
    use std::time::{SystemTime, UNIX_EPOCH};

    pub fn format_rfc3339_nanos(t: SystemTime) -> String {
        let d = t.duration_since(UNIX_EPOCH).unwrap_or_default();
        let secs = d.as_secs() as i64;
        let nanos = d.subsec_nanos();
        let (year, month, day, hour, minute, second) = epoch_to_ymdhms(secs);
        format!(
            "{:04}-{:02}-{:02}T{:02}:{:02}:{:02}.{:09}Z",
            year, month, day, hour, minute, second, nanos
        )
    }

    fn epoch_to_ymdhms(secs: i64) -> (i32, u32, u32, u32, u32, u32) {
        let days = secs.div_euclid(86_400);
        let rem = secs.rem_euclid(86_400);
        let hour = (rem / 3600) as u32;
        let minute = ((rem % 3600) / 60) as u32;
        let second = (rem % 60) as u32;
        // Date from days (1970-01-01 is day 0).
        let mut z = days + 719_468;
        let era = if z >= 0 { z / 146_097 } else { (z - 146_096) / 146_097 };
        z -= era * 146_097;
        let doe = z as u64;
        let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146_096) / 365;
        let y = (yoe as i64) + era * 400;
        let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
        let mp = (5 * doy + 2) / 153;
        let d = (doy - (153 * mp + 2) / 5 + 1) as u32;
        let m = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
        let year = (y + if m <= 2 { 1 } else { 0 }) as i32;
        (year, m, d, hour, minute, second)
    }
}

// ---------------------------------------------------------------------------
// meta.json
// ---------------------------------------------------------------------------

pub struct MetaJson {
    pub slug: String,
    pub uid: String,
    pub pid: i32,
    pub started_at: SystemTime,
    pub mode: String,
    pub transport: String,
    pub address: String,
    pub metrics_addr: String,
    pub log_path: String,
    pub log_bytes_rotated: i64,
    pub organism_uid: String,
    pub organism_slug: String,
    pub is_default: bool,
}

pub fn write_meta_json(run_dir: &str, m: &MetaJson) -> std::io::Result<()> {
    std::fs::create_dir_all(run_dir)?;
    let mut parts: Vec<(&str, String)> = vec![
        ("slug", json_string(&m.slug)),
        ("uid", json_string(&m.uid)),
        ("pid", m.pid.to_string()),
        ("started_at", json_string(&rfc3339(m.started_at))),
        ("mode", json_string(&m.mode)),
        ("transport", json_string(&m.transport)),
        ("address", json_string(&m.address)),
    ];
    if !m.metrics_addr.is_empty() {
        parts.push(("metrics_addr", json_string(&m.metrics_addr)));
    }
    if !m.log_path.is_empty() {
        parts.push(("log_path", json_string(&m.log_path)));
    }
    if m.log_bytes_rotated > 0 {
        parts.push(("log_bytes_rotated", m.log_bytes_rotated.to_string()));
    }
    if !m.organism_uid.is_empty() {
        parts.push(("organism_uid", json_string(&m.organism_uid)));
    }
    if !m.organism_slug.is_empty() {
        parts.push(("organism_slug", json_string(&m.organism_slug)));
    }
    if m.is_default {
        parts.push(("default", "true".to_string()));
    }
    let body = object(parts);
    let path = Path::new(run_dir).join("meta.json");
    let tmp = path.with_extension("json.tmp");
    {
        let mut f = File::create(&tmp)?;
        f.write_all(body.as_bytes())?;
    }
    std::fs::rename(&tmp, &path)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn fresh() {
        reset();
        std::env::remove_var("OP_OBS");
    }

    #[test]
    fn parse_op_obs_basic() {
        assert_eq!(parse_op_obs("").len(), 0);
        assert_eq!(parse_op_obs("logs"), [Family::Logs].into_iter().collect());
        let all: HashSet<Family> = [Family::Logs, Family::Metrics, Family::Events, Family::Prom]
            .into_iter()
            .collect();
        assert_eq!(parse_op_obs("all"), all);
        assert_eq!(parse_op_obs("all,otel"), all);
        assert_eq!(parse_op_obs("all,sessions"), all);
        assert_eq!(parse_op_obs("unknown").len(), 0);
    }

    #[test]
    fn check_env_rejects_otel_and_unknown() {
        let mut env = HashMap::new();
        env.insert("OP_OBS".to_string(), "logs,otel".to_string());
        assert!(check_env_from(&env).is_err());
        env.insert("OP_OBS".to_string(), "logs,sessions".to_string());
        assert!(check_env_from(&env).is_err());
        env.insert("OP_OBS".to_string(), "".to_string());
        env.insert("OP_SESSIONS".to_string(), "metrics".to_string());
        assert!(check_env_from(&env).is_err());
        env.remove("OP_SESSIONS");
        env.insert("OP_OBS".to_string(), "bogus".to_string());
        assert!(check_env_from(&env).is_err());
        env.insert(
            "OP_OBS".to_string(),
            "logs,metrics,events,prom,all".to_string(),
        );
        assert!(check_env_from(&env).is_ok());
    }

    #[test]
    fn disabled_is_noop() {
        fresh();
        let o = configure(Config {
            slug: "t".to_string(),
            ..Default::default()
        });
        assert!(!o.enabled(Family::Logs));
        assert!(o.counter("t_total", "", BTreeMap::new()).is_none());
    }

    #[test]
    fn registry_counter_and_histogram() {
        let reg = Registry::new();
        let c = reg.counter("t_total", "", BTreeMap::new());
        for _ in 0..1000 {
            c.inc();
        }
        assert_eq!(c.value(), 1000);

        let h = reg.histogram(
            "lat_s",
            "",
            BTreeMap::new(),
            Some(vec![1e-3, 1e-2, 1e-1, 1.0]),
        );
        for _ in 0..900 {
            h.observe(0.5e-3);
        }
        for _ in 0..100 {
            h.observe(0.5);
        }
        let snap = h.snapshot();
        assert_eq!(snap.quantile(0.5), 1e-3);
        assert_eq!(snap.quantile(0.99), 1.0);
    }

    #[test]
    fn log_ring_retention() {
        let r = LogRing::new(3);
        for ch in ['a', 'b', 'c', 'd', 'e'] {
            r.push(LogEntry {
                timestamp: SystemTime::now(),
                level: Level::Info,
                slug: "g".to_string(),
                instance_uid: String::new(),
                session_id: String::new(),
                rpc_method: String::new(),
                message: ch.to_string(),
                fields: BTreeMap::new(),
                caller: String::new(),
                chain: Vec::new(),
            });
        }
        let entries = r.drain();
        assert_eq!(entries.len(), 3);
        assert_eq!(entries[0].message, "c");
        assert_eq!(entries[2].message, "e");
    }

    #[test]
    fn chain_append_and_enrich() {
        let c1 = append_direct_child(&[], "gabriel-greeting-rust", "1c2d");
        assert_eq!(c1.len(), 1);
        assert_eq!(c1[0].slug, "gabriel-greeting-rust");
        let c2 = enrich_for_multilog(&c1, "gabriel-greeting-go", "ea34");
        assert_eq!(c2.len(), 2);
        assert_eq!(c2[1].slug, "gabriel-greeting-go");
        // original unchanged
        assert_eq!(c1.len(), 1);
    }

    #[test]
    fn is_organism_root() {
        fresh();
        let o1 = configure(Config {
            slug: "g".to_string(),
            instance_uid: "x".to_string(),
            organism_uid: "x".to_string(),
            ..Default::default()
        });
        assert!(o1.is_organism_root());
        reset();
        let o2 = configure(Config {
            slug: "g".to_string(),
            instance_uid: "x".to_string(),
            organism_uid: "y".to_string(),
            ..Default::default()
        });
        assert!(!o2.is_organism_root());
    }
}
