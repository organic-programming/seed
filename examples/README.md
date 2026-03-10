> ⚠️ These examples are **raw proofs of concept** and not production-ready.

# Organic Programming — Hello World

Each subdirectory contains a minimal holon in a single language.
Gophers may start with [`go-hello-world`](./go-hello-world/) for a simple example.

Current status: a subset of these hello-worlds already import their
matching SDK directly (`go`, `js`, `swift`, `c`, and the browser/web
pairing). The rest are still intentionally useful raw gRPC baselines.
The audited list lives in [`../sdk/SDK_GUIDE.md`](../sdk/SDK_GUIDE.md).

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
- [Python](./python-hello-world/)
- [Ruby](./ruby-hello-world/)
- [Rust](./rust-hello-world/)
- [Swift](./swift-hello-world/)
- [Web (browser)](./web-hello-world/)

## Godart examples

Godart packages a Go host and a Dart front-end into a single holon.
Examples live in the recipe: [`go-dart-holons/examples/greeting`](../recipes/go-dart-holons/examples/greeting/).

- [greeting-daemon](../recipes/go-dart-holons/examples/greeting/greeting-daemon/) — headless Go daemon
- [greeting-godart](../recipes/go-dart-holons/examples/greeting/greeting-godart/) — full Godart holon with Dart UI
