# c-holons

C SDK for holons.

`Describe` is static-only at runtime. `op build` generates `gen/describe_generated.c`, which exports `holons_generated_describe_response()`. Register that response before startup; otherwise serve fails fast with `no Incode Description registered — run op build`.

## serve

Expose a public gRPC holon with the bridge wrapper and a private backend:

```sh
./sdk/c-holons/bin/grpc-bridge \
  --listen tcp://127.0.0.1:9090 \
  --backend ./build/my-holon-backend \
  --proto-dir ./api \
  --describe-static ./gen/describe_generated.json
```

The bridge serves gRPC and the static `Describe` response. Your backend can stay on `stdio://`, `tcp://`, or `unix://`.

## transport

Choose the public listener with `--listen`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`.

`holons_connect()` also dials `ws://`, `wss://`, and `rest+sse://`, for example `ws://127.0.0.1:8080/rpc`, `wss://127.0.0.1:8443/rpc`, or `rest+sse://127.0.0.1:8080/api/v1/rpc`.

## identity / describe

Wire the build-generated Incode Description with one line:

```c
holons_use_static_describe_response(holons_generated_describe_response());
```

At build time, `op build` generates `gen/describe_generated.c` and `gen/describe_generated.json`. For build and dev tooling, `holons_resolve_manifest("api/v1/holon.proto", &manifest, resolved_path, sizeof(resolved_path), err, sizeof(err))` reads the manifest without affecting the runtime `Describe` path.

## discover

```c
holon_entry_t *entry = holons_find_by_slug("gabriel-greeting-c", err, sizeof(err));
```

Release the returned entry with `holons_free_entries(entry)`.

## connect

```c
grpc_channel *channel = holons_connect("gabriel-greeting-c");
```

Close the channel with `holons_disconnect(channel)`.

## Build and test

```sh
make test
```
