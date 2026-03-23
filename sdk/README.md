# Organic Programming SDKs

An SDK provides the native runtime for holons in a given language. It replaces hand-rolled gRPC boilerplate with a standardized set of modules.

## Every SDK implements:

- **`serve`**: The high-level orchestrator. Takes a listener, wires it to an RPC server, auto-registers the `HolonMeta.Describe` service, and gracefully handles OS signals for shutdown.
- **`transport`**: Opens sockets or pipes (`tcp://`, `unix://`, `stdio://`, `ws://`, `wss://`, `rest+sse://`).
- **`identity`**: The in-memory representation of the `holons.v1.HolonManifest` schema (from `manifest.proto`). It holds the holon's name, version, artifacts, and **slug** (e.g., `"rob-go"`, `"gabriel-greeting-app-swiftui"`), exposing them via the `Describe` service.
- **`discover`**: Scans the local filesystem to find nearby holons by slug.
- **`connect`**: Resolves a slug, starts the daemon if needed, and returns a ready channel.
- **`describe`**: Self-documentation and dynamic dispatch schema (via `HolonMeta.Describe`).

## Language SDKs

The following languages have official SDK implementations in their respective directories:

- `c-holons`
- `cpp-holons`
- `csharp-holons`
- `dart-holons`
- `go-holons`[^1]
- `java-holons`
- `js-holons`
- `js-web-holons`[^2]
- `kotlin-holons`
- `python-holons`
- `ruby-holons`
- `rust-holons`
- `swift-holons`

## CLI Holonization SDK

Expected for V1.0.0 but neither implemented nor fully specified : [go-cli-holonization](./go-cli-holonization/README.md)

## Transport Matrix 

Transports provide the underlying connection mechanism. Not all SDKs support all transports, an SDK may **Dial** (act as a client calling out) or **Serve** (act as a server listening) on a transport, and these capabilities are not always symmetrically supported [^3].

*   **both**: The SDK can both Dial (Client) and Serve (Server).
*   **dial**: The SDK can only Dial (Client).
*   **-**: Not implemented.
*   **?**: Implementation has not been controlled

| SDK | `tcp://` | `unix://` | `stdio://` | `ws://` | `wss://` | `rest+sse` |
|-----|:--------:|:---------:|:----------:|:-------:|:--------:|:----------:|
| `go-holons` | both | both | both | ? | ? | ? |
| `dart-holons` | ? | ? | ? | ? | ? | ? |
| `kotlin-holons` | ? | ? | ? | ? | ? | ? |
| `swift-holons` | ? | ? | ? | ? | ? | ? |
| `rust-holons` | ? | ? | ? | ? | ? | ? |
| `js-holons` | ? | ? | ? | ? | ? | ? |
| `js-web-holons` | - | - | - | ?| ? | ? |
| `c-holons` | ? | ? | ? | ? | ? | ? |
| `cpp-holons` | ? | ? | ? | ? | ? | ? |
| `csharp-holons`| ? | ? | ? | ? | ? | ? |
| `java-holons` | ? | ? | ? | ? | ? | ? |
| `python-holons`| ? | ? | ? | ? | ? | ? |
| `ruby-holons` | ? | ? | ? | ? | ? | ? |


## Expected v0.6 transport Matrix 

| SDK | `tcp://` | `unix://` | `stdio://` | `ws://` | `wss://` | `rest+sse` |
|-----|:--------:|:---------:|:----------:|:-------:|:--------:|:----------:|
| `go-holons` | both | both | both | both | both | both |
| `dart-holons` | both | both | both | dial | dial | dial |
| `kotlin-holons` | both | both | both | dial | dial | dial |
| `swift-holons` | both | both | both | dial | dial | dial |
| `rust-holons` | both | both | both | dial | dial | dial |
| `js-holons` | both | both | both | dial | dial | dial |
| `js-web-holons` | - | - | - | dial| dial | dial |
| `c-holons` | both | both | both | dial | dial | dial |
| `cpp-holons` | both | both | both | dial | dial | dial |
| `csharp-holons`| both | both | both | dial | dial | dial |
| `java-holons` | both | both | both | dial | dial | dial |
| `python-holons`| both | both | both | dial | dial | dial |
| `ruby-holons` | both | both | both | dial | dial | dial |


## References

- Protocol details, including the dynamic dispatch workflow: [`../PROTOCOL.md`](../PROTOCOL.md)
- **UNVERIFIED DEEP REVIEW REQUIRED DON'T USE THIS CURRENTLY**:  Directory and naming conventions for holon structure: [`../CONVENTIONS.md`](../CONVENTIONS.md)
- Examples and "Hello World" implementations: [`../examples/README.md`](../examples/README.md)

---

[^1]: **Reference Implementation**: Fully implements Dial and Serve across all transports.
[^2]: Browser-only: Can only Dial outward; cannot bind to a port to Serve.
[^3]: This is the direct transport matrix per sdk, using [op proxy](../holons/grace-op/PROXY.md) enables to serve any transports.
