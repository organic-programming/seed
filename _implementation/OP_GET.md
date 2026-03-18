# Implementation: `op get`

## Concept

`op get` fetches a remote holon (source or binary) into the local cache.
It is the bridge between "remote" and "reachable". After `op get`, the
holon appears in `op list --cached`.

```
remote repo  ──op get──►  cache  ──op build──►  built  ──op install──►  installed
```

`op get` does **not** build. It just makes the holon locally available.

---

## Command Surface

```bash
op get <git-url>                    # fetch source from git
op get <git-url>@<version>          # fetch specific version
op get <slug>                       # fetch from registry (future)
op get <slug>@<version>             # fetch specific version from registry
```

### Result

Fetched holon lands in `$OPPATH/cache/<slug>.holon/`:

- **Source fetch**: `<slug>.holon/git/` contains the cloned repo
- **Binary fetch**: `<slug>.holon/bin/<arch>/` contains the binary
- **Either way**: `.holon.json` is generated (via proto or describe probe)

### Flags

| Flag | Description |
|------|-------------|
| `--source` | Force source fetch even if binary is available |
| `--binary` | Force binary fetch (fail if unavailable) |
| *(none)* | Prefer binary, fall back to source |

---

## Lifecycle After `op get`

```bash
op get <url>                      # fetch → cached
op list --cached                  # now visible
op build <slug>                   # build from cached source (if source)
op install <slug>                 # build + install into $OPBIN
```

If `op get` fetched a binary package, it is immediately runnable via
`op run <slug>` without a build step.

---

## Relationship to Existing Commands

| Command | Role |
|---------|------|
| `op get` | Fetch remote → cache (new) |
| `op mod pull` | Fetch declared dependencies → cache (keeps, narrower scope) |
| `op build` | Compile source → `.op/build/` |
| `op install` | Build + copy → `$OPBIN/` |
| `op list` | Show all reachable holons |

`op mod pull` remains for declared dependencies (like `go mod download`).
`op get` is for ad-hoc fetch of any remote holon (like `go get`).

---

## Tasks

- [ ] Define `op get` CLI parsing and flags
- [ ] Implement git clone into `$OPPATH/cache/<slug>.holon/git/`
- [ ] Generate `.holon.json` after fetch (from proto or describe probe)
- [ ] Binary fetch from registry (future, depends on registry)
- [ ] `op list --cached` sees `op get` results
- [ ] `op build` and `op install` work on cached holons
- [ ] Update `op help` to include `op get`
- [ ] Document in OP.md
