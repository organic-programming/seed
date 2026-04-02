# Integration

`integration/` is no longer the active `ader` configuration root.

The active verification model lives under [`verification/`](../verification/README.md):

- catalogue-local `ader.yaml`
- catalogue-local `checks.yaml`
- suite files under `verification/catalogues/*/suites/`
- bouquet files under `verification/bouquets/`

What remains in `integration/` is still important:

- [`tests/`](./tests/) contains the black-box Go tests used by the `op` catalogue
- legacy seed files may remain here during migration, but they are not the primary runtime surface anymore

Use commands against catalogue roots, not `integration/`:

```bash
ader test verification/catalogues/op --suite op-proxy --profile smoke
ader promote verification/catalogues/op --suite op-proxy --step <step-id>
ader downgrade verification/catalogues/op --suite op-proxy --all
ader history verification/catalogues/op
ader test-bouquet verification --name local-dev
```

Reports now live per catalogue:

```text
verification/catalogues/<catalogue>/reports/<history-id>/
verification/catalogues/<catalogue>/archives/<commit-hash>/<history-id>.tar.gz
```

Bouquet reports live at:

```text
verification/reports/bouquets/<bouquet-history-id>/
verification/archives/bouquets/<bouquet-history-id>.tar.gz
```
