# Gabriel Greeting Ruby

Reference implementation of a Ruby holon. Gabriel is a multilingual greeting daemon that exposes `SayHello` and `ListLanguages` over the shared protobuf contract and keeps the business logic in a pure Code API facet.

This port follows the gabriel architecture exactly: local proto manifest, 4-facet split, committed generated stubs, 56 languages with localized default names, and SDK-backed serving. On Apple Silicon, prefer a native arm64 Ruby/Bundler toolchain so Bundler can reuse the prebuilt `grpc` and `google-protobuf` gems instead of compiling the C/C++ extensions from source.

## Discovery

This holon is source-discoverable from the repo root:

```bash
op list --source
```

Programmatically:

```text
Discover(LOCAL, "gabriel-greeting-ruby", null, SOURCE, NO_LIMIT, NO_TIMEOUT)
```

Today this works in the Go SDK. The other non-browser SDKs will support the same source lookup once their Phase 1 discovery tasks land. The browser SDK is excluded because it has no filesystem-based discovery.

## Facets

| Facet | Visibility | Location | Role |
|-------|-----------|----------|------|
| **Code API** | public | `api/public.rb` | Pure functions consuming and returning protobuf types. |
| **CLI** | public | `api/cli.rb` | Parses command-line args, calls the Code API, and formats text or JSON output. |
| **RPC** | internal | `internal/server.rb` | gRPC `GreetingService` implementation delegating to the Code API and registering `HolonMeta`. |
| **Tests** | public + internal | `api/*_test.rb`, `internal/server_test.rb` | One test file per facet for Code API, CLI, and RPC. |

## Structure

```text
.op/build/bin/gabriel-greeting-ruby  Wrapper binary for local execution.
cmd/main.rb                          Entry point.
api/public.rb                        Code API facade.
api/cli.rb                           CLI facade.
api/v1/holon.proto                   Local proto manifest.
internal/greetings.rb                Private 56-language greeting table.
internal/server.rb                   RPC server facet.
gen/ruby/greeting/v1/                Generated protobuf code.
scripts/generate_proto.sh            Regenerates Ruby protobuf stubs.
```

## How to launch

```bash
cd /Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/hello-world/gabriel-greeting-ruby
op sdk install ruby
./scripts/generate_proto.sh
bundle exec ruby -I. -e 'Dir["api/*_test.rb", "internal/*_test.rb"].sort.each { |file| load File.expand_path(file) }'
.op/build/bin/gabriel-greeting-ruby version
.op/build/bin/gabriel-greeting-ruby listLanguages --format json
.op/build/bin/gabriel-greeting-ruby sayHello Bob fr
.op/build/bin/gabriel-greeting-ruby serve --port 9090
grpcurl -plaintext -import-path /Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/_protos -proto v1/greeting.proto -d '{}' 127.0.0.1:9090 greeting.v1.GreetingService/ListLanguages
grpcurl -plaintext -import-path /Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/_protos -proto v1/greeting.proto -d '{"name":"Bob","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
