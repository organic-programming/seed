---
# Cartouche v1
title: "cpp-hello-world — Hello World Holon in C++"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
# cpp-hello-world

A minimal holon implementing HelloService.Greet in C++.

## Build & Test

```bash
clang++ -std=c++20 test_hello.cpp -o test_runner && ./test_runner
```

## Invoke via stdio (zero config)

```bash
clang++ -std=c++20 hello.cpp -o hello
op grpc+stdio://./hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
