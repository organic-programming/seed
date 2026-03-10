# TASK006 — Implement `connect` in `rust-holons`

## Context

The Organic Programming SDK fleet requires a `connect` module in every SDK.
`connect` composes discover + start + dial into a single name-based resolution
primitive. See `AGENT.md` Article 11 "Connect — Name-Based Resolution".

The **reference implementation** is `go-holons/pkg/connect/connect.go` — study
it before starting.

## Workspace

- SDK root: `sdk/rust-holons/`
- Existing modules: `src/discover.rs`, `src/identity.rs`, `src/serve.rs`, `src/transport.rs`
- Reference: `sdk/go-holons/pkg/connect/connect.go`
- Spec: `sdk/TODO_CONNECT.md` § `rust-holons`

## What to implement

Create `src/connect.rs` and add it to `src/lib.rs`.

### Public API

```rust
pub async fn connect(target: &str) -> Result<tonic::transport::Channel>
pub async fn connect_with_opts(target: &str, opts: ConnectOptions) -> Result<Channel>
pub async fn disconnect(channel: Channel) -> Result<()>
```

### Resolution logic

1. If `target` contains `:` or `://` → direct dial (treat as `host:port`).
2. Else → it's a holon slug:
   a. Call `discover::find_by_slug(target)` to locate the holon.
   b. Check port file at `.op/run/<slug>.port` — if exists AND server responds, reuse.
   c. If not running and `opts.start == true` → find binary via `artifacts.binary`
      from `holon.yaml`, launch with `serve --listen stdio://` (default) and
      dial over the child's pipes. TCP fallback: `serve --listen tcp://127.0.0.1:0`.
   d. Dial via stdio pipe (default) or `tonic::transport::Channel::from_shared(uri)` (tcp).
3. Return ready channel.

### ConnectOptions

```rust
pub struct ConnectOptions {
    pub timeout: Duration,      // default 5s
    pub transport: String,      // "stdio" (default) or "tcp"
    pub start: bool,            // true = start if not running (default true)
    pub port_file: Option<String>,
}
```

### Process management

- Use `tokio::process::Command` to start holon binary.
- Track started processes in a module-level `Mutex<HashMap<*, ProcessHandle>>`.
- `disconnect()`: close channel, if ephemeral → send SIGTERM, wait 2s, then SIGKILL.
- Clean up stale port files (port file exists but process is dead).

### Port file convention

Path: `$CWD/.op/run/<slug>.port`
Content: `tcp://127.0.0.1:<port>\n`

## Testing

1. **Direct dial test**: manually start `echo-server`, call `connect("localhost:PORT")`,
   verify channel, disconnect.
2. **Slug resolution test**: create temp tree with `holon.yaml`, call `connect("slug")`,
   verify binary started, channel works, disconnect kills process.
3. **Port file reuse test**: start holon, write port file, call `connect("slug")`,
   verify no new process spawned.
4. **Stale port file test**: write port file for dead PID, call `connect("slug")`,
   verify cleanup and fresh start.

## Rules

- Follow existing code style in `src/discover.rs`.
- Do not modify `src/transport.rs` or `src/serve.rs`.
- Add `tokio` process dependency if not already present in `Cargo.toml`.
- Run `cargo test` — all existing tests must still pass.
- Run `cargo clippy` — no new warnings.
