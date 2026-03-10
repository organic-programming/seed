# OP.md §11 — Mesh (Draft)

> This section is designed to be inserted into `OP.md` as section 11,
> after "Serve & Dial". Existing §11–§15 become §12–§16.

---

## 11. Mesh

`op mesh` manages the **network topology** — the set of hosts that
run holons together. It generates certificates, registers hosts,
deploys credentials via SSH, and introspects the running mesh.

`op mesh` is an **operator tool**, not a runtime service. It runs at
setup time to configure the mesh. Once configured, holons communicate
autonomously via mTLS using the SDK's `discover` + `connect`.

### Design boundary

`op mesh` targets **small, intentional networks of known hosts** — a
team's build farm, a multi-region deployment of 5–10 machines, or a
dev environment spanning a laptop and a few servers. Every host is
explicitly added by the operator.

There is no auto-discovery, no gossip protocol, no distributed
consensus. The mesh is **human-readable** (`mesh.yaml` stays under
50 lines), **fully debuggable** (`op mesh status` pings everything),
and **zero-infrastructure** (no etcd, no consul, no service mesh).

### 11.1 `op mesh init`

Creates the mesh Certificate Authority. Run once on the operator's
machine.

```bash
op mesh init
```

Produces:
```
~/.op/mesh/
├── ca.key          # CA private key (GUARD THIS)
├── ca.crt          # CA public certificate (distributed to all hosts)
└── mesh.yaml       # Empty registry, ready for hosts
```

Implementation: pure Go using `crypto/x509`, `crypto/ecdsa` (P-256).
No `openssl` dependency.

CA parameters:
- Algorithm: ECDSA P-256
- Validity: 10 years
- Subject: `CN=OP Mesh CA`

### 11.2 `op mesh add <host>`

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

Produces (locally):
```
~/.op/mesh/hosts/
└── lyon.example.com/
    ├── host.key        # Host private key
    └── host.crt        # Host certificate (signed by CA)
```

Updates `~/.op/mesh/mesh.yaml` with the new entry.

With `--deploy`, additionally:
1. SSH into the remote host
2. Create `~/.op/mesh/` on the remote
3. Copy `host.key`, `host.crt`, and `ca.crt`
4. Verify the files are in place

Certificate parameters:
- Algorithm: ECDSA P-256
- Validity: 1 year (renewable)
- Subject: `CN=<hostname>`
- SAN: DNS name + IP if resolvable

SSH uses Go's `golang.org/x/crypto/ssh` — no system `ssh` binary.
Reads `~/.ssh/id_ed25519` or `~/.ssh/id_rsa` by default, supports
`--key` override.

| Flag | Default | Description |
|---|---|---|
| `--deploy` | false | SSH-deploy certs to the remote host |
| `--user` | current user | SSH username |
| `--key` | `~/.ssh/id_ed25519` | SSH private key path |
| `--port` | 9090 | gRPC listen port on the remote |
| `--force` | false | Re-generate cert (renewal) |

### 11.3 `op mesh remove <host>`

Revokes a host's certificate and removes it from the registry.

```bash
op mesh remove lyon.example.com
```

Actions:
1. Deletes local cert files for the host
2. Removes the entry from `mesh.yaml`
3. Adds the cert serial to `~/.op/mesh/revoked.yaml`
4. With `--deploy`: deletes certs from the remote host

### 11.4 `op mesh list`

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

### 11.5 `op mesh status`

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

Connects to each host via mTLS gRPC, calls
`grpc.health.v1.Health/Check`. Optionally calls
`HolonMeta/Describe` to count running holons.

### 11.6 `op mesh describe <host>`

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

### 11.7 `mesh.yaml` format

See [MESH_YAML.md](./MESH_YAML.md) for the full schema reference.

```yaml
ca:
  cert: ~/.op/mesh/ca.crt
  created: 2026-03-09T12:00:00Z

hosts:
  - address: paris.example.com
    port: 9090
    cert: ~/.op/mesh/hosts/paris.example.com/host.crt
    added: 2026-03-09T12:01:00Z
```

### Complete `op mesh` reference

| Command | Description |
|---|---|
| `op mesh init` | Create private CA |
| `op mesh add <host>` | Register host + generate cert |
| `op mesh remove <host>` | Revoke host cert + remove from registry |
| `op mesh list` | Show all registered hosts |
| `op mesh status` | Health-check all hosts |
| `op mesh describe <host>` | Show host details + holon inventory |
