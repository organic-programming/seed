/*
 * C reference implementation of the cross-SDK observability layer.
 * See sdk/c-holons/include/holons/observability.h.
 */

#include "holons/observability.h"
#include "holons/v1/observability.pb-c.h"

#include <errno.h>
#include <float.h>
#include <math.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>
#include <unistd.h>

#if defined(__GNUC__) || defined(__clang__)
#define HOLONS_WEAK __attribute__((weak))
#else
#define HOLONS_WEAK
#endif

HOLONS_WEAK void holons_cpp_obs_log_from_c(const char *logger_name,
                                           int level,
                                           const char *message,
                                           const char *const *fields) {
    (void)logger_name;
    (void)level;
    (void)message;
    (void)fields;
}

HOLONS_WEAK void holons_cpp_obs_log_fields_from_c(const char *logger_name,
                                                  int level,
                                                  const char *message,
                                                  const holons_field_t *fields,
                                                  size_t field_count) {
    (void)logger_name;
    (void)level;
    (void)message;
    (void)fields;
    (void)field_count;
}

HOLONS_WEAK void holons_cpp_obs_event_from_c(int type,
                                             const char *const *payload) {
    (void)type;
    (void)payload;
}

HOLONS_WEAK void holons_cpp_obs_event_fields_from_c(int type,
                                                    const holons_field_t *payload,
                                                    size_t payload_count) {
    (void)type;
    (void)payload;
    (void)payload_count;
}

HOLONS_WEAK void holons_cpp_obs_counter_add_from_c(const char *name,
                                                   const char *help,
                                                   const char *const *labels,
                                                   int64_t n) {
    (void)name;
    (void)help;
    (void)labels;
    (void)n;
}

HOLONS_WEAK int holons_cpp_obs_replay_logs_from_c(
    int follow,
    holon_obs_log_snapshot_fn callback,
    void *user_data) {
    (void)follow;
    (void)callback;
    (void)user_data;
    return -1;
}

HOLONS_WEAK int holons_cpp_obs_replay_events_from_c(
    int follow,
    holon_obs_event_snapshot_fn callback,
    void *user_data) {
    (void)follow;
    (void)callback;
    (void)user_data;
    return -1;
}

/* -------- helpers -------- */

static char *dup_or_null(const char *s) {
    if (!s) return NULL;
    size_t n = strlen(s) + 1;
    char *c = (char *)malloc(n);
    if (c) memcpy(c, s, n);
    return c;
}

static char *derive_run_dir(const char *root, const char *slug, const char *uid) {
    if (!root || !*root || !slug || !*slug || !uid || !*uid) {
        return dup_or_null(root);
    }
    size_t nroot = strlen(root);
    size_t nslug = strlen(slug);
    size_t nuid = strlen(uid);
    size_t need = nroot + 1 + nslug + 1 + nuid + 1;
    char *out = (char *)malloc(need);
    if (!out) return NULL;
    snprintf(out, need, "%s/%s/%s", root, slug, uid);
    return out;
}

static int mkdir_p(const char *path) {
    if (!path || !*path) return 0;
    char buf[4096];
    size_t len = strlen(path);
    if (len >= sizeof(buf)) return -ENAMETOOLONG;
    memcpy(buf, path, len + 1);

    for (char *p = buf + 1; *p; ++p) {
        if (*p != '/') continue;
        *p = '\0';
        if (mkdir(buf, 0755) != 0 && errno != EEXIST) return -errno;
        *p = '/';
    }
    if (mkdir(buf, 0755) != 0 && errno != EEXIST) return -errno;
    return 0;
}

static const char *level_label(holon_level_t l) {
    switch (l) {
        case HOLON_LEVEL_TRACE: return "TRACE";
        case HOLON_LEVEL_DEBUG: return "DEBUG";
        case HOLON_LEVEL_INFO:  return "INFO";
        case HOLON_LEVEL_WARN:  return "WARN";
        case HOLON_LEVEL_ERROR: return "ERROR";
        case HOLON_LEVEL_FATAL: return "FATAL";
        default:                return "UNSPECIFIED";
    }
}

static const char *event_label(holon_event_type_t t) {
    switch (t) {
        case HOLON_EVENT_INSTANCE_SPAWNED: return "INSTANCE_SPAWNED";
        case HOLON_EVENT_INSTANCE_READY:   return "INSTANCE_READY";
        case HOLON_EVENT_INSTANCE_EXITED:  return "INSTANCE_EXITED";
        case HOLON_EVENT_INSTANCE_CRASHED: return "INSTANCE_CRASHED";
        case HOLON_EVENT_SESSION_STARTED:  return "SESSION_STARTED";
        case HOLON_EVENT_SESSION_ENDED:    return "SESSION_ENDED";
        case HOLON_EVENT_HANDLER_PANIC:    return "HANDLER_PANIC";
        case HOLON_EVENT_CONFIG_RELOADED:  return "CONFIG_RELOADED";
        default:                            return "UNSPECIFIED";
    }
}

static const char *event_name(holon_event_type_t t) {
    switch (t) {
        case HOLON_EVENT_INSTANCE_SPAWNED: return HOLON_EVENT_NAME_INSTANCE_SPAWNED;
        case HOLON_EVENT_INSTANCE_READY:   return HOLON_EVENT_NAME_INSTANCE_READY;
        case HOLON_EVENT_INSTANCE_EXITED:  return HOLON_EVENT_NAME_INSTANCE_EXITED;
        case HOLON_EVENT_INSTANCE_CRASHED: return HOLON_EVENT_NAME_INSTANCE_CRASHED;
        case HOLON_EVENT_SESSION_STARTED:  return HOLON_EVENT_NAME_SESSION_STARTED;
        case HOLON_EVENT_SESSION_ENDED:    return HOLON_EVENT_NAME_SESSION_ENDED;
        case HOLON_EVENT_HANDLER_PANIC:    return HOLON_EVENT_NAME_HANDLER_PANIC;
        case HOLON_EVENT_CONFIG_RELOADED:  return HOLON_EVENT_NAME_CONFIG_RELOADED;
        default:                            return "unspecified";
    }
}

static uint64_t wall_unix_nanos(void) {
    struct timespec ts;
    if (clock_gettime(CLOCK_REALTIME, &ts) != 0) return 0;
    return (uint64_t)ts.tv_sec * 1000000000ULL + (uint64_t)ts.tv_nsec;
}

/* -------- OP_OBS parsing -------- */

static int streq(const char *a, const char *b) { return a && b && strcmp(a, b) == 0; }

static int tok_matches(const char *tok, const char *keyword) {
    return streq(tok, keyword);
}

