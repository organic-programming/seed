
# Codex Orchestrator Design

> **Note:** Requires a ChatGPT Pro subscription. Authentication uses a Pro key, not standard usage-based API keys.

## 1. Overview

The Codex Orchestrator is a Go-based CLI tool that automates the sequential execution of markdown-based task specifications using the Codex CLI and advanced GPT-family models. It enforces strict version-control constraints, provides file-based memory for resumable runs, and standardizes how tasks are dispatched to the LLM.

## 2. Codex CLI Integration

### 2.1 Canonical Invocation

The confirmed working invocation pattern is:

```bash
codex exec --json --skip-git-repo-check \
  -C <ROOT_REPO> \
  -s workspace-write \
  -m gpt-5.4 \
  -c 'model_reasoning_effort="xhigh"' \
  -o <TASK>.result.md \
  '<PROMPT>'
```

| Flag | Purpose |
|---|---|
| `--json` | Emit events as JSONL to stdout, enabling structured output parsing |
| `--skip-git-repo-check` | Required because the orchestrator manages git state externally |
| `-C <ROOT_REPO>` | Pin the agent's working root to the repository directory |
| `-s workspace-write` | Sandbox: allow writes only within the workspace, nothing outside |
| `-m <MODEL>` | Target model (e.g. `gpt-5.4`, `gpt-5.4-thinking`) |
| `-c key=value` | Override config values at invocation (e.g. `model_reasoning_effort`) |
| `-o <FILE>` | Write the agent's final message to a file |

### 2.2 Workspace Isolation

The orchestrator **must** guarantee that codex cannot modify files outside the root repository. This is enforced by two complementary mechanisms:

1. **`-C <ROOT_REPO>`** — sets the agent's working directory to the repository root. The agent sees this as its workspace boundary.
2. **`-s workspace-write`** — the Codex sandbox only permits writes within the workspace. Any attempt to write outside is blocked at the OS level (Seatbelt on macOS, Landlock on Linux).

> [!CAUTION]
> Never use `-s danger-full-access` or `--dangerously-bypass-approvals-and-sandbox` in orchestrated runs. These disable the isolation boundary entirely.

### 2.3 Session Persistence

By default Codex persists session state to disk, enabling `codex exec resume` on the same task. This is the **recommended approach** for orchestrated runs:

- **Context continuity:** If a task fails mid-execution, the orchestrator can resume from where the model left off rather than replaying the full prompt.
- **Version-scoped sessions:** Each version set (e.g. `v0.5`) accumulates session history, giving the model growing context of what was already accomplished within the set.
- **Post-mortem:** Persisted sessions enable `codex exec resume --last` for manual debugging after an automated run.

Use `--ephemeral` only for throwaway experiments or smoke tests where session data is noise.

### 2.4 Execution Modes

The orchestrator supports two execution modes via the `--approval` flag:

| Mode | Codex Flags | Use Case |
|---|---|---|
| **Full-Auto** | `--full-auto` | Sandboxed, workspace-write; model decides when to ask |
| **Unattended** | `-a never -s workspace-write` | Never prompt; failures returned directly to model |

Default is **Full-Auto**. Use **Unattended** for fully non-interactive batch processing.

### 2.5 Additional Useful Flags

| Flag | Purpose |
|---|---|
| `-o <FILE>` / `--output-last-message` | Capture the model's final message to a file |
| `--output-schema <FILE>` | Constrain model output to a JSON Schema |
| `--search` | Enable web search tool within the model |
| `-C <DIR>` / `--cd` | Override the agent's working directory |
| `--add-dir <DIR>` | Grant write access to additional directories |

### 2.6 Config Overrides (`-c`)

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
- **Task Discovery:** For each set, the orchestrator reads `_TASKS.md` in the corresponding `design/<project>/<set>/` directory.
- **Ordering:** Tasks are ordered by the dependency DAG (§3.6), not by filename.

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

### 3.4 Logging and Output Capture

Every codex invocation produces **three** output streams. The orchestrator captures all of them — **nothing is discarded, nothing is masked.** Every line is visible to the operator in real time.

#### Tee Principle

All output is **tee'd**: each line from codex is written simultaneously to:

1. The **log file** (for post-mortem analysis)
2. The **orchestrator's own stdout or stderr** (for real-time operator visibility)

The orchestrator never swallows output. If codex prints it, the operator sees it.

#### Timestamp Prefix

Every line — whether from stdout or stderr — is prefixed with a human-readable timestamp before being written anywhere:

```
<YYYY_MM_DD_HH_MM_SS_mmm> <original line>
```

