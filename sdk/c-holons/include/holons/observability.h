/*
 * C reference implementation of the cross-SDK observability layer.
 * Mirrors sdk/go-holons/pkg/observability at a minimal surface.
 *
 * C programs get the core activation model (OP_OBS parsing +
 * zero-cost disabled), a structured log API with six levels,
 * typed OTLP-shaped attributes, atomic counters, gauges, histograms,
 * and JSONL disk output.
 */

#ifndef HOLON_OBSERVABILITY_H
#define HOLON_OBSERVABILITY_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stddef.h>
#include <stdint.h>

/* Families bit flags. */
#define HOLON_FAMILY_LOGS    0x01u
#define HOLON_FAMILY_METRICS 0x02u
#define HOLON_FAMILY_EVENTS  0x04u
#define HOLON_FAMILY_PROM    0x08u
#define HOLON_FAMILY_OTEL    0x10u /* reserved v2 */

/* Levels mirror the proto SeverityNumber enum values emitted by the SDK. */
typedef enum {
    HOLON_LEVEL_UNSET = 0,
    HOLON_LEVEL_TRACE = 1,
    HOLON_LEVEL_DEBUG = 5,
    HOLON_LEVEL_INFO  = 9,
    HOLON_LEVEL_WARN  = 13,
    HOLON_LEVEL_ERROR = 17,
    HOLON_LEVEL_FATAL = 21,
} holon_level_t;

/* Event types mirror the proto EventType enum. */
typedef enum {
    HOLON_EVENT_UNSPECIFIED      = 0,
    HOLON_EVENT_INSTANCE_SPAWNED = 1,
    HOLON_EVENT_INSTANCE_READY   = 2,
    HOLON_EVENT_INSTANCE_EXITED  = 3,
    HOLON_EVENT_INSTANCE_CRASHED = 4,
    HOLON_EVENT_SESSION_STARTED  = 5,
    HOLON_EVENT_SESSION_ENDED    = 6,
    HOLON_EVENT_HANDLER_PANIC    = 7,
    HOLON_EVENT_CONFIG_RELOADED  = 8,
} holon_event_type_t;

#define HOLON_EVENT_NAME_INSTANCE_SPAWNED "instance.spawned"
#define HOLON_EVENT_NAME_INSTANCE_READY   "instance.ready"
#define HOLON_EVENT_NAME_INSTANCE_EXITED  "instance.exited"
#define HOLON_EVENT_NAME_INSTANCE_CRASHED "instance.crashed"
#define HOLON_EVENT_NAME_SESSION_STARTED  "session.started"
#define HOLON_EVENT_NAME_SESSION_ENDED    "session.ended"
#define HOLON_EVENT_NAME_HANDLER_PANIC    "handler.panic"
#define HOLON_EVENT_NAME_CONFIG_RELOADED  "config.reloaded"

typedef enum {
    HOLONS_ANYVALUE_STRING = 1,
    HOLONS_ANYVALUE_BOOL = 2,
    HOLONS_ANYVALUE_INT = 3,
    HOLONS_ANYVALUE_DOUBLE = 4,
} holons_anyvalue_type_t;

typedef struct {
    holons_anyvalue_type_t type;
    union {
        const char *string_value;
        int bool_value;
        int64_t int_value;
        double double_value;
    } value;
} holons_anyvalue_t;

typedef struct {
    const char *key;
    holons_anyvalue_t value;
} holons_field_t;

typedef struct {
    unsigned char *data;
    size_t len;
} holons_packed_message_t;

static inline holons_anyvalue_t holons_anyvalue_string(const char *value) {
    holons_anyvalue_t out;
    out.type = HOLONS_ANYVALUE_STRING;
    out.value.string_value = value;
    return out;
}

static inline holons_anyvalue_t holons_anyvalue_bool(int value) {
    holons_anyvalue_t out;
    out.type = HOLONS_ANYVALUE_BOOL;
    out.value.bool_value = value != 0;
    return out;
}

static inline holons_anyvalue_t holons_anyvalue_int(int64_t value) {
    holons_anyvalue_t out;
    out.type = HOLONS_ANYVALUE_INT;
    out.value.int_value = value;
    return out;
}

