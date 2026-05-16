#ifndef OBSERVABILITY_CASCADE_NODE_C_INTERNAL_SERVER_H
#define OBSERVABILITY_CASCADE_NODE_C_INTERNAL_SERVER_H

#include "holons/holons.h"

#include <stddef.h>
#include <stdio.h>

#ifdef __cplusplus
extern "C" {
#endif

int cascade_node_c_serve(const char *const *listen_uris,
                         size_t listen_uri_count,
                         const char *transport,
                         const holons_composite_child_spec_t *children,
                         size_t child_count,
                         FILE *stderr_stream);

#ifdef __cplusplus
}
#endif

#endif
