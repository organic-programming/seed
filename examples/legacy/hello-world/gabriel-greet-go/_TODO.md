# @BPDS

il faudrait aussi montrer un cas d'usage un code go, utilise le SDK connect pour lancer et communiquer avec gabriel.
C'est encore un autre cas d'usage. Holon to holon mais illustré en code.





Je vais répondre en partant du comportement exact du runtime actuel, puis je distinguerai ce qui est impossible aujourd’hui de ce qui est théoriquement faisable avec une autre architecture.

Explored 3 files, 1 search
Worked for 38s
Short answer: not for the current meaning of mem, but yes with a different holon runtime model.

Today mem in this codebase is literally in-process gRPC over bufconn, not “fast local IPC”. You can see that in sdk/go-holons/pkg/transport/mem.go, sdk/go-holons/pkg/grpcclient/mem.go, and the process-local registry in sdk/go-holons/pkg/connect/mem_registry.go. So an arbitrary external binary cannot be called via that transport, because it does not share the same address space.

So the real answer is:

Any existing standalone holon binary callable via current grpc+mem://: no.
Any holon callable via a future dynamic in-process host model: yes, theoretically.
The general approach would be to make mem a capability of “hostable holons”, not of “all binaries”. That means every holon would need a loadable artifact and a stable host ABI, for example:

a WASM/component artifact
a native plugin ABI
a language-specific embeddable module system
Then op could do this:

Resolve the holon slug.
Detect whether it has a hostable artifact.
Load it dynamically into the op process.
Instantiate its service in-process.
Register a mem dialer on the fly.
Call it through the same mem path.
That would make dynamic mem possible without compile-time registration.

My recommendation is:

Keep auto as the universal mechanism for all holons.
Keep mem as “same-process hosted holon”.
If you want universal dynamic mem, introduce a new standard hosted artifact kind, ideally cross-language, instead of trying to make arbitrary native binaries magically become in-process.
The cleanest long-term model is probably:

native binary holons: stdio / unix / tcp
hosted holons: mem
auto picks mem first only when the holon advertises a hostable artifact
If you want, I can now turn that into a concrete architecture plan for OP: manifest changes, loader API, host ABI, and migration path.


