# cpp-hello-world

A minimal holon implementing HelloService.Greet in C++.

## Build & Test

```bash
clang++ -std=c++20 test_hello.cpp -o test_runner && ./test_runner
```

## Invoke via stdio (zero config)

```bash
clang++ -std=c++20 hello.cpp -o hello
op grpc+stdio://./hello Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
