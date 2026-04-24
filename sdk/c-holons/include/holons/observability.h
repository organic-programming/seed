/*
 * C reference implementation of the cross-SDK observability layer.
 * Mirrors sdk/go-holons/pkg/observability at a minimal surface.
 *
 * C programs get the core activation model (OP_OBS parsing +
 * zero-cost disabled), a structured log API with six levels,
 * atomic counters, and JSONL disk output. Histograms / prom HTTP /
 * organism relay land as follow-ups once the C SDK core has its
 * transport-level hooks ready.
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

/* Levels mirror the proto LogLevel enum. */
typedef enum {
    HOLON_LEVEL_UNSET = 0,
    HOLON_LEVEL_TRACE = 1,
    HOLON_LEVEL_DEBUG = 2,
    HOLON_LEVEL_INFO  = 3,
    HOLON_LEVEL_WARN  = 4,
    HOLON_LEVEL_ERROR = 5,
    HOLON_LEVEL_FATAL = 6,
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

/* Configuration passed to holon_obs_configure. */
typedef struct {
    const char *slug;
    const char *instance_uid;
    const char *organism_uid;
    const char *organism_slug;
    const char *run_dir;
    holon_level_t default_log_level;
} holon_obs_config_t;

/*
 * Parses OP_OBS (the second argument takes precedence over getenv).
 * Returns a bitmask of HOLON_FAMILY_*. V2-only tokens such as "otel"
 * and "sessions" are dropped in v1; "all" expands to
 * logs|metrics|events|prom. Unknown tokens are dropped by this
 * function; call holon_obs_check_env to fail-fast on them.
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
 * Structured logging. `fields` is an optional NULL-terminated array of
 * alternating key / value C strings (keys ending the array with NULL).
 * When `fields == NULL` or an empty list, no fields are emitted. The
 * call is a no-op when the level is below the configured threshold or
 * logs are disabled.
 */
void holon_obs_log(holon_level_t level, const char *message, const char *const *fields);

/* Emits a lifecycle event. `payload` has the same alternating-kv shape. */
void holon_obs_emit(holon_event_type_t type, const char *const *payload);

/*
 * Atomic counter increment. Name must be a stable C string. Labels are
 * an optional alternating-kv array (same convention as fields). The
 * counter's value is incremented atomically; returns the new value.
 * No-op (returns 0) when metrics are disabled.
 */
int64_t holon_obs_counter_inc(const char *name, const char *const *labels);
int64_t holon_obs_counter_add(const char *name, const char *const *labels, int64_t n);
int64_t holon_obs_counter_value(const char *name, const char *const *labels);

/*
 * Gauge set / add / read. Uses a mutex internally.
 */
void holon_obs_gauge_set(const char *name, const char *const *labels, double v);
void holon_obs_gauge_add(const char *name, const char *const *labels, double d);
double holon_obs_gauge_value(const char *name, const char *const *labels);

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

/*
 * Releases all resources held by the singleton. Further log / emit
 * calls become no-ops until holon_obs_configure is called again.
 */
void holon_obs_reset(void);

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* HOLON_OBSERVABILITY_H */
