# Integration

`integration/` is now legacy.

The active `ader` runtime model lives under [`verification/`](../verification/README.md).

The old black-box scenario package was moved to:

- [`verification/catalogues/op/integration/`](../verification/catalogues/op/integration)

Use catalogue roots, not `integration/`, when running `ader`:

```bash
ader test verification/catalogues/op --suite op-proxy --profile smoke
ader test verification/catalogues/ader --suite ader-self --profile smoke
ader test-bouquet verification --name local-dev
```
