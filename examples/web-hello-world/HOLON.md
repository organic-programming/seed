---
# Holon Identity v1
uuid: "9b646caf-2a97-49f3-8256-ce51fd6c8922"
given_name: "web-hello-world"
family_name: "Example Holons"
motto: "Browser-accessible hello holon using WebSocket transport."
composer: "B. ALTER"
clade: "deterministic/pure"
status: draft
born: "2026-02-12"

# Lineage
parents: []
reproduction: "manual"

# Metadata
generated_by: "manual"
proto_status: draft
---

# web-hello-world Example Holons

> *"Browser-accessible hello holon using WebSocket transport."*

## Description

A browser-accessible Go holon that demonstrates the full browser to WebSocket
to Go pipeline using `go-holons/pkg/transport.WebBridge` and `js-web-holons`.

## Introspection Notes

- Serves static files and the WebSocket bridge from the same Go HTTP server.
- Uses `holon-rpc` JSON-RPC 2.0 over WebSocket rather than the standard gRPC
  serve facet.
- The executable entrypoint is `main.go` at the holon root.
