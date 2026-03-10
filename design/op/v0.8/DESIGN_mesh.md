# `op mesh` — Distributed Topology Management for Grace

## Overview

`op mesh` is a new verb family in Grace that manages the **network of hosts** running holons together. It handles certificate generation, host registration, remote provisioning, and topology introspection — everything needed to go from single-host to multi-host holon execution.

**Key principle**: `op mesh` is an **operator tool**, not a runtime service. It runs at setup time to configure the mesh. Once configured, holons communicate autonomously via mTLS-secured gRPC using the SDK's `discover` + `connect`.

## Scope

`op mesh` targets **small, intentional networks of known hosts** — a team's build farm, a multi-region deployment of 5–10 machines, or a dev environment spanning a laptop and a few servers. Every host is explicitly added by the operator.

This is a deliberate design boundary. There is no auto-discovery, no gossip protocol, no distributed consensus. The mesh is **human-readable** (`mesh.yaml` stays under 50 lines), **fully debuggable** (`op mesh status` pings everything), and **zero-infrastructure** (no etcd, no consul, no service mesh). Large-scale swarm orchestration is out of scope.

## Command Surface

```
op mesh
├── init                              # Create a private CA for the mesh
├── add <host> [--deploy]             # Register a host + generate its certificate
├── remove <host>                     # Revoke a host's certificate
├── list                              # Show all registered hosts
├── status                            # Ping all hosts, report health
└── describe <host>                   # Show holons running on a specific host
```

---

## Commands in Detail

### `op mesh init`

Creates the mesh Certificate Authority. Run once on the operator's machine.

```bash
op mesh init
```

**Produces:**
```
~/.op/mesh/
├── ca.key          # CA private key (GUARD THIS)
├── ca.crt          # CA public certificate (distributed to all hosts)
└── mesh.yaml       # Empty registry, ready for hosts
```

**Implementation**: Pure Go using `crypto/x509`, `crypto/ecdsa` (P-256), and `encoding/pem`. No `openssl` dependency.

**CA parameters:**
- Algorithm: ECDSA P-256 (fast, secure, small certs)
- Validity: 10 years
- Subject: `CN=OP Mesh CA`

---

### `op mesh add <host> [--deploy]`

Generates a certificate for a host and registers it in the mesh.

```bash
# Generate cert only (manual transfer)
op mesh add lyon.example.com

# Generate + deploy via SSH
op mesh add lyon.example.com --deploy

# With custom SSH user
op mesh add lyon.example.com --deploy --user=op

# With custom gRPC port
op mesh add lyon.example.com --port=9443
```

**Produces** (locally):
```
~/.op/mesh/hosts/
└── lyon.example.com/
    ├── host.key        # Host private key
    └── host.crt        # Host certificate (signed by CA)
```

**Updates** `~/.op/mesh/mesh.yaml` with the new entry.

**With `--deploy`**, additionally:
1. SSH into the remote host
2. Create `~/.op/mesh/` on the remote
3. Copy `host.key`, `host.crt`, and `ca.crt`
4. Verify the files are in place

**Certificate parameters:**
- Algorithm: ECDSA P-256
- Validity: 1 year (renewable)
- Subject: `CN=<hostname>`
- SAN (Subject Alternative Names): DNS name + IP if resolvable

**SSH deploy** uses Go's `golang.org/x/crypto/ssh` — no dependency on the system `ssh` binary. Reads `~/.ssh/id_ed25519` or `~/.ssh/id_rsa` by default, supports `--key` override.

---

### `op mesh remove <host>`

Revokes a host's certificate and removes it from the registry.

```bash
op mesh remove lyon.example.com
```

**Actions:**
1. Deletes local cert files for the host
2. Removes the entry from `mesh.yaml`
3. Adds the cert serial to `~/.op/mesh/revoked.yaml` (simple CRL)
4. Optionally `--deploy` to delete certs from the remote host

> [!NOTE]
> At the target scale (up to ~10 hosts), revocation via registry removal is sufficient. The SDK checks `mesh.yaml` — if a host isn't listed, it's unreachable regardless of its certificate. A full CRL mechanism is out of scope.

---

### `op mesh list`

Displays the current mesh topology.

```bash
op mesh list
```

```
MESH: OP Mesh CA (created 2026-03-09)

HOST                       PORT   CERT EXPIRES   STATUS
paris.example.com          9090   2027-03-09     –
lyon.example.com           9090   2027-03-09     –
bordeaux.example.com       9443   2027-03-09     –
```

---

### `op mesh status`

Pings all registered hosts and reports health.

```bash
op mesh status
```

```
HOST                       PORT   HOLONS   LATENCY   STATUS
paris.example.com          9090   3        2ms       ✅ healthy
lyon.example.com           9090   1        45ms      ✅ healthy
bordeaux.example.com       9443   –        –         ❌ unreachable
```

**Implementation**: connects to each host's gRPC endpoint using mTLS, calls the standard gRPC health check (`grpc.health.v1.Health/Check`). Optionally calls `HolonMeta/Describe` to enumerate running holons.

---

### `op mesh describe <host>`

Shows details of a specific host and its holons.

```bash
op mesh describe lyon.example.com
```

