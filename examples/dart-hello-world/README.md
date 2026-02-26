---
# Cartouche v1
title: "dart-hello-world — Hello World Holon in Dart"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
# dart-hello-world

A minimal holon implementing HelloService.Greet in Dart.

## Test

```bash
dart pub get && dart test
```

## Invoke via stdio (zero config)

```bash
dart compile exe bin/hello.dart -o hello
op grpc+stdio://./hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
