# TASK004 — Implement `connect` in `ruby-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`connect` composes discover + start + dial into a single name-based resolution
primitive. See `AGENT.md` Article 11 "Connect — Name-Based Resolution".

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it before starting.

## Workspace

- SDK root: `sdk/ruby-holons/`
- Existing modules: `lib/holons/discover.rb`, `lib/holons/identity.rb`,
  `lib/holons/serve.rb`, `lib/holons/transport.rb`
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `ruby-holons`

## What to implement

Create `lib/holons/connect.rb` and require it from the main `lib/holons.rb`.

### Public API

```ruby
module Holons
  def self.connect(target) → GRPC::Core::Channel
  def self.connect(target, opts) → GRPC::Core::Channel
  def self.disconnect(channel)
end
```

### Resolution logic

Same 3-step algorithm as reference:
1. `target` contains `:` → direct dial.
2. Else → slug → `Holons.discover_by_slug(target)` → port file → start → dial.

### Process management

- Use `Process.spawn` to launch the binary.
- Track started PIDs in a module-level `Hash`.
- `disconnect()`: close channel, if ephemeral → `Process.kill("TERM", pid)`,
  wait 2s, then `Process.kill("KILL", pid)`.

### Port file convention

Path: `$CWD/.op/run/<slug>.port`
Content: `tcp://127.0.0.1:<port>\n`

## Testing

Follow `rake test` patterns from existing test files.

1. Direct dial test
2. Slug resolution test
3. Port file reuse test
4. Stale port file cleanup test

## Rules

- Follow existing code style in `discover.rb`.
- Use the `grpc` gem for channel creation.
- Run `rake test` — all existing tests must still pass.
