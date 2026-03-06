---
# Cartouche v1
title: "Organic Programming — Examples"
author:
  name: "B. ALTER"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-12
revised: 2026-03-06
lang: en-US
origin_lang: en-US
translation_of: null
translator: null
access:
  humans: true
  agents: false
status: draft
---


> ⚠️ These examples are **raw proofs of concept** and not production-ready.

# Organic Programming — Hello World

Each subdirectory contains a minimal holon in a single language.
Gophers may start with [`go-hello-world`](./go-hello-world/) for a simple example.

## Prerequisite

Install **op** (the holon operator):

```bash
export OPPATH="${OPPATH:-$HOME/.op}"
export OPBIN="${OPBIN:-$OPPATH/bin}"
mkdir -p "$OPBIN"
export PATH="$OPBIN:$PATH"
GOBIN="$OPBIN" go install github.com/organic-programming/grace-op/cmd/op@latest
```

`OPPATH` is the Organic Programming runtime home. `OPBIN` is the
standard directory for installed Organic Programming binaries.

## Examples

- [C](./c-hello-world/)
- [C++](./cpp-hello-world/)
- [C#](./csharp-hello-world/)
- [Dart](./dart-hello-world/)
- [Go](./go-hello-world/)
- [Java](./java-hello-world/)
- [JavaScript](./js-hello-world/)
- [Kotlin](./kotlin-hello-world/)
- [Objective-C](./objc-hello-world/)
- [Python](./python-hello-world/)
- [Ruby](./ruby-hello-world/)
- [Rust](./rust-hello-world/)
- [Swift](./swift-hello-world/)
- [Web (browser)](./web-hello-world/)

## Godart examples

Godart packages a Go host and a Dart front-end into a single holon.
Examples live in the SDK: [`go-dart-holons/examples`](../sdk/go-dart-holons/examples/).

- [greeting-daemon](../sdk/go-dart-holons/examples/greeting-daemon/) — headless Go daemon
- [greeting-godart](../sdk/go-dart-holons/examples/greeting-godart/) — full Godart holon with Dart UI

