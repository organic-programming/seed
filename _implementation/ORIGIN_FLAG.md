# Implementation: `--origin` Flag

Rename `--bin` to `--origin`. Make it universal — works with any `op` command that resolves a holon.

Spec: [OP_DISCOVERY.md](../holons/grace-op/OP_DISCOVERY.md#the---origin-flag)

---

## What `--origin` Does

When any command resolves a `<holon>`, `--origin` outputs the resolved origin to **stderr**:

```
$ op gabriel-greeting-go SayHello '{"name":"Marie"}' --origin
origin: installed $OPBIN/gabriel-greeting-go.holon/bin/darwin_arm64/gabriel-greeting-go
{"greeting":"Hello Marie"}

$ op build gabriel-greeting-go --origin
origin: source holons/gabriel-greeting-go
00:00:01 built gabriel-greeting-go… ✓

$ op run gabriel-greeting-go --origin
origin: installed $OPBIN/gabriel-greeting-go.holon/bin/darwin_arm64/gabriel-greeting-go
listening on tcp://127.0.0.1:54321
```

**Output format:** `origin: <layer> <path>` on stderr. The command's normal output goes to stdout unchanged.

---

## Tasks

### Rename flag

- [ ] Rename `--bin` to `--origin` in `parseGlobalOptions` (`api/cli.go`)
- [ ] Update help text
- [ ] Keep `--bin` as a hidden alias for backward compatibility (optional)

### Make it universal

- [ ] Move origin output from the current `--bin` one-shot behavior to a **stderr annotation** that works alongside any command
- [ ] Current `--bin` prints the path and exits (no command execution). `--origin` should **not** exit — it prints origin then continues normally.
- [ ] Add origin output to the `Discover()` result — every resolution should carry its layer and path

### Fix `op run <holon> --origin`

- [ ] Currently `op run gabriel-greeting-go --origin` fails with `op: holon "run" not found` — the global flag parser doesn't consume `--origin` before command dispatch
- [ ] Root cause: `--bin` is parsed in `parseGlobalOptions` but `op run` has its own flag parsing that doesn't expect it

### Update documentation

- [ ] Update `OP_DISCOVERY.md` (already done)
- [ ] Update `OP_RUN.md` — remove `--bin` references
- [ ] Update `op --help` output