This applies uniformly to:
- JSONL events tee'd to stdout and written to `.jsonl`
- Diagnostic lines tee'd to stderr and written to `.stderr.log`
- Orchestrator's own log messages (retry waits, lifecycle events, git operations)

#### Output Streams

| Codex stream | Orchestrator tees to | Log file suffix |
|---|---|---|
| stdout (`--json`) | orchestrator **stdout** | `.jsonl` |
| stderr | orchestrator **stderr** | `.stderr.log` |
| `-o` (final message) | — (file only) | `.result.md` |

#### Per-Task File Layout

For a task file `TASK04_tier2_runners.md`, the orchestrator produces:

```
TASK04_tier2_runners.md            # the original task spec
TASK04_tier2_runners.md.jsonl      # timestamped JSONL event stream
TASK04_tier2_runners.md.stderr.log # timestamped stderr diagnostics
TASK04_tier2_runners.md.result.md  # final agent message (-o)
```

All log files are placed adjacent to the task file they belong to.

#### JSONL Event Protocol

The `--json` flag emits one JSON object per line on stdout. The orchestrator **prepends a human-readable timestamp** to each line before writing to the `.jsonl` log file:

```
<YYYY_MM_DD_HH_MM_SS_mmm> <JSON>
```

Example output in the log file:

```
2026_03_10_19_15_03_042 {"type":"thread.started","thread_id":"019cd901-42fe-..."}
2026_03_10_19_15_03_044 {"type":"turn.started"}
2026_03_10_19_15_07_891 {"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"..."}}
2026_03_10_19_15_07_893 {"type":"turn.completed","usage":{"input_tokens":7917,"cached_input_tokens":7040,"output_tokens":43}}
```

To parse, split on the first `{` — everything before it is the timestamp, everything from `{` onward is valid JSON.

| Event | Orchestrator use |
|---|---|
| `thread.started` | Record `thread_id` in state for `codex exec resume` |
| `item.completed` | Log agent messages and tool calls |
| `turn.completed` | Record token usage; detect completion |

#### Success / Failure Detection

The orchestrator determines task outcome from **two signals**:

1. **Exit code** — `0` = codex completed normally, non-zero = crash or sandbox violation.
2. **JSONL stream** — if the last event is `turn.completed` with no error field, the task succeeded. If the stream contains error events or terminates abnormally, the task failed.

> [!IMPORTANT]
> Exit code `0` does not guarantee the LLM actually completed the task correctly — it only means codex itself ran without errors. The orchestrator treats it as success for lifecycle purposes; semantic verification is out of scope.

#### State File

A JSON state file (`.codex_orchestrator_state.json`) tracks:

- Completed task paths (for resume after interruption)
- Per-task `thread_id` (for `codex exec resume`)
- Per-task token usage (cumulative accounting)

### 3.5 Task Lifecycle Doctrine

The orchestrator enforces the workflow doctrine defined in `.agent/workflows/`. Every state transition is performed programmatically — the LLM never manages task metadata itself.

#### State Machine

```
  (discovered)  ──▶  🔨 In Progress  ──▶  ✅ Success
                                       ──▶  ❌ Failure
```

The version folder stays as `v0.X` throughout execution. Emoji prefixes are applied **only on completion**, never mid-run.

#### Before Execution — Reset (Re-Run Only)

If the version folder has an emoji prefix from a previous run, the orchestrator strips it before starting:

1. **Strip folder prefix** — `✅ v0.X` or `⚠️ v0.X` → `v0.X`.
2. **Reset `_TASKS.md` Status column** — clear all status emojis back to `—`.
3. **Remove `## Status` blocks** from task files.
4. **Remove failure reports** — delete any `.failure.md` files from the previous run.
5. **Commit and push** the reset.

#### Before Execution — Start Task

1. **Update `_TASKS.md` Status column** — set the task's status to `💭`.
2. **Commit and push** the marking before dispatching to codex.

#### After Execution — Complete Task

The orchestrator inspects the codex exit code and output to determine success or failure. Task files are **never renamed** — status is tracked in `_TASKS.md` and inside the task file itself.

**On ✅ Success:**

1. Inject a `## Status` block in the task file with commit SHA(s) and verification URL(s).
2. Update the `_TASKS.md` Status column to `✅`.
3. Commit and push.

**On ❌ Failure:**

1. Inject a `## Status` block in the task file linking to a failure report.
2. Generate `TASK03_bar.failure.md` containing: summary, error output, context, root cause (if determinable), and next steps.
3. Update the `_TASKS.md` Status column to `❌`.
4. Commit and push.
5. **Halt the set** — do not proceed to subsequent tasks.

