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
