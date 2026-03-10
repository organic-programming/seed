# Design Docs — Implementation Gaps

Features defined in the 4 DESIGN documents that are **not implemented**
and **not planned** in any v0.3 task.

v0.3 only covers **spec documentation** for these areas:
- TASK06 writes mesh/transport/security spec docs
- TASK07 writes setup/provisioning spec docs

All **code implementation** below is unplanned.

---

## DESIGN_mesh.md

### `op mesh` command suite

| Feature | Description |
|---|---|
| `op mesh init` | Generate private CA (ECDSA P-256, pure Go `crypto/x509`) |
| `op mesh add <host>` | Generate host certificate, sign with CA, register in `mesh.yaml` |
| `op mesh add --deploy` | SSH-based cert provisioning (`golang.org/x/crypto/ssh`) |
| `op mesh remove <host>` | Revoke cert, remove from registry, optional remote cleanup |
| `op mesh list` | Display topology table from `mesh.yaml` |
| `op mesh status` | mTLS health check via `grpc.health.v1.Health/Check` per host |
| `op mesh describe <host>` | Remote holon enumeration via `HolonMeta/Describe` |
| `mesh.yaml` parser | YAML registry: CA ref, host list with address/port/cert path |
| `~/.op/mesh/` directory | Layout: `ca.key`, `ca.crt`, `mesh.yaml`, `hosts/<name>/` |
| `revoked.yaml` | Simple CRL for removed hosts |
| Cert renewal | `op mesh add --force` to reissue + redeploy |

### SDK mesh integration

| Feature | Description |
|---|---|
| Mesh-aware `discover` | Extend resolution: local → `mesh.yaml` → remote `HolonMeta/Describe` |
| Mesh-aware `connect` | Auto-load `host.key` + `host.crt` + `ca.crt` for mTLS dial |
| `serve.Run` mTLS | Auto-detect `~/.op/mesh/` certs, enable `RequireAndVerifyClientCert` |

---

## DESIGN_public_holons.md

### Per-listener security in SDK

| Feature | Description |
|---|---|
| `serve.listeners` in `holon.yaml` | Parse N-listener declarations with `security` + `auth` fields |
| `security: none` | No TLS for local transports — auto-detect `stdio://`, `unix://` |
| `security: mesh` | Load mesh certs, configure mTLS on TCP listener |
| `security: public` | Load standard TLS cert, attach auth interceptor |
| Multi-listener `serve.Run` | Start all listeners on one gRPC server with different TLS configs |

### Auth interceptors

| Feature | Description |
|---|---|
| API key interceptor | Validate `x-api-key` metadata against `serve.api_keys` in `holon.yaml` |
| JWT interceptor | Validate Bearer token signature + claims |
| OAuth interceptor | Validate Bearer token against external JWKS endpoint |
| Consumer identity on context | Attach authenticated consumer ID to gRPC context for holon logic |

### TLS cert configuration

| Feature | Description |
|---|---|
| `serve.tls.cert` / `serve.tls.key` | Path to TLS cert/key for public listeners in `holon.yaml` |

### Open questions (unresolved)

1. API key storage: `holon.yaml` vs separate `secrets.yaml`
2. Built-in rate limiting per consumer
3. `gRPC-Web` / Connect protocol on public listeners

---

## DESIGN_setup.md

### `op setup` command

| Feature | Description |
|---|---|
| `op setup <image.yaml>` | Parse and execute a declarative provisioning image |
| `op setup` (no arg) | Auto-discover `./setup.yaml` or `~/.op/setup.yaml` |
| Image file parser | YAML: `name`, `toolchains`, `holons`, `platform`, `mesh`, `include` |

### 6-phase execution engine

| Phase | Description |
|---|---|
| 1. Resolve | Fetch holon manifests, build full dependency graph |
| 2. Toolchains | Download/verify Go, Rust from official sources |
| 3. System deps | Install via platform pkg manager (`brew`, `apt`, `winget`) |
| 4. Holons | `go install` / `cargo install` / source build per holon |
| 5. Environment | Verify `OPPATH`, `OPBIN`, `PATH` |
| 6. Mesh | Optional: `op mesh add --deploy` to join mesh |

### Dependency resolution

| Feature | Description |
|---|---|
| Cross-category resolution | Toolchains → system commands → delegated commands → source builds → holon deps |
| Bootstrap driver | Built-in minimal `brew install`/`apt install`/`winget install` for bootstrapping |
| `requires.sources` in `holon.yaml` | Clone repo, checkout pinned ref, build via declared runner |
| Source cache | `~/.op/cache/sources/<name>` |
| Ref pinning validation | `op check` rejects floating branch refs |

### Provisioning logic

| Feature | Description |
|---|---|
| Idempotency | Version-check before install, skip if present |
| Multi-image composition | `include:` to layer images, merge with last-wins |
| Platform overrides | `platform.darwin`, `platform.windows` per-OS holon lists |
| Holon install method selection | Priority: prebuilt binary > `go install` / `cargo install` > source build |

### Open questions (unresolved)

1. `requires.sources` placement: `holon.yaml` vs separate file
2. Toolchain install method: direct download vs platform pkg manager
3. Wrapper holon install: system binary AND wrapper, or just system binary
4. Image registry: local files only, or shareable
5. Rollback on partial failure

---

## DESIGN_transport_rest_sse.md

### REST + SSE transport adapter

| Feature | Description |
|---|---|
| Unary RPC → POST mapping | `POST /v1/<service>/<method>` with JSON body |
| Server-streaming → SSE | `GET /v1/<service>/<method>` with `text/event-stream` |
| Client-streaming → chunked POST | `Transfer-Encoding: chunked` or multipart |
| Proto ↔ JSON transcoding | `protojson` encoding for SSE text-only constraint |

### SDK integration

| Feature | Description |
|---|---|
| REST+SSE transport in `serve.Run` | Additional HTTP handler alongside gRPC server |
| REST+SSE transport in `connect` | HTTP client adapter for remote holon calls |
| mTLS over REST+SSE | Standard HTTPS with mutual TLS for cross-network |

### No open questions — design is advisory

This document is a **transport analysis** recommending REST+SSE as the
default distributed transport. It does not define a specific API surface
beyond the RPC mapping pattern.

---

## Summary

| Design doc | Documented by v0.3 | Implemented by v0.3 |
|---|---|---|
| DESIGN_mesh | TASK06 (spec docs) | ❌ Nothing |
| DESIGN_public_holons | TASK06 (spec docs) | ❌ Nothing |
| DESIGN_setup | TASK07 (spec docs) | ❌ Nothing |
| DESIGN_transport_rest_sse | TASK06 (spec docs) | ❌ Nothing |
