# observability-cascade

Cross-language regression checks for the observability relay primitive.

## What this is

Each `observability-cascade-<lang>` composite starts a short relay chain of
`observability-cascade-node-<lang>` members, invokes `RelayService.Tick`, and
checks that relayed logs, metrics, and events surface at the root.

This example is a development regression gate for SDK observability behavior.
It is not a stress harness and does not try to exhaust every language pair.

## Build one variant

Build a composite by slug:

```bash
op build observability-cascade-go
```

The build stays local to the holon output. Example holons are not installed into
`$OPBIN`.

Run it directly if you want to inspect the default report:

```bash
op invoke observability-cascade-go RunDefault '{}' -f json
```

## Local checker

Run the regression gate from the repo root:

```bash
./examples/observability-cascade/run-cascade.sh
```

The checker builds each requested composite and invokes `RunDefault` once. It
prints one `PASS` or `FAIL` line per language and exits non-zero if any language
fails.

By default it checks the currently passing set:

```text
go dart rust
```

Override that set with `CASCADE_LANGS`:

```bash
CASCADE_LANGS="go dart" ./examples/observability-cascade/run-cascade.sh
```

CI runs this as a regression gate.
