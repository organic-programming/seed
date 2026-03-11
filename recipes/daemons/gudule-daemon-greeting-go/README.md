# Gudule Greeting Daemon Go

Standalone Go daemon for the shared `greeting.v1.GreetingService` contract.

## Regenerate stubs

```sh
protoc \
  -I ../../protos \
  --go_out=paths=source_relative,Mgreeting/v1/greeting.proto=github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1;greetingv1:gen/go \
  --go-grpc_out=paths=source_relative,Mgreeting/v1/greeting.proto=github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1;greetingv1:gen/go \
  ../../protos/greeting/v1/greeting.proto
```

## Build and run

```sh
op build recipes/daemons/gudule-daemon-greeting-go
op run recipes/daemons/gudule-daemon-greeting-go --listen tcp://127.0.0.1:9091
```

## Test

```sh
go test ./...
```
