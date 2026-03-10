# HOLON_YAML.md — `serve.listeners` Additions (Draft)

> These fields are designed to be added to `HOLON_YAML.md` in the
> operational section, after the current `build` and `requires` fields.

---

## Serve Configuration

### `serve.listeners`

Declare how the holon listens for incoming connections. Each listener
specifies a transport URI and a security mode.

```yaml
serve:
  listeners:
    - uri: tcp://:9090
      security: mesh

    - uri: tcp://:443
      security: public
      auth: api-key

    - uri: unix:///tmp/holon.sock
      security: none

    - uri: stdio://
      security: none
```

| Field | Type | Required | Description |
|---|---|---|---|
| `serve.listeners` | list | no | Listener declarations. If omitted, SDK uses defaults. |
| `serve.listeners[].uri` | string | yes | Transport URI (see PROTOCOL.md §2) |
| `serve.listeners[].security` | enum | no | Security mode: `none`, `mesh`, `public`. Default: auto-detect. |
| `serve.listeners[].auth` | enum | no | Auth strategy (only when `security: public`): `api-key`, `jwt`, `oauth` |

### `serve.tls`

TLS certificate paths for `public` listeners.

```yaml
serve:
  tls:
    cert: /etc/letsencrypt/live/myholon.example.com/fullchain.pem
    key: /etc/letsencrypt/live/myholon.example.com/privkey.pem
```

| Field | Type | Required | Description |
|---|---|---|---|
| `serve.tls.cert` | string | conditional | Path to TLS certificate. Required when any listener uses `security: public`. |
| `serve.tls.key` | string | conditional | Path to TLS private key. Required when any listener uses `security: public`. |

### `serve.api_keys`

API key store for `api-key` auth strategy.

```yaml
serve:
  api_keys:
    - name: consumer-alpha
      key: sk_live_abc123...
      scopes: [read]
    - name: consumer-beta
      key: sk_live_def456...
      scopes: [read, write]
```

| Field | Type | Required | Description |
|---|---|---|---|
| `serve.api_keys` | list | no | API keys for `api-key` auth. Consider using `secrets.yaml` instead. |
| `serve.api_keys[].name` | string | yes | Consumer identifier |
| `serve.api_keys[].key` | string | yes | API key value |
| `serve.api_keys[].scopes` | list of string | no | Allowed scopes |

> [!WARNING]
> API keys in `holon.yaml` are committed to version control.
> For production, use a separate `secrets.yaml` (gitignored).

### Security mode auto-detection

If `security` is omitted, the SDK applies defaults:

| URI scheme | Default security |
|---|---|
| `stdio://` | `none` |
| `unix://` | `none` |
| `mem://` | `none` |
| `tcp://` (mesh certs present) | `mesh` |
| `tcp://` (no mesh certs) | `none` (with warning) |

### Validation rules (`op check`)

- `security` must be one of: `none`, `mesh`, `public`
- `auth` only valid when `security: public`
- `serve.tls` required when any listener uses `public`
- Warn if `none` is used with `tcp://` (insecure remote)
- Warn if `api_keys` found in `holon.yaml` (suggest `secrets.yaml`)