uint32_t holon_obs_parse_families(const char *raw) {
    if (!raw) return 0;
    uint32_t out = 0;
    char buf[1024];
    strncpy(buf, raw, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';
    char *save = NULL;
    for (char *tok = strtok_r(buf, ",", &save); tok; tok = strtok_r(NULL, ",", &save)) {
        while (*tok == ' ' || *tok == '\t') tok++;
        size_t len = strlen(tok);
        while (len > 0 && (tok[len - 1] == ' ' || tok[len - 1] == '\t')) tok[--len] = '\0';
        if (!*tok) continue;
        if (tok_matches(tok, "otel") || tok_matches(tok, "sessions")) return 0;
        if (tok_matches(tok, "all")) {
            out |= HOLON_FAMILY_LOGS | HOLON_FAMILY_METRICS | HOLON_FAMILY_EVENTS | HOLON_FAMILY_PROM;
        } else if (tok_matches(tok, "logs")) {
            out |= HOLON_FAMILY_LOGS;
        } else if (tok_matches(tok, "metrics")) {
            out |= HOLON_FAMILY_METRICS;
        } else if (tok_matches(tok, "events")) {
            out |= HOLON_FAMILY_EVENTS;
        } else if (tok_matches(tok, "prom")) {
            out |= HOLON_FAMILY_PROM;
        } else {
            return 0;
        }
    }
    return out;
}

int holon_obs_check_env(const char *env_or_null, char out_token[HOLON_OBS_TOKEN_MAX]) {
    if (!env_or_null) {
        const char *sessions = getenv("OP_SESSIONS");
        if (sessions && *sessions) {
            if (out_token) {
                strncpy(out_token, sessions, HOLON_OBS_TOKEN_MAX - 1);
                out_token[HOLON_OBS_TOKEN_MAX - 1] = '\0';
            }
            return -EINVAL;
        }
    }
    const char *raw = env_or_null ? env_or_null : getenv("OP_OBS");
    if (!raw || !*raw) return 0;
    char buf[1024];
    strncpy(buf, raw, sizeof(buf) - 1);
    buf[sizeof(buf) - 1] = '\0';
    char *save = NULL;
    for (char *tok = strtok_r(buf, ",", &save); tok; tok = strtok_r(NULL, ",", &save)) {
        while (*tok == ' ' || *tok == '\t') tok++;
        size_t len = strlen(tok);
        while (len > 0 && (tok[len - 1] == ' ' || tok[len - 1] == '\t')) tok[--len] = '\0';
        if (!*tok) continue;
        if (streq(tok, "otel") || streq(tok, "sessions") ||
            (!streq(tok, "logs") && !streq(tok, "metrics") &&
             !streq(tok, "events") && !streq(tok, "prom") && !streq(tok, "all"))) {
            if (out_token) {
                strncpy(out_token, tok, HOLON_OBS_TOKEN_MAX - 1);
                out_token[HOLON_OBS_TOKEN_MAX - 1] = '\0';
            }
            return -EINVAL;
        }
    }
    return 0;
}

/* -------- Configuration singleton -------- */

typedef struct counter_entry {
    char                 *key;
    char                 *name;
    char                 *help;
    char                 *labels; /* serialized k=v,k=v */
    atomic_int_fast64_t   value;
    struct counter_entry *next;
} counter_entry_t;

typedef struct gauge_entry {
    char                *key;
    char                *name;
    char                *help;
    char                *labels;
    double               value;
    struct gauge_entry  *next;
} gauge_entry_t;

typedef struct histogram_entry {
    char                   *key;
    char                   *name;
    char                   *help;
    char                   *labels;
    double                 *bounds;
    int64_t                *counts;
    size_t                  bucket_count;
    int64_t                 total;
    double                  sum;
    double                  min;
    double                  max;
    struct histogram_entry *next;
} histogram_entry_t;

typedef struct {
    holon_level_t     default_log_level;
    uint32_t          families;
    char             *slug;
    char             *instance_uid;
    char             *session_id;
    char             *organism_uid;
    char             *organism_slug;
    char             *run_dir;
    uint64_t          start_unix_nano;

    pthread_mutex_t   lock;
    counter_entry_t  *counters;
    gauge_entry_t    *gauges;
    histogram_entry_t *histograms;

    char             *log_path;    /* <run_dir>/stdout.log when enabled */
    char             *events_path; /* <run_dir>/events.jsonl when enabled */
} holon_obs_t;

static holon_obs_t *g_obs = NULL;
static pthread_mutex_t g_obs_lock = PTHREAD_MUTEX_INITIALIZER;

static void obs_free(holon_obs_t *o) {
    if (!o) return;
    free(o->slug); free(o->instance_uid); free(o->session_id); free(o->organism_uid);
    free(o->organism_slug); free(o->run_dir);
    free(o->log_path); free(o->events_path);

    counter_entry_t *c = o->counters;
    while (c) { counter_entry_t *n = c->next; free(c->key); free(c->name); free(c->help); free(c->labels); free(c); c = n; }
    gauge_entry_t *g = o->gauges;
    while (g) { gauge_entry_t *n = g->next; free(g->key); free(g->name); free(g->help); free(g->labels); free(g); g = n; }
    histogram_entry_t *h = o->histograms;
    while (h) {
        histogram_entry_t *n = h->next;
        free(h->key); free(h->name); free(h->help); free(h->labels);
        free(h->bounds); free(h->counts); free(h);
        h = n;
    }
    pthread_mutex_destroy(&o->lock);
    free(o);
}

int holon_obs_configure(const holon_obs_config_t *cfg) {
    char token[HOLON_OBS_TOKEN_MAX];
    if (holon_obs_check_env(NULL, token) != 0) return 0;

    const char *raw = getenv("OP_OBS");
    uint32_t families = holon_obs_parse_families(raw ? raw : "");

    holon_obs_t *o = (holon_obs_t *)calloc(1, sizeof(*o));
    if (!o) return 0;
    pthread_mutex_init(&o->lock, NULL);
    o->families = families;
    o->default_log_level = cfg && cfg->default_log_level ? cfg->default_log_level : HOLON_LEVEL_INFO;
    o->slug = dup_or_null(cfg && cfg->slug ? cfg->slug : "");
    o->instance_uid = dup_or_null(cfg && cfg->instance_uid ? cfg->instance_uid : getenv("OP_INSTANCE_UID"));
    o->session_id = dup_or_null(cfg && cfg->session_id ? cfg->session_id : getenv("OP_SESSION_ID"));
    o->organism_uid = dup_or_null(cfg && cfg->organism_uid ? cfg->organism_uid : getenv("OP_ORGANISM_UID"));
    o->organism_slug = dup_or_null(cfg && cfg->organism_slug ? cfg->organism_slug : getenv("OP_ORGANISM_SLUG"));
    o->run_dir = derive_run_dir(cfg && cfg->run_dir ? cfg->run_dir : getenv("OP_RUN_DIR"),
                                o->slug ? o->slug : "",
                                o->instance_uid ? o->instance_uid : "");
    o->start_unix_nano = wall_unix_nanos();

    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *old = g_obs;
    g_obs = o;
    pthread_mutex_unlock(&g_obs_lock);
    obs_free(old);
    return families != 0;
}

uint32_t holon_obs_families(void) {
    pthread_mutex_lock(&g_obs_lock);
    uint32_t f = g_obs ? g_obs->families : 0;
    pthread_mutex_unlock(&g_obs_lock);
    return f;
}

int holon_obs_enabled(uint32_t family) { return (holon_obs_families() & family) != 0; }

int holon_obs_current_run_dir(char *out, size_t out_len) {
    if (!out || out_len == 0) return -EINVAL;
    pthread_mutex_lock(&g_obs_lock);
    const char *run_dir = g_obs && g_obs->run_dir ? g_obs->run_dir : "";
    size_t n = strlen(run_dir);
    if (n + 1 > out_len) {
        pthread_mutex_unlock(&g_obs_lock);
        return -ENOSPC;
    }
    memcpy(out, run_dir, n + 1);
    pthread_mutex_unlock(&g_obs_lock);
    return 0;
}

/* -------- JSON helpers -------- */

static void json_escape(const char *s, FILE *f) {
    fputc('"', f);
    for (const unsigned char *p = (const unsigned char *)s; p && *p; p++) {
        switch (*p) {
            case '\\': fputs("\\\\", f); break;
            case '"':  fputs("\\\"", f); break;
            case '\n': fputs("\\n",  f); break;
            case '\r': fputs("\\r",  f); break;
            case '\t': fputs("\\t",  f); break;
            default:
                if (*p < 0x20) fprintf(f, "\\u%04x", *p);
                else fputc(*p, f);
        }
    }
    fputc('"', f);
}

static void json_any_value(holons_anyvalue_t value, FILE *f) {
    switch (value.type) {
        case HOLONS_ANYVALUE_BOOL:
            fputs(value.value.bool_value ? "true" : "false", f);
            return;
        case HOLONS_ANYVALUE_INT:
            fprintf(f, "%lld", (long long)value.value.int_value);
            return;
        case HOLONS_ANYVALUE_DOUBLE:
            fprintf(f, "%.17g", value.value.double_value);
            return;
        case HOLONS_ANYVALUE_STRING:
        default:
            json_escape(value.value.string_value ? value.value.string_value : "", f);
            return;
    }
}

static void json_field_map(const holons_field_t *fields, size_t field_count, FILE *f) {
    fputc('{', f);
    for (size_t i = 0; i < field_count; ++i) {
        if (fields[i].key == NULL || fields[i].key[0] == '\0') continue;
        if (i > 0) fputc(',', f);
        json_escape(fields[i].key, f);
        fputc(':', f);
        json_any_value(fields[i].value, f);
    }
    fputc('}', f);
}

static size_t legacy_kv_count(const char *const *kv) {
    size_t count = 0;
    if (!kv) return 0;
    for (size_t i = 0; kv[i]; i += 2) {
        if (!kv[i + 1]) break;
        ++count;
    }
    return count;
}

static holons_field_t *legacy_kv_to_fields(const char *const *kv, size_t *out_count) {
    size_t count = legacy_kv_count(kv);
    holons_field_t *fields = NULL;
    if (out_count) *out_count = count;
    if (count == 0) return NULL;
    fields = (holons_field_t *)calloc(count, sizeof(*fields));
    if (!fields) {
        if (out_count) *out_count = 0;
        return NULL;
    }
    for (size_t i = 0, j = 0; j < count; i += 2, ++j) {
        fields[j] = holons_field_string(kv[i], kv[i + 1]);
    }
    return fields;
}

static char *any_value_text(holons_anyvalue_t value, char *buf, size_t buf_len) {
    if (buf_len == 0) return NULL;
    switch (value.type) {
        case HOLONS_ANYVALUE_BOOL:
            snprintf(buf, buf_len, "%s", value.value.bool_value ? "true" : "false");
            return buf;
        case HOLONS_ANYVALUE_INT:
            snprintf(buf, buf_len, "%lld", (long long)value.value.int_value);
            return buf;
        case HOLONS_ANYVALUE_DOUBLE:
            snprintf(buf, buf_len, "%.17g", value.value.double_value);
            return buf;
        case HOLONS_ANYVALUE_STRING:
        default:
            snprintf(buf, buf_len, "%s", value.value.string_value ? value.value.string_value : "");
            return buf;
    }
}

static const char **fields_to_legacy_kv(const holons_field_t *fields, size_t field_count) {
    const char **kv = NULL;
    char *values = NULL;
    size_t value_cap = 1;
    size_t value_used = 0;
    if (field_count == 0) return NULL;
    for (size_t i = 0; i < field_count; ++i) {
        if (fields[i].value.type == HOLONS_ANYVALUE_STRING && fields[i].value.value.string_value) {
            value_cap += strlen(fields[i].value.value.string_value) + 1;
        } else {
            value_cap += 64;
        }
    }
    kv = (const char **)calloc(1, sizeof(*kv) * (field_count * 2 + 1) + value_cap);
    if (!kv) {
        return NULL;
    }
    values = (char *)(kv + field_count * 2 + 1);
    for (size_t i = 0; i < field_count; ++i) {
        char tmp[64];
        const char *text = NULL;
        size_t text_len;
        kv[i * 2] = fields[i].key ? fields[i].key : "";
        text = any_value_text(fields[i].value, tmp, sizeof(tmp));
        text_len = strlen(text) + 1;
        memcpy(values + value_used, text, text_len);
        kv[i * 2 + 1] = values + value_used;
        value_used += text_len;
    }
    kv[field_count * 2] = NULL;
    return kv;
}

/* Returns a malloc'd timestamp in RFC3339 nano, caller frees. */
static void fmt_rfc3339(char *out, size_t outsz) {
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    struct tm gm;
    gmtime_r(&ts.tv_sec, &gm);
    snprintf(out, outsz, "%04d-%02d-%02dT%02d:%02d:%02d.%09ldZ",
             gm.tm_year + 1900, gm.tm_mon + 1, gm.tm_mday,
             gm.tm_hour, gm.tm_min, gm.tm_sec, ts.tv_nsec);
}

/* -------- Logging + events -------- */

void holon_obs_log_named_fields(const char *logger_name,
                                holon_level_t level,
                                const char *message,
                                const holons_field_t *fields,
                                size_t field_count) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    int enabled = o && (o->families & HOLON_FAMILY_LOGS) && level >= o->default_log_level;
    const char *log_path = enabled ? o->log_path : NULL;
    const char *slug = o ? o->slug : "";
    const char *uid = o && o->instance_uid ? o->instance_uid : "";
    pthread_mutex_unlock(&g_obs_lock);
    if (!enabled) return;

    holons_cpp_obs_log_fields_from_c(logger_name, (int)level, message, fields, field_count);
    {
        const char **legacy = fields_to_legacy_kv(fields, field_count);
        holons_cpp_obs_log_from_c(logger_name, (int)level, message, legacy);
        free(legacy);
    }

    if (!log_path) return;

    FILE *f = fopen(log_path, "a");
    if (!f) return;
    char ts[64];
    fmt_rfc3339(ts, sizeof(ts));
    fprintf(f, "{\"kind\":\"log\",\"ts\":\"%s\",\"level\":\"%s\",", ts, level_label(level));
    fputs("\"slug\":", f); json_escape(slug, f);
    fputs(",\"instance_uid\":", f); json_escape(uid, f);
    fputs(",\"message\":", f); json_escape(message ? message : "", f);
    if (fields && field_count > 0) {
        fputs(",\"fields\":", f);
        json_field_map(fields, field_count, f);
    }
    fputs("}\n", f);
    fclose(f);
}

