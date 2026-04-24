# ruby-holons

Ruby SDK for holons.

## serve

```ruby
require "holons"
require "describe_generated"
require "v1/my_service_services_pb"

Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)

class MyService < My::V1::MyService::Service
end

parsed = Holons::Serve.parse_options(ARGV)
Holons::Serve.run_with_options(
  parsed.listen_uri,
  ->(server) { server.handle(MyService.new) },
  parsed.reflect
)
```

## transport

Use `--listen` with `tcp://127.0.0.1:9090`, `unix:///tmp/my-holon.sock`, or `stdio://` when serving.

For dial-only Holon-RPC transports, use `Holons::HolonRPCClient` with `ws://` or `wss://`, and `Holons::HolonRPCHTTPClient` with `http://127.0.0.1:8080/api/v1/rpc` or `https://127.0.0.1:8443/api/v1/rpc`.

## identity / describe

Wire the generated Incode Description with one line:

```ruby
Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)
```

`op build` generates `gen/describe_generated.rb`; `serve` fails fast with `no Incode Description registered — run op build` if that file is not wired at startup.

## discover

```ruby
entry = Holons::Discover.find_by_slug("gabriel-greeting-ruby")
```

## connect

```ruby
channel = Holons.connect("gabriel-greeting-ruby")
```

## Build and test

```sh
rake test
```
