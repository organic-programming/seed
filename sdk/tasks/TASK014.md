# TASK014 — `op inspect` Command

## Context

`op inspect` provides offline API documentation for any holon by
reading its `.proto` files directly from the filesystem. The holon
does not need to be running.

This replaces the earlier `op describe` concept (which called the
`Describe` RPC). `op inspect` works offline, produces richer output
(including `@required` and `@example` tags), and falls back to
`Describe` RPC only when given a `host:port` address.

See `OP.md` §14 "Introspection" for the full specification.

## Workspace

- `op` source: `holons/grace-op/`
- Reference: `OP.md` §14, `PROTOCOL.md` §3.5 (proto definition)
- Proto parser: reuse or create `pkg/inspect/` in `grace-op`

## What to implement

### CLI interface

```bash
op inspect <slug>              # human-readable API reference
op inspect <slug> --json       # structured JSON output
op inspect <host:port>         # fallback: call Describe RPC
```

### Resolution logic

1. If `<slug>` contains `:` → treat as `host:port`, call `Describe` RPC.
2. Else → use `discover.FindBySlug(slug)` to locate the holon directory.
3. Read `protos/` from the holon directory.
4. Parse `.proto` files — extract services, methods, types, comments.
5. Extract `@required` and `@example` tags from comments.
6. Format and print.

### Proto parser

Create `pkg/inspect/parser.go` — a Go proto parser that extracts:
- `service Name { ... }` blocks with leading comments
- `rpc Method(Input) returns (Output)` with leading comments
- `message Name { ... }` blocks with field names, types, numbers, comments
- `@required` and `@example` tags from comment lines

Use `github.com/jhump/protoreflect/desc/protoparse` or
`github.com/bufbuild/protocompile` for robust parsing, or a
simpler text-based parser if dependencies are a concern.

### Human-readable output format

```
rob-go — Build what you mean.

  rob_go.v1.RobGoService
    Wraps the go command for gRPC access.

    Build(BuildRequest) → BuildResponse
      Compile Go packages.

      Request:
        package  string  [required]  The Go package to build.
                                     @example "./cmd/rob"

      Response:
        output   string  Compiler output.
        success  bool    Whether the build succeeded.
```

### JSON output

Structured JSON with the same information (services, methods,
fields, types, required flags, examples).

### Describe RPC fallback

When `<slug>` contains `:` (host:port), use `connect.Connect` to dial
the running holon and call `HolonMeta.Describe`. Format the
`DescribeResponse` the same way as parsed proto output.

## Testing

1. Parse a holon's protos (e.g. echo-server), verify correct output.
2. Parse protos with `@required` and `@example` tags, verify extraction.
3. Verify `--json` produces valid JSON.
4. Verify `host:port` mode calls Describe RPC.
5. Verify clean error for nonexistent slug.
6. Verify skills from `holon.yaml` are displayed after the API reference.

## Rules

- Add to the existing `op` command dispatch in Grace.
- The proto parser module (`pkg/inspect/`) will be reused by TASK015
  (`op mcp`/`op tools`).
- Read `skills` from `holon.yaml` and display after the proto output.
- Follow existing `op` CLI code style.