void holon_obs_log_fields(holon_level_t level,
                          const char *message,
                          const holons_field_t *fields,
                          size_t field_count) {
    holon_obs_log_named_fields(NULL, level, message, fields, field_count);
}

void holon_obs_log_named(const char *logger_name,
                         holon_level_t level,
                         const char *message,
                         const char *const *fields) {
    size_t field_count = 0;
    holons_field_t *typed = legacy_kv_to_fields(fields, &field_count);
    holon_obs_log_named_fields(logger_name, level, message, typed, field_count);
    free(typed);
}

void holon_obs_log(holon_level_t level, const char *message, const char *const *fields) {
    holon_obs_log_named(NULL, level, message, fields);
}

void holon_obs_emit_fields(holon_event_type_t type,
                           const holons_field_t *payload,
                           size_t payload_count) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    int enabled = o && (o->families & HOLON_FAMILY_EVENTS);
    const char *events_path = enabled ? o->events_path : NULL;
    const char *slug = o ? o->slug : "";
    const char *uid = o && o->instance_uid ? o->instance_uid : "";
    pthread_mutex_unlock(&g_obs_lock);
    if (!enabled) return;

    holons_cpp_obs_event_fields_from_c((int)type, payload, payload_count);
    {
        const char **legacy = fields_to_legacy_kv(payload, payload_count);
        holons_cpp_obs_event_from_c((int)type, legacy);
        free(legacy);
    }

    if (!events_path) return;

    FILE *f = fopen(events_path, "a");
    if (!f) return;
    char ts[64];
    fmt_rfc3339(ts, sizeof(ts));
    fprintf(f, "{\"kind\":\"event\",\"ts\":\"%s\",\"event_name\":\"%s\",\"type\":\"%s\",", ts, event_name(type), event_label(type));
    fputs("\"slug\":", f); json_escape(slug, f);
    fputs(",\"instance_uid\":", f); json_escape(uid, f);
    if (payload && payload_count > 0) {
        fputs(",\"payload\":", f);
        json_field_map(payload, payload_count, f);
    }
    fputs("}\n", f);
    fclose(f);
}

