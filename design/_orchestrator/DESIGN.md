
# Codex Orchestrator Design

> **Note:** Requires a ChatGPT Pro subscription. Authentication uses a Pro key, not standard usage-based API keys.

## 1. Overview

The Codex Orchestrator (`codex_orchestrator`) is a Go-based CLI tool that automates the sequential execution of markdown-based task specifications using the Codex CLI and advanced GPT-family models. It enforces strict version-control constraints, provides file-based memory for resumable runs, and standardizes how tasks are dispatched to the LLM.

## 2. Codex CLI Integration

### 2.1 Canonical Invocation

The confirmed working invocation pattern is:

```bash
codex exec --json --skip-git-repo-check \
  -m gpt-5.4 \
  -c 'model_reasoning_effort="xhigh"' \
  '<PROMPT>'
```

| Flag | Purpose |
|---|---|
| `--json` | Emit events as JSONL to stdout, enabling structured output parsing |
| `--skip-git-repo-check` | Required because the orchestrator manages git state externally |
| `-m <MODEL>` | Target model (e.g. `gpt-5.4`, `gpt-5.4-thinking`) |
| `-c key=value` | Override config values at invocation (e.g. `model_reasoning_effort`) |

### 2.2 Session Persistence

By default Codex persists session state to disk, enabling `codex exec resume` on the same task. This is the **recommended approach** for orchestrated runs:

- **Context continuity:** If a task fails mid-execution, the orchestrator can resume from where the model left off rather than replaying the full prompt.
- **Version-scoped sessions:** Each version set (e.g. `v0.5`) accumulates session history, giving the model growing context of what was already accomplished within the set.
- **Post-mortem:** Persisted sessions enable `codex exec resume --last` for manual debugging after an automated run.

Use `--ephemeral` only for throwaway experiments or smoke tests where session data is noise.

### 2.3 Execution Modes

The orchestrator supports two execution modes via the `--approval` flag:

| Mode | Codex Flags | Use Case |
|---|---|---|
| **Full-Auto** | `--full-auto` | Sandboxed, workspace-write; model decides when to ask |
| **Unattended** | `-a never -s workspace-write` | Never prompt; failures returned directly to model |

Default is **Full-Auto**. Use **Unattended** for fully non-interactive batch processing.

### 2.4 Additional Useful Flags

| Flag | Purpose |
|---|---|
| `-o <FILE>` / `--output-last-message` | Capture the model's final message to a file |
| `--output-schema <FILE>` | Constrain model output to a JSON Schema |
| `--search` | Enable web search tool within the model |
| `-C <DIR>` / `--cd` | Override the agent's working directory |
| `--add-dir <DIR>` | Grant write access to additional directories |

### 2.5 Config Overrides (`-c`)

The `-c` flag accepts dotted TOML paths and is the primary mechanism for fine-tuning model behavior without modifying `~/.codex/config.toml`:

```bash
-c 'model_reasoning_effort="xhigh"'
-c 'sandbox_permissions=["disk-full-read-access"]'
-c 'shell_environment_policy.inherit=all'
```

## 3. Core Capabilities

### 3.1 Incremental Task Sets

One or more task "sets" (version directories) are specified on the command line:

```bash
go run design/_orchestrator/codex.go --set v0.4 --set v0.5 --model gpt-5.4
```

- **Independence:** Each set is processed sequentially and independently.
- **Task Discovery:** For each set, the orchestrator reads all `.md` files in the corresponding `design/<project>/<set>/` directory.
- **Ordering:** Files are processed in lexicographic order; use numeric prefixes (e.g. `TASK01_`, `TASK02_`) for explicit sequencing.

### 3.2 Git Module & Submodule Strictness

Before executing any task, the orchestrator verifies the Git state across the main repository and all submodules recursively.

- **Consistency Check:** All modules must be on a branch ending in `-dev`. For example, when orchestrating `v0.6` deriving from `v0.5`, the expected base branch is `grace-op-v0.5-dev`.
- **Feature Branch Creation:** If the check passes, the orchestrator derives and checks out the new working branch (e.g. `grace-op-v0.6-dev`) on the main repo and all submodules before beginning LLM work.
- **Branch Naming:** `<parent-folder>-<set>-dev`, where `<parent-folder>` is the design subfolder containing the set (e.g. `grace-op`).
- **Skip-Git-Repo-Check:** Because the orchestrator manages branching externally, `codex exec` is invoked with `--skip-git-repo-check` so the LLM agent does not interfere with the git state.

### 3.3 MCP Workflows

To give the AI access to established practices, the orchestrator can mount file-based MCP tooling:

```bash
codex mcp add workflows -- npx -y @modelcontextprotocol/server-filesystem .agent/workflows
```

This surfaces the `.agent/workflows` directory as an MCP resource, enabling the LLM to read workflow files natively. The orchestrator checks `codex mcp list` before adding to avoid duplicates.

### 3.4 File-Based Memory and Logging

