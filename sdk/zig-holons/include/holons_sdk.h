#ifndef HOLONS_SDK_H
#define HOLONS_SDK_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define HOLONS_SDK_ABI_VERSION_MAJOR 0u
#define HOLONS_SDK_ABI_VERSION_MINOR 1u
#define HOLONS_SDK_ABI_VERSION_PATCH 0u

typedef enum holons_status {
  HOLONS_STATUS_OK = 0,
  HOLONS_STATUS_INVALID_ARGUMENT = 1,
  HOLONS_STATUS_RUNTIME_ERROR = 2,
  HOLONS_STATUS_NOT_FOUND = 3,
  HOLONS_STATUS_UNSUPPORTED = 4,
  HOLONS_STATUS_OUT_OF_MEMORY = 5
} holons_status;

typedef struct holons_sdk_context holons_sdk_context;
typedef struct holons_connection holons_connection;
typedef struct holons_server holons_server;
typedef struct holons_discovery_result holons_discovery_result;
typedef struct holons_hub_client holons_hub_client;

typedef struct holons_string_result {
  holons_status status;
  char *data;
  char *error_message;
} holons_string_result;

enum {
  HOLONS_DISCOVER_SCOPE_LOCAL = 0,
  HOLONS_DISCOVER_SCOPE_PROXY = 1,
  HOLONS_DISCOVER_SCOPE_DELEGATED = 2,
  HOLONS_DISCOVER_SIBLINGS = 0x01,
  HOLONS_DISCOVER_CWD = 0x02,
  HOLONS_DISCOVER_SOURCE = 0x04,
  HOLONS_DISCOVER_BUILT = 0x08,
  HOLONS_DISCOVER_INSTALLED = 0x10,
  HOLONS_DISCOVER_CACHED = 0x20,
  HOLONS_DISCOVER_ALL = 0x3f,
  HOLONS_DISCOVER_NO_LIMIT = 0,
  HOLONS_DISCOVER_NO_TIMEOUT = 0
};

unsigned int holons_sdk_abi_version_major(void);
unsigned int holons_sdk_abi_version_minor(void);
unsigned int holons_sdk_abi_version_patch(void);
const char *holons_sdk_version(void);
const char *holons_status_message(holons_status status);

holons_status holons_sdk_init(holons_sdk_context **out);
void holons_sdk_shutdown(holons_sdk_context *context);
void holons_sdk_free(void *ptr);
void holons_string_result_free(holons_string_result *result);

holons_status holons_connect(holons_sdk_context *context, const char *uri, holons_connection **out);
holons_string_result holons_connection_describe_json(holons_connection *connection);
void holons_connection_close(holons_connection *connection);

holons_status holons_describe_register_static_json(holons_sdk_context *context, const char *json);
holons_status holons_describe_register_static_proto(holons_sdk_context *context, const uint8_t *bytes, size_t len);
void holons_describe_clear_static(void);
holons_string_result holons_describe_static_json(void);

holons_status holons_serve_blocking(holons_sdk_context *context, const char *listen_uri);
holons_status holons_server_start(holons_sdk_context *context, const char *listen_uri, holons_server **out);
void holons_server_shutdown(holons_server *server);
void holons_server_wait(holons_server *server);
void holons_server_free(holons_server *server);

holons_status holons_discover(holons_sdk_context *context, const char *expression, const char *root, int specifiers, int limit, uint32_t timeout_ms, holons_discovery_result **out);
size_t holons_discovery_result_len(const holons_discovery_result *result);
holons_string_result holons_discovery_result_json(const holons_discovery_result *result);
void holons_discovery_result_free(holons_discovery_result *result);

holons_status holons_hub_client_connect(holons_sdk_context *context, const char *uri, holons_hub_client **out);
holons_string_result holons_hub_client_invoke_json(holons_hub_client *client, const char *method, const char *params_json);
void holons_hub_client_close(holons_hub_client *client);

#ifdef __cplusplus
}
#endif

#endif