void holon_obs_emit(holon_event_type_t type, const char *const *payload) {
    size_t payload_count = 0;
    holons_field_t *typed = legacy_kv_to_fields(payload, &payload_count);
    holon_obs_emit_fields(type, typed, payload_count);
    free(typed);
}

const char *holon_obs_private(void) {
    return "__holons_private";
}

int holon_obs_replay_logs(int follow,
                          holon_obs_log_snapshot_fn callback,
                          void *user_data) {
    if (callback == NULL) return -EINVAL;
    return holons_cpp_obs_replay_logs_from_c(follow != 0, callback, user_data);
}

int holon_obs_replay_events(int follow,
                            holon_obs_event_snapshot_fn callback,
                            void *user_data) {
    if (callback == NULL) return -EINVAL;
    return holons_cpp_obs_replay_events_from_c(follow != 0, callback, user_data);
}

/* -------- Metrics -------- */

static char *labels_serialize(const char *const *kv) {
    if (!kv || !kv[0]) return NULL;
    /* Approximate buffer: sum of lengths + separators. */
    size_t cap = 64;
    char *buf = (char *)malloc(cap);
    if (!buf) return NULL;
    buf[0] = '\0';
    size_t len = 0;
    for (size_t i = 0; kv[i]; i += 2) {
        if (!kv[i + 1]) break;
        size_t nk = strlen(kv[i]);
        size_t nv = strlen(kv[i + 1]);
        size_t need = len + nk + nv + 3; /* '|' 'k' '=' 'v' or ',' */
        if (need + 1 > cap) {
            while (cap < need + 1) cap *= 2;
            char *grown = (char *)realloc(buf, cap);
            if (!grown) { free(buf); return NULL; }
            buf = grown;
        }
        if (len > 0) buf[len++] = ',';
        memcpy(buf + len, kv[i], nk); len += nk;
        buf[len++] = '=';
        memcpy(buf + len, kv[i + 1], nv); len += nv;
        buf[len] = '\0';
    }
    return buf;
}

static char *counter_key(const char *name, const char *const *labels) {
    char *lab = labels_serialize(labels);
    size_t nname = strlen(name);
    size_t nlab = lab ? strlen(lab) : 0;
    char *k = (char *)malloc(nname + 1 + nlab + 1);
    if (!k) { free(lab); return NULL; }
    memcpy(k, name, nname);
    k[nname] = '|';
    if (lab) memcpy(k + nname + 1, lab, nlab + 1); else k[nname + 1] = '\0';
    free(lab);
    return k;
}

static counter_entry_t *counter_find_or_add(holon_obs_t *o,
                                            const char *name,
                                            const char *help,
                                            const char *const *labels) {
    char *key = counter_key(name, labels);
    if (!key) return NULL;
    for (counter_entry_t *c = o->counters; c; c = c->next) {
        if (streq(c->key, key)) { free(key); return c; }
    }
    counter_entry_t *c = (counter_entry_t *)calloc(1, sizeof(*c));
    if (!c) { free(key); return NULL; }
    c->key = key;
    c->name = dup_or_null(name);
    c->help = dup_or_null(help ? help : "");
    c->labels = labels_serialize(labels);
    atomic_init(&c->value, 0);
    c->next = o->counters;
    o->counters = c;
    return c;
}

