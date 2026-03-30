# cpp-holons

C++ SDK for building and connecting holons.

## serve

```cpp
#include "gen/describe_generated.hpp"
#include "greeting.grpc.pb.h"
#include "holons/describe.hpp"
#include "holons/serve.hpp"

#include <memory>
#include <vector>

class GreetingServer final : public greeting::v1::GreetingService::Service {
public:
  grpc::Status SayHello(grpc::ServerContext *,
                        const greeting::v1::SayHelloRequest *,
                        greeting::v1::SayHelloResponse *response) override {
    response->set_message("Hello from cpp-holons");
    return grpc::Status::OK;
  }
};

int main(int argc, char **argv) {
  holons::describe::use_static_response(gen::StaticDescribeResponse());

  std::vector<std::string> args(argv + 1, argv + argc);
  auto parsed = holons::serve::parse_options(args);

  auto service = std::make_shared<GreetingServer>();
  holons::serve::options opts;
  opts.enable_reflection = parsed.reflect;

  holons::serve::serve(
      parsed.listeners,
      [service](grpc::ServerBuilder &builder) {
        builder.RegisterService(service.get());
      },
      opts);
}
```

## transport

Choose the server listener with `--listen tcp://127.0.0.1:9090`, `--listen unix:///tmp/gabriel.sock`, or `--listen stdio://`.

`holons::serve::parse_options(args)` resolves `--listen`, `--port`, and `--reflect` for you.

For dial-only transports, use `holons::holon_rpc_client` with `ws://` or `wss://`, and `holons::holon_rpc_http_client` with `http://127.0.0.1:8080/api/v1/rpc` or `https://127.0.0.1:8443/api/v1/rpc` for HTTP+SSE JSON-RPC.

## identity / describe

Wire the generated Incode Description with one line:

```cpp
holons::describe::use_static_response(gen::StaticDescribeResponse());
```

At build or dev time, resolve the nearby manifest with:

```cpp
auto manifest = holons::identity::resolve(holons::identity::resolve_manifest("."));
```

`op build` generates `gen/describe_generated.hpp`; runtime startup fails with `no Incode Description registered — run op build` until that static response is registered.

## discover

```cpp
auto holon = holons::find_nearby_by_slug(".", "gabriel-greeting-cpp");
```

## connect

```cpp
auto channel = holons::connect("gabriel-greeting-cpp");
```
