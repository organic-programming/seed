# Recipes — Design Documents

_(Cross-cutting track — evolves with every phase.)_

Assembly manifests, composition patterns (direct-call,
pipeline, fan-out), and testmatrix validation. Recipes
grow each time a new holon or transport is added.

## Current State

Grace v0.4.1–v0.4.3 established the recipe ecosystem:
8 daemons, 6 HostUIs, 48 assembly manifests, combinatorial
testmatrix.

## Upcoming

- New recipes as holons ship (Line, Phill, Wisupaa, Megg)
- Transport-specific assembly patterns (REST+SSE, mesh)
- Cross-holon composition (Wisupaa + Megg pipelines)
