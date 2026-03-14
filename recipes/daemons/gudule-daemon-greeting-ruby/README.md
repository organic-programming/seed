# Gudule Greeting Daemon Ruby

Standalone Ruby daemon for the shared `greeting.v1.GreetingService`
contract.

It now uses the repo-local `ruby-holons` SDK for:
- `serve` flag parsing
- gRPC server lifecycle
- stdio bridging for `connect(slug)`
- automatic `HolonMeta.Describe` registration from local protos

On Apple Silicon, `op build` creates a launcher that runs the daemon
through Rosetta with `arch -x86_64` because the `grpc` gem is currently
the practical path with the system Ruby in this workspace.

## Build and run

```sh
op build recipes/daemons/gudule-daemon-greeting-ruby
op run recipes/daemons/gudule-daemon-greeting-ruby --listen tcp://127.0.0.1:9091
```

## Test

```sh
arch -x86_64 bundle exec rake test
```
