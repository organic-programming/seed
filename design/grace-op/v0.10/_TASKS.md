# Grace-OP v0.10 Design Tasks — Public Holons

## Tasks

| # | File | Summary | Depends on |
|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.10_TASK01_listener_security_schema.md) | Per-listener `security` + `auth` schema in `holon.yaml` | v0.9 |
| 02 | [TASK02](./grace-op_v0.10_TASK02_sdk_multi_listener.md) | SDK `serve.Run` with mixed security modes | TASK01, v0.9 |
| 03 | [TASK03](./grace-op_v0.10_TASK03_auth_interceptors.md) | Auth interceptors (API key, JWT, OAuth) + consumer identity | TASK02 |
| 04 | [TASK04](./grace-op_v0.10_TASK04_op_serve_public.md) | `op serve` public listener + TLS support | TASK01, TASK03 |
| 05 | [TASK05](./grace-op_v0.10_TASK05_secrets_management.md) | Secrets management (`secrets.yaml`, gitignored) | TASK03 |
| 06 | [TASK06](./grace-op_v0.10_TASK06_docs.md) | Documentation (spec updates → output/ for review) | TASK01–05 |

Design document: [DESIGN_public_holons.md](./DESIGN_public_holons.md)
