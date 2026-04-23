// C++ reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.
//
// Header-only for easy dropping into holon projects. Thread-safe via
// std::mutex. No external dependencies beyond C++17.

#pragma once

#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <filesystem>
#include <fstream>
#include <functional>
#include <map>
#include <memory>
#include <mutex>
#include <sstream>
#include <string>
#include <string_view>
#include <unordered_map>
#include <unordered_set>
#include <utility>
#include <vector>

namespace holons::observability {

enum class Family : std::uint32_t {
    Logs = 1 << 0,
    Metrics = 1 << 1,
    Events = 1 << 2,
    Prom = 1 << 3,
    Otel = 1 << 4, // reserved v2
};

inline constexpr std::uint32_t families_to_mask(std::initializer_list<Family> fs) {
    std::uint32_t m = 0;
    for (auto f : fs) m |= static_cast<std::uint32_t>(f);
    return m;
}

inline bool has_family(std::uint32_t mask, Family f) {
    return (mask & static_cast<std::uint32_t>(f)) != 0;
}

enum class Level : int {
    Unset = 0, Trace = 1, Debug = 2, Info = 3, Warn = 4, Error = 5, Fatal = 6,
};

inline const char* level_label(Level l) {
    switch (l) {
        case Level::Trace: return "TRACE";
        case Level::Debug: return "DEBUG";
        case Level::Info: return "INFO";
        case Level::Warn: return "WARN";
        case Level::Error: return "ERROR";
        case Level::Fatal: return "FATAL";
        default: return "UNSPECIFIED";
    }
}

enum class EventType : int {
    Unspecified = 0,
    InstanceSpawned = 1, InstanceReady = 2, InstanceExited = 3, InstanceCrashed = 4,
    SessionStarted = 5, SessionEnded = 6,
    HandlerPanic = 7, ConfigReloaded = 8,
};

inline const char* event_label(EventType t) {
    switch (t) {
        case EventType::InstanceSpawned: return "INSTANCE_SPAWNED";
        case EventType::InstanceReady: return "INSTANCE_READY";
        case EventType::InstanceExited: return "INSTANCE_EXITED";
        case EventType::InstanceCrashed: return "INSTANCE_CRASHED";
        case EventType::SessionStarted: return "SESSION_STARTED";
        case EventType::SessionEnded: return "SESSION_ENDED";
        case EventType::HandlerPanic: return "HANDLER_PANIC";
        case EventType::ConfigReloaded: return "CONFIG_RELOADED";
        default: return "UNSPECIFIED";
    }
}

class InvalidTokenError : public std::runtime_error {
public:
    std::string token;
    InvalidTokenError(std::string tok, const std::string& reason)
        : std::runtime_error("OP_OBS: " + reason + ": " + tok), token(std::move(tok)) {}
};

inline std::uint32_t parse_op_obs(std::string_view raw) {
    std::uint32_t out = 0;
    if (raw.empty()) return out;
    std::string s(raw);
    std::size_t pos = 0;
    while (pos < s.size()) {
        std::size_t comma = s.find(',', pos);
        std::string tok = s.substr(pos, comma == std::string::npos ? std::string::npos : comma - pos);
        pos = (comma == std::string::npos) ? s.size() : comma + 1;
        // trim
        while (!tok.empty() && (tok.front() == ' ' || tok.front() == '\t')) tok.erase(tok.begin());
        while (!tok.empty() && (tok.back() == ' ' || tok.back() == '\t')) tok.pop_back();
        if (tok.empty()) continue;
        if (tok == "otel") continue;
        if (tok == "all") {
            out |= static_cast<std::uint32_t>(Family::Logs) |
                   static_cast<std::uint32_t>(Family::Metrics) |
                   static_cast<std::uint32_t>(Family::Events) |
                   static_cast<std::uint32_t>(Family::Prom);
        } else if (tok == "logs") out |= static_cast<std::uint32_t>(Family::Logs);
        else if (tok == "metrics") out |= static_cast<std::uint32_t>(Family::Metrics);
        else if (tok == "events") out |= static_cast<std::uint32_t>(Family::Events);
        else if (tok == "prom") out |= static_cast<std::uint32_t>(Family::Prom);
    }
    return out;
}

inline void check_env(const std::map<std::string, std::string>& env = {}) {
    std::string raw;
    auto it = env.find("OP_OBS");
    if (it != env.end()) raw = it->second;
    else { const char* v = std::getenv("OP_OBS"); if (v) raw = v; }
    // trim
    while (!raw.empty() && (raw.front() == ' ' || raw.front() == '\t')) raw.erase(raw.begin());
    while (!raw.empty() && (raw.back() == ' ' || raw.back() == '\t')) raw.pop_back();
    if (raw.empty()) return;

    std::size_t pos = 0;
    while (pos < raw.size()) {
        auto comma = raw.find(',', pos);
        std::string tok = raw.substr(pos, comma == std::string::npos ? std::string::npos : comma - pos);
        pos = (comma == std::string::npos) ? raw.size() : comma + 1;
        while (!tok.empty() && (tok.front() == ' ' || tok.front() == '\t')) tok.erase(tok.begin());
        while (!tok.empty() && (tok.back() == ' ' || tok.back() == '\t')) tok.pop_back();
        if (tok.empty()) continue;
        if (tok == "otel")
            throw InvalidTokenError(tok, "otel export is reserved for v2; not implemented in v1");
        if (tok != "logs" && tok != "metrics" && tok != "events" && tok != "prom" && tok != "all")
            throw InvalidTokenError(tok, "unknown OP_OBS token");
    }
}

struct Hop {
    std::string slug;
    std::string instance_uid;
};

inline std::vector<Hop> append_direct_child(const std::vector<Hop>& src,
                                             std::string child_slug, std::string child_uid) {
    std::vector<Hop> out = src;
    out.push_back({std::move(child_slug), std::move(child_uid)});
    return out;
}

inline std::vector<Hop> enrich_for_multilog(const std::vector<Hop>& wire,
                                             std::string src_slug, std::string src_uid) {
    return append_direct_child(wire, std::move(src_slug), std::move(src_uid));
}

struct LogEntry {
    std::chrono::system_clock::time_point timestamp;
    Level level{Level::Info};
    std::string slug;
    std::string instance_uid;
    std::string session_id;
    std::string rpc_method;
    std::string message;
    std::map<std::string, std::string> fields;
    std::string caller;
    std::vector<Hop> chain;
};

struct Event {
    std::chrono::system_clock::time_point timestamp;
    EventType type{EventType::Unspecified};
    std::string slug;
    std::string instance_uid;
    std::string session_id;
    std::map<std::string, std::string> payload;
    std::vector<Hop> chain;
};

class LogRing {
public:
    explicit LogRing(std::size_t capacity = 1024) : capacity_(std::max<std::size_t>(1, capacity)) {}

