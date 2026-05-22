# hello-world

Shared greeting examples for the `GreetingService` contract.

Each `gabriel-greeting-<lang>` holon implements the same `SayHello` and
`ListLanguages` RPCs over the shared protobuf contract in `_protos/`.

## Observability emission

`SayHello` emits a handler-boundary log and counter in every language whose SDK
exposes logger and counter primitives.

Log fields:

```text
lang_code, language, name, greeting, transport, duration_ns
```

Counter:

```text
greeting_emitted_total{lang_code, language, transport}
```

Go reports the active handler transport through `serve.CurrentTransport()`.
Other SDKs do not yet expose a handler-visible current-transport primitive, so
their handlers emit `transport = "unknown"` until that SDK surface exists.
