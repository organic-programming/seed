# csharp-holons

C# SDK for building and connecting holons.

## serve

```csharp
using Gen;
using Greeting.V1;
using Holons;

Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());

Serve.Run("tcp://127.0.0.1:9090", [
    Serve.Service<GreetingServiceImpl>()
]);
```

## transport

Choose the server listener with a listen URI such as `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`.

If you parse CLI flags, `Serve.ParseOptions(args)` resolves `--listen` and `--reflect` for you.

For dial-only transports, use `HolonRPCClient` with `ws://`, `wss://`, `http://`, or `https://` endpoints:

```csharp
await using var rpc = new HolonRPCClient();
await rpc.ConnectAsync("https://127.0.0.1:8443/api/v1/rpc");
```

## identity / describe

Wire the generated Incode Description with one line:

```csharp
Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());
```

At build or dev time, resolve the nearby manifest with:

```csharp
var manifest = Identity.Resolve(".");
```

## discover

```csharp
var holon = Discover.FindBySlug("gabriel-greeting-csharp");
```

## connect

```csharp
using var channel = Connect.ConnectTarget("gabriel-greeting-csharp");
```

## Build and test

```sh
dotnet test csharp-holons.sln
```
