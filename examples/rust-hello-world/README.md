# rust-hello-world

A minimal holon implementing `HelloService.Greet` in Rust with
`rust-holons` flag parsing and `holons::connect`.

## Build & Test

```bash
cargo test
```

## Run

```bash
cargo run --bin hello -- serve --listen tcp://127.0.0.1:9090
```

## Invoke via stdio (zero config)

```bash
cargo build --release
op grpc+stdio://target/release/hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

## Connect example

```bash
cargo run --bin connect_example
# → {"message":"hello-from-rust","sdk":"rust-holons"}
```
