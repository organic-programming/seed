# cpp-hello-world

A minimal holon implementing `HelloService.Greet` in C++ with
`cpp-holons` serve parsing and `holons::connect`.

## Build & Test

```bash
cmake -S . -B build
cmake --build build
ctest --test-dir build
```

## Run

```bash
./build/cpp-hello-world Alice
./build/cpp-hello-world serve --listen tcp://127.0.0.1:9090
```

## Invoke via stdio (zero config)

```bash
cmake -S . -B build
cmake --build build --target cpp-hello-world
op grpc+stdio://./build/cpp-hello-world Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

## Connect example

```bash
cmake -S . -B build
cmake --build build --target connect_example
./build/connect_example
# → {"message":"hello-from-cpp","sdk":"cpp-holons","version":"0.1.0"}
```
