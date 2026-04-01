# TDD — Test-Driven Development Pipeline

The seed uses a promotion-based TDD pipeline driven by [clem-ader](holons/clem-ader/README.md).

Tests progress through a lifecycle, from active development to permanent safety net, across two validation tiers.

## 1. Validation Hierarchy

Two tiers, bottom-up. Fix the lower tier before running the upper one.

| Tier | Validates | Profile | Typical Source |
|------|-----------|---------|----------------|
| **Unit** | Internal logic per module | `unit`, `quick` | `workspace` |
| **Integration** | Black-box contract between components via `op` | `integration`, `full` | `committed` |

Each tier is defined as a set of **steps** in the [suite YAML](integration/suites/seed.yaml). Steps are shell commands or scripts, each with a `workdir`, `prereqs`, and `description`.

## 2. Test Lifecycle

Every test step lives in one of two **lanes** at any given time. Lane is a property of the **step**, not the profile, so one step has one lane everywhere it is reused.

```
progression ──[clean pass]──▶ promotion ──[review + apply]──▶ regression
     ▲                                                            │
     └────────────────────[downgrade]─────────────────────────────┘
```

### Progression (Active TDD)

The step is under active development. The agent iterates until all steps in the lane pass cleanly.

```bash
ader test integration --suite seed --profile unit --lane progression --source workspace
```

### Promotion (Transition)

When a `progression` run passes completely, ader generates two files in the report directory:

- `promotion.json` — machine-readable: eligible steps, suggested `ader promote ...` command, suggested git commands
- `promotion.md` — human-readable summary

These files propose moving the passing steps from `progression` to `regression` in the suite YAML. Neither ader nor the agent mutate the suite automatically during `test`. A human reviews and applies the explicit promote command.

### Regression (Safety Net)

Promoted steps become the immutable baseline. Any failure in `regression` signals a breakage of existing behavior.

```bash
ader test integration --suite seed --profile unit --lane regression
```

Default lane is `regression`. Default source is `committed`.

### Downgrade (Reset)

To move tests back to `progression` for re-evaluation:

```bash
ader downgrade integration --all                              # all regression steps
ader downgrade integration --step sdk-go-unit --step X        # specific steps
```

Rules: step `lane` changes from `regression` to `progression`. The suite YAML is rewritten immediately, never auto-committed.[^1]

[^1]: `ader downgrade` is specified in the [Codex prompt](prompts/codex-cross-tier-promotion.md).

## 3. Bottom-Up Agent Loop

An AI agent follows this loop when developing or validating code:

```
1. unit --lane progression --source workspace
   → iterate until clean pass
   → review promotion.md → run `ader promote ...`

2. integration --lane progression --source workspace
   → iterate until clean pass
   → review promotion.md → run `ader promote ...`

3. full --lane regression --source committed
   → final proof → archive
```

Each tier must pass before moving to the next. This isolation ensures errors are caught at the most granular level.

### Why This Works for Agents

- **Noise reduction**: the agent focuses on one tier at a time, not the entire suite
- **Auditability**: `promotion.json` forces the agent to produce evidence before code enters the safety net
- **Resilience**: every promotion strengthens the regression baseline automatically

## 4. Suite Structure

The suite YAML (`integration/suites/seed.yaml`) maps steps to profiles. Lane lives on the step itself:

```yaml
steps:
  sdk-go-unit:
    workdir: sdk/go-holons
    prereqs: [go]
    command: go test ./...
    description: Go SDK unit tests
    lane: progression

profiles:
  unit:
    steps: [sdk-go-unit, new-feature-unit]
  integration:
    steps: [integration-deterministic, integration-short]
```

A step can appear in multiple profiles. Its lane assignment is global: `sdk-go-unit` is either `progression` or `regression` everywhere.

### Profile Ladder

Profiles form a hierarchy for cross-tier suggestions:

```
quick → unit → integration → full
```

`stress` is outside this ladder.

When a step is promoted in a profile, ader suggests adding it to the `progression` lane of the next profile in the ladder — if it isn't already present there. This is purely informational; no automatic mutation.

## 5. Commands

| Phase | Command |
|-------|---------|
| Unit progression | `ader test integration --suite seed --profile unit --lane progression --source workspace` |
| Unit regression | `ader test integration --suite seed --profile unit` |
| Integration progression | `ader test integration --suite seed --profile integration --lane progression --source workspace` |
| Integration regression | `ader test integration --suite seed --profile integration` |
| Full regression | `ader test integration --suite seed --profile full` |
| Quick check | `ader test integration --suite seed --profile quick` |
| Promote specific | `ader promote integration --step sdk-go-unit` |
| Promote all progression | `ader promote integration --all` |
| Downgrade all | `ader downgrade integration --all` |
| Downgrade specific | `ader downgrade integration --step sdk-go-unit` |
| Archive latest | `ader archive integration --latest` |
| View history | `ader history integration` |
| Show report | `ader show integration --id <history-id>` |
| Cleanup residue | `ader cleanup integration` |

## 6. Promotion Artifacts

### `promotion.json`

```json
{
  "suite": "seed",
  "profile": "unit",
  "lane": "progression",
  "destination_lane": "regression",
  "suite_file": "integration/suites/seed.yaml",
  "eligible_steps": ["sdk-go-unit", "example-go-unit"],
  "suggested_command": "ader promote integration --step sdk-go-unit --step example-go-unit",
  "suggested_git_commands": [
    "git add integration/suites/seed.yaml",
    "git commit -m \"Promote sdk-go-unit, example-go-unit to regression\""
  ],
  "suggested_commit_message": "Promote sdk-go-unit, example-go-unit to regression",
  "cross_tier_suggestions": [
    {
      "step_id": "sdk-go-unit",
      "from_profile": "unit",
      "to_profile": "integration",
      "to_lane": "progression",
      "reason": "Promoted in unit; not yet present in integration"
    }
  ]
}
```

### `promotion.md`

A markdown summary: eligible steps, the exact `ader promote ...` command, git commands, and **cross-profile suggestions** showing which steps should be considered for the next profile in the ladder.

## References

- [clem-ader README](holons/clem-ader/README.md) — engine documentation
- [integration README](integration/README.md) — config root and commands
- [Suite definition](integration/suites/seed.yaml) — step and profile definitions
- [holon.proto skills](holons/clem-ader/api/v1/holon.proto) — `regression-loop`, `progression-loop`, `report-hygiene`
