---
# Cartouche v1
title: "Seed — Organic Programming"
author:
  name: "B. ALTER & Claude"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-09
revised: 2026-02-26
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: false
status: review
---


![A .proto is all you need – Organic Programming](assets/images/op-proto.jpg)

*"Calculemus!"* [^1]

# Organic Programming

---

# Why?

## Biological and synthetic intelligences now work as one

Coding was never a solitary act — Babbage had Lovelace, Thompson had
Ritchie, RSA took three minds. Science publishes collectively; so does
software. A deep movement has steadily widened the circle:
pair programming formalized the dialogue; teams and DevOps blurred the
line between writing code and running it; platforms — GitHub, GitLab,
SourceForge — turned sharing into a reflex, creating an unprecedented
**code humus**: millions of modules, recipes, and battle-tested
patterns available to the entire profession. In the span of barely six
months, agents (large language models and autonomous tools) have
pushed this collective trajectory past a tipping point.

## Humans' code has become matrix code

It is no longer the final product. It is the seed, the example, the
template from which agents generate, adapt, and compose. Natural-language
prompts are the RNA — the molecular scissors that cut and recombine
fragments of matrix code into new programs.

Natural languages have become the primary programming language[^2].
English dominates today, but the linguistic barrier is fading: a prompt
in French, Japanese, or Swahili will soon produce the same code as one
in English.

## The human becomes the demiurge

Veteran developers — those who have written code their entire lives —
barely write any directly anymore. They correct, they steer, they
provide examples (matrix code), but the agent implements. The human
becomes a demiurge — the one who shapes the world from raw material
without fabricating every atom. Like a composer who hears the full
orchestra and scores the intent — themes, structure, harmony — while
the instruments perform the parts.

## Nothing starts from scratch

This new world rests on an old one: compilers, languages, containers,
protocols, operating systems — an immense stack of accumulated
inventions. Agents never start from nothing. They consume, combine, and
extend this heritage.

## All of these reasons demand the birth of Organic Programming

This bio-inspired, Unix-inspired paradigm lays down solid principles for
a hybrid human-agent world where legacy code coexists with new code,
machines compose with machines, and humans orchestrate the whole.

To bring order to this joyful mess, Organic Programming relies on two
foundational devices:

1. **The proto contract**[^3] — a single formal language
   (Protocol Buffers[^4]) that makes everything naturally interoperable.
   Legacy code, new code, humans, agents: all speak the same interface.

2. **The facets** — automatically generated projections of that contract
   (API stubs, CLI wrappers, client/server skeletons, serialization).
   Because they are generated, they can be numerous, consistent, and
   cheap. The human writes only the domain logic; the machine produces
   everything else.

The contract unifies. The facets multiply. Together they turn a chaotic
ecosystem into a composable one.

---

# What?

> For the full specification, see [AGENT.md](./AGENT.md).

Organic Programming rests on five core ideas.

## The Holon[^5]

A **holon** is an independent, composable functional unit at any scale.
It exposes **facets** — automatically generated from a single contract:

- **Contract** (`.proto`) — the universal interface. Any language, any
  machine, local or remote.
- **External code** — a thin API for same-language, in-process calls.
- **CLI** — a `stdin`/`stdout` bridge to scripts, CI, and humans.
- **Tests** — the executable specification.

Because facets are generated, they are cheap, numerous, and always
consistent. The human writes only the domain logic.

## The Contract[^6]

Every holon's public surface is defined by a single `.proto` file.
Every public function is an `rpc`. Every type is a `message`. The
contract is the schema, the interface, and the documentation — all in
one artifact. `protoc` generates native stubs in every target language;
the human never writes API plumbing by hand.

## Composition[^7]

*"Expect the output of every holon to become the input to another,
as yet unknown, holon."*

Holons compose at three levels:
1. **In-process** — direct calls via external code, no serialization.
2. **At runtime** — via serve & dial, any transport.
3. **At the shell** — classic CLI piping.

## Mimesis[^8]

*"Invent only what is truly new; for everything else, imitate what
works."*

Replicate mechanisms that have survived natural selection in other
ecosystems, then adapt them to the holon substrate.

## Serve & Dial[^9]

Every holon can **serve** (listen for calls) and **dial** (call another
holon) — simultaneously, on any transport (TCP, stdio, Unix socket,
WebSocket, in-memory). Two holons from different technology stacks
interlock like Lego bricks: one serves, the other dials.

## Try it

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
- **[Go+Dart](https://github.com/organic-programming/go-dart-holons)** — cross-language assembly example (Go backend + Dart frontend).
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

[^1]: "Calculemus!" means "Let us calculate!" in Latin. It is a quote
from Leibniz: *"If controversies were to arise, there would be no more
need of disputation between two philosophers than between two
accountants. For it would suffice to take their pencils in their hands,
to sit down to their slates, and to say to each other (with a friend as
witness, if they liked): Let us calculate."* — English translation in
Philip P. Wiener (ed.), *Leibniz: Selections*, Charles Scribner’s Sons,
New York, 1951, “The Art of Discovery”, p. 25. - Gottfried Wilhelm Leibniz, *Scientia Generalis. Characteristica*, ca. 1685.

[^2]: Jensen Huang (NVIDIA, CES 2024): *"Everyone is now a
programmer — you just have to say something to the computer."*
Andrej Karpathy (2023): *"The hottest new programming language is
English."*

[^3]: The proto contract is also an act of **metaprogramming**: the
`.proto` file is a program that generates programs. It defines types
and services once; `protoc` produces native stubs, clients, servers,
and serialization in every target language. The human writes the
specification; the machine writes the implementation scaffolding.

[^4]: **Protocol Buffers** (protobuf) bring three decisive properties:
*performance* — a radical slimming of the wire format compared to JSON
or XML; *type safety* — the contract is the schema, enforced at compile
time; *backward compatibility* — field numbering and evolution rules
let services evolve without breaking consumers. Combined with **gRPC**
— the RPC framework that uses protobuf as its wire format — they
provide streaming, multiplexing, and cross-language interop out of
the box.

[^5]: See [AGENT.md — Article 1](./AGENT.md#article-1--the-holon)
[^6]: See [AGENT.md — Article 2](./AGENT.md#article-2--the-contract-protocol-buffers)
[^7]: See [AGENT.md — Article 5](./AGENT.md#article-5--composition)
[^8]: See [AGENT.md — Article 9](./AGENT.md#article-9--mimesis)
[^9]: See [AGENT.md — Article 11](./AGENT.md#article-11--the-serve--dial-convention)
