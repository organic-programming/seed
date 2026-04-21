# ader

`ader/` is the root owned by the `ader` proof engine.

Native `ader` objects live here:

- `catalogues/`
- `bouquets/`
- `reports/`
- `archives/`

`ader` proves, groups, executes, and reports. It does not own external loop or brief mechanics.

Catalogue roots live under `ader/catalogues/`.
Bouquets live under `ader/bouquets/`.
Each catalogue owns its own `ader.yaml`, `checks.yaml`, suites, reports, archives, `.artifacts`, and `.t` alias.

Catalogue-owned scenario source may also live inside the catalogue itself. The current shared black-box scenario package is:

- [`ader/catalogues/grace-op/integration/`](./catalogues/grace-op/integration)

`grace-op`, `gabriel-greeting-app-swiftui`, and `gabriel-greeting-app-flutter` suites currently reuse that package through their local checks.


# Examples : 

```shell
go run holons/clem-ader/cmd/main.go test ader/catalogues/grace-op --suite op_build --lane progression
```
