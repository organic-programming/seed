#ifndef OBSERVABILITY_CASCADE_NODE_C_INTERNAL_SERVER_H
#define OBSERVABILITY_CASCADE_NODE_C_INTERNAL_SERVER_H

#include "holons/holons.h"

#include <stddef.h>
#include <stdio.h>

int cascade_node_c_serve(const char *listen_uri,
                         const holons_grpc_member_ref_t *members,
                         size_t member_count,
                         FILE *stderr_stream);

#endif

