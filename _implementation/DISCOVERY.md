# Implementation: Holon Discovery

Extracted from [HOLON_DISCOVERY.md](../HOLON_DISCOVERY.md).

---

---

## Core Rule

**Every holon is a regular holon.** There are no special cases. `op` is
`grace-op` with `aliases: ["op"]` — the same mechanism any holon can use.
The only reason `op` is on `$PATH` is a deployment choice, not a
structural distinction.

## Note

`$OPBIN` itself is **not** on `$PATH`. The user adds
`$OPBIN/grace-op.holon/bin/<arch>/` to `$PATH` to make `op` callable.

`op env --init` creates the directory structure, `op env --shell` outputs the
shell snippet to add `grace-op.holon/bin/<arch>/` to `$PATH`. Combined:

```bash
eval "$(op env --init --shell)"
```


## Tasks

- [ ] `op` is installed as `grace-op.holon/` — no bare binary exception
- [ ] `op env --shell` outputs PATH guidance for `grace-op.holon/bin/<arch>/`
- [ ] `op discover` removed — `op list` absorbs all discovery
- [ ] `op list` defaults to `--all` (all layers)
- [ ] `op list --source`, `--built`, `--installed`, `--cached` filter by layer
- [ ] `op list --root <path>` overrides CWD
- [ ] `op list --format json` outputs structured JSON
- [ ] Discovery algorithm is the same across all SDKs
- [ ] `.holon.json` is an accelerator — missing = probe via stdio Describe
- [ ] Only `op` writes `.holon.json`; SDKs read or probe
- [ ] Add `version` field to `.holon.json` and proto
- [ ] Add `aliases` field to `.holon.json` and proto
- [ ] Unify grace-op internal discovery with SDK `go-holons/pkg/discover`