static inline holons_anyvalue_t holons_anyvalue_double(double value) {
    holons_anyvalue_t out;
    out.type = HOLONS_ANYVALUE_DOUBLE;
    out.value.double_value = value;
    return out;
}

static inline holons_field_t holons_field_string(const char *key, const char *value) {
    holons_field_t out;
    out.key = key;
    out.value = holons_anyvalue_string(value);
    return out;
}

static inline holons_field_t holons_field_bool(const char *key, int value) {
    holons_field_t out;
    out.key = key;
    out.value = holons_anyvalue_bool(value);
    return out;
}

static inline holons_field_t holons_field_int(const char *key, int64_t value) {
    holons_field_t out;
    out.key = key;
    out.value = holons_anyvalue_int(value);
    return out;
}

static inline holons_field_t holons_field_double(const char *key, double value) {
    holons_field_t out;
    out.key = key;
    out.value = holons_anyvalue_double(value);
    return out;
}

/* Configuration passed to holon_obs_configure. */
typedef struct {
    const char *slug;
    const char *instance_uid;
    const char *session_id;
    const char *organism_uid;
    const char *organism_slug;
    const char *run_dir;
    holon_level_t default_log_level;
} holon_obs_config_t;

/*
 * Parses an already-validated OP_OBS string into a bitmask of
 * HOLON_FAMILY_*. "all" expands to logs|metrics|events|prom.
 * Returns 0 for empty or invalid input; call holon_obs_check_env before
 * parsing untrusted environment input when the caller needs diagnostics.
 */
uint32_t holon_obs_parse_families(const char *raw);

/*
 * Returns 0 on success, or a negative error code when OP_OBS contains
 * a v2-only or unknown token, or when OP_SESSIONS is set in v1. The
 * offending token is copied into @out_token (caller provides,
 * HOLON_OBS_TOKEN_MAX bytes).
 */
#define HOLON_OBS_TOKEN_MAX 64
int holon_obs_check_env(const char *env_or_null, char out_token[HOLON_OBS_TOKEN_MAX]);

/*
 * Installs the active configuration. Returns 1 if any family was
 * activated (via OP_OBS), 0 otherwise. Safe to call repeatedly; the
 * most recent configuration wins. Strings are copied internally.
 */
int holon_obs_configure(const holon_obs_config_t *cfg);

/* Returns the active family bitmask, or 0 when disabled. */
uint32_t holon_obs_families(void);

/*
 * Copies the currently configured instance run directory into out.
 * The SDK derives this from <OP_RUN_DIR or cfg.run_dir>/<slug>/<uid>.
 * Returns 0 on success, or -EINVAL / -ENOSPC.
 */
int holon_obs_current_run_dir(char *out, size_t out_len);

/* Returns non-zero when the family is enabled. */
int holon_obs_enabled(uint32_t family);

/*
 * Structured logging. Prefer the `_fields` variants for typed attributes.
 * The legacy functions accept an optional NULL-terminated alternating
 * key/value C string array and emit every value as string_value.
 */
void holon_obs_log(holon_level_t level, const char *message, const char *const *fields);
void holon_obs_log_named(const char *logger_name,
                         holon_level_t level,
                         const char *message,
                         const char *const *fields);
void holon_obs_log_fields(holon_level_t level,
                          const char *message,
                          const holons_field_t *fields,
                          size_t field_count);
void holon_obs_log_named_fields(const char *logger_name,
                                holon_level_t level,
                                const char *message,
                                const holons_field_t *fields,
                                size_t field_count);

/* Emits a lifecycle event. Prefer `_fields` for typed payload attributes. */
void holon_obs_emit(holon_event_type_t type, const char *const *payload);
void holon_obs_emit_fields(holon_event_type_t type,
                           const holons_field_t *payload,
                           size_t payload_count);

/*
 * Atomic counter increment. Name must be a stable C string. Labels are
 * an optional alternating-kv array (same convention as fields). The
 * counter's value is incremented atomically; returns the new value.
 * No-op (returns 0) when metrics are disabled.
 */
