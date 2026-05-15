# dart-holons

Dart SDK for holons.

## serve

```dart
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import 'gen/describe_generated.dart';
import 'gen/dart/my_service/v1/my_service.pbgrpc.dart';

class MyService extends MyServiceBase {
  @override
  Future<PingResponse> ping(ServiceCall call, PingRequest request) async {
    return PingResponse()..message = request.message;
  }
}

Future<void> main(List<String> args) async {
  final parsed = parseOptions(args);
  useStaticResponse(staticDescribeResponse());

  await runWithOptions(
    parsed.listenUri,
    <Service>[MyService()],
    options: ServeOptions(reflect: parsed.reflect),
  );
}
```

## transport

Choose the gRPC listener with `--listen`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`.

For HolonRPC transports, bind `HolonRPCServer('ws://127.0.0.1:8080/rpc')` for WebSocket or `HolonRPCHTTPServer('http://127.0.0.1:8080/api/v1/rpc')` for HTTP+SSE. `https://` enables TLS for the HTTP+SSE server, and `HolonRPCClient` can dial both `ws://` and `wss://`.

## identity / describe

At build or dev time, read the manifest with:

```dart
final manifest = resolve('.');
```

At runtime, wire the generated Incode Description with one line:

```dart
useStaticResponse(staticDescribeResponse());
```

`op build` generates `gen/describe_generated.dart`. Runtime `Describe` is static-only; if no static response is registered, `serve` fails fast with `no Incode Description registered — run op build`.

`buildDescribeResponse(...)` is a build-time utility for `op build`, not a runtime fallback.

## discover

```dart
final entry = await findBySlug('gabriel-greeting-dart');
```

## connect

```dart
final channel = await connect('gabriel-greeting-dart');
```

## Transitive observability

Every parent→child connection is transitive by default: spawning a
child via `spawnMember` opens long-lived
`HolonObservability.Logs(follow=true)` and `Events(follow=true)`
streams in background and republishes received entries into the
parent's local rings, appending a `ChainHop` for the child.
Peer-to-peer `connect` defaults to OFF; opt-in explicitly when needed.

```dart
// Default-ON: spawnMember owns the child's lifecycle. The named
// parameter travels through to the relay attached on the connection.
final silent = await spawnMember(
  binaryPath: '/abs/path/to/gabriel-greeting-dart',
  slug: 'gabriel-greeting-dart',
  withTransitiveObservability: false,
);

// Peer-to-peer dial: connect takes a ConnectOptions object as its
// positional second argument. Transitivity is OFF by default; opt-in
// to tail a remote holon.
final peer = await connect(
  'gabriel-greeting-rust',
  const ConnectOptions(withTransitiveObservability: true),
);
```

See [OBSERVABILITY.md §Transitive Observability](../../OBSERVABILITY.md#transitive-observability)
for the full doctrine (defaults, peer-vs-spawn rules, per-emission
`private: true` opt-out, subscription replay-then-live, v1 metrics
non-coverage).

## Build and test

```sh
dart pub get
dart test
```
