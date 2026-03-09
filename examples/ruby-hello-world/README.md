# ruby-hello-world

A minimal holon implementing `HelloService.Greet` in Ruby with
`ruby-holons` serve helpers and `Holons.connect`.

## Run

```bash
ruby hello.rb
ruby hello.rb Alice
ruby hello.rb serve --listen tcp://127.0.0.1:9090
```

## Test

```bash
ruby test_hello.rb
```

## Invoke via stdio (zero config)

```bash
op grpc+stdio://"ruby hello.rb" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

## Connect example

```bash
bundle install
bundle exec ruby connect_example.rb
# → {"message":"hello-from-ruby","sdk":"ruby-holons","version":"0.1.0"}
```
