#!/usr/bin/env sh
set -eu

payload="$(cat || true)"
text="$(printf '%s' "$payload" | tr '[:upper:]' '[:lower:]')"

case "$text" in
  *op-worktree*)
    exit 0
    ;;
  *worktree*|*"git worktree"*)
    cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"For seed worktree creation, prefer op worktree create <branch> --isolated|--plain --json so the user chooses whether to isolate OPPATH/OPBIN."}}
JSON
    ;;
esac
