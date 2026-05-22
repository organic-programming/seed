#include "api/cli.h"

#include <stdio.h>

int main(int argc, char **argv) {
  return cascade_node_c_run_cli(argc - 1, argv + 1, stdout, stderr);
}

