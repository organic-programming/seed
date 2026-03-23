# gabriel-greet-go

> *"The simplest possible idiomatic holon — a greeting service."*

A minimal, complete holon demonstrating the OP-first execution model, alongside its three facets (CLI, gRPC, API) and all four transports (tcp, unix, stdio, ws). Use this as a reference when building your own.

## Build

The recommended way to build a holon is via `op build`:


```bash
op gabriel-greet-go Greet '{"name": "Bob"}'
op grpc://gabriel-greet-go Greet '{"name":"Bob"}'
op grpc+stdio://gabriel-greet-go Greet '{"name":"Bob"}'
op grpc+tcp://gabriel-greet-go Greet '{"name":"Bob"}'

??? 
op grpc://127.0.0.1:9090 Greet '{"name":"Bob"}'
op grpc+tcp://127.0.0.1:9090 Greet '{"name":"Bob"}'
op grpc+unix:///tmp/gabriel-greet-go.sock Greet '{"name":"Bob"}'
op grpc+ws://127.0.0.1:8080/grpc Greet '{"name":"Bob"}'
op grpc+wss://example.com/grpc Greet '{"name":"Bob"}'

op grpc://127.0.0.1:9090
op grpc+unix:///tmp/gabriel-greet-go.sock

op gabriel-greet-go Greet '{"name":"Bob"}'
op grpc://gabriel-greet-go Greet '{"name":"Bob"}'

````

```bash
op build gabriel-greet-go
```


This ensures the binary is properly constructed using the runner specified in `holon.yaml`. You can still build natively via `go build ./cmd/gabriel-greet-go`.

## Usage

### 1. The Standard Way: gRPC facet via OP

The most idiomatic way to orchestrate a holon is using `op`. `op` acts as the runner, taking care of starting the binary, connecting over the `stdio` pipe, calling the method, and gracefully shutting it down:

```bash
op grpc+stdio://gabriel-greet-go Greet '{"name": "Bob"}'
# → { "message": "Hello, Bob!" }
```

No server to explicitly start, no port to figure out. **This is the recommended way to call a holon.**

### 2. The Native Way: CLI facet

Underneath the OP execution, `gabriel-greet-go` is still just a standard binary with a native CLI facet:

```bash
./gabriel-greet-go greet              # → Hello, World!
./gabriel-greet-go greet Bob        # → Hello, Bob!
```

### 3. The Composed Way: Persistent Server

When a holon needs to stay alive—handling multiple clients, exposed on a network, or composed behind a gateway—start it as a long-running server in one terminal, and connect to it using OP from another terminal:

### 1. TCP (Standard gRPC)
**Terminal 1 (Server):**
```bash
./gabriel-greet-go serve --listen tcp://localhost:1234
```
**Terminal 2 (Client):**
```bash
op tcp://localhost:1234 Greet '{"name": "Bob"}'
```

### 2. Unix Domain Sockets
**Terminal 1 (Server):**
```bash
./gabriel-greet-go serve --listen unix:///tmp/gabriel.sock
```
**Terminal 2 (Client):**
```bash
op unix:///tmp/gabriel.sock Greet '{"name": "Bob"}'
```

### 3. WebSocket (gRPC-Web over ws://)
**Terminal 1 (Server):**
```bash
./gabriel-greet-go serve --listen ws://localhost:8080/grpc
```
**Terminal 2 (Client):**
```bash
op ws://localhost:8080/grpc Greet '{"name": "Bob"}'
```

### 4. STDIO (Standard I/O streams)
*STDIO is a special transport where the client spawns the server as a subprocess and communicates over stdin/stdout. You only need one terminal:*
```bash
op stdio://gabriel-greet-go Greet '{"name": "Bob"}'
```



## Test

You can run tests natively using the standard go toolchain:

```bash
go test ./... -count=1
```

Or idiomatic via op:

```bash
op test gabriel-greet-go
```