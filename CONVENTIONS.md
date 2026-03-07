# Conventions — Per-Language Holon Structure

Every holon follows a **universal directory structure** with
**language-idiomatic source and test layouts**. This document defines
both: the common skeleton and the per-language mapping.

---

## 1. Universal Structure

Regardless of language, every holon repository contains:

```
my-holon/
├── holon.yaml          ← identity + operational manifest (always present)
├── protos/             ← .proto source files
│   └── <package>/<version>/
│       └── <service>.proto
├── gen/                ← generated code (protoc output)
├── cmd/                ← CLI entry points (when applicable)
├── <idiomatic-src>/    ← source code (see §3)
├── <idiomatic-test>/ ← tests (language-specific)
└── <manifest>        ← dependency file (go.mod, pubspec.yaml, Cargo.toml, etc.)
```

### Rules

1. **`protos/`** is the single, universal location for `.proto` files.
   Same folder name, same position (holon root), every language. Proto
   files are organized by package and version:

   ```
   protos/
   └── echo/v1/
       └── echo.proto
   ```

2. **`gen/`** contains generated code from `protoc`. The internal
   structure follows the language idiom (see §3). Generated code is
   **committed** — it is part of the distribution. Consumers should not
   need `protoc` to use a holon.

3. **`cmd/`** contains CLI entry points. Each subdirectory is one binary.
   When a holon has no CLI facet, `cmd/` is absent.

4. **`holon.yaml`** is always at the holon root.

---

## 2. Global Runtime Directories

Organic Programming defines a user-local runtime home independent of
any implementation language. These directories are for installation,
cache, and runtime support files; they are not a source workspace and
they do not replace a language-native development layout such as
Go modules.

Organic Programming does not currently standardize project-local dotfiles
or hidden working directories such as `.holonconfig`, `.holonallow`,
`.holonignore`, or `.holon/`. Prefer visible repository paths such as
`holons/` and explicit CLI flags.

### `OPPATH`

`OPPATH` is the root directory for the local Organic Programming
environment.

- Default: `~/.op`
- Scope: per-user runtime home
- Purpose: anchors standard subdirectories such as `bin/` and `cache/`

### `OPBIN`

`OPBIN` is the standard install directory for Organic Programming
binaries and wrappers.

- Default: `$OPPATH/bin`
- Purpose: holds commands such as `op`, `who`, and other holon-facing
  executables installed for direct use

Shell environments should add `OPBIN` to `PATH` so these binaries are
discoverable without language-specific knowledge:

```sh
export OPPATH="${OPPATH:-$HOME/.op}"
export OPBIN="${OPBIN:-$OPPATH/bin}"
mkdir -p "$OPBIN"
export PATH="$OPBIN:$PATH"
```

When a tool is installed through Go, `GOBIN` may be pointed at `OPBIN`
for that installation step. `OPBIN` is the Organic Programming
convention; `GOBIN` remains a Go-specific implementation detail.

### Cache

When a holon is fetched as a dependency (by Atlas), it has its own
root in the global cache under `OPPATH`:

```
$OPPATH/cache/<host>/<owner>/<name>@<version>/
├── holon.yaml
├── protos/
├── gen/
└── <idiomatic-src>/
```

With the default runtime home, this resolves to:

```
~/.op/cache/<host>/<owner>/<name>@<version>/
```

Each cached holon is a self-contained directory. There is no merging,
no flattening — every dependency keeps its own structure. This mirrors
Go's module cache where each module version has its own directory tree.

~~See `DEPENDENCIES.md` (in marco-atlas) for the cache layout.~~ currently not public.

---

## 3. Per-Language Conventions

The following table maps each SDK to its idiomatic conventions. The
**source directory**, **generated code location**, **test directory**,
**manifest file**, and **build command** follow each language's
established practices.

### Go

