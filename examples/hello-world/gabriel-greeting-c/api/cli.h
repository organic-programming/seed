#pragma once

#include <stdio.h>

extern const char *gabriel_greeting_c_version;

int gabriel_greeting_c_run_cli(int argc, char **argv, FILE *stdout_stream,
                               FILE *stderr_stream);
void gabriel_greeting_c_print_usage(FILE *output);
