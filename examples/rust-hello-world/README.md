---
# Cartouche v1
title: "rust-hello-world — Hello World Holon in Rust"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
# rust-hello-world

A minimal holon implementing `HelloService.Greet` in Rust (tonic).

## Build & Test

```bash
cargo test
```

## Run

```bash
cargo run
```

## Invoke via stdio (zero config)

```bash
cargo build --release
op grpc+stdio://target/release/hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