| Aspect | Convention |
|--------|-----------|
| Source | `internal/` (private) + `pkg/` (public API) |
| Generated | `gen/go/` |
| Tests | `*_test.go` (co-located, Go convention) |
| Manifest | `go.mod` |
| Build | `go build ./...` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── gen/go/echo/v1/echo.pb.go
├── internal/          ← private implementation
│   └── server/server.go
├── pkg/               ← public API for compositing (see §7)
│   └── myholon/myholon.go
├── cmd/echo-server/main.go
└── go.mod
```

---

### Dart

| Aspect | Convention |
|--------|-----------|
| Source | `lib/` |
| Generated | `lib/gen/` |
| Tests | `test/` |
| Manifest | `pubspec.yaml` |
| Build | `dart compile exe` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── lib/
│   ├── gen/echo/v1/echo.pb.dart
│   └── src/server.dart
├── test/echo_test.dart
├── cmd/echo_server/main.dart
└── pubspec.yaml
```

---

### Python

| Aspect | Convention |
|--------|-----------|
| Source | `<package_name>/` (top-level package) |
| Generated | `gen/python/` |
| Tests | `tests/` |
| Manifest | `pyproject.toml` |
| Build | `pip install -e .` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── gen/python/echo/v1/echo_pb2.py
├── holons/
│   ├── __init__.py
│   └── server.py
├── tests/test_echo.py
├── cmd/echo_server.py
└── pyproject.toml
```

---

### Rust

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `src/gen/` |
| Tests | `tests/` (integration) + `src/*` (`#[cfg(test)]` inline) |
| Manifest | `Cargo.toml` |
| Build | `cargo build` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── src/
│   ├── gen/echo.v1.rs
│   ├── lib.rs
│   └── server.rs
├── tests/integration_test.rs
├── cmd/echo-server/main.rs
└── Cargo.toml
```

---

### JavaScript (Node)

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `src/gen/` |
| Tests | `tests/` |
| Manifest | `package.json` |
| Build | `npm run build` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── src/
│   ├── gen/echo/v1/echo_grpc_pb.js
│   └── server.js
├── tests/echo.test.js
├── cmd/echo-server/index.js
└── package.json
```

---

### JavaScript (Browser / WebSocket)

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `src/gen/` |
| Tests | `tests/` |
| Manifest | `package.json` |
| Build | `npm run build` |

Same layout as Node JS. The difference is transport (WebSocket + JSON-RPC
instead of gRPC over TCP), not file structure.

---

### Java

| Aspect | Convention |
|--------|-----------|
| Source | `src/main/java/` (Maven/Gradle standard) |
| Generated | `src/main/java/gen/` |
| Tests | `src/test/java/` |
| Manifest | `build.gradle` |
| Build | `gradle build` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── src/
│   ├── main/java/
│   │   ├── gen/echo/v1/EchoProto.java
│   │   └── com/example/Server.java
│   └── test/java/
│       └── com/example/EchoTest.java
├── cmd/echo-server/Main.java
└── build.gradle
```

---

### Kotlin

| Aspect | Convention |
|--------|-----------|
| Source | `src/main/kotlin/` |
| Generated | `src/main/kotlin/gen/` |
| Tests | `src/test/kotlin/` |
| Manifest | `build.gradle.kts` |
| Build | `gradle build` |

Same Maven/Gradle layout as Java, with `.kts` build script.

---

### C#

| Aspect | Convention |
|--------|-----------|
| Source | `<ProjectName>/` (e.g., `Holons/`) |
| Generated | `<ProjectName>/Gen/` |
| Tests | `<ProjectName>.Tests/` |
| Manifest | `*.csproj` / `*.sln` |
| Build | `dotnet build` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── Holons/
│   ├── Gen/Echo/V1/Echo.cs
│   └── Server.cs
├── Holons.Tests/EchoTest.cs
├── cmd/EchoServer/Program.cs
└── csharp-holons.sln
```

---

### Swift

| Aspect | Convention |
|--------|-----------|
| Source | `Sources/` |
| Generated | `Sources/Gen/` |
| Tests | `Tests/` |
| Manifest | `Package.swift` |
| Build | `swift build` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── Sources/
│   ├── Gen/Echo_V1_Echo.pb.swift
│   └── Server.swift
├── Tests/EchoTests/EchoTest.swift
├── cmd/echo-server/main.swift
└── Package.swift
```

---

### C

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `gen/c/` |
| Tests | `tests/` |
| Manifest | `Makefile` |
| Build | `make` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── gen/c/echo/v1/echo.pb-c.h
├── src/
│   └── server.c
├── tests/test_echo.c
├── cmd/echo-server/main.c
└── Makefile
```

---

### C++

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `gen/cpp/` |
| Tests | `tests/` |
| Manifest | `CMakeLists.txt` |
| Build | `cmake --build .` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── gen/cpp/echo/v1/echo.pb.h
├── src/
│   └── server.cpp
├── tests/test_echo.cpp
├── cmd/echo-server/main.cpp
└── CMakeLists.txt
```

---

### Objective-C

| Aspect | Convention |
|--------|-----------|
| Source | `src/` |
| Generated | `gen/objc/` |
| Tests | `tests/` |
| Manifest | `Makefile` or Xcode project |
| Build | `xcodebuild` or `make` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── gen/objc/Echo.pbobjc.h
├── src/
│   └── Server.m
├── tests/EchoTest.m
└── Makefile
```

---

### Ruby

| Aspect | Convention |
|--------|-----------|
| Source | `lib/` |
| Generated | `lib/gen/` |
| Tests | `test/` |
| Manifest | `Gemfile` |
| Build | `bundle exec` |

```
my-holon/
├── protos/echo/v1/echo.proto
├── lib/
│   ├── gen/echo/v1/echo_pb.rb
│   └── server.rb
├── test/test_echo.rb
├── cmd/echo-server/main.rb
└── Gemfile
```

---

## 4. Summary Matrix

| SDK | Source | Generated | Tests | Manifest |
|-----|--------|-----------|-------|----------|
| Go | `pkg/` | `gen/go/` | co-located `*_test.go` | `go.mod` |
| Dart | `lib/` | `lib/gen/` | `test/` | `pubspec.yaml` |
| Python | `<pkg>/` | `gen/python/` | `tests/` | `pyproject.toml` |
| Rust | `src/` | `src/gen/` | `tests/` + inline | `Cargo.toml` |
| JS (Node) | `src/` | `src/gen/` | `tests/` | `package.json` |
| JS (Web) | `src/` | `src/gen/` | `tests/` | `package.json` |
| Java | `src/main/java/` | `src/main/java/gen/` | `src/test/java/` | `build.gradle` |
| Kotlin | `src/main/kotlin/` | `src/main/kotlin/gen/` | `src/test/kotlin/` | `build.gradle.kts` |
| C# | `Holons/` | `Holons/Gen/` | `Holons.Tests/` | `*.csproj` |
| Swift | `Sources/` | `Sources/Gen/` | `Tests/` | `Package.swift` |
| C | `src/` | `gen/c/` | `tests/` | `Makefile` |
| C++ | `src/` | `gen/cpp/` | `tests/` | `CMakeLists.txt` |
| Obj-C | `src/` | `gen/objc/` | `tests/` | `Makefile` |
| Ruby | `lib/` | `lib/gen/` | `test/` | `Gemfile` |

---

## 5. Rules for Generated Code

1. **Generated code is committed.** It is part of the holon's
   distribution. A consumer cloning the repository should be able to
   build and run without installing `protoc`.

2. **Never edit generated code.** If the generated output needs
   changes, modify the `.proto` and regenerate. The `gen/` directory
   is a derived artifact of `protos/`.

3. **Regeneration command.** Each holon should document its protoc
   invocation (in `holon.yaml` or a `Makefile`/script) so that any
   actant can regenerate from source protos.

4. **The `gen/` directory mirrors the proto package structure.** If
   the proto declares `package echo.v1`, the generated files live
   under `.../echo/v1/` within `gen/`.

---

## 6. Build Artifacts & `.gitignore`

Every holon **MUST** have a `.gitignore` at its root that prevents build
artifacts from being committed. A clean working tree is essential for
the validate/fix workflow, where git is used as a scalpel (`git checkout`,
`git reset`).

### Common patterns (all languages)

```gitignore
# OS
.DS_Store
Thumbs.db

# IDE
.idea/
.vscode/
*.swp
*~
```

### Per-language artifact patterns

| Language | Patterns to ignore |
|----------|-------------------|
| Go | `*.exe`, `*.test`, `*.out`, `*.dll`, `*.so`, `*.dylib`, binary names |
| Dart | `.dart_tool/`, `.packages`, `build/`, `pubspec.lock` |
| Rust | `target/` |
| JS | `node_modules/`, `dist/`, `package-lock.json` |
| Python | `__pycache__/`, `*.pyc`, `*.pyo`, `.venv/`, `*.egg-info/`, `dist/` |
| Java | `build/`, `.gradle/`, `bin/`, `*.class`, `*.jar` |
| Kotlin | `build/`, `.gradle/`, `bin/`, `*.class`, `*.jar` |
| C# | `bin/`, `obj/`, `*.user`, `*.suo` |
| Swift | `.build/`, `.swiftpm/`, `Package.resolved` |
| C | `*.o`, `*.a`, `*.so`, binary names |
| C++ | `build/`, `*.o`, `*.a`, `*.so`, binary names |
| Obj-C | `*.o`, binary names, `build/`, `DerivedData/` |
| Ruby | `.bundle/`, `vendor/bundle/`, `Gemfile.lock` |

### Rules

1. **Holon-specific binaries.** Each holon names its compiled binaries
   explicitly (e.g., `/hello`, `/op`, `/who`). Using a leading `/`
   prevents matching files in subdirectories.

2. **Lock files are ignored.** Holons are libraries, not deployable
   applications. Lock files (`pubspec.lock`, `Cargo.lock`,
   `package-lock.json`, `Gemfile.lock`) are not committed.

3. **Generated code is committed** (see §5). The `gen/` directory is
   never in `.gitignore`.

---

## 7. Compositing: the Bridge Pattern

Holons may compose other holons **in-process** via `mem://` — a
`bufconn`-backed gRPC transport with zero network overhead. The SDK
provides `transport.NewMemListener()` and `grpcclient.DialMem()`.

Go's `internal/` visibility prevents cross-module imports. The
**bridge pattern** solves this: every holon that serves gRPC **MUST**
expose a `pkg/<name>/` package with a `Register` function.

### Convention

```go
// pkg/myholon/myholon.go
package myholon

import (
    pb "github.com/organic-programming/my-holon/gen/go/myholon/v1"
    "github.com/organic-programming/my-holon/internal/server"
    "google.golang.org/grpc"
)

// Register adds the service to a gRPC server.
// Used by compositor holons for mem:// in-process wiring.
func Register(gs *grpc.Server) {
    pb.RegisterMyHolonServiceServer(gs, server.New())
}
```

### Usage by a compositor holon

```go
import (
    "github.com/organic-programming/my-holon/pkg/myholon"
    "github.com/organic-programming/go-holons/pkg/transport"
    "github.com/organic-programming/go-holons/pkg/grpcclient"
)

mem := transport.NewMemListener()
gs := grpc.NewServer()
myholon.Register(gs)
go gs.Serve(mem)

conn, _ := grpcclient.DialMem(ctx, mem)
client := pb.NewMyHolonServiceClient(conn)
```

### Rules

1. **`Register(gs)` is the only exported function required.** Keep the
   public API surface minimal — the proto stubs define the contract.
2. **`internal/` stays internal.** All implementation lives in
   `internal/`. The `pkg/` bridge is a thin adapter, not a second API.
3. **Same `go.mod` module.** The bridge lives in the same module as
   the holon — no separate package.
4. **Compositor holons use `replace` directives** in `go.mod` for local
   development ~~(see `DEPENDENCIES.md` in marco-atlas)~~ currently not public.
