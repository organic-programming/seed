# Gabriel Greeting Ruby

Reference implementation of a Ruby holon. Gabriel is a multilingual greeting daemon that exposes `SayHello` and `ListLanguages` over the shared protobuf contract and keeps the business logic in a pure Code API facet.

This port follows the gabriel architecture exactly: local proto manifest, 4-facet split, committed generated stubs, 56 languages with localized default names, and SDK-backed serving. In this workspace the practical Ruby gRPC path is `arch -x86_64` on Apple Silicon, so the checked-in wrapper under `.op/build/bin/` launches the holon through Rosetta.

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
BUNDLE_PATH=vendor/bundle arch -x86_64 bundle install
./scripts/generate_proto.sh
arch -x86_64 bundle exec ruby -I. -e 'Dir["api/*_test.rb", "internal/*_test.rb"].sort.each { |file| load File.expand_path(file) }'
.op/build/bin/gabriel-greeting-ruby version
.op/build/bin/gabriel-greeting-ruby listLanguages --format json
.op/build/bin/gabriel-greeting-ruby sayHello Alice fr
.op/build/bin/gabriel-greeting-ruby serve --port 9090
grpcurl -plaintext -import-path /Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/_protos -proto v1/greeting.proto -d '{}' 127.0.0.1:9090 greeting.v1.GreetingService/ListLanguages
grpcurl -plaintext -import-path /Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/_protos -proto v1/greeting.proto -d '{"name":"Alice","lang_code":"fr"}' 127.0.0.1:9090 greeting.v1.GreetingService/SayHello
```