int64_t holon_obs_counter_inc(const char *name, const char *const *labels) {
    return holon_obs_counter_add(name, labels, 1);
}

int64_t holon_obs_counter_add(const char *name, const char *const *labels, int64_t n) {
    return holon_obs_counter_add_with_help(name, "", labels, n);
}

int64_t holon_obs_counter_inc_with_help(const char *name,
                                        const char *help,
                                        const char *const *labels) {
    return holon_obs_counter_add_with_help(name, help, labels, 1);
}

int64_t holon_obs_counter_add_with_help(const char *name,
                                        const char *help,
                                        const char *const *labels,
                                        int64_t n) {
    if (n < 0) return 0;
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (!o || !(o->families & HOLON_FAMILY_METRICS)) { pthread_mutex_unlock(&g_obs_lock); return 0; }
    counter_entry_t *c = counter_find_or_add(o, name, help, labels);
    int64_t v = 0;
    if (c) v = atomic_fetch_add(&c->value, n) + n;
    pthread_mutex_unlock(&g_obs_lock);
    holons_cpp_obs_counter_add_from_c(name, help, labels, n);
    return v;
}

int64_t holon_obs_counter_value(const char *name, const char *const *labels) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (!o) { pthread_mutex_unlock(&g_obs_lock); return 0; }
    counter_entry_t *c = counter_find_or_add(o, name, "", labels);
    int64_t v = c ? atomic_load(&c->value) : 0;
    pthread_mutex_unlock(&g_obs_lock);
    return v;
}

/* Gauges follow the same allocator pattern with a mutex-protected double. */

static gauge_entry_t *gauge_find_or_add(holon_obs_t *o, const char *name, const char *const *labels) {
    char *key = counter_key(name, labels);
    if (!key) return NULL;
    for (gauge_entry_t *g = o->gauges; g; g = g->next) {
        if (streq(g->key, key)) { free(key); return g; }
    }
    gauge_entry_t *g = (gauge_entry_t *)calloc(1, sizeof(*g));
    if (!g) { free(key); return NULL; }
    g->key = key;
    g->name = dup_or_null(name);
    g->labels = labels_serialize(labels);
    g->next = o->gauges;
    o->gauges = g;
    return g;
}

void holon_obs_gauge_set(const char *name, const char *const *labels, double v) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (o && (o->families & HOLON_FAMILY_METRICS)) {
        gauge_entry_t *g = gauge_find_or_add(o, name, labels);
        if (g) g->value = v;
    }
    pthread_mutex_unlock(&g_obs_lock);
}

void holon_obs_gauge_add(const char *name, const char *const *labels, double d) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (o && (o->families & HOLON_FAMILY_METRICS)) {
        gauge_entry_t *g = gauge_find_or_add(o, name, labels);
        if (g) g->value += d;
    }
    pthread_mutex_unlock(&g_obs_lock);
}

double holon_obs_gauge_value(const char *name, const char *const *labels) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    double v = 0;
    if (o) {
        gauge_entry_t *g = gauge_find_or_add(o, name, labels);
        if (g) v = g->value;
    }
    pthread_mutex_unlock(&g_obs_lock);
    return v;
}

static const double default_histogram_bounds[] = {
    50e-6, 100e-6, 250e-6, 500e-6,
    1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
    1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
};

static histogram_entry_t *histogram_find_or_add(holon_obs_t *o,
                                                const char *name,
                                                const char *help,
                                                const char *const *labels) {
    char *key = counter_key(name, labels);
    if (!key) return NULL;
    for (histogram_entry_t *h = o->histograms; h; h = h->next) {
        if (streq(h->key, key)) { free(key); return h; }
    }
    histogram_entry_t *h = (histogram_entry_t *)calloc(1, sizeof(*h));
    if (!h) { free(key); return NULL; }
    h->key = key;
    h->name = dup_or_null(name);
    h->help = dup_or_null(help ? help : "");
    h->labels = labels_serialize(labels);
    h->bucket_count = sizeof(default_histogram_bounds) / sizeof(default_histogram_bounds[0]);
    h->bounds = (double *)calloc(h->bucket_count, sizeof(*h->bounds));
    h->counts = (int64_t *)calloc(h->bucket_count, sizeof(*h->counts));
    if (!h->bounds || !h->counts) {
        free(h->key); free(h->name); free(h->help); free(h->labels);
        free(h->bounds); free(h->counts); free(h);
        return NULL;
    }
    memcpy(h->bounds, default_histogram_bounds, sizeof(default_histogram_bounds));
    h->min = DBL_MAX;
    h->max = -DBL_MAX;
    h->next = o->histograms;
    o->histograms = h;
    return h;
}

void holon_obs_histogram_observe(const char *name,
                                 const char *help,
                                 const char *const *labels,
                                 double v) {
    if (!name || !*name || isnan(v)) return;
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (o && (o->families & HOLON_FAMILY_METRICS)) {
        histogram_entry_t *h = histogram_find_or_add(o, name, help, labels);
        if (h) {
            h->total++;
            h->sum += v;
            if (v < h->min) h->min = v;
            if (v > h->max) h->max = v;
            for (size_t i = 0; i < h->bucket_count; ++i) {
                if (v <= h->bounds[i]) h->counts[i]++;
            }
        }
    }
    pthread_mutex_unlock(&g_obs_lock);
}

/* -------- OTLP-shaped protobuf-c builders -------- */

static Holons__V1__AnyValue *proto_any_value(holons_anyvalue_t value) {
    Holons__V1__AnyValue *out = (Holons__V1__AnyValue *)malloc(sizeof(*out));
    if (!out) return NULL;
    holons__v1__any_value__init(out);
    switch (value.type) {
        case HOLONS_ANYVALUE_BOOL:
            out->value_case = HOLONS__V1__ANY_VALUE__VALUE_BOOL_VALUE;
            out->bool_value = value.value.bool_value != 0;
            break;
        case HOLONS_ANYVALUE_INT:
            out->value_case = HOLONS__V1__ANY_VALUE__VALUE_INT_VALUE;
            out->int_value = value.value.int_value;
            break;
        case HOLONS_ANYVALUE_DOUBLE:
            out->value_case = HOLONS__V1__ANY_VALUE__VALUE_DOUBLE_VALUE;
            out->double_value = value.value.double_value;
            break;
        case HOLONS_ANYVALUE_STRING:
        default:
            out->value_case = HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE;
            out->string_value = (char *)(value.value.string_value ? value.value.string_value : "");
            break;
    }
    return out;
}

static Holons__V1__KeyValue *proto_key_value(const char *key, holons_anyvalue_t value) {
    Holons__V1__KeyValue *out = (Holons__V1__KeyValue *)malloc(sizeof(*out));
    if (!out) return NULL;
    holons__v1__key_value__init(out);
    out->key = (char *)(key ? key : "");
    out->value = proto_any_value(value);
    if (!out->value) {
        free(out);
        return NULL;
    }
    return out;
}

