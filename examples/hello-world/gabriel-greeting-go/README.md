# Gabriel Greeting go


## How to launch

```bash
op gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
op grpc://gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
op grpc+stdio://gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
op grpc+tcp://gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
```

## Currently not supported .

mem, unix, ws, ws, sse+rest 

```bash
op grpc+mem://gabriel-greeting-go SayHello '{"name":"Alice","lang_code":"en"}'
```

# How to compile manually the [holon.proto](v1/holon.proto)

```bash
cd examples/hello-world/gabriel-greeting-go/v1
protoc --proto_path=. --proto_path=../../../../_protos --proto_path=../../../_protos holon.proto --descriptor_set_out=/dev/null
```


# This holon exposes : 

- [A Cli facet](api/cli.go) the CLI facade to the Code API functions (can be used by CI or scripts)
- [The RPC facet](internal/server.go) exposed to the gRPC via the cli sub command "serve" (used by grace-op or any op holon)
- [Code API Facet](api/public.go) can be used for code composition (can be used to integrate in code classic lib mode) 
- [Internal implementation](internal/) should not be exposed.
- Tests for each layer.