    void push(const LogEntry& e) {
        std::vector<std::function<void(const LogEntry&)>> copy;
        {
            std::scoped_lock lk(mu_);
            buf_.push_back(e);
            if (buf_.size() > capacity_) buf_.erase(buf_.begin());
            copy = subs_;
        }
        for (auto& fn : copy) try { fn(e); } catch (...) {}
    }

    std::vector<LogEntry> drain() {
        std::scoped_lock lk(mu_);
        return buf_;
    }

    std::vector<LogEntry> drain_since(std::chrono::system_clock::time_point cutoff) {
        std::scoped_lock lk(mu_);
        std::vector<LogEntry> out;
        for (const auto& e : buf_) if (e.timestamp >= cutoff) out.push_back(e);
        return out;
    }

    std::size_t size() const { std::scoped_lock lk(mu_); return buf_.size(); }
    std::size_t capacity() const { return capacity_; }

    void subscribe(std::function<void(const LogEntry&)> fn) {
        std::scoped_lock lk(mu_);
        subs_.push_back(std::move(fn));
    }

private:
    std::size_t capacity_;
    mutable std::mutex mu_;
    std::vector<LogEntry> buf_;
    std::vector<std::function<void(const LogEntry&)>> subs_;
};

class EventBus {
public:
    explicit EventBus(std::size_t capacity = 256) : capacity_(std::max<std::size_t>(1, capacity)) {}

    void emit(const Event& e) {
        std::vector<std::function<void(const Event&)>> copy;
        {
            std::scoped_lock lk(mu_);
            if (closed_) return;
            buf_.push_back(e);
            if (buf_.size() > capacity_) buf_.erase(buf_.begin());
            copy = subs_;
        }
        for (auto& fn : copy) try { fn(e); } catch (...) {}
    }

    std::vector<Event> drain() { std::scoped_lock lk(mu_); return buf_; }

    void subscribe(std::function<void(const Event&)> fn) {
        std::scoped_lock lk(mu_);
        subs_.push_back(std::move(fn));
    }