#### After All Tasks — Update Version Status

The orchestrator renames the version folder **once**, after all tasks are processed (or on halt):

| Condition | Folder rename |
|---|---|
| All tasks ✅ | `v0.X` → `✅ v0.X` |
| Any task ❌ or blocked | `v0.X` → `⚠️ v0.X` |

The rename is committed as part of the final completion commit.

#### On ✅ Set Completion — Release

When a version set reaches ✅ (all tasks done), the orchestrator performs two release steps:

1. **Bump `holon.yaml`** — update the `version:` field to match the completed version.
2. **Tag the repository** — `git tag -a v0.X.0 -m "<project> v0.X — <subtitle>"` + push.

> [!IMPORTANT]
> The tag is created only after the `holon.yaml` bump is committed and pushed.


### 3.6 Task Structure Convention

The orchestrator expects a strict directory layout. The `grace-op` project illustrates the full structure with 9 version folders and 62+ tasks.

#### Project-Level Files

```
design/<project>/
├── ROADMAP.md        # version plan with dependency chain
├── INDEX.md          # links to specs, design docs, and _TASKS per version
├── v0.3/
├── v0.4/
└── ...
```

- **`ROADMAP.md`** — lists every version with a summary, its design document, and a dependency chain diagram (e.g. `v0.3 → v0.4 → v0.6 → ...`).
- **`INDEX.md`** — cross-references specs, holon source, and all `_TASKS.md` files.

#### Version Folder Layout

```
design/<project>/v0.X/
├── _TASKS.md                                    # task index (table)
├── DESIGN_<topic>.md                            # one or more design docs
├── <project>_v0.X_TASK01_<slug>.md              # task spec
├── <project>_v0.X_TASK02_<slug>.md
└── ...
```

#### `_TASKS.md` Schema

Every `_TASKS.md` follows this exact table format:

```md
| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./grace-op_v0.3_TASK01_install_no_build.md) | `op install --no-build` flag | — | — |
| 04 | [TASK04](./grace-op_v0.3_TASK04_tier2_runners.md) | `npm`, `gradle` runners | TASK03 | — |
```

Columns: task number, link to task file, one-line summary, intra-version dependencies (or `—` for none), status (`—`, `💭`, `✅`, `❌`, `⚠️`).

#### Task File Naming

Pattern: `<project>_v<X.Y>_TASKNN_<slug>.md`

Task files are **never renamed**. Status is tracked in the `_TASKS.md` Status column and in the task file's `## Status` block.

#### Task File Sections

Each task file follows a consistent structure:

| Section | Required | Content |
|---|---|---|
| `# TASKNN — <title>` | ✅ | H1 with task number and human-readable title |
| `## Context` | ✅ | Background, references to specs or design docs |
| `## Objective` | ✅ | What the task achieves (one paragraph) |
| `## Changes` | ✅ | File-by-file or component-by-component breakdown |
| `## Acceptance Criteria` | ✅ | Checklist of verifiable conditions |
| `## Dependencies` | ✅ | Explicit list (or "None") |
| `## Status` | After completion | Injected by orchestrator (§3.5) |

#### Dependency Resolution

Dependencies come in two forms:

1. **Intra-version** — `TASK04` depends on `TASK03` within the same version folder. The orchestrator resolves these into a topological execution order.
2. **Cross-version** — `TASK05` depends on `v0.6` (a prior version set). Cross-version pre-checks are listed in §6 (future work); v1 enforces intra-version dependencies only.

> [!WARNING]
> The orchestrator must never execute a task whose intra-version dependencies are not satisfied. Tasks without dependencies can be executed in any order; tasks with dependencies must wait.

### 3.7 Error Handling and Retry Policy

Not every failure means the task itself failed. The orchestrator distinguishes **transient infrastructure errors** from **task failures** and handles them differently.

#### Error Classification

| Category | Detection | Action |
|---|---|---|
| **Network error** | Non-zero exit + stderr contains `connection`, `timeout`, `DNS`, `ECONNREFUSED`, or empty JSONL stream | Retry with backoff |
| **Quota / rate limit** | Non-zero exit + stderr contains `429`, `rate limit`, `quota`, `capacity` | Wait and retry |
| **Task failure** | Non-zero exit + JSONL stream contains error events, or codex ran but LLM could not complete | Mark ❌, halt set (§3.5) |
| **Sandbox violation** | Non-zero exit + stderr contains `sandbox`, `permission denied` | Mark ❌, halt set |

#### Retry Strategy — Network Errors

