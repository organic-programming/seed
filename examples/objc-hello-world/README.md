# objc-hello-world

A minimal holon implementing `HelloService.Greet` in Objective-C with
`objc-holons` serve parsing and `Holons connect`.

## Build & Test

```bash
make test
```

## Run

```bash
make hello
./hello Alice
./hello serve --listen tcp://127.0.0.1:9090
```

## Invoke via stdio (zero config)

```bash
make hello
op grpc+stdio://./hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

## Connect example

```bash
make connect_example
./connect_example
# → {"message":"hello-from-objc","sdk":"objc-holons","version":"0.1.0"}
```
