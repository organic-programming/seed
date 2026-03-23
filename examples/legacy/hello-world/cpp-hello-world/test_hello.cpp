#include "hello.hpp"
#include <cassert>
#include <cstdio>

int main() {
  assert(hello::greet("Bob") == "Hello, Bob!");
  assert(hello::greet("") == "Hello, World!");
  std::printf("2 passed, 0 failed\n");
  return 0;
}