    void close() {
        std::scoped_lock lk(mu_);
        closed_ = true;
        subs_.clear();
    }

private:
    std::size_t capacity_;
    std::mutex mu_;
    std::vector<Event> buf_;
    std::vector<std::function<void(const Event&)>> subs_;
    bool closed_{false};
};

class Counter {
public:
    std::string name, help;
    std::map<std::string, std::string> labels;
    Counter(std::string n, std::string h, std::map<std::string, std::string> l)
        : name(std::move(n)), help(std::move(h)), labels(std::move(l)) {}
    void inc() { v_.fetch_add(1, std::memory_order_relaxed); }
    void add(std::int64_t n) { if (n >= 0) v_.fetch_add(n, std::memory_order_relaxed); }
    std::int64_t value() const { return v_.load(std::memory_order_relaxed); }
private:
    std::atomic<std::int64_t> v_{0};
};

class Gauge {
public:
    std::string name, help;
    std::map<std::string, std::string> labels;
    Gauge(std::string n, std::string h, std::map<std::string, std::string> l)
        : name(std::move(n)), help(std::move(h)), labels(std::move(l)) {}
    void set(double x) { std::scoped_lock lk(mu_); v_ = x; }
    void add(double d) { std::scoped_lock lk(mu_); v_ += d; }
    double value() const { std::scoped_lock lk(mu_); return v_; }
private:
    mutable std::mutex mu_;
    double v_{0};
};

struct HistogramSnapshot {
    std::vector<double> bounds;
    std::vector<std::int64_t> counts;
    std::int64_t total{0};
    double sum{0};

    double quantile(double q) const {
        if (total == 0) return std::numeric_limits<double>::quiet_NaN();
        double target = total * q;
        for (std::size_t i = 0; i < counts.size(); ++i)
            if (static_cast<double>(counts[i]) >= target) return bounds[i];
        return std::numeric_limits<double>::infinity();
    }
};

inline const std::vector<double>& default_buckets() {
    static const std::vector<double> b = {
        50e-6, 100e-6, 250e-6, 500e-6,
        1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
        1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
    };
    return b;
}

class Histogram {
public:
    std::string name, help;
    std::map<std::string, std::string> labels;

    Histogram(std::string n, std::string h, std::map<std::string, std::string> l,
              std::vector<double> bounds)
        : name(std::move(n)), help(std::move(h)), labels(std::move(l)),
          bounds_(bounds.empty() ? default_buckets() : std::move(bounds)),
          counts_(bounds_.size(), 0) {
        std::sort(bounds_.begin(), bounds_.end());
    }

    void observe(double v) {
        std::scoped_lock lk(mu_);
        ++total_;
        sum_ += v;
        for (std::size_t i = 0; i < bounds_.size(); ++i)
            if (v <= bounds_[i]) ++counts_[i];
    }

    HistogramSnapshot snapshot() const {
        std::scoped_lock lk(mu_);
        return {bounds_, counts_, total_, sum_};
    }

private:
    mutable std::mutex mu_;
    std::vector<double> bounds_;
    std::vector<std::int64_t> counts_;
    std::int64_t total_{0};
    double sum_{0};
};

inline std::string metric_key(const std::string& name,
                               const std::map<std::string, std::string>& labels) {
    if (labels.empty()) return name;
    std::string s = name;
    for (const auto& [k, v] : labels) { s += '|'; s += k; s += '='; s += v; }
    return s;
}

class Registry {
public:
    std::shared_ptr<Counter> counter(const std::string& name, const std::string& help,
                                      const std::map<std::string, std::string>& labels) {
        std::scoped_lock lk(mu_);
        auto k = metric_key(name, labels);
        auto it = counters_.find(k);
        if (it != counters_.end()) return it->second;
        auto c = std::make_shared<Counter>(name, help, labels);
        counters_[k] = c;
        return c;
    }

    std::shared_ptr<Gauge> gauge(const std::string& name, const std::string& help,
                                  const std::map<std::string, std::string>& labels) {
        std::scoped_lock lk(mu_);
        auto k = metric_key(name, labels);
        auto it = gauges_.find(k);
        if (it != gauges_.end()) return it->second;
        auto g = std::make_shared<Gauge>(name, help, labels);
        gauges_[k] = g;
        return g;
    }

