---
# Cartouche v1
title: "hello-world — Example Holon"
author:
  name: "B. ALTER"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-12
revised: 2026-02-12
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: false
status: draft
---
# hello-world

> *"The simplest possible holon — a greeting service."*

A minimal, complete holon demonstrating all three facets and all five transports.
Use this as a reference when building your own.

## Build

```bash
go build ./cmd/hello
```

## Usage

### CLI facet (direct)

```bash
hello greet              # → Hello, World!
hello greet Alice        # → Hello, Alice!
```

### gRPC facet via OP (stdio — zero config, the default)

The simplest way to call any holon over gRPC. OP launches the binary,
pipes stdin/stdout, calls the method, and tears everything down:

```bash
op grpc+stdio://hello Greet '{"name": "Alice"}'
# → { "message": "Hello, Alice!" }
```

No server to start, no port to allocate, no process to manage.
**This is the recommended way to call a holon.**

### Persistent server (composed approach)

When a holon needs to stay alive — handling multiple clients, exposed
on a network, or composed behind a gateway — start it as a long-running
server:

```bash
# Terminal 1: start the server on any transport
hello serve --listen tcp://:9090
hello serve --listen unix:///tmp/hello.sock
hello serve --listen ws://:8080

# Terminal 2: call it
op grpc://localhost:9090 Greet '{"name": "Alice"}'
op grpc+unix:///tmp/hello.sock Greet '{"name": "Alice"}'
op grpc+ws://localhost:8080 Greet '{"name": "Alice"}'
```

## Test

```bash
go test ./... -count=1
```

## Structure

See [AGENT.md](./AGENT.md) for the full project layout and build directives.
