/*
 * C reference implementation of the cross-SDK observability layer.
 * See sdk/c-holons/include/holons/observability.h.
 */

#include "holons/observability.h"

#include <errno.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>
#include <unistd.h>

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
    char                 *labels; /* serialized k=v,k=v */
    atomic_int_fast64_t   value;
    struct counter_entry *next;
} counter_entry_t;

typedef struct gauge_entry {
    char                *key;
    char                *name;
    char                *labels;
    double               value;
    struct gauge_entry  *next;
} gauge_entry_t;

typedef struct {
    holon_level_t     default_log_level;
    uint32_t          families;
    char             *slug;
    char             *instance_uid;
    char             *organism_uid;
    char             *organism_slug;
    char             *run_dir;

    pthread_mutex_t   lock;
    counter_entry_t  *counters;
    gauge_entry_t    *gauges;

    char             *log_path;    /* <run_dir>/stdout.log when enabled */
    char             *events_path; /* <run_dir>/events.jsonl when enabled */
} holon_obs_t;

static holon_obs_t *g_obs = NULL;
static pthread_mutex_t g_obs_lock = PTHREAD_MUTEX_INITIALIZER;

static void obs_free(holon_obs_t *o) {
    if (!o) return;
    free(o->slug); free(o->instance_uid); free(o->organism_uid);
    free(o->organism_slug); free(o->run_dir);
    free(o->log_path); free(o->events_path);

    counter_entry_t *c = o->counters;
    while (c) { counter_entry_t *n = c->next; free(c->key); free(c->name); free(c->labels); free(c); c = n; }
    gauge_entry_t *g = o->gauges;
    while (g) { gauge_entry_t *n = g->next; free(g->key); free(g->name); free(g->labels); free(g); g = n; }
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
    o->organism_uid = dup_or_null(cfg && cfg->organism_uid ? cfg->organism_uid : getenv("OP_ORGANISM_UID"));
    o->organism_slug = dup_or_null(cfg && cfg->organism_slug ? cfg->organism_slug : getenv("OP_ORGANISM_SLUG"));
    o->run_dir = derive_run_dir(cfg && cfg->run_dir ? cfg->run_dir : getenv("OP_RUN_DIR"),
                                o->slug ? o->slug : "",
                                o->instance_uid ? o->instance_uid : "");

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

static void json_kv_map(const char *const *kv, FILE *f) {
    fputc('{', f);
    int first = 1;
    if (kv) {
        for (size_t i = 0; kv[i]; i += 2) {
            if (!kv[i + 1]) break;
            if (!first) fputc(',', f);
            first = 0;
            json_escape(kv[i], f);
            fputc(':', f);
            json_escape(kv[i + 1], f);
        }
    }
    fputc('}', f);
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

void holon_obs_log(holon_level_t level, const char *message, const char *const *fields) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    int enabled = o && (o->families & HOLON_FAMILY_LOGS) && level >= o->default_log_level;
    const char *log_path = enabled ? o->log_path : NULL;
    const char *slug = o ? o->slug : "";
    const char *uid = o && o->instance_uid ? o->instance_uid : "";
    pthread_mutex_unlock(&g_obs_lock);
    if (!enabled || !log_path) return;

    FILE *f = fopen(log_path, "a");
    if (!f) return;
    char ts[64];
    fmt_rfc3339(ts, sizeof(ts));
    fprintf(f, "{\"kind\":\"log\",\"ts\":\"%s\",\"level\":\"%s\",", ts, level_label(level));
    fputs("\"slug\":", f); json_escape(slug, f);
    fputs(",\"instance_uid\":", f); json_escape(uid, f);
    fputs(",\"message\":", f); json_escape(message ? message : "", f);
    if (fields && fields[0]) {
        fputs(",\"fields\":", f);
        json_kv_map(fields, f);
    }
    fputs("}\n", f);
    fclose(f);
}

void holon_obs_emit(holon_event_type_t type, const char *const *payload) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    int enabled = o && (o->families & HOLON_FAMILY_EVENTS);
    const char *events_path = enabled ? o->events_path : NULL;
    const char *slug = o ? o->slug : "";
    const char *uid = o && o->instance_uid ? o->instance_uid : "";
    pthread_mutex_unlock(&g_obs_lock);
    if (!enabled || !events_path) return;

    FILE *f = fopen(events_path, "a");
    if (!f) return;
    char ts[64];
    fmt_rfc3339(ts, sizeof(ts));
    fprintf(f, "{\"kind\":\"event\",\"ts\":\"%s\",\"type\":\"%s\",", ts, event_label(type));
    fputs("\"slug\":", f); json_escape(slug, f);
    fputs(",\"instance_uid\":", f); json_escape(uid, f);
    if (payload && payload[0]) {
        fputs(",\"payload\":", f);
        json_kv_map(payload, f);
    }
    fputs("}\n", f);
    fclose(f);
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

static counter_entry_t *counter_find_or_add(holon_obs_t *o, const char *name, const char *const *labels) {
    char *key = counter_key(name, labels);
    if (!key) return NULL;
    for (counter_entry_t *c = o->counters; c; c = c->next) {
        if (streq(c->key, key)) { free(key); return c; }
    }
    counter_entry_t *c = (counter_entry_t *)calloc(1, sizeof(*c));
    if (!c) { free(key); return NULL; }
    c->key = key;
    c->name = dup_or_null(name);
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
    if (n < 0) return 0;
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (!o || !(o->families & HOLON_FAMILY_METRICS)) { pthread_mutex_unlock(&g_obs_lock); return 0; }
    counter_entry_t *c = counter_find_or_add(o, name, labels);
    int64_t v = 0;
    if (c) v = atomic_fetch_add(&c->value, n) + n;
    pthread_mutex_unlock(&g_obs_lock);
    return v;
}

int64_t holon_obs_counter_value(const char *name, const char *const *labels) {
    pthread_mutex_lock(&g_obs_lock);
    holon_obs_t *o = g_obs;
    if (!o) { pthread_mutex_unlock(&g_obs_lock); return 0; }
    counter_entry_t *c = counter_find_or_add(o, name, labels);
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
