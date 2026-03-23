> ⚠️ These examples are **raw proofs of concept** and not production-ready.

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

# Organic Programming 

Each subdirectory contains a minimal holon in a single language.
Gophers may start with [`go-hello-world`](./hello-world/go-hello-world/) for a simple example.

Current status: a subset of these hello-worlds already import their
matching SDK directly (`go`, `js`, `swift`, `c`, and the browser/web
pairing). The rest are still intentionally useful raw gRPC baselines.
The audited list lives in [`./sdk/SDK_GUIDE.md`](./sdk/SDK_GUIDE.md).

## UI examples : 

@bpds todo index the SWIFTUI apps, ... 

## Hello world Examples

- [C](./hello-world/c-hello-world/)
- [C++](./hello-world/cpp-hello-world/)
- [C#](./hello-world/csharp-hello-world/)
- [Dart](./hello-world/dart-hello-world/)
- [Go](./hello-world/go-hello-world/)
- [Java](./hello-world/java-hello-world/)
- [JavaScript](./hello-world/js-hello-world/)
- [Kotlin](./hello-world/kotlin-hello-world/)
- [Python](./hello-world/python-hello-world/)
- [Ruby](./hello-world/ruby-hello-world/)
- [Rust](./hello-world/rust-hello-world/)
- [Swift](./hello-world/swift-hello-world/)
- [Web (browser)](./hello-world/web-hello-world/)


