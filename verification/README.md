# verification

Catalogue roots live under `verification/catalogues/`.
Bouquets live under `verification/bouquets/`.
Each catalogue owns its own `ader.yaml`, `checks.yaml`, suites, reports, archives, `.artifacts`, and `.t` alias.

Catalogue-owned scenario source may also live inside the catalogue itself. The current shared black-box scenario package is:

- [`verification/catalogues/op/integration/`](./catalogues/op/integration)

`op`, `swiftui`, and `flutter` suites currently reuse that package through their local checks.
