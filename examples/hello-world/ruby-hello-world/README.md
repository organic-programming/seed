# ruby-hello-world

A minimal Ruby `hello.v1.HelloService` holon using the repo-local
`ruby-holons` SDK for `serve` and `connect`.

## Setup

```bash
arch -x86_64 bundle install --path vendor/bundle
```

On this Apple Silicon workspace, Ruby gRPC currently runs through the
prebuilt `x86_64-darwin` gem under Rosetta.

## Run

```bash
arch -x86_64 bundle exec ruby hello.rb
arch -x86_64 bundle exec ruby hello.rb Alice
arch -x86_64 bundle exec ruby hello.rb serve --listen tcp://127.0.0.1:9090
```

## Test

```bash
arch -x86_64 bundle exec ruby test_hello.rb
```

## Invoke via stdio

```bash
op grpc+stdio://"arch -x86_64 bundle exec ruby hello.rb" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

## Connect example

```bash
arch -x86_64 bundle exec ruby connect_example.rb
# → {"message":"hello-from-ruby","sdk":"ruby-holons","version":"0.1.0"}
```