static void free_key_values(Holons__V1__KeyValue **attrs, size_t count) {
    if (!attrs) return;
    for (size_t i = 0; i < count; ++i) {
        if (!attrs[i]) continue;
        free(attrs[i]->value);
        free(attrs[i]);
    }
    free(attrs);
}

static size_t resource_attr_count(void) {
    return 5;
}

static int add_resource_attrs(Holons__V1__KeyValue **attrs,
                              size_t *idx,
                              const char *slug,
                              const char *uid,
                              const char *session_id) {
    attrs[(*idx)++] = proto_key_value("holons.slug", holons_anyvalue_string(slug ? slug : ""));
    attrs[(*idx)++] = proto_key_value("service.name", holons_anyvalue_string(slug ? slug : ""));
    attrs[(*idx)++] = proto_key_value("holons.instance_uid", holons_anyvalue_string(uid ? uid : ""));
    attrs[(*idx)++] = proto_key_value("service.instance.id", holons_anyvalue_string(uid ? uid : ""));
    attrs[(*idx)++] = proto_key_value("holons.session_id", holons_anyvalue_string(session_id ? session_id : ""));
    for (size_t i = 0; i < *idx; ++i) {
        if (!attrs[i]) return -1;
    }
    return 0;
}

static Holons__V1__KeyValue **log_attrs_from_fields(const holons_field_t *fields,
                                                    size_t field_count,
                                                    const char *slug,
                                                    const char *uid,
                                                    const char *session_id,
                                                    size_t *out_count) {
    size_t count = resource_attr_count() + field_count;
    Holons__V1__KeyValue **attrs = (Holons__V1__KeyValue **)calloc(count ? count : 1, sizeof(*attrs));
    size_t idx = 0;
    if (!attrs) return NULL;
    if (add_resource_attrs(attrs, &idx, slug, uid, session_id) != 0) {
        free_key_values(attrs, idx);
        return NULL;
    }
    for (size_t i = 0; i < field_count; ++i) {
        if (!fields[i].key || fields[i].key[0] == '\0') continue;
        attrs[idx] = proto_key_value(fields[i].key, fields[i].value);
        if (!attrs[idx]) {
            free_key_values(attrs, idx);
            return NULL;
        }
        ++idx;
    }
    *out_count = idx;
    return attrs;
}

static int pack_message(const ProtobufCMessage *message, unsigned char **out, size_t *out_len) {
    size_t len;
    unsigned char *buf;
    if (!message || !out || !out_len) return -EINVAL;
    *out = NULL;
    *out_len = 0;
    len = protobuf_c_message_get_packed_size(message);
    buf = (unsigned char *)malloc(len ? len : 1);
    if (!buf) return -ENOMEM;
    if (len > 0) {
        protobuf_c_message_pack(message, buf);
    }
    *out = buf;
    *out_len = len;
    return 0;
}

int holon_obs_pack_log_record(holon_level_t level,
                              const char *message,
                              const holons_field_t *fields,
                              size_t field_count,
                              unsigned char **out,
                              size_t *out_len) {
    Holons__V1__LogRecord record = HOLONS__V1__LOG_RECORD__INIT;
    Holons__V1__AnyValue body = HOLONS__V1__ANY_VALUE__INIT;
    Holons__V1__KeyValue **attrs = NULL;
    size_t attr_count = 0;
    char *slug = NULL;
    char *uid = NULL;
    char *session_id = NULL;
    int rc;

    pthread_mutex_lock(&g_obs_lock);
    slug = dup_or_null(g_obs && g_obs->slug ? g_obs->slug : "");
    uid = dup_or_null(g_obs && g_obs->instance_uid ? g_obs->instance_uid : "");
    session_id = dup_or_null(g_obs && g_obs->session_id ? g_obs->session_id : "");
    pthread_mutex_unlock(&g_obs_lock);

    attrs = log_attrs_from_fields(fields, field_count, slug, uid, session_id, &attr_count);
    if (!attrs) {
        free(slug); free(uid); free(session_id);
        return -ENOMEM;
    }
    body.value_case = HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE;
    body.string_value = (char *)(message ? message : "");

    record.time_unix_nano = wall_unix_nanos();
    record.observed_time_unix_nano = record.time_unix_nano;
    record.severity_number = (Holons__V1__SeverityNumber)level;
    record.severity_text = (char *)level_label(level);
    record.body = &body;
    record.attributes = attrs;
    record.n_attributes = attr_count;

    rc = pack_message(&record.base, out, out_len);
    free_key_values(attrs, attr_count);
    free(slug); free(uid); free(session_id);
    return rc;
}

int holon_obs_pack_event_record(holon_event_type_t type,
                                const holons_field_t *payload,
                                size_t payload_count,
                                unsigned char **out,
                                size_t *out_len) {
    Holons__V1__LogRecord record = HOLONS__V1__LOG_RECORD__INIT;
    Holons__V1__AnyValue body = HOLONS__V1__ANY_VALUE__INIT;
    Holons__V1__KeyValue **attrs = NULL;
    size_t attr_count = 0;
    char *slug = NULL;
    char *uid = NULL;
    char *session_id = NULL;
    const char *name = event_name(type);
    int rc;

    pthread_mutex_lock(&g_obs_lock);
    slug = dup_or_null(g_obs && g_obs->slug ? g_obs->slug : "");
    uid = dup_or_null(g_obs && g_obs->instance_uid ? g_obs->instance_uid : "");
    session_id = dup_or_null(g_obs && g_obs->session_id ? g_obs->session_id : "");
    pthread_mutex_unlock(&g_obs_lock);

    attrs = log_attrs_from_fields(payload, payload_count, slug, uid, session_id, &attr_count);
    if (!attrs) {
        free(slug); free(uid); free(session_id);
        return -ENOMEM;
    }
    body.value_case = HOLONS__V1__ANY_VALUE__VALUE_STRING_VALUE;
    body.string_value = (char *)name;

    record.time_unix_nano = wall_unix_nanos();
    record.observed_time_unix_nano = record.time_unix_nano;
    record.severity_number = HOLONS__V1__SEVERITY_NUMBER__SEVERITY_NUMBER_INFO;
    record.severity_text = (char *)"INFO";
    record.body = &body;
    record.attributes = attrs;
    record.n_attributes = attr_count;
    record.event_name = (char *)name;

    rc = pack_message(&record.base, out, out_len);
    free_key_values(attrs, attr_count);
    free(slug); free(uid); free(session_id);
    return rc;
}

