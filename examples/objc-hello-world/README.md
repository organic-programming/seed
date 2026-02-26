---
# Cartouche v1
title: "objc-hello-world — Hello World Holon in Objective-C"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
# objc-hello-world

A minimal holon implementing HelloService.Greet in Objective-C.

## Build & Test

```bash
clang -framework Foundation -DTEST_BUILD hello.m test_hello.m -o test_runner && ./test_runner
```

## Invoke via stdio (zero config)

```bash
clang -framework Foundation hello.m -o hello
op grpc+stdio://./hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
