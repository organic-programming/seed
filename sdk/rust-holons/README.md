# rust-holons

Rust SDK for holons.

## serve

```rust
use holons::describe;
use holons::serve;
use my_holon::gen::{describe_generated, rust::my_service::v1 as pb};

fn register_static_describe() {
    static INIT: std::sync::Once = std::sync::Once::new();
    INIT.call_once(|| {
        describe::use_static_response(describe_generated::static_describe_response());
    });
}

#[tokio::main]
async fn main() -> serve::Result<()> {
    register_static_describe();

    let args: Vec<String> = std::env::args().skip(1).collect();
    let opts = serve::parse_options(&args);

    serve::run_single(
        &opts.listen_uri,
        pb::my_service_server::MyServiceServer::new(Server::default()),
    )
    .await
}
```

## transport

Choose the listener with `--listen`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`. `serve` currently binds gRPC over `tcp://`, `unix://`, and `stdio://`.

For dial-only Holon-RPC transports, use `holons::holonrpc::Client::connect("ws://127.0.0.1:8080/rpc").await?`, `holons::holonrpc::Client::connect("wss://example.com/rpc").await?`, or `holons::holonrpc::HTTPClient::new("rest+sse://127.0.0.1:8080/api/v1/rpc")?`.

## identity / describe

Wire the generated Incode Description with one line:

```rust
holons::describe::use_static_response(gen::describe_generated::static_describe_response());
```

At build time, `op build` generates `gen/describe_generated.rs`; at runtime, `serve` fails fast with `no Incode Description registered — run op build` if that static response was not registered before startup.

## discover

```rust
let entry = holons::discover::find_by_slug("gabriel-greeting-rust")?;
```

## connect

```rust
let channel = holons::connect::connect("gabriel-greeting-rust").await?;
```

## Build and test

```sh
cargo test
```
