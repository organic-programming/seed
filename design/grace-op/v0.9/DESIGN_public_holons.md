# Public Holons — Exposing Services to External Consumers

## Problem

A holon may need to serve two audiences simultaneously:
- **Mesh peers** — other holons on known hosts, authenticated via mTLS
- **Public consumers** — external clients on the internet, authenticated via credentials (API key, JWT)

Each consumer's data must remain confidential — consumer A cannot see consumer B's traffic.

## Solution: Per-Listener Security Policy

The existing transport architecture already supports N listeners via `--listen` URIs.
The security policy extends this with a **per-listener `security` annotation** that tells the SDK
what authentication and encryption to apply on each listener.

### Security Modes

| Mode | Encryption | Authentication | Use case |
|---|---|---|---|
| `none` | None | None | Local transports (`stdio://`, `unix://`, `mem://`) |
| `mesh` | mTLS (TLS 1.3) | Mutual certificates from mesh CA | Holon-to-holon on the mesh |
| `public` | TLS (standard) | API key, JWT, or OAuth via interceptor | Internet-facing API |

### Configuration in `holon.yaml`

```yaml
serve:
  listeners:
    - uri: tcp://:9090
      security: mesh            # mTLS — only mesh peers with valid certs

    - uri: tcp://:443
      security: public          # standard TLS + credential validation
      auth: api-key             # auth strategy: api-key | jwt | oauth

    - uri: unix:///tmp/holon.sock
      security: none            # local IPC, no encryption

    - uri: stdio://
      security: none            # pipes, inherently local
```

### SDK Behavior per Security Mode

#### `none`
- No TLS, no authentication
- Default for `stdio://`, `unix://`, `mem://`
- SDK applies this automatically for local schemes (no config needed)

#### `mesh`
- SDK loads `~/.op/mesh/{ca.crt, host.key, host.crt}`
- Configures TLS 1.3 with `RequireAndVerifyClientCert`
- Rejects any connection without a valid mesh CA-signed certificate
- Auto-detected: if mesh certs exist and scheme is `tcp://`, SDK defaults to `mesh`

#### `public`
- SDK loads a standard TLS certificate (Let's Encrypt or custom)
- TLS cert path configurable: `tls.cert` and `tls.key` in `holon.yaml`
- Applies an **auth interceptor** based on the `auth` field
- The interceptor validates credentials before any RPC reaches holon logic

### Auth Strategies for Public Listeners

| Strategy | `auth` value | How it works |
|---|---|---|
| **API Key** | `api-key` | Client sends key in `x-api-key` gRPC metadata. SDK validates against a local key store. |
| **JWT** | `jwt` | Client sends Bearer token in `authorization` metadata. SDK validates signature + claims. |
| **OAuth** | `oauth` | Client sends Bearer token. SDK validates against an OAuth provider's JWKS endpoint. |

Auth config in `holon.yaml`:

```yaml
serve:
  listeners:
    - uri: tcp://:443
      security: public
      auth: api-key

  tls:
    cert: /etc/letsencrypt/live/myholon.example.com/fullchain.pem
    key: /etc/letsencrypt/live/myholon.example.com/privkey.pem

  api_keys:
    - name: consumer-alpha
      key: sk_live_abc123...
      scopes: [read]
    - name: consumer-beta
      key: sk_live_def456...
      scopes: [read, write]
```

### Consumer Confidentiality

TLS guarantees per-connection confidentiality by design:
- Each consumer's TLS session uses **unique session keys** derived from the handshake
- Consumer A's traffic is **cryptographically independent** from consumer B's
- An observer capturing both streams cannot cross-decrypt
- The holon (server) sees all plaintext — but consumers cannot see each other's data

No additional encryption is needed for consumer-to-consumer confidentiality. TLS handles it.

### Per-Consumer Isolation at Application Level

Transport-level confidentiality protects the wire. Application-level isolation ensures the **holon's business logic** scopes data correctly:

- The auth interceptor attaches consumer identity to the gRPC context
- Holon logic reads the consumer identity and scopes operations accordingly
- Example: Phill serves different paths per API key, preventing cross-consumer data access

This is the **holon developer's responsibility**, not the SDK's. The SDK provides the identity; the holon enforces the boundary.

### How the SDK Wires It

```go
// serve.Run reads holon.yaml listeners and configures each one
serve.Run(func(s *grpc.Server) {
    phillpb.RegisterFileSystemServer(s, &myServer{})
})

// Internally, serve.Run does:
// 1. Parse holon.yaml listeners
// 2. For each listener:
//    a. security: none  → plain listener
//    b. security: mesh  → load mesh certs, configure mTLS
//    c. security: public → load TLS cert, attach auth interceptor
// 3. Start all listeners on the same gRPC server
```

The holon developer writes **zero security code**. They declare listeners in `holon.yaml` and implement business logic. The SDK handles TLS, mTLS, and auth.

## Relationship to `op mesh`

| Concern | Tool |
|---|---|
| Generating mesh certificates (CA + host) | `op mesh init`, `op mesh add` |
| Deploying mesh certs to remote hosts | `op mesh add --deploy` |
| Configuring which listeners use which security | `holon.yaml` |
| Obtaining public TLS certificates | External (Let's Encrypt, certbot) |
| Managing API keys / JWT secrets | `holon.yaml` or external secret store |

## Open Questions

1. **API key storage** — is `holon.yaml` the right place for API keys, or should they live in a separate `secrets.yaml` (not committed to git)?
2. **Rate limiting** — should the `public` security mode include built-in rate limiting per consumer, or is that a separate concern?
3. **gRPC-Web / Connect on public listeners** — should a public listener automatically support Connect protocol for browser consumers, or is that a separate listener?
