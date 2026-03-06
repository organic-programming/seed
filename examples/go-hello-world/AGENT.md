---
# Cartouche v1
title: "hello-world — Agent Directives"
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
  agents: true
status: draft
---

# hello-world — Agent Directives

This document explains how to build a minimal holon from scratch. It serves
as a **step-by-step tutorial** for both humans and agents.

---

## What this holon does

`hello-world` is a greeting service. It accepts a name and returns a greeting.
That's it — deliberately trivial so the structure is the focus, not the logic.

## How it was scaffolded

The identity and dependency files were created using the standard toolchain
(see [AGENT.md — Article 12](../../AGENT.md)):

```bash
# 1. Create identity
op grpc+stdio://who CreateIdentity '{
  "given_name": "hello-world",
  "family_name": "Example Holons",
  "motto": "The simplest possible holon — a greeting service.",
  "composer": "B. ALTER",
  "clade": "DETERMINISTIC_PURE",
  "lang": "go",
  "output_dir": "./examples/hello-world"
}'

# 2. Init dependency file
op grpc+stdio://atlas Init '{
  "directory": "./examples/hello-world",
  "holon_path": "github.com/organic-programming/examples/hello-world"
}'

# 3. Add the SDK
op grpc+stdio://atlas Add '{
  "directory": "./examples/hello-world",
  "path": "github.com/organic-programming/go-holons",
  "version": "v0.2.0"
}'
```

No files were written by hand for identity or dependencies.

---

## How to build a holon like this one

### Step 1 — Write the contract (`api/hello.proto`)

The `.proto` file IS the specification. Keep it minimal:

- One `service` block.
- One `rpc` per verb.
- Messages express the domain, not internal types.

### Step 2 — Generate Go code (`proto/`)

```bash
protoc --go_out=proto --go-grpc_out=proto api/hello.proto
```

The generated code goes in `proto/`. Never edit it.

### Step 3 — Implement the server (`internal/server/`)

- `server.go` — implements the generated `HelloServiceServer` interface.
- `ListenAndServe(uri, reflection)` — the standard entry point, delegating
  to `serve.Run` from `go-holons/pkg/serve`.

### Step 4 — Write the CLI (`internal/cli/`)

The CLI is a thin wrapper over the server logic:

```
hello greet [name]    →  calls GreetRequest{ name }
hello serve           →  starts the gRPC server
hello version         →  prints version info
```

### Step 5 — Write tests (`internal/server/server_test.go`)

Three mandatory levels:

1. **Contract tests**: every RPC — nominal case + error case.
2. **Transport tests**: `mem://` and `ws://` round-trips.
3. **CLI smoke test**: build the binary and run `hello greet World`.

### Step 6 — Build and verify

```bash
go build ./...
go test ./... -count=1
```

---

## Project structure

```
hello-world/
├── AGENT.md                     ← you are here
├── holon.yaml                   ← identity + operational manifest
├── README.md                    ← user-facing documentation
├── holon.mod                    ← organic dependency manifest
├── api/
│   └── hello.proto              ← THE contract
├── proto/
│   ├── hello.pb.go              ← generated protobuf code
│   └── hello_grpc.pb.go         ← generated gRPC stubs
├── cmd/
│   └── hello/
│       └── main.go              ← entry point
├── internal/
│   ├── cli/
│   │   └── commands.go          ← CLI facet
│   └── server/
│       ├── server.go            ← gRPC server implementation
│       └── server_test.go       ← contract + transport tests
├── go.mod
├── go.sum
└── .gitignore
```

## Conventions

- **Binary name**: matches slug in holon.yaml → `hello`
- **Go module path**: `github.com/organic-programming/examples/hello-world`
- **Proto package**: `hello.v1`
- **Three facets**: CLI (subprocess), gRPC (server), API (Go package)
- **Five transports**: tcp, unix, stdio, mem, ws — all via `go-holons/pkg/serve`
