---
# Holon Identity v1
uuid: "95a7a527-16f4-4363-a390-6604a391b966"
given_name: "c-hello-world"
family_name: "Example Holons"
motto: "The simplest possible holon - a greeting service in C."
composer: "B. ALTER"
clade: "deterministic/pure"
status: draft
born: "2026-02-12"

# Lineage
parents: []
reproduction: "manual"

# Optional
aliases: []

# Metadata
generated_by: "sophia-who"
lang: "c"
proto_status: draft
---

# c-hello-world Example Holons

> *"The simplest possible holon - a greeting service in C."*

## Description

A minimal, deterministic greeting holon implemented in C.
It exposes:
- a CLI facet (`greet`, `serve`, `version`)
- a transport-complete serve facet through `c-holons`
- unit tests for greeting logic and transport plumbing

## Introspection Notes

`ws://` and `wss://` are supported at the socket URI layer in this sample.
End-to-end WebSocket framing can be added by placing a WS gateway in front of
the serve loop when needed.
