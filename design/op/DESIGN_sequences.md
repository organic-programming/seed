# Sequences

## Context

Holons expose three layers to agents and humans:

| Layer | Purpose | Exists today |
|---|---|---|
| **Tools** | Individual RPCs via MCP | ✅ `op mcp` |
| **Skills** | Documentation: when/why to use the holon | ✅ `holon.yaml` |
| **Sequences** | Deterministic composed tool calls | ❌ NEW |

Skills tell agents WHEN to act. Tools let agents act one step at a
time. Sequences let agents (or humans, or CI) run a pre-packaged
batch of steps as a single action.

---

## Manifest Spec (addition to OP.md §7)

New `sequences:` section in `holon.yaml`, sibling to `skills:`:

```yaml
# ── Skills (unchanged) ──────────────────────────────────
skills:
  - name: discover-and-build
    description: Discover a holon, inspect it, build it.
    when: User wants to find and build a holon.
    steps:
      - Discover local holons with `op list .`
      - Inspect the contract with `op inspect <holon>`
      - Build and install with `op build` then `op install`

# ── Sequences (NEW) ─────────────────────────────────────
sequences:
  - name: build-and-install
    description: Build, test, and install a holon.
    params:
      - name: holon
        description: Target holon slug
        required: true
      - name: config
        description: Build configuration name
        required: false
    steps:
      - run: op check {{ .holon }}
      - run: op build {{ .holon }}
      - run: op test {{ .holon }}
      - run: op install {{ .holon }}
```

### Sequences fields

| Field | Type | Required | Description |
|---|---|---|---|
| `sequences` | list | no | Deterministic step sequences. |
| `sequences[].name` | string | yes | Identifier, kebab-case. |
| `sequences[].description` | string | yes | What it achieves. |
| `sequences[].params` | list | no | Input parameters. |
| `sequences[].params[].name` | string | yes | Parameter name. |
| `sequences[].params[].description` | string | no | What this param controls. |
| `sequences[].params[].required` | bool | no | Default: false. |
| `sequences[].params[].default` | string | no | Default value if not provided. |
| `sequences[].steps` | list | yes | Ordered steps. |
| `sequences[].steps[].run` | string | yes | Shell command (Go template). |

### Template syntax

Steps use Go `text/template` syntax. Parameters are accessed via
`{{ .paramName }}`. No conditionals, no loops — sequences are
strictly linear.

---

## CLI: `op do`

```bash
op do <holon> <sequence> [--param=value ...]
```

Behavior:
1. Resolves holon by slug
2. Finds sequence by name in `holon.yaml`
3. Validates required params are provided
4. Substitutes params into step templates
5. Executes each step sequentially
6. Stops on first non-zero exit code
7. Reports progress per step

### Output

```bash
$ op do grace-op build-and-install --holon=rob-go

[1/4] op check rob-go
  ✅ All checks passed
[2/4] op build rob-go
  ✅ Built in 2.3s
[3/4] op test rob-go
  ✅ 42 passed, 0 failed
[4/4] op install rob-go
  ✅ Installed to $OPBIN/rob-go

✅ build-and-install completed (4 steps, 6.1s)
```

### Flags

| Flag | Effect |
|---|---|
| `--dry-run` | Print steps without executing |
| `--continue-on-error` | Don't stop on failure |

---

## MCP Integration

`op mcp <holon>` exposes sequences as **MCP tools** alongside
individual RPCs:

```json
{
  "name": "sequence_build-and-install",
  "description": "Build, test, and install a holon.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "holon": {
        "type": "string",
        "description": "Target holon slug"
      }
    },
    "required": ["holon"]
  }
}
```

The agent chooses:
- **Sequence tool** → fast, deterministic, 1 call
- **Individual tools** → adaptive, reasoning between steps

---

## Backward Compatibility

- `skills:` unchanged — free-text steps, documentation-only
- `sequences:` is additive — holons without it work exactly
  as before
- `op do` on a holon with no sequences: clear error

## Not in Scope (deferred)

- Conditional steps (`if:`)
- Loops
- Output capture between steps
- External workflow files
- Retry logic

These may be added later if needed. For now, sequences are
strictly linear and deterministic.
