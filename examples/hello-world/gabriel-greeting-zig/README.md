# gabriel-greeting-zig

Zig implementation of the hello-world GreetingService holon.

```sh
op sdk install zig
op sdk verify zig
op build gabriel-greeting-zig
op gabriel-greeting-zig ListLanguages
op gabriel-greeting-zig SayHello '{"name":"Bob","lang_code":"fr"}'
op run gabriel-greeting-zig -- --listen tcp://127.0.0.1:9090
```

The holon serves the same `ListLanguages` and `SayHello` contract as the Go and Rust greeting examples. It uses `sdk/zig-holons` for gRPC Core transport, protobuf-c message encoding, static `HolonMeta.Describe`, and process lifecycle handling.
