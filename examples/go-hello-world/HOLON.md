---
# Holon Identity v1
uuid: "b943b196-2844-4419-bcc2-131efdb3a682"
given_name: "go-hello-world"
family_name: "Example Holons"
motto: "The simplest possible holon — a greeting service."
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
lang: "go"
proto_status: draft
---

# hello-world Example Holons

> *"The simplest possible holon — a greeting service."*

## Description

A minimal, complete holon that demonstrates all three facets (CLI, gRPC, API)
and all five transports (tcp, unix, stdio, mem, ws). Use this as a reference
when building your own holons.

## Contract

```protobuf
service HelloService {
  rpc Greet(GreetRequest) returns (GreetResponse);
}
```

## Introspection Notes

- Pure deterministic: same input → same output, no side effects.
- Single RPC with one string field — the irreducible minimum.
