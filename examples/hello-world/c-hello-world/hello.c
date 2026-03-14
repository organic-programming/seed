#include "hello.h"

#include <stdio.h>

int hello_greet(const char *name, char *out, size_t out_len) {
  const char *target = "World";
  int written;

  if (out == NULL || out_len == 0) {
    return -1;
  }

  if (name != NULL && name[0] != '\0') {
    target = name;
  }

  written = snprintf(out, out_len, "Hello, %s!", target);
  if (written < 0 || (size_t)written >= out_len) {
    return -1;
  }

  return 0;
}