```
HOST: lyon.example.com:9090
CERT: valid until 2027-03-09 (365 days remaining)
OS:   linux/amd64

HOLONS:
  NAME           VERSION   STATUS
  phill-files    0.1.0     serving
  line-git       0.2.0     serving
```

---

## Data Model

### `~/.op/mesh/mesh.yaml`

```yaml
ca:
  cert: ~/.op/mesh/ca.crt
  created: 2026-03-09T12:00:00Z

hosts:
  - address: paris.example.com
    port: 9090
    cert: ~/.op/mesh/hosts/paris.example.com/host.crt
    added: 2026-03-09T12:01:00Z

  - address: lyon.example.com
    port: 9090
    cert: ~/.op/mesh/hosts/lyon.example.com/host.crt
    added: 2026-03-09T12:02:00Z

  - address: bordeaux.example.com
    port: 9443
    cert: ~/.op/mesh/hosts/bordeaux.example.com/host.crt
    added: 2026-03-09T12:03:00Z
```

### Remote host file layout

```
~/.op/mesh/           # on each remote host
├── ca.crt            # CA certificate (trust anchor)
├── host.key          # This host's private key
└── host.crt          # This host's certificate
```

---

## SDK Integration

### Enhanced `discover`

The SDK's `discover` function gains a `mesh.yaml`-aware path:

```
discover("phill-files") search order:
1. Local OPPATH scan (existing behavior)
2. Read ~/.op/mesh/mesh.yaml
3. For each host, query HolonMeta/Describe (cached)
4. Return matching holon with remote address
```

### Enhanced `connect`

When connecting to a remote holon, the SDK automatically uses mTLS:

```
connect("phill-files") behavior:
1. If local → existing behavior (stdio/unix/tcp)
2. If remote → load host.key + host.crt + ca.crt
                dial with mTLS credentials
```

### `serve.Run` mTLS mode

When a holon detects mesh certificates in `~/.op/mesh/`, it enables mTLS:

```go
// Inside serve.Run (SDK internal)
if meshCerts := detectMeshCerts(); meshCerts != nil {
    // Serve with mTLS — require client certificates signed by our CA
    tlsConfig := meshCerts.ServerTLSConfig()
    server = grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
}
```

No configuration needed in `holon.yaml` — the SDK detects the mesh automatically.

---

## Security Model

### Per-Listener Security Policy

Each listener declared in `holon.yaml` gets a security mode. The existing N-listener architecture (PROTOCOL.md §2.2) stays unchanged — the security mode is a per-listener annotation.

| Mode | Encryption | Authentication | Use case |
|---|---|---|---|
| `none` | None | None | Local transports (`stdio://`, `unix://`, `mem://`) |
| `mesh` | mTLS (TLS 1.3) | Mutual certificates from mesh CA | Holon-to-holon across hosts |
| `public` | TLS (standard) | API key / JWT / OAuth via interceptor | Internet-facing API |

```yaml
# holon.yaml — example with all three modes
serve:
  listeners:
    - uri: tcp://:9090
      security: mesh
    - uri: tcp://:443
      security: public
      auth: api-key
    - uri: unix:///tmp/holon.sock
      security: none
```

For the `public` mode (exposing a holon to external consumers with per-consumer confidentiality), see [DESIGN_public_holons.md](./DESIGN_public_holons.md).

### Mesh Security Properties

| Property | Mechanism |
|---|---|
| **Encryption** | TLS 1.3 (AES-256-GCM) — all traffic encrypted |
| **Server authentication** | Client verifies server cert against CA |
| **Client authentication** | Server verifies client cert against CA (mutual) |
| **Authorization** | Holon-level ACL (e.g., Phill's path rules) — separate concern |
| **CA protection** | `ca.key` stays on operator's machine, never deployed |
| **Revocation** | Remove from `mesh.yaml` + delete remote certs |
| **Cert renewal** | `op mesh add <host> --deploy --force` (reissue + redeploy) |

---

## Implementation Roadmap

### Phase 1: Foundation
- `op mesh init` (CA generation)
- `op mesh add <host>` (cert generation, local only)
- `mesh.yaml` registry
- `op mesh list`

### Phase 2: Deployment
- `--deploy` flag (SSH-based cert provisioning)
- `op mesh remove`
- `op mesh add --force` (cert renewal)

### Phase 3: Introspection
- `op mesh status` (health check via gRPC)
- `op mesh describe` (remote holon enumeration)

### Phase 4: SDK Integration
- Enhanced `discover` (mesh-aware remote resolution)
- Enhanced `connect` (automatic mTLS for remote holons)
- `serve.Run` mTLS auto-detection

---

## Dependencies

| Dependency | Source | Purpose |
|---|---|---|
| `crypto/x509` | Go stdlib | Certificate generation and parsing |
| `crypto/ecdsa` | Go stdlib | Key generation (P-256) |
| `crypto/tls` | Go stdlib | mTLS configuration |
| `encoding/pem` | Go stdlib | Certificate encoding |
| `golang.org/x/crypto/ssh` | Go extended | SSH client for `--deploy` |
| `gopkg.in/yaml.v3` | Third party (already used) | `mesh.yaml` parsing |

**No `openssl` binary required.** Everything is pure Go.