static size_t labels_attr_count(const char *labels) {
    size_t count = labels && *labels ? 1 : 0;
    if (!labels) return 0;
    for (const char *p = labels; *p; ++p) {
        if (*p == ',') ++count;
    }
    return count;
}

static Holons__V1__KeyValue **metric_attrs_from_labels(const char *labels,
                                                       const char *slug,
                                                       const char *uid,
                                                       const char *session_id,
                                                       size_t *out_count) {
    size_t count = resource_attr_count() + labels_attr_count(labels);
    Holons__V1__KeyValue **attrs = (Holons__V1__KeyValue **)calloc(count ? count : 1, sizeof(*attrs));
    size_t idx = 0;
    char *copy = NULL;
    char *save = NULL;
    if (!attrs) return NULL;
    if (add_resource_attrs(attrs, &idx, slug, uid, session_id) != 0) {
        free_key_values(attrs, idx);
        return NULL;
    }
    copy = dup_or_null(labels);
    for (char *tok = copy ? strtok_r(copy, ",", &save) : NULL; tok; tok = strtok_r(NULL, ",", &save)) {
        char *eq = strchr(tok, '=');
        if (!eq) continue;
        *eq = '\0';
        attrs[idx] = proto_key_value(tok, holons_anyvalue_string(eq + 1));
        if (!attrs[idx]) {
            free(copy);
            free_key_values(attrs, idx);
            return NULL;
        }
        ++idx;
    }
    free(copy);
    *out_count = idx;
    return attrs;
}

static int pack_counter_metric(const counter_entry_t *c,
                               uint64_t start_ns,
                               uint64_t time_ns,
                               const char *slug,
                               const char *uid,
                               const char *session_id,
                               holons_packed_message_t *out) {
    Holons__V1__Metric metric = HOLONS__V1__METRIC__INIT;
    Holons__V1__Sum sum = HOLONS__V1__SUM__INIT;
    Holons__V1__NumberDataPoint point = HOLONS__V1__NUMBER_DATA_POINT__INIT;
    Holons__V1__NumberDataPoint *points[1] = {&point};
    size_t attr_count = 0;
    int rc;
    point.attributes = metric_attrs_from_labels(c->labels, slug, uid, session_id, &attr_count);
    if (!point.attributes) return -ENOMEM;
    point.n_attributes = attr_count;
    point.start_time_unix_nano = start_ns;
    point.time_unix_nano = time_ns;
    point.value_case = HOLONS__V1__NUMBER_DATA_POINT__VALUE_AS_INT;
    point.as_int = atomic_load(&c->value);
    sum.n_data_points = 1;
    sum.data_points = points;
    sum.aggregation_temporality = HOLONS__V1__AGGREGATION_TEMPORALITY__AGGREGATION_TEMPORALITY_CUMULATIVE;
    sum.is_monotonic = 1;
    metric.name = c->name ? c->name : "";
    metric.description = c->help ? c->help : "";
    metric.data_case = HOLONS__V1__METRIC__DATA_SUM;
    metric.sum = &sum;
    rc = pack_message(&metric.base, &out->data, &out->len);
    free_key_values(point.attributes, point.n_attributes);
    return rc;
}

static int pack_gauge_metric(const gauge_entry_t *g,
                             uint64_t start_ns,
                             uint64_t time_ns,
                             const char *slug,
                             const char *uid,
                             const char *session_id,
                             holons_packed_message_t *out) {
    Holons__V1__Metric metric = HOLONS__V1__METRIC__INIT;
    Holons__V1__Gauge gauge = HOLONS__V1__GAUGE__INIT;
    Holons__V1__NumberDataPoint point = HOLONS__V1__NUMBER_DATA_POINT__INIT;
    Holons__V1__NumberDataPoint *points[1] = {&point};
    size_t attr_count = 0;
    int rc;
    point.attributes = metric_attrs_from_labels(g->labels, slug, uid, session_id, &attr_count);
    if (!point.attributes) return -ENOMEM;
    point.n_attributes = attr_count;
    point.start_time_unix_nano = start_ns;
    point.time_unix_nano = time_ns;
    point.value_case = HOLONS__V1__NUMBER_DATA_POINT__VALUE_AS_DOUBLE;
    point.as_double = g->value;
    gauge.n_data_points = 1;
    gauge.data_points = points;
    metric.name = g->name ? g->name : "";
    metric.description = g->help ? g->help : "";
    metric.data_case = HOLONS__V1__METRIC__DATA_GAUGE;
    metric.gauge = &gauge;
    rc = pack_message(&metric.base, &out->data, &out->len);
    free_key_values(point.attributes, point.n_attributes);
    return rc;
}

static int pack_histogram_metric(const histogram_entry_t *h,
                                 uint64_t start_ns,
                                 uint64_t time_ns,
                                 const char *slug,
                                 const char *uid,
                                 const char *session_id,
                                 holons_packed_message_t *out) {
    Holons__V1__Metric metric = HOLONS__V1__METRIC__INIT;
    Holons__V1__Histogram histogram = HOLONS__V1__HISTOGRAM__INIT;
    Holons__V1__HistogramDataPoint point = HOLONS__V1__HISTOGRAM_DATA_POINT__INIT;
    Holons__V1__HistogramDataPoint *points[1] = {&point};
    uint64_t *bucket_counts = NULL;
    size_t attr_count = 0;
    int64_t prev = 0;
    int rc;
    point.attributes = metric_attrs_from_labels(h->labels, slug, uid, session_id, &attr_count);
    if (!point.attributes) return -ENOMEM;
    point.n_attributes = attr_count;
    bucket_counts = (uint64_t *)calloc(h->bucket_count + 1, sizeof(*bucket_counts));
    if (!bucket_counts) {
        free_key_values(point.attributes, point.n_attributes);
        return -ENOMEM;
    }
    for (size_t i = 0; i < h->bucket_count; ++i) {
        int64_t delta = h->counts[i] - prev;
        bucket_counts[i] = (uint64_t)(delta > 0 ? delta : 0);
        prev = h->counts[i];
    }
    bucket_counts[h->bucket_count] = (uint64_t)(h->total > prev ? h->total - prev : 0);
    point.start_time_unix_nano = start_ns;
    point.time_unix_nano = time_ns;
    point.count = (uint64_t)h->total;
    point.sum = h->sum;
    point.n_bucket_counts = h->bucket_count + 1;
    point.bucket_counts = bucket_counts;
    point.n_explicit_bounds = h->bucket_count;
    point.explicit_bounds = h->bounds;
    point.min = h->total > 0 ? h->min : 0;
    point.max = h->total > 0 ? h->max : 0;
    histogram.n_data_points = 1;
    histogram.data_points = points;
    histogram.aggregation_temporality = HOLONS__V1__AGGREGATION_TEMPORALITY__AGGREGATION_TEMPORALITY_CUMULATIVE;
    metric.name = h->name ? h->name : "";
    metric.description = h->help ? h->help : "";
    metric.data_case = HOLONS__V1__METRIC__DATA_HISTOGRAM;
    metric.histogram = &histogram;
    rc = pack_message(&metric.base, &out->data, &out->len);
    free(bucket_counts);
    free_key_values(point.attributes, point.n_attributes);
    return rc;
}