    std::shared_ptr<Histogram> histogram(const std::string& name, const std::string& help,
                                          const std::map<std::string, std::string>& labels,
                                          std::vector<double> bounds = {}) {
        std::scoped_lock lk(mu_);
        auto k = metric_key(name, labels);
        auto it = histograms_.find(k);
        if (it != histograms_.end()) return it->second;
        auto h = std::make_shared<Histogram>(name, help, labels, std::move(bounds));
        histograms_[k] = h;
        return h;
    }

private:
    std::mutex mu_;
    std::unordered_map<std::string, std::shared_ptr<Counter>> counters_;
    std::unordered_map<std::string, std::shared_ptr<Gauge>> gauges_;
    std::unordered_map<std::string, std::shared_ptr<Histogram>> histograms_;
};

struct Config {
    std::string slug;
    Level default_log_level{Level::Info};
    std::string prom_addr;
    std::vector<std::string> redacted_fields;
    std::size_t logs_ring_size{1024};
    std::size_t events_ring_size{256};
    std::string run_dir;
    std::string instance_uid;
    std::string organism_uid;
    std::string organism_slug;
};

class Observability;

class Logger {
public:
    std::string name;
    Logger(Observability* obs, std::string n) : obs_(obs), name(std::move(n)) {
        if (obs_) level_.store(static_cast<int>(defaultLevel()));
    }
    void set_level(Level l) { level_.store(static_cast<int>(l)); }
    bool enabled(Level l) const;

    void log(Level l, std::string_view message,
             const std::map<std::string, std::string>& fields = {}) const;

    void trace(std::string_view m, const std::map<std::string, std::string>& f = {}) const { log(Level::Trace, m, f); }
    void debug(std::string_view m, const std::map<std::string, std::string>& f = {}) const { log(Level::Debug, m, f); }
    void info(std::string_view m, const std::map<std::string, std::string>& f = {}) const { log(Level::Info, m, f); }
    void warn(std::string_view m, const std::map<std::string, std::string>& f = {}) const { log(Level::Warn, m, f); }
    void error(std::string_view m, const std::map<std::string, std::string>& f = {}) const { log(Level::Error, m, f); }
    void fatal(std::string_view m, const std::map<std::string, std::string>& f = {}) const { log(Level::Fatal, m, f); }

private:
    Level defaultLevel() const;

    Observability* obs_;
    mutable std::atomic<int> level_{static_cast<int>(Level::Info)};
};

class Observability {
public:
    Config cfg;
    std::uint32_t families{0};
    std::unique_ptr<LogRing> log_ring;
    std::unique_ptr<EventBus> event_bus;
    std::unique_ptr<Registry> registry;

    Observability(Config c, std::uint32_t fam) : cfg(std::move(c)), families(fam) {
        if (has_family(families, Family::Logs))
            log_ring = std::make_unique<LogRing>(cfg.logs_ring_size);
        if (has_family(families, Family::Events))
            event_bus = std::make_unique<EventBus>(cfg.events_ring_size);
        if (has_family(families, Family::Metrics))
            registry = std::make_unique<Registry>();
    }

    bool enabled(Family f) const { return has_family(families, f); }

    bool is_organism_root() const {
        return !cfg.organism_uid.empty() && cfg.organism_uid == cfg.instance_uid;
    }

    Logger& logger(const std::string& name) {
        if (!enabled(Family::Logs)) return disabled_logger();
        std::scoped_lock lk(loggers_mu_);
        auto it = loggers_.find(name);
        if (it != loggers_.end()) return *it->second;
        auto ptr = std::make_unique<Logger>(this, name);
        Logger& ref = *ptr;
        loggers_.emplace(name, std::move(ptr));
        return ref;
    }

    std::shared_ptr<Counter> counter(const std::string& name, const std::string& help = "",
                                      const std::map<std::string, std::string>& labels = {}) {
        return registry ? registry->counter(name, help, labels) : nullptr;
    }
    std::shared_ptr<Gauge> gauge(const std::string& name, const std::string& help = "",
                                  const std::map<std::string, std::string>& labels = {}) {
        return registry ? registry->gauge(name, help, labels) : nullptr;
    }
    std::shared_ptr<Histogram> histogram(const std::string& name, const std::string& help = "",
                                          const std::map<std::string, std::string>& labels = {},
                                          std::vector<double> bounds = {}) {
        return registry ? registry->histogram(name, help, labels, std::move(bounds)) : nullptr;
    }

