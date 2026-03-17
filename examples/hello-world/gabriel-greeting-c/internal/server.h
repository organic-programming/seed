#pragma once

#include <stdio.h>

int gabriel_greeting_c_exec_bridge(const char *listen_uri, FILE *stderr_stream);
int gabriel_greeting_c_backend_serve(const char *listen_uri, FILE *stdout_stream,
                                     FILE *stderr_stream);
