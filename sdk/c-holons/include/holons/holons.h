#ifndef HOLONS_H
#define HOLONS_H

#include <stdbool.h>
#include <signal.h>
#include <stddef.h>
#include <sys/types.h>

#ifdef __cplusplus
extern "C" {
#endif

#define HOLONS_DEFAULT_URI "tcp://:9090"
#define HOLONS_MAX_URI_LEN 512
#define HOLONS_MAX_FIELD_LEN 256
#define HOLONS_MAX_DOC_LEN 512

#define HOLONS_LOCAL 0
#define HOLONS_PROXY 1
#define HOLONS_DELEGATED 2

#define HOLONS_SIBLINGS 0x01
#define HOLONS_CWD 0x02
#define HOLONS_SOURCE 0x04
#define HOLONS_BUILT 0x08
#define HOLONS_INSTALLED 0x10
#define HOLONS_CACHED 0x20
#define HOLONS_ALL 0x3F

#define HOLONS_NO_LIMIT 0
#define HOLONS_NO_TIMEOUT 0

typedef enum {
  HOLONS_SCHEME_INVALID = 0,
  HOLONS_SCHEME_TCP,
  HOLONS_SCHEME_UNIX,
  HOLONS_SCHEME_STDIO,
  HOLONS_SCHEME_WS,
  HOLONS_SCHEME_WSS
} holons_scheme_t;

typedef struct {
  holons_scheme_t scheme;
  char host[128];
  int port;
  char path[256];
} holons_uri_t;

typedef struct {
  char uuid[96];
  char given_name[96];
  char family_name[96];
  char motto[256];
  char composer[128];
  char clade[128];
  char status[64];
  char born[64];
  char lang[64];
} holons_identity_t;

typedef struct {
  char runner[HOLONS_MAX_FIELD_LEN];
  char main[HOLONS_MAX_FIELD_LEN];
} holons_build_t;

typedef struct {
  char binary[HOLONS_MAX_FIELD_LEN];
  char primary[HOLONS_MAX_FIELD_LEN];
} holons_artifacts_t;

typedef struct {
  holons_identity_t identity;
  char lang[64];
  char kind[64];
  holons_build_t build;
  holons_artifacts_t artifacts;
} holons_manifest_t;

typedef struct {
  char slug[HOLONS_MAX_FIELD_LEN];
  char uuid[96];
  char dir[HOLONS_MAX_URI_LEN];
  char relative_path[HOLONS_MAX_URI_LEN];
  char origin[32];
  holons_identity_t identity;
  holons_manifest_t manifest;
  int has_manifest;
} holon_entry_t;

typedef struct {
  int read_fd;
  int write_fd;
  holons_scheme_t scheme;
  int owns_read_fd;
  int owns_write_fd;
} holons_conn_t;

typedef struct {
  holons_uri_t uri;
  int fd;
  int consumed;
  char bound_uri[HOLONS_MAX_URI_LEN];
  char unix_path[256];
} holons_listener_t;

typedef int (*holons_conn_handler_t)(const holons_conn_t *conn, void *ctx);
typedef struct grpc_channel grpc_channel;

typedef struct {
  int timeout_ms;
  const char *transport;
  int start;
  const char *port_file;
} holons_connect_options;

typedef enum {
  HOLONS_FIELD_LABEL_UNSPECIFIED = 0,
  HOLONS_FIELD_LABEL_OPTIONAL = 1,
  HOLONS_FIELD_LABEL_REPEATED = 2,
  HOLONS_FIELD_LABEL_MAP = 3,
  HOLONS_FIELD_LABEL_REQUIRED = 4
} holons_field_label_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  int number;
  char description[HOLONS_MAX_DOC_LEN];
} holons_enum_value_doc_t;

typedef struct holons_field_doc {
  char name[HOLONS_MAX_FIELD_LEN];
  char type[HOLONS_MAX_FIELD_LEN];
  int number;
  char description[HOLONS_MAX_DOC_LEN];
  holons_field_label_t label;
  char map_key_type[HOLONS_MAX_FIELD_LEN];
  char map_value_type[HOLONS_MAX_FIELD_LEN];
  struct holons_field_doc *nested_fields;
  size_t nested_field_count;
  holons_enum_value_doc_t *enum_values;
  size_t enum_value_count;
  int required;
  char example[HOLONS_MAX_DOC_LEN];
} holons_field_doc_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char description[HOLONS_MAX_DOC_LEN];
  char input_type[HOLONS_MAX_FIELD_LEN];
  char output_type[HOLONS_MAX_FIELD_LEN];
  holons_field_doc_t *input_fields;
  size_t input_field_count;
  holons_field_doc_t *output_fields;
  size_t output_field_count;
  int client_streaming;
  int server_streaming;
  char example_input[HOLONS_MAX_DOC_LEN];
} holons_method_doc_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char description[HOLONS_MAX_DOC_LEN];
  holons_method_doc_t *methods;
  size_t method_count;
} holons_service_doc_t;