- **Task Logs:** Every executed task produces a `.log` file adjacent to the task document (e.g. `TASK04_tier2_runners.md` → `TASK04_tier2_runners.md.log`).
- **State File:** A JSON file (`.codex_orchestrator_state.json`) tracks completed tasks. Interrupted runs resume from the last uncompleted task.
- **Output Capture:** When using `--json`, stdout produces JSONL events that can be parsed for structured logging (event types, tool calls, model messages).

### 3.5 Task Lifecycle Doctrine

The orchestrator enforces the workflow doctrine defined in `.agent/workflows/`. Every state transition is performed programmatically — the LLM never manages task metadata itself.

#### State Machine

```
  (discovered)  ──▶  🔨 In Progress  ──▶  ✅ Success
                                       ──▶  ❌ Failure
```

#### Before Execution — Start Task

1. **Mark `_TASKS.md`** — add 🔨 to the summary column of the task row.
2. **Version folder status** — if this is the first task in the set, rename the folder from `v0.X` to `💭 v0.X` via `git mv`.
3. **Commit and push** the marking before dispatching to codex.

#### After Execution — Complete Task

The orchestrator inspects the codex exit code and output to determine success or failure.

**On ✅ Success:**

1. Rename the task file: `TASK01_foo.md` → `TASK01_foo.✅.md`
2. Inject a `## Status` block with commit SHA(s) and verification URL(s).
3. Update the `_TASKS.md` row with ✅ and the renamed filename link.
4. Commit and push.

**On ❌ Failure:**

1. Rename the task file: `TASK03_bar.md` → `TASK03_bar.❌.md`
2. Inject a `## Status` block linking to a failure report.
3. Generate `TASK03_bar.failure.md` containing: summary, error output, context, root cause (if determinable), and next steps.
4. Update the `_TASKS.md` row with ❌.
5. Commit and push.
6. **Halt the set** — do not proceed to subsequent tasks (the version is now ⚠️).

#### After Every Task — Update Version Status

The orchestrator evaluates the version folder state after each task completion:

| Condition | Folder transition |
|---|---|
| Any task ❌ or blocked | → `⚠️ v0.X` |
| All tasks in version are ✅ | `💭 v0.X` → `✅ v0.X` |
| Otherwise | stays `💭 v0.X` |

Folder renames are performed via `git mv` and committed immediately.

#### On ✅ Set Completion — Release

When a version set reaches ✅ (all tasks done), the orchestrator performs two release steps:

1. **Bump `holon.yaml`** — update the `version:` field to match the completed version.
2. **Tag the repository** — `git tag -a v0.X.0 -m "<project> v0.X — <subtitle>"` + push.

> [!IMPORTANT]
> The tag is created only after the `holon.yaml` bump is committed and pushed.

## 4. Architecture (`codex.go`)

```
┌─────────────────────┐
│   CLI Parsing       │  flag: --set, --model, --root
├─────────────────────┤
│   State Manager     │  load/save .codex_orchestrator_state.json
├─────────────────────┤
│   Git Engine        │  branch check, branch creation (recursive submodules)
├─────────────────────┤
│   MCP Setup         │  verify/install workflow MCP server
├─────────────────────┤
│   Execution Engine  │  os/exec → codex exec, output piping + logging
├─────────────────────┤
│   Lifecycle Engine  │  start/complete task, folder status, version release
└─────────────────────┘
```

1. **CLI Parsing:** Standard `flag` package. Parses `--set` (repeatable), `--model`, `--root`.
2. **Git Engine:** Wraps local `git` to recursively walk `.gitmodules`, verify branches (`git rev-parse --abbrev-ref HEAD`), and derive new branches (`git checkout -b`).
3. **Execution Engine:** Dispatches `os/exec.Command("codex", "exec", ...)` with the flags from §2.1. Pipes stdout/stderr to both the terminal and the task log file.
4. **State Manager:** JSON marshal/unmarshal for task completion tracking. Saves after each successful task.
5. **Lifecycle Engine:** Implements §3.5 — manages `_TASKS.md` updates, file renames (✅/❌), failure report generation, version folder status transitions (`git mv`), and release steps (`holon.yaml` bump + git tag).

## 5. Usage

```bash
# Single set, default model
go run design/_orchestrator/codex.go --set v0.4

# Multiple sets, explicit model
go run design/_orchestrator/codex.go --set v0.4 --set v0.5 --model gpt-5.4

# From a specific root directory
go run design/_orchestrator/codex.go --root /path/to/repo --set v0.6 --model gpt-5.4-thinking
```

## 6. Open Questions / Future Work

- [ ] **JSONL parsing:** Parse `--json` output for richer logging (event types, tool invocations, errors).
- [ ] **Output Schema:** Use `--output-schema` to enforce structured task completion reports.
- [ ] **Retry policy:** Automatic retry on transient API failures.
- [ ] **Task filtering:** Support glob or tag-based filtering of tasks within a set (e.g. only `OP_TASK*`).
- [ ] **Dry-run mode:** Print the planned codex commands without executing them.
- [ ] **Approval escalation:** Detect when codex is waiting for human approval (in full-auto mode) and surface it.
