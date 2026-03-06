![A .proto illuminated by four facets is all you need – Organic Programming](assets/images/op-proto.jpg)

# Organic Programming

A bio-inspired, Unix-inspired paradigm for a hybrid human–agent world.
One `.proto` contract, four facets — and legacy code, new code, humans,
and agents naturally interoperate.

> For the full motivation, see [Why?](./WHY.md).
> For the full specification, see [AGENT.md](./AGENT.md).

---

## Five core ideas

### The Holon[^1]

A **holon** is an independent, composable functional unit at any scale.
It exposes **facets** — automatically generated from a single contract:

- **Contract** (`.proto`) — the universal interface. Any language, any
  machine, local or remote.
- **External code** — a thin API for same-language, in-process calls.
- **CLI** — a `stdin`/`stdout` bridge to scripts, CI, and humans.
- **Tests** — the executable specification.

Because facets are generated, they are cheap, numerous, and always
consistent. The human writes only the domain logic.

### The Contract[^2]

Every holon's public surface is defined by a single `.proto` file.
Every public function is an `rpc`. Every type is a `message`. The
contract is the schema, the interface, and the documentation — all in
one artifact. `protoc` generates native stubs in every target language;
the human never writes API plumbing by hand.

### Composition[^3]

*"Expect the output of every holon to become the input to another,
as yet unknown, holon."*

Holons compose at three levels:
1. **In-process** — direct calls via external code, no serialization.
2. **At runtime** — via serve & dial, any transport.
3. **At the shell** — classic CLI piping.

### Mimesis[^4]

*"Invent only what is truly new; for everything else, imitate what
works."*

Replicate mechanisms that have survived natural selection in other
ecosystems, then adapt them to the holon substrate.

### Serve & Dial[^5]

Every holon can **serve** (listen for calls) and **dial** (call another
holon) — simultaneously, on any transport (TCP, stdio, Unix socket,
WebSocket, in-memory). Two holons from different technology stacks
interlock like Lego bricks: one serves, the other dials.

### Try it

These ideas are not theoretical. Working SDKs and functional examples
are available to experiment with:

- **[sdk/](./sdk/)** — language SDKs:
  [C](https://github.com/organic-programming/c-holons),
  [C++](https://github.com/organic-programming/cpp-holons),
  [C#](https://github.com/organic-programming/csharp-holons),
  [Dart](https://github.com/organic-programming/dart-holons),
  [Go](https://github.com/organic-programming/go-holons),
  [Java](https://github.com/organic-programming/java-holons),
  [JavaScript](https://github.com/organic-programming/js-holons),
  [JavaScript Web](https://github.com/organic-programming/js-web-holons),
  [Kotlin](https://github.com/organic-programming/kotlin-holons),
  [Objective-C](https://github.com/organic-programming/objc-holons),
  [Python](https://github.com/organic-programming/python-holons),
  [Ruby](https://github.com/organic-programming/ruby-holons),
  [Rust](https://github.com/organic-programming/rust-holons),
  [Swift](https://github.com/organic-programming/swift-holons).
- **[examples/](./examples/)** — runnable hello world holon examples in more than 12 languages built with the SDK.
- **[Go+Dart](https://github.com/organic-programming/go-dart-holons)** — cross-language recipe (Go backend + Dart frontend).
- **[holons/](./holons/)** — the blueprint toolchain (`op`, `who`).

---

# This Seed

This repository is the **seed** — the foundational specification from
which the ecosystem grows.

| Document | What it answers |
|----------|----------------|
| [Constitution](./AGENT.md) | What is a holon? |
| [Protocol](./PROTOCOL.md) | How do holons communicate? |
| [Conventions](./CONVENTIONS.md) | How is a holon structured per language? |
| [Index](./INDEX.md) | Full list of all documents |

© 2026 Benoit Pereira da Silva. All rights reserved.

---

[^1]: See [AGENT.md — Article 1](./AGENT.md#article-1--the-holon)
[^2]: See [AGENT.md — Article 2](./AGENT.md#article-2--the-contract-protocol-buffers)
[^3]: See [AGENT.md — Article 5](./AGENT.md#article-5--composition)
[^4]: See [AGENT.md — Article 9](./AGENT.md#article-9--mimesis)
[^5]: See [AGENT.md — Article 11](./AGENT.md#article-11--the-serve--dial-convention)