int holon_obs_snapshot_metrics(holons_packed_message_t **out, size_t *out_count) {
    holons_packed_message_t *messages = NULL;
    size_t count = 0;
    size_t idx = 0;
    uint64_t start_ns = 0;
    uint64_t time_ns = wall_unix_nanos();
    char *slug = NULL;
    char *uid = NULL;
    char *session_id = NULL;
    int rc = 0;

    if (!out || !out_count) return -EINVAL;
    *out = NULL;
    *out_count = 0;
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (!o || !(o->families & HOLON_FAMILY_METRICS)) {
        pthread_mutex_unlock(&g_obs_lock);
        return 0;
    }
    for (counter_entry_t *c = o->counters; c; c = c->next) ++count;
    for (gauge_entry_t *g = o->gauges; g; g = g->next) ++count;
    for (histogram_entry_t *h = o->histograms; h; h = h->next) ++count;
    messages = (holons_packed_message_t *)calloc(count ? count : 1, sizeof(*messages));
    slug = dup_or_null(o->slug ? o->slug : "");
    uid = dup_or_null(o->instance_uid ? o->instance_uid : "");
    session_id = dup_or_null(o->session_id ? o->session_id : "");
    start_ns = o->start_unix_nano;
    if (!messages || !slug || !uid || !session_id) {
        pthread_mutex_unlock(&g_obs_lock);
        free(messages); free(slug); free(uid); free(session_id);
        return -ENOMEM;
    }
    for (counter_entry_t *c = o->counters; c && rc == 0; c = c->next) {
        rc = pack_counter_metric(c, start_ns, time_ns, slug, uid, session_id, &messages[idx]);
        if (rc == 0) ++idx;
    }
    for (gauge_entry_t *g = o->gauges; g && rc == 0; g = g->next) {
        rc = pack_gauge_metric(g, start_ns, time_ns, slug, uid, session_id, &messages[idx]);
        if (rc == 0) ++idx;
    }
    for (histogram_entry_t *h = o->histograms; h && rc == 0; h = h->next) {
        rc = pack_histogram_metric(h, start_ns, time_ns, slug, uid, session_id, &messages[idx]);
        if (rc == 0) ++idx;
    }
    pthread_mutex_unlock(&g_obs_lock);
    free(slug); free(uid); free(session_id);
    if (rc != 0) {
        holon_obs_free_packed_messages(messages, idx);
        return rc;
    }
    *out = messages;
    *out_count = idx;
    return 0;
}

void holon_obs_free_packed_messages(holons_packed_message_t *messages, size_t count) {
    if (!messages) return;
    for (size_t i = 0; i < count; ++i) {
        free(messages[i].data);
    }
    free(messages);
}

/* -------- Disk writers -------- */

int holon_obs_enable_disk_writers(const char *run_dir) {
    if (!run_dir || !*run_dir) return 0;
    int mkerr = mkdir_p(run_dir);
    if (mkerr != 0) return mkerr;
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (!o) { pthread_mutex_unlock(&g_obs_lock); return -EINVAL; }
    free(o->log_path);
    free(o->events_path);
    size_t n = strlen(run_dir);
    o->log_path = (char *)malloc(n + 1 + strlen("stdout.log") + 1);
    o->events_path = (char *)malloc(n + 1 + strlen("events.jsonl") + 1);
    if (o->log_path) sprintf(o->log_path, "%s/stdout.log", run_dir);
    if (o->events_path) sprintf(o->events_path, "%s/events.jsonl", run_dir);
    pthread_mutex_unlock(&g_obs_lock);
    return 0;
}

int holon_obs_write_meta_json(const char *run_dir, const holon_meta_t *meta) {
    if (!run_dir || !meta) return -EINVAL;
    int mkerr = mkdir_p(run_dir);
    if (mkerr != 0) return mkerr;
    char path[4096], tmp[4096];
    snprintf(path, sizeof(path), "%s/meta.json", run_dir);
    snprintf(tmp, sizeof(tmp), "%s.tmp", path);
    FILE *f = fopen(tmp, "w");
    if (!f) return -errno;

    fputs("{", f);
    fputs("\"slug\":", f); json_escape(meta->slug ? meta->slug : "", f);
    fputs(",\"uid\":", f); json_escape(meta->uid ? meta->uid : "", f);
    fprintf(f, ",\"pid\":%d", meta->pid);
    /* started_at as RFC3339 from epoch seconds. */
    if (meta->started_at_epoch > 0) {
        time_t t = (time_t)meta->started_at_epoch;
        struct tm gm;
        gmtime_r(&t, &gm);
        char ts[32];
        strftime(ts, sizeof(ts), "%Y-%m-%dT%H:%M:%SZ", &gm);
        fputs(",\"started_at\":", f); json_escape(ts, f);
    }
    fputs(",\"mode\":", f); json_escape(meta->mode ? meta->mode : "persistent", f);
    fputs(",\"transport\":", f); json_escape(meta->transport ? meta->transport : "", f);
    fputs(",\"address\":", f); json_escape(meta->address ? meta->address : "", f);
    if (meta->metrics_addr && *meta->metrics_addr) {
        fputs(",\"metrics_addr\":", f); json_escape(meta->metrics_addr, f);
    }
    if (meta->log_path && *meta->log_path) {
        fputs(",\"log_path\":", f); json_escape(meta->log_path, f);
    }
    if (meta->log_bytes_rotated > 0) {
        fprintf(f, ",\"log_bytes_rotated\":%lld", (long long)meta->log_bytes_rotated);
    }
    if (meta->organism_uid && *meta->organism_uid) {
        fputs(",\"organism_uid\":", f); json_escape(meta->organism_uid, f);
    }
    if (meta->organism_slug && *meta->organism_slug) {
        fputs(",\"organism_slug\":", f); json_escape(meta->organism_slug, f);
    }
    if (meta->is_default) fputs(",\"default\":true", f);
    fputs("}", f);
    fclose(f);

    if (rename(tmp, path) != 0) return -errno;
    return 0;
}

void holon_obs_reset(void) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *old = g_obs;
    g_obs = NULL;
    pthread_mutex_unlock(&g_obs_lock);
    obs_free(old);
}
