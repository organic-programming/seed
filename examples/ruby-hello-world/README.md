# ruby-hello-world

A minimal holon implementing HelloService.Greet in Ruby.

## Test

```bash
ruby test_hello.rb
```

## Invoke via stdio (zero config)

```bash
op grpc+stdio://"ruby hello.rb" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
