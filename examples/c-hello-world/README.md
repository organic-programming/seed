# c-hello-world

Minimal hello-world holon in C, backed by `sdk/c-holons`.

This example is transport-complete at the SDK layer:
`tcp`, `unix`, `stdio`, `mem`, `ws`, `wss`.

Note:
- `ws` and `wss` in this C example are socket-layer URI/listener support.
- Full browser WebSocket handshake/framing is not implemented in-process here.

## Build

```bash
clang -std=c11 -Wall -Wextra -pedantic \
  -I ../../sdk/c-holons/include \
  hello.c main.c ../../sdk/c-holons/src/holons.c \
  -o hello
```

## Test

```bash
clang -std=c11 -Wall -Wextra -pedantic \
  -I ../../sdk/c-holons/include \
  hello.c test_hello.c ../../sdk/c-holons/src/holons.c \
  -o test_runner
./test_runner
```

## Usage

```bash
./hello greet
./hello greet Alice
```

Serve on any transport:

```bash
./hello serve --listen tcp://127.0.0.1:9090
./hello serve --listen unix:///tmp/c-hello-world.sock
./hello serve --listen stdio:// --once
./hello serve --listen ws://127.0.0.1:8080/grpc
./hello serve --listen wss://127.0.0.1:8443/grpc
```

Quick smoke calls:

```bash
printf "Alice\n" | ./hello serve --listen stdio:// --once
printf "Alice\n" | nc 127.0.0.1 9090
printf "Alice\n" | nc -U /tmp/c-hello-world.sock
```