    void emit(EventType type, const std::map<std::string, std::string>& payload = {}) {
        if (!event_bus) return;
        std::unordered_set<std::string> redact(cfg.redacted_fields.begin(), cfg.redacted_fields.end());
        std::map<std::string, std::string> p;
        for (const auto& [k, v] : payload) {
            p[k] = redact.count(k) ? "<redacted>" : v;
        }
        Event e;
        e.timestamp = std::chrono::system_clock::now();
        e.type = type;
        e.slug = cfg.slug;
        e.instance_uid = cfg.instance_uid;
        e.payload = std::move(p);
        event_bus->emit(e);
    }

    void close() { if (event_bus) event_bus->close(); }

private:
    static Logger& disabled_logger() {
        static Logger l(nullptr, "");
        return l;
    }
    std::mutex loggers_mu_;
    std::unordered_map<std::string, std::unique_ptr<Logger>> loggers_;
};

inline Level Logger::defaultLevel() const {
    return obs_ ? obs_->cfg.default_log_level : Level::Fatal;
}

inline bool Logger::enabled(Level l) const {
    return obs_ != nullptr && static_cast<int>(l) >= level_.load();
}

inline void Logger::log(Level l, std::string_view message,
                         const std::map<std::string, std::string>& fields) const {
    if (!enabled(l)) return;
    std::unordered_set<std::string> redact(obs_->cfg.redacted_fields.begin(), obs_->cfg.redacted_fields.end());
    std::map<std::string, std::string> f;
    for (const auto& [k, v] : fields) {
        if (k.empty()) continue;
        f[k] = redact.count(k) ? std::string{"<redacted>"} : v;
    }
    LogEntry e;
    e.timestamp = std::chrono::system_clock::now();
    e.level = l;
    e.slug = obs_->cfg.slug;
    e.instance_uid = obs_->cfg.instance_uid;
    e.message = std::string(message);
    e.fields = std::move(f);
    if (obs_->log_ring) obs_->log_ring->push(e);
}

// Package-scope singleton.
inline std::mutex& current_mu() { static std::mutex m; return m; }
inline std::unique_ptr<Observability>& current_ptr() {
    static std::unique_ptr<Observability> p; return p;
}

inline Observability& configure(Config cfg) {
    const char* raw = std::getenv("OP_OBS");
    std::uint32_t families = parse_op_obs(raw ? raw : "");
    if (cfg.slug.empty()) cfg.slug = "holon";
    auto obs = std::make_unique<Observability>(std::move(cfg), families);
    std::scoped_lock lk(current_mu());
    current_ptr() = std::move(obs);
    return *current_ptr();
}

inline Observability& from_env(Config base = {}) {
    auto get = [](const char* k) -> std::string { const char* v = std::getenv(k); return v ? std::string(v) : ""; };
    if (base.instance_uid.empty()) base.instance_uid = get("OP_INSTANCE_UID");
    if (base.organism_uid.empty()) base.organism_uid = get("OP_ORGANISM_UID");
    if (base.organism_slug.empty()) base.organism_slug = get("OP_ORGANISM_SLUG");
    if (base.prom_addr.empty()) base.prom_addr = get("OP_PROM_ADDR");
    if (base.run_dir.empty()) base.run_dir = get("OP_RUN_DIR");
    return configure(std::move(base));
}

inline Observability& current() {
    std::scoped_lock lk(current_mu());
    if (current_ptr()) return *current_ptr();
    static Observability disabled(Config{}, 0);
    return disabled;
}

inline void reset() {
    std::scoped_lock lk(current_mu());
    if (current_ptr()) current_ptr()->close();
    current_ptr().reset();
}

// --- Disk writers + meta.json ---

namespace detail {
    inline std::string json_escape(std::string_view s) {
        std::string out;
        out.reserve(s.size() + 2);
        out += '"';
        for (char c : s) {
            switch (c) {
                case '\\': out += "\\\\"; break;
                case '"':  out += "\\\""; break;
                case '\n': out += "\\n";  break;
                case '\r': out += "\\r";  break;
                case '\t': out += "\\t";  break;
                default:
                    if (static_cast<unsigned char>(c) < 0x20) {
                        char tmp[8]; std::snprintf(tmp, sizeof(tmp), "\\u%04x", c);
                        out += tmp;
                    } else out += c;
            }
        }
        out += '"';
        return out;
    }

