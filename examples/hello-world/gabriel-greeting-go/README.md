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


# Architecture : 

- [Public funcs](api/public.go) can be used for code composition.
- [Internal implementation](internal/) should not be exposed.