int64_t holon_obs_counter_inc(const char *name, const char *const *labels);
int64_t holon_obs_counter_add(const char *name, const char *const *labels, int64_t n);
int64_t holon_obs_counter_inc_with_help(const char *name,
                                        const char *help,
                                        const char *const *labels);
int64_t holon_obs_counter_add_with_help(const char *name,
                                        const char *help,
                                        const char *const *labels,
                                        int64_t n);
int64_t holon_obs_counter_value(const char *name, const char *const *labels);

/*
 * Gauge set / add / read. Uses a mutex internally.
 */
void holon_obs_gauge_set(const char *name, const char *const *labels, double v);
void holon_obs_gauge_add(const char *name, const char *const *labels, double d);
double holon_obs_gauge_value(const char *name, const char *const *labels);

void holon_obs_histogram_observe(const char *name,
                                 const char *help,
                                 const char *const *labels,
                                 double v);

int holon_obs_pack_log_record(holon_level_t level,
                              const char *message,
                              const holons_field_t *fields,
                              size_t field_count,
                              unsigned char **out,
                              size_t *out_len);
int holon_obs_pack_event_record(holon_event_type_t type,
                                const holons_field_t *payload,
                                size_t payload_count,
                                unsigned char **out,
                                size_t *out_len);
int holon_obs_snapshot_metrics(holons_packed_message_t **out, size_t *out_count);
void holon_obs_free_packed_messages(holons_packed_message_t *messages, size_t count);

/*
 * Enables disk writers under run_dir/stdout.log and run_dir/events.jsonl.
 * Must be called after holon_obs_configure. Returns 0 on success, or a
 * negative errno on failure to create the directory.
 */
int holon_obs_enable_disk_writers(const char *run_dir);

/*
 * Writes a meta.json to run_dir describing this running instance.
 * Fields optional via NULL / 0. Returns 0 on success.
 */
typedef struct {
    const char *slug;
    const char *uid;
    int         pid;
    int64_t     started_at_epoch; /* unix seconds */
    const char *mode;             /* "persistent" */
    const char *transport;
    const char *address;
    const char *metrics_addr;
    const char *log_path;
    int64_t     log_bytes_rotated;
    const char *organism_uid;
    const char *organism_slug;
    int         is_default;       /* 0 or 1 */
} holon_meta_t;

int holon_obs_write_meta_json(const char *run_dir, const holon_meta_t *meta);

typedef struct {
    const char *key;
    const char *value;
} holon_obs_kv_t;

typedef struct {
    const char *slug;
    const char *instance_uid;
} holon_obs_chain_hop_t;

typedef struct {
    int64_t unix_nanos;
    holon_level_t level;
    const char *slug;
    const char *instance_uid;
    const char *logger_name;
    const char *message;
    const holon_obs_kv_t *fields;
    size_t field_count;
    const holon_obs_chain_hop_t *chain;
    size_t chain_count;
    int private_entry;
} holon_obs_log_snapshot_t;

typedef struct {
    int64_t unix_nanos;
    holon_event_type_t type;
    const char *slug;
    const char *instance_uid;
    const holon_obs_kv_t *payload;
    size_t payload_count;
    const holon_obs_chain_hop_t *chain;
    size_t chain_count;
    int private_entry;
} holon_obs_event_snapshot_t;

typedef int (*holon_obs_log_snapshot_fn)(const holon_obs_log_snapshot_t *entry,
                                         void *user_data);
typedef int (*holon_obs_event_snapshot_fn)(const holon_obs_event_snapshot_t *event,
                                           void *user_data);

const char *holon_obs_private(void);
int holon_obs_replay_logs(int follow,
                          holon_obs_log_snapshot_fn callback,
                          void *user_data);
int holon_obs_replay_events(int follow,
                            holon_obs_event_snapshot_fn callback,
                            void *user_data);

/*
 * Releases all resources held by the singleton. Further log / emit
 * calls become no-ops until holon_obs_configure is called again.
 */
void holon_obs_reset(void);

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* HOLON_OBSERVABILITY_H */
