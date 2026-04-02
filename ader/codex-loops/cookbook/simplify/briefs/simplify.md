Refactor the selected code to reduce source lines of code while preserving the existing perimeter.

## Invariant
- The metric is boolean: keep the change only if the gate still passes
- Preserve behavior and public API shape
- No new dependencies
- Stay inside the current package or module boundary unless the brief says otherwise

## Priority
- Remove duplication
- Factor shared logic into smaller helpers
- Delete dead code
- Collapse verbose conditionals and control flow
- Prefer fewer lines and simpler structure over cleverness

## Iteration rule
Make one focused simplification, stop, and let the next iteration continue from the kept baseline.
