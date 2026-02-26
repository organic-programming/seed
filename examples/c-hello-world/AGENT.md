---
# Cartouche v1
title: "c-hello-world - Agent Directives"
author:
  name: "B. ALTER"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-12
revised: 2026-02-12
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: true
status: draft
---

# c-hello-world - Agent Directives

`c-hello-world` is a minimal deterministic holon implemented in C.
It demonstrates how to use `c-holons` for:
- CLI command parsing (`greet`, `serve`, `version`)
- transport-aware serve loop
- HOLON identity compatibility

Read the root constitution first: `/Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/AGENT.md`.

## Layout

```
examples/c-hello-world/
├── AGENT.md
├── HOLON.md
├── README.md
├── hello.h
├── hello.c
├── main.c
└── test_hello.c
```

## Behavioral contract

- `hello greet [name]` prints `Hello, <name>!` and defaults to `World`.
- `hello serve` accepts `--listen <URI>` and legacy `--port <N>`.
- `hello serve` supports all URI schemes parsed by `c-holons`.
- `--once` and `--max <N>` are operational controls for testability and bounded serving.

## Transport notes

The example validates:
- parser compatibility for `tcp`, `unix`, `stdio`, `mem`, `ws`, `wss`
- live round-trips for `mem` always
- live round-trips for bind-based schemes when environment permits bind

As with the SDK:
- `ws://` and `wss://` are currently socket-level in this example.
- full WebSocket HTTP upgrade/framing is not implemented in this sample process.

## Editing directives

When extending the example:
1. Keep `hello_greet` pure and deterministic.
2. Keep CLI behavior stable and backward compatible.
3. Add tests before changing serve behavior.
4. Do not add external dependencies unless explicitly required.

## Build and test

From `/Users/bpds/Documents/Entrepot/Git/Compilons/videosteno/organic-programming/examples/c-hello-world`:

```bash
clang -std=c11 -Wall -Wextra -pedantic -I ../../sdk/c-holons/include hello.c test_hello.c ../../sdk/c-holons/src/holons.c -o test_runner
./test_runner

clang -std=c11 -Wall -Wextra -pedantic -I ../../sdk/c-holons/include hello.c main.c ../../sdk/c-holons/src/holons.c -o hello
./hello greet Alice
```
