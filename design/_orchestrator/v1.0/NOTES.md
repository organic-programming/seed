# Codex CLI — Model Compatibility Notes

Tested on **March 10, 2026** with ChatGPT-authenticated `codex-cli 0.104.0`.

## Working Explicit Codex Slugs

No `model_reroute` events observed — the CLI accepted these names directly.

| Model | `model_reasoning_effort` |
|---|---|
| `gpt-5.3-codex` | `xhigh` |
| `gpt-5.1-codex-max` | `xhigh` |
| `gpt-5.1-codex` | `high` |
| `gpt-5.1-codex-mini` | `high` |
| `gpt-5-codex` | `high` |
| `gpt-5-codex-mini` | `high` |

## Working Non-Codex Slug

| Model | `model_reasoning_effort` |
|---|---|
| `gpt-5.4` | `xhigh` |

This is **not** an explicit `*-codex` slug but works fine with codex exec.

## Does Not Work

- `gpt-5.4-codex`

## Recommendations

**Best explicit Codex model:**

```bash
codex exec --json --skip-git-repo-check --ephemeral \
  -m gpt-5.3-codex \
  -c 'model_reasoning_effort="xhigh"' \
  'Reply with exactly OK and nothing else.'
```

**Best overall (5.4, non-codex slug):**

```bash
codex exec --json --skip-git-repo-check --ephemeral \
  -m gpt-5.4 \
  -c 'model_reasoning_effort="xhigh"' \
  'Reply with exactly OK and nothing else.'
```

**Older Codex slugs — use `high`, not `xhigh`:**

```bash
codex exec --json --skip-git-repo-check --ephemeral \
  -m gpt-5.1-codex \
  -c 'model_reasoning_effort="high"' \
  'Reply with exactly OK and nothing else.'
```
