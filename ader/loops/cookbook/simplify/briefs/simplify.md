Refactor the code to reduce source lines of code while preserving behavior.

## Rules
- All existing tests must keep passing
- No change to the public API surface (exported types, functions, methods)
- No new dependencies
- Do not move code to other packages - reduce, don't relocate

## Strategies
- Extract duplicated logic into shared helpers
- Remove dead code (unreachable branches, unused types)
- Simplify conditionals (flatten nested if/else, use early returns)
- Replace verbose patterns with idiomatic Go (e.g., table-driven switches)
- Merge small files that share a single concern

Make one focused change and stop. Do not ask questions.
