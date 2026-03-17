# Gudule Greeting HostUI Flutter

Standalone Flutter HostUI for the shared `greeting.v1.GreetingService`.

## Regenerate stubs

```sh
protoc \
  -I ../../protos \
  --plugin=protoc-gen-dart="$HOME/.pub-cache/bin/protoc-gen-dart" \
  --dart_out=grpc:lib/gen \
  ../../protos/greeting/v1/greeting.proto
```

## Build

```sh
op build recipes/hostui/gudule-greeting-hostui-flutter
```

## Run against an external daemon

```sh
GREETING_TARGET=tcp://127.0.0.1:9091 flutter run -d macos
```

## Build the sibling Go daemon for local development

```sh
./scripts/build_daemon.sh
```