Exponential backoff with jitter:

| Attempt | Base delay | Max delay |
|---|---|---|
| 1 | 5s | — |
| 2 | 15s | — |
| 3 | 45s | — |
| 4 | 2min | — |
| 5 | 5min | — |

After **5 failed attempts**, the orchestrator marks the task ❌ and halts the set.

#### Retry Strategy — Quota Exhaustion

Quota errors require longer waits because they indicate account-level limits, not transient network blips.

| Attempt | Wait time |
|---|---|
| 1 | 15 min |
| 2 | 30 min |
| 3 | 1 hour |

After **3 failed attempts**, the orchestrator marks the version folder ⚠️ and exits cleanly (does not mark the task ❌ — the task was never attempted). This allows a later run to resume.

#### Wait Behavior

During any wait period, the orchestrator:

1. Logs the error category, attempt number, and next retry time to both the task `.stderr.log` and the terminal.
2. Writes a `waiting` status to the state file so an interrupted wait can be resumed.
3. Sleeps with periodic heartbeat messages (every 60s) so the operator knows the process is alive.

> [!NOTE]
> All retry timings are defaults. A future `--max-retries` and `--backoff-base` flag could make these configurable.

### 3.8 Prompt Construction and Context Management

The orchestrator assembles the prompt sent to `codex exec` from four context layers, ordered from most stable to most volatile.

#### Context Layers

```
┌───────────────────────────────────┐
│  Layer 1: System Context          │  always present, rarely changes
│  CONVENTIONS.md, agent role       │
├───────────────────────────────────┤
│  Layer 2: Version Context         │  stable within a version set
│  DESIGN_*.md for current version  │
├───────────────────────────────────┤
│  Layer 3: History Context         │  grows with each completed task
│  .result.md from prior tasks      │
├───────────────────────────────────┤
│  Layer 4: Task Context            │  unique per invocation
│  the task .md file itself         │
└───────────────────────────────────┘
```

| Layer | Source files | Inclusion rule |
|---|---|---|
| **System** | `CONVENTIONS.md`, `AGENT.md` | Included if present relative to `--root` |
| **Version** | `DESIGN_*.md` in current version folder | Always included verbatim |
| **History** | `.result.md` from completed tasks in the set | Full until budget exceeded, then compressed |
| **Task** | The current task `.md` file | Always included verbatim |

#### Prompt Template

The orchestrator concatenates the layers into a single prompt string:

```
You are implementing tasks for the <project> project, version <set>.
Follow the conventions and design documents below.

--- CONVENTIONS ---
<CONVENTIONS.md content>

--- DESIGN ---
<DESIGN_*.md content for this version>

--- COMPLETED TASKS ---
<history: prior .result.md files, or compressed summary>

--- CURRENT TASK ---
<task .md file content>

Implement this task. Do not modify task files or _TASKS.md — the
orchestrator manages those. Focus exclusively on the code changes
described in the task.
```

#### Context Compression

As tasks complete, the history layer grows. The orchestrator manages this with a **token budget**:

