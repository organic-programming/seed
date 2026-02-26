---
# Cartouche v1
title: "swift-hello-world — Hello World Holon in Swift"
author:
  name: "B. ALTER"
created: 2026-02-12
lang: en-US
access:
  humans: true
  agents: false
status: draft
---
# swift-hello-world

A minimal Swift holon example with deterministic `Greet` behavior.

This example uses `swift-holons` for standard flag/transport parsing.

## Build & Test

```bash
swift test
```

## Run

```bash
swift run swift-hello-world
swift run swift-hello-world --name Alice
swift run swift-hello-world --listen tcp://:8080 --name Alice
```

## Invoke via stdio (zero config)

```bash
op grpc+stdio://"swift run swift-hello-world" Greet '{"name":"Alice"}'
# -> { "message": "Hello, Alice!" }
```