typedef struct {
  holons_manifest_t manifest;
  holons_service_doc_t *services;
  size_t service_count;
} holons_describe_response_t;

typedef struct {
  int reserved;
} holons_describe_request_t;

typedef struct {
  char service_name[HOLONS_MAX_FIELD_LEN];
  char method_name[HOLONS_MAX_FIELD_LEN];
  const holons_describe_response_t *response;
} holons_describe_registration_t;

typedef struct {
  char *given_name;
  char *family_name;
  char *motto;
  char **aliases;
  size_t aliases_len;
} HolonsIdentityInfo;

typedef struct {
  char *slug;
  char *uuid;
  HolonsIdentityInfo identity;
  char *lang;
  char *runner;
  char *status;
  char *kind;
  char *transport;
  char *entrypoint;
  char **architectures;
  size_t architectures_len;
  bool has_dist;
  bool has_source;
} HolonsHolonInfo;

typedef struct {
  char *url;
  HolonsHolonInfo *info;
  char *error;
} HolonsHolonRef;

typedef struct {
  HolonsHolonRef *found;
  size_t found_len;
  char *error;
} HolonsDiscoverResult;

typedef struct {
  HolonsHolonRef *ref;
  char *error;
} HolonsResolveResult;

typedef struct {
  void *channel;
  char *uid;
  HolonsHolonRef *origin;
  char *error;
} HolonsConnectResult;

const char *holons_default_uri(void);
holons_scheme_t holons_scheme_from_uri(const char *uri);
const char *holons_scheme_name(holons_scheme_t scheme);

int holons_parse_flags(int argc, char **argv, char *out_uri, size_t out_uri_len);
int holons_parse_uri(const char *uri, holons_uri_t *out, char *err, size_t err_len);

int holons_listen(const char *uri, holons_listener_t *out, char *err, size_t err_len);
int holons_accept(holons_listener_t *listener, holons_conn_t *out, char *err, size_t err_len);
int holons_dial_tcp(const char *host,
                    int port,
                    holons_conn_t *out,
                    char *err,
                    size_t err_len);
int holons_dial_stdio(holons_conn_t *out, char *err, size_t err_len);

ssize_t holons_conn_read(const holons_conn_t *conn, void *buf, size_t n);
ssize_t holons_conn_write(const holons_conn_t *conn, const void *buf, size_t n);
int holons_conn_close(holons_conn_t *conn);
int holons_close_listener(holons_listener_t *listener);

int holons_serve(const char *listen_uri,
                 holons_conn_handler_t handler,
                 void *ctx,
                 int max_connections,
                 int install_signal_handlers,
                 char *err,
                 size_t err_len);

int holons_resolve_manifest(const char *path,
                            holons_manifest_t *out,
                            char *resolved_path,
                            size_t resolved_path_len,
                            char *err,
                            size_t err_len);
int holons_parse_holon(const char *path, holons_identity_t *out, char *err, size_t err_len);
int holons_build_describe_response(const char *proto_dir,
                                   holons_describe_response_t *out,
                                   char *err,
                                   size_t err_len);
void holons_use_static_describe_response(const holons_describe_response_t *response);
int holons_make_describe_registration(holons_describe_registration_t *out,
                                      char *err,
                                      size_t err_len);
int holons_invoke_describe(const holons_describe_registration_t *registration,
                           const holons_describe_request_t *request,
                           holons_describe_response_t *out,
                           char *err,
                           size_t err_len);
void holons_free_describe_response(holons_describe_response_t *response);

HolonsDiscoverResult holons_discover(int scope,
                                     const char *expression,
                                     const char *root,
                                     int specifiers,
                                     int limit,
                                     int timeout);
HolonsResolveResult holons_resolve(int scope,
                                   const char *expression,
                                   const char *root,
                                   int specifiers,
                                   int timeout);
HolonsConnectResult holons_connect(int scope,
                                   const char *expression,
                                   const char *root,
                                   int specifiers,
                                   int timeout);
void holons_disconnect(HolonsConnectResult *result);

void holons_discover_result_free(HolonsDiscoverResult *result);
void holons_resolve_result_free(HolonsResolveResult *result);
void holons_connect_result_free(HolonsConnectResult *result);

volatile sig_atomic_t *holons_stop_token(void);
void holons_request_stop(void);

#ifdef __cplusplus
}
#endif

#endif