1. **Estimate tokens** — count characters in the assembled prompt (rough: 1 token ≈ 4 chars). The budget is `model_max_context × 0.40` (reserve 60% for codex's own system prompt, tool output, and generation).

2. **Under budget** — include all prior `.result.md` files verbatim, newest first.

3. **Over budget** — compress the history:
   - Take all prior `.result.md` files
   - Call `codex exec --ephemeral -m gpt-5.1-codex-mini` with a compression prompt:
     ```
     Summarize these task completion reports into a single concise
     briefing. Preserve: what was implemented, which files were changed,
     and any decisions or caveats. Remove verbosity.
     ```
   - The compressed summary replaces the individual `.result.md` files in the history layer
   - Cache the summary as `<version>/_HISTORY_SUMMARY.md` to avoid re-compressing on retry

4. **Still over budget** — if even the compressed summary is too large, truncate to the most recent N tasks and prepend: `(Earlier tasks omitted — see _HISTORY_SUMMARY.md for details)`

#### What the LLM Must Not Do

The prompt includes explicit guardrails:

- Do not modify task files, `_TASKS.md`, or `ROADMAP.md` — the orchestrator owns these
- Do not create or switch git branches — the orchestrator manages branching
- Do not install system-level dependencies without documenting them in the task output
- Focus exclusively on the code changes described in the current task

### 3.9 Execution Loop

A single codex invocation may produce code that doesn't pass verification. Rather than immediately marking ❌, the orchestrator runs a **create → verify → fix** loop per task.

#### Loop State Machine

```
                ┌─────────────────────────────────────────┐
                │                                         │
  ──▶  CREATE  ──▶  VERIFY  ──┬──▶  ✅ pass  ──▶  DONE     │
                               │                          │
                               └──▶  ❌ fail  ──▶  FIX  ──┘
                                                   │
                                              max attempts?
                                                   │
                                              ❌ ABORT
```

#### Phases

| Phase | What the orchestrator does | Codex invocation |
|---|---|---|
| **CREATE** | Build the 4-layer prompt (§3.8) and dispatch | `codex exec ... '<prompt>'` |
| **VERIFY** | Run verification commands extracted from the task | Direct `os/exec` (no codex) |
| **FIX** | Feed verification errors into the same session | `codex exec resume <thread_id> '<fix prompt>'` |

#### CREATE Phase

Standard prompt construction per §3.8. The first codex invocation for this task. The `thread_id` from the JSONL `thread.started` event is recorded in the state file.

#### VERIFY Phase

The orchestrator extracts verifiable commands from the task file's **Acceptance Criteria** and **Checklist** sections. It looks for lines matching shell command patterns:

- `go test ./...`
- `op build`, `op check`, `op run`
- `cargo test`, `swift test`, `flutter test`
- Any line starting with a backtick-quoted command

These are executed **outside codex**, directly via `os/exec` in the workspace, with output captured. Each command produces a pass/fail result.

If the task contains no extractable commands, the VERIFY phase is skipped and the orchestrator relies solely on codex exit code from CREATE.

#### FIX Phase

When verification fails, the orchestrator resumes the existing codex session with a fix prompt:

```bash
codex exec resume <thread_id> \
  --json --skip-git-repo-check \
  -C <ROOT_REPO> -s workspace-write \
  -m <MODEL> \
  'The following verification commands failed after your implementation:

   --- COMMAND ---
   go test ./...

   --- OUTPUT ---
   <captured stderr/stdout from the failed command>

   Fix the issues and ensure all tests pass.'
```

Using `codex exec resume` preserves the model's full context of what it already implemented and why. However, if resume does not preserve `--add-dir` and workspace constraints (see §6), the FIX phase should fall back to a fresh `codex exec` with compressed prior-attempt context in the prompt.

#### Iteration Limits

| Setting | Default | Description |
|---|---|---|
| `max_fix_attempts` | 3 | Maximum FIX→VERIFY cycles before giving up |
| `verify_timeout` | 5 min | Per-command timeout during VERIFY |

After `max_fix_attempts` FIX cycles without a passing VERIFY, the orchestrator marks the task ❌ and generates the failure report with the full history of attempts:

```md
## Failure Report

### Attempt 1 (CREATE)
- Verification: `go test ./...` → FAIL
- Error: <output>

### Attempt 2 (FIX)
- Verification: `go test ./...` → FAIL
- Error: <output>

### Attempt 3 (FIX)
- Verification: `go test ./...` → FAIL
- Error: <output>

### Conclusion
Task failed after 3 fix attempts. Root cause appears to be: <...>
```

#### Loop Interaction with Session Persistence

Each CREATE and FIX phase uses the same codex session (`thread_id`). This means:
- The model has full conversation history — it knows what it tried before
- Token usage accumulates across the loop (tracked in state file)
- If the orchestrator is interrupted mid-loop, it can resume from the last phase

### 3.10 Pre-Flight Checks

Before starting any work, the orchestrator runs a pre-flight checklist. If any check fails, it exits immediately with a clear diagnostic.

| Check | Command / Method | Failure message |
|---|---|---|
| Codex installed | `codex --version` | `codex not found on PATH` |
| Codex authenticated | `codex login status` | `codex not logged in — run codex login` |
| Model valid | `codex exec --ephemeral -m <MODEL> 'Reply OK'` | `model <MODEL> not available` |
| Git clean | `git status --porcelain` | `uncommitted changes — commit or stash first` |
| Submodules initialized | `git submodule status --recursive` | `submodule <name> not initialized` |
| Set directories exist | `os.Stat(design/<project>/<set>)` | `set directory <set> not found` |
| `_TASKS.md` present | file check | `_TASKS.md missing in <set>` |

The model validation check uses `--ephemeral` to avoid polluting session history — it's a throwaway smoke test.

### 3.11 Submodule Write Access

Tasks frequently modify code in git submodules (e.g. `go-holons`, `rust-holons`, `swift-holons`). The default `-s workspace-write` sandbox only permits writes within the `-C` root. Submodules may reside outside this tree.

The orchestrator solves this by:

1. **Scanning the task file** for repository references (lines containing `github.com/organic-programming/<repo>` or `## Repository` sections).
2. **Resolving submodule paths** via `git submodule foreach --recursive` to map repo names to local paths.
3. **Passing `--add-dir <path>`** for each referenced submodule.

```bash
codex exec --json --skip-git-repo-check \
  -C <ROOT_REPO> -s workspace-write \
  --add-dir go-holons \
  --add-dir rust-holons \
  -m gpt-5.4 \
  '<PROMPT>'
```

If no submodule references are detected, no `--add-dir` flags are added.

### 3.12 Token Accounting

The orchestrator tracks token usage from JSONL `turn.completed` events across the entire run.

#### Tracked Metrics

| Metric | Source |
|---|---|
| `input_tokens` | Per-turn input token count |
| `cached_input_tokens` | Tokens served from cache (no cost) |
| `output_tokens` | Generated tokens |

#### Aggregation Levels

- **Per task** — total across all CREATE + FIX turns
- **Per version set** — sum of all tasks in the set
- **Per run** — grand total

The state file includes cumulative token counts. After each task, the orchestrator logs a one-line summary:

```
2026_03_10_19_15_07_893 [tokens] TASK01: input=12,450 cached=8,200 output=3,100
```

### 3.13 Concurrency Lock

Only one orchestrator instance may run per repository at a time. On startup:

1. Attempt to create `.codex_orchestrator.lock` with the current PID.
2. If the file exists, read the stored PID and check if the process is alive (`kill -0 <pid>`).
3. If alive → exit with `another orchestrator is running (PID <pid>)`.
4. If dead → reclaim the lock (stale lock from a crash).

The lock is released on clean exit and on signal handling (§3.14).

### 3.14 Signal Handling

The orchestrator handles `SIGINT` (Ctrl-C) and `SIGTERM` gracefully:

1. **If codex is running** — forward the signal to the codex child process and wait for it to exit. Do not kill it abruptly — let it finish writing its session state.
2. **Save orchestrator state** — write the current loop phase, attempt count, and any partial results to the state file.
3. **Release the lock** — remove `.codex_orchestrator.lock`.
4. **Log the interruption** — write a timestamped message to both the terminal and the task log.

A second `SIGINT` within 3 seconds forces an immediate exit (for stuck processes).

The next run will detect the incomplete task in the state file and resume from the interrupted phase.

### 3.15 Post-Run Summary

After all sets are processed (or on early termination), the orchestrator prints a structured summary:

```
═══════════════════════════════════════════
  Codex Orchestrator — Run Summary
═══════════════════════════════════════════

  Sets:    v0.4, v0.5
  Model:   gpt-5.4
  Elapsed: 47m 12s

  ┌─────────┬───────┬────────┬─────────┬────────────┐
  │ Set     │ Tasks │ ✅ Pass │ ❌ Fail │ Tokens     │
  ├─────────┼───────┼────────┼─────────┼────────────┤
  │ v0.4    │ 6     │ 5      │ 1       │ 142,300    │
  │ v0.5    │ 3     │ 3      │ 0       │  58,700    │
  ├─────────┼───────┼────────┼─────────┼────────────┤
  │ Total   │ 9     │ 8      │ 1       │ 201,000    │
  └─────────┴───────┴────────┴─────────┴────────────┘

  Folder status:
    v0.4 → ⚠️ (TASK03 failed)
    v0.5 → ✅

  Failed tasks:
    - grace-op_v0.4_TASK03_assembly_manifests.❌.md
      → see grace-op_v0.4_TASK03_assembly_manifests.failure.md

═══════════════════════════════════════════
```

This summary is also written to `<root>/.codex_orchestrator_summary.md` for post-mortem review.

## 4. Architecture

### 4.1 Project Layout

The orchestrator follows [Effective Go](https://go.dev/doc/effective_go) and the standard Go project layout:

```
_orchestrator/
├── go.mod                          # module: github.com/organic-programming/codex-orchestrator
├── cmd/
│   └── orchestrator/
│       └── main.go                 # entry point — wire + run
├── internal/
│   ├── cli/
│   │   └── cli.go                  # flag parsing, Config struct
│   ├── codex/
│   │   ├── exec.go                 # codex exec wrapper (os/exec)
│   │   ├── jsonl.go                # JSONL event parsing, thread_id extraction
│   │   └── retry.go                # error classification + retry policy (§3.7)
│   ├── git/
│   │   ├── branch.go               # branch check, creation (recursive submodules)
│   │   ├── submodule.go            # submodule discovery, --add-dir resolution (§3.11)
│   │   └── ops.go                  # git mv, commit, push, tag
│   ├── lifecycle/
│   │   ├── start.go                # start task (§3.5)
│   │   ├── complete.go             # complete task ✅/❌ (§3.5)
│   │   ├── status.go               # version folder status (💭/✅/⚠️)
│   │   └── release.go              # holon.yaml bump + git tag
│   ├── logging/
│   │   └── tee.go                  # timestamped tee writer (§3.4)
│   ├── prompt/
│   │   ├── builder.go              # 4-layer prompt assembly (§3.8)
│   │   └── compress.go             # history compression
│   ├── state/
│   │   ├── state.go                # state file load/save
│   │   └── lock.go                 # PID-based concurrency lock (§3.13)
│   ├── tasks/
│   │   ├── parser.go               # _TASKS.md table parser
│   │   └── dag.go                  # dependency DAG + topological sort (§3.6)
│   ├── verify/
│   │   ├── extract.go              # extract verifiable commands from task files
│   │   └── runner.go               # run verification commands (§3.9 VERIFY phase)
│   ├── preflight/
│   │   └── checks.go               # pre-flight validation (§3.10)
│   └── summary/
│       └── summary.go              # post-run summary report (§3.15)
└── v1.0/                           # design docs + task specs (this folder)
    ├── DESIGN.md
    ├── _TASKS.md
    └── orchestrator_v1_TASK*.md
```

### 4.2 Package Responsibilities

| Package | Responsibility | DESIGN.md section |
|---|---|---|
| `cmd/orchestrator` | Entry point: wire dependencies, run the main loop | — |
| `internal/cli` | Parse `--set`, `--model`, `--root` flags; build `Config` | §3.1 |
| `internal/codex` | Wrap `codex exec`, parse JSONL, classify errors, retry | §2.1, §3.4, §3.7 |
| `internal/git` | Branch ops, submodule traversal, `git mv/commit/push/tag` | §3.2, §3.11 |
| `internal/lifecycle` | Task start/complete, version status, release | §3.5 |
| `internal/logging` | Timestamped tee writer for stdout/stderr | §3.4 |
| `internal/prompt` | 4-layer prompt assembly, token estimation, compression | §3.8 |
| `internal/state` | State file persistence, concurrency lock | §3.4, §3.13 |
| `internal/tasks` | `_TASKS.md` parser, dependency DAG, topological sort | §3.6 |
| `internal/verify` | Extract + run verification commands from task files | §3.9 |
| `internal/preflight` | Pre-flight environment validation | §3.10 |
| `internal/summary` | Post-run summary report | §3.15 |

### 4.3 Package Contracts

Canonical type definitions and cross-package function signatures.
Task specs reference this section — any signature conflict must be
resolved here first.

```go
// --- internal/cli ---

type Config struct {
    Sets      []string
    Model     string
    Root      string
    StateFile string // default: <root>/.codex_orchestrator_state.json
}

// --- internal/tasks ---

type Entry struct {
    Number    string
    FilePath  string   // resolved to absolute path
    Summary   string
    DependsOn []string // e.g. ["TASK01", "TASK03"]
}

func Parse(tasksFile string) ([]Entry, error)
func Sort(entries []Entry) ([]Entry, error)
func FindSetDir(root, setName string) (setDir, project string, err error)

// --- internal/state ---

type State struct { /* per-task state keyed by file path */ }

func Load(stateFile string) *State
func (s *State) Save() error
func (s *State) IsCompleted(taskFile string) bool
func (s *State) CompletedResults(setDir string) []string

type Lock struct { path string }

func Acquire(root string) (*Lock, error)
func (l *Lock) Release() error

// --- internal/codex ---

type Result struct {
    Success   bool
    ThreadID  string
    Output    string
    Attempts  int
    Tokens    state.TokenUsage
}

func ExecuteLoop(cfg cli.Config, task tasks.Entry, prompt string, addDirs []string, st *state.State) Result
func SetCurrentCmd(cmd *exec.Cmd)  // signal handler accessor
func CurrentCmd() *exec.Cmd        // signal handler accessor

// --- internal/git ---

type Ops struct { Root string }

func (o *Ops) Rename(from, to string) error
func (o *Ops) AddCommitPush(msg string, files ...string) error
func (o *Ops) Tag(name, msg string) error
func EnsureConsistency(root, project, setName string) error
func ListSubmodulePaths(root string) ([]string, error)
func DetectRefs(taskContent string, submodules []string) []string

// --- internal/lifecycle ---

func StartTask(task tasks.Entry, setDir string, git *git.Ops) error
func CompleteTask(task tasks.Entry, result codex.Result, setDir string, git *git.Ops) error
func UpdateVersionStatus(setDir string, entries []tasks.Entry, git *git.Ops) error
func Reset(setDir string, st *state.State, git *git.Ops) error
func Release(setDir, version string, git *git.Ops) error

// --- internal/summary ---

type SetResult struct {
    Name     string
    Tasks    int
    Passed   int
    Failed   int
    Tokens   state.TokenUsage
    Status   string   // ✅, ⚠️, 💭
    Failures []string
}

func Print(st *state.State, sets []SetResult, elapsed time.Duration)
func BuildSetResult(setName string, entries []tasks.Entry, st *state.State) SetResult
```

### 4.4 Execution Flow

```go
// cmd/orchestrator/main.go — simplified
func main() {
    cfg := cli.Parse()
    lock, _ := state.Acquire(cfg.Root)
    defer lock.Release()

    preflight.Run(cfg)           // §3.10
    st := state.Load(cfg.StateFile)
    setupSignalHandler(st, lock) // §3.14

    startTime := time.Now()
    var setResults []summary.SetResult

    for _, setName := range cfg.Sets {
        setDir, project, _ := tasks.FindSetDir(cfg.Root, setName)
        git.EnsureConsistency(cfg.Root, project, setName)
        entries, _ := tasks.Parse(filepath.Join(setDir, "_TASKS.md"))
        ordered, _ := tasks.Sort(entries)

        for _, task := range ordered {
            if st.IsCompleted(task.FilePath) { continue }
            gitOps := &git.Ops{Root: cfg.Root}
            lifecycle.StartTask(task, setDir, gitOps)

            priorResults := st.CompletedResults(setDir)
            p, _ := prompt.Build(cfg, setDir, task.FilePath, priorResults)
            submodules, _ := git.ListSubmodulePaths(cfg.Root)
            addDirs := git.DetectRefs(string(must(os.ReadFile(task.FilePath))), submodules)

            result := codex.ExecuteLoop(cfg, task, p, addDirs, st) // §3.9
            lifecycle.CompleteTask(task, result, setDir, gitOps)
            lifecycle.UpdateVersionStatus(setDir, ordered, gitOps)
            st.Save()
        }

        setResults = append(setResults, summary.BuildSetResult(setName, ordered, st))
        if allPassed(ordered, st) {
            lifecycle.Release(setDir, setName, &git.Ops{Root: cfg.Root})
        }
    }

    summary.Print(st, setResults, time.Since(startTime)) // §3.15
}
```

### 4.5 Design Principles

- **No global state.** Dependencies are passed explicitly. The `main.go` function is the only place where components are wired together. Exception: `codex.SetCurrentCmd`/`CurrentCmd` are package-level accessors required by the signal handler (§3.14), since `os/signal.Notify` callbacks cannot receive injected dependencies.
- **`internal/` enforced.** All packages live under `internal/` — they are not importable by external modules. This is intentional: the orchestrator is a standalone tool, not a library.
- **Interfaces at boundaries.** The `codex` package defines an `Executor` interface so tests can mock codex invocations without hitting the real CLI.
- **Errors are values.** Functions return `error`, never call `log.Fatal`. Only `main.go` decides whether to exit.

## 5. Usage

```bash
# Build the binary
go build -o orchestrator ./cmd/orchestrator

# Single set, default model
./orchestrator --set v0.4

# Multiple sets, explicit model
./orchestrator --set v0.4 --set v0.5 --model gpt-5.4

# From a specific root directory
./orchestrator --root /path/to/repo --set v0.6 --model gpt-5.4
```

## 6. Open Questions / Future Work

- [ ] **Output Schema:** Use `--output-schema` to enforce structured task completion reports.
- [ ] **Task filtering:** Support glob or tag-based filtering of tasks within a set (e.g. only `OP_TASK*`).
- [ ] **Dry-run mode:** Print the planned codex commands without executing them.
- [ ] **Approval escalation:** Detect when codex is waiting for human approval (in full-auto mode) and surface it.
- [ ] **Parallel execution:** Independent tasks (no shared dependencies) could run concurrently. Requires careful git state management.
- [ ] **Cross-version pre-check:** Before starting a set, verify all referenced cross-version dependencies are `✅`.
- [ ] **Resume sandbox parity:** `codex exec resume` may not preserve the same `--add-dir` and workspace constraints as the original CREATE session. Until verified, the FIX loop should fall back to a fresh `codex exec` with compressed prior-attempt context.