    inline std::string rfc3339(std::chrono::system_clock::time_point tp) {
        auto t = std::chrono::system_clock::to_time_t(tp);
        auto ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                      tp.time_since_epoch()).count() % 1'000'000'000;
        std::tm gm{}; gmtime_r(&t, &gm);
        char buf[40];
        std::snprintf(buf, sizeof(buf),
                      "%04d-%02d-%02dT%02d:%02d:%02d.%09lldZ",
                      gm.tm_year + 1900, gm.tm_mon + 1, gm.tm_mday,
                      gm.tm_hour, gm.tm_min, gm.tm_sec,
                      static_cast<long long>(ns));
        return buf;
    }
}

inline void enable_disk_writers(const std::string& run_dir) {
    auto& obs = current();
    if (run_dir.empty()) return;
    std::filesystem::create_directories(run_dir);

    if (obs.enabled(Family::Logs) && obs.log_ring) {
        auto fp = std::filesystem::path(run_dir) / "stdout.log";
        obs.log_ring->subscribe([fp](const LogEntry& e) {
            std::ofstream f(fp, std::ios::app);
            if (!f) return;
            f << "{\"kind\":\"log\""
              << ",\"ts\":" << detail::json_escape(detail::rfc3339(e.timestamp))
              << ",\"level\":\"" << level_label(e.level) << "\""
              << ",\"slug\":" << detail::json_escape(e.slug)
              << ",\"instance_uid\":" << detail::json_escape(e.instance_uid)
              << ",\"message\":" << detail::json_escape(e.message);
            if (!e.fields.empty()) {
                f << ",\"fields\":{";
                bool first = true;
                for (const auto& [k, v] : e.fields) {
                    if (!first) f << ','; first = false;
                    f << detail::json_escape(k) << ':' << detail::json_escape(v);
                }
                f << '}';
            }
            f << "}\n";
        });
    }

    if (obs.enabled(Family::Events) && obs.event_bus) {
        auto fp = std::filesystem::path(run_dir) / "events.jsonl";
        obs.event_bus->subscribe([fp](const Event& e) {
            std::ofstream f(fp, std::ios::app);
            if (!f) return;
            f << "{\"kind\":\"event\""
              << ",\"ts\":" << detail::json_escape(detail::rfc3339(e.timestamp))
              << ",\"type\":\"" << event_label(e.type) << "\""
              << ",\"slug\":" << detail::json_escape(e.slug)
              << ",\"instance_uid\":" << detail::json_escape(e.instance_uid);
            if (!e.payload.empty()) {
                f << ",\"payload\":{";
                bool first = true;
                for (const auto& [k, v] : e.payload) {
                    if (!first) f << ','; first = false;
                    f << detail::json_escape(k) << ':' << detail::json_escape(v);
                }
                f << '}';
            }
            f << "}\n";
        });
    }
}

struct MetaJson {
    std::string slug, uid, mode{"persistent"}, transport, address;
    std::string metrics_addr, log_path, organism_uid, organism_slug;
    int pid{0};
    std::chrono::system_clock::time_point started_at{std::chrono::system_clock::now()};
    std::int64_t log_bytes_rotated{0};
    bool is_default{false};
};

inline void write_meta_json(const std::string& run_dir, const MetaJson& m) {
    std::filesystem::create_directories(run_dir);
    auto path = std::filesystem::path(run_dir) / "meta.json";
    auto tmp = path.string() + ".tmp";
    std::ofstream f(tmp);
    f << "{";
    f << "\"slug\":" << detail::json_escape(m.slug);
    f << ",\"uid\":" << detail::json_escape(m.uid);
    f << ",\"pid\":" << m.pid;
    f << ",\"started_at\":" << detail::json_escape(detail::rfc3339(m.started_at));
    f << ",\"mode\":" << detail::json_escape(m.mode);
    f << ",\"transport\":" << detail::json_escape(m.transport);
    f << ",\"address\":" << detail::json_escape(m.address);
    if (!m.metrics_addr.empty()) f << ",\"metrics_addr\":" << detail::json_escape(m.metrics_addr);
    if (!m.log_path.empty())     f << ",\"log_path\":" << detail::json_escape(m.log_path);
    if (m.log_bytes_rotated > 0) f << ",\"log_bytes_rotated\":" << m.log_bytes_rotated;
    if (!m.organism_uid.empty()) f << ",\"organism_uid\":" << detail::json_escape(m.organism_uid);
    if (!m.organism_slug.empty()) f << ",\"organism_slug\":" << detail::json_escape(m.organism_slug);
    if (m.is_default) f << ",\"default\":true";
    f << "}";
    f.close();
    std::filesystem::rename(tmp, path);
}

} // namespace holons::observability
