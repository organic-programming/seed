# swift-holons

Swift SDK for holons.

## serve

```swift
import GRPC
import Holons

try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())

let flags = Serve.parseOptions(CommandLine.arguments.dropFirst().map(String.init))
try Serve.runWithOptions(
  flags.listenURI,
  serviceProviders: [MyServiceProvider()],
  options: Serve.Options(reflect: flags.reflect)
)
```

## transport

Choose the listener with `--listen`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`.

`Serve` currently listens on gRPC transports only. For outbound JSON-RPC, `HolonRPCClient` can dial `ws://`, `wss://`, and `rest+sse://` endpoints.

## identity / describe

Wire the generated Incode Description with one line:

```swift
try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
```

At build time, `op build` generates `gen/describe_generated.swift`; at runtime, `Serve` fails fast with `no Incode Description registered — run op build` if that static response is missing.

## discover

```swift
let entry = try findBySlug("gabriel-greeting-swift")
```

## connect

```swift
let channel = try connect("gabriel-greeting-swift")
```
