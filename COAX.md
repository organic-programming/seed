# COAX : 

> A **holon** is, at any scale, an independent, composable, software functional unit
> built for [**coaccessibility** (**COAX**)](COAX.md) — equally accessible to humans
> and machines (agents, CI, LLM) through the same structural contracts.
>
> — [AGENT.md Article 1](./AGENT.md#article-1--the-holon)

## So What is COAX ? 

## Concretely how does it work ? 


### What happens when i call **op Greet jesus** on a COAX server?

1. You have launched [gabriel-greeting-app-swiftui](./examples/hello-world/gabriel-greeting-app-swiftui/) either by clicking on the app icon or by calling `op run gabriel-greeting-app-swiftui`. 
2. You have opted in for COAX by toggling it. 

#### Initial state of the app  

![Gabriel Greeting App SwiftUI greets Mary](./assets/images/gabriel-greeting-app-swiftui-mary.png)


#### You call **op Greet jesus** on the COAX server 

```shell
$ op grpc+tcp://127.0.0.1:60062 Greet '{"name":"Jesus"}'
{
  "greeting": "Hello Jesus"
}
```

#### State of the app after the call 

![Gabriel Greeting App SwiftUI greet Jesus](./assets/images/gabriel-greeting-app-swiftui-jesus.png)



#### Detailed step by step :

1. **OP CLI dispatch** — `op` parses `grpc+tcp://127.0.0.1:60062` as a gRPC URI.
   [commands.go](./internal/cli/commands.go) → `cmdGRPC` → `cmdGRPCDirect`
   since `127.0.0.1:60062` is a `host:port` target.

2. **OP TCP dial** — [client.go](./internal/grpcclient/client.go) → `Dial`
   opens a standard gRPC/HTTP2 connection over TCP to the COAX server exposed
   by gabriel-greeting-app-swiftui.

3. **OP sends Describe RPC** — [client.go](./internal/grpcclient/client.go) → `InvokeConn`
   tries `invokeViaDescribe` first.
   [describe_catalog.go](./internal/grpcclient/describe_catalog.go) → `fetchDescribeCatalog`
   calls `HolonMeta/Describe` on the COAX server.

4. **COAX receives Describe** — The Swift app's in-process gRPC server
   ([CoaxServer.swift](../../examples/hello-world/gabriel-greeting-app-swiftui/Modules/Sources/GreetingKit/CoaxServer.swift))
   routes the call to
   [CoaxDescribeProvider.swift](../../examples/hello-world/gabriel-greeting-app-swiftui/Modules/Sources/GreetingKit/CoaxDescribeProvider.swift) → `describe`.
   It returns a pre-built `DescribeResponse` containing the full schema for
   two services: `holons.v1.CoaxService` (member management) and
   `greeting.v1.GreetingAppService` (domain actions: `SelectHolon`,
   `SelectLanguage`, `Greet`), with all field definitions, types, and examples.

5. **OP builds a catalog** — [describe_catalog.go](./internal/grpcclient/describe_catalog.go) → `buildDescribeCatalog`
   indexes every method from the `DescribeResponse` into a lookup table
   (`byExact` for fully-qualified names, `byName` for short names). The catalog
   is cached per connection for subsequent calls.

6. **OP resolves the method Greet** — `catalog.resolve("Greet")` looks up
   `"Greet"` by short name, finds a single match in
   `greeting.v1.GreetingAppService`, and returns the `describeMethod` binding
   with the full gRPC path `/greeting.v1.GreetingAppService/Greet` and the
   `MethodDoc` field definitions (`name: string`, `lang_code: string`).

7. **OP builds synthetic descriptors** — `method.descriptors()` lazily builds
   real `protoreflect.MessageDescriptor` objects from the `MethodDoc` field
   definitions. `syntheticProtoBuilder` constructs a `FileDescriptorProto` with
   `GreetRequest` / `GreetResponse` message types, then compiles it via
   `protodesc.NewFiles` into proper protobuf descriptors — the same kind
   produced by `protoc` at compile time.

8. **OP JSON → dynamic protobuf message** — `protojson.Unmarshal` parses the CLI
   JSON `{"name":"Jesus"}` into a `dynamicpb.Message` backed by the synthetic
   `GreetRequest` descriptor.

9. **OP gRPC invoke (binary protobuf on the wire)** — `conn.Invoke` sends the
   request as **standard binary protobuf** over HTTP/2 to the COAX server.

10. **COAX receives the Greet RPC** — The gRPC server dispatches the call to
    [GreetingAppServiceProvider.swift](../../examples/hello-world/gabriel-greeting-app-swiftui/Modules/Sources/GreetingKit/GreetingAppServiceProvider.swift) → `greet`.
    The handler runs on `@MainActor` (the main thread), and:

    - Sets `holon.userName = "Jesus"` — because `HolonProcess` is an
      `@ObservableObject`, this **immediately updates the SwiftUI text field**
      in
      [ContentView.swift](../../examples/hello-world/gabriel-greeting-app-swiftui/App/ContentView.swift).
      The name "Jesus" appears in the input field in real time.
    - If `lang_code` is provided, sets `holon.selectedLanguageCode` accordingly
      (which updates the language picker in the UI).

11. **COAX delegates to the child holon** — The handler calls
    `holon.sayHello(name: "Jesus", langCode: langCode)` →
    [HolonProcess.swift](../../examples/hello-world/gabriel-greeting-app-swiftui/Modules/Sources/GreetingKit/HolonProcess.swift) → `sayHello`
    which forwards the request to the **currently connected child holon**
    (e.g. `gabriel-greeting-go`, `gabriel-greeting-rust`, etc.) via
    [GreetingClient.swift](../../examples/hello-world/gabriel-greeting-app-swiftui/Modules/Sources/GreetingKit/GreetingClient.swift) → `sayHello`.
    This is a second gRPC call — from the Swift app to the holon subprocess
    over stdio or tcp, using compiled protobuf stubs
    (`/greeting.v1.GreetingService/SayHello`).

12. **Child holon responds** — The selected holon (e.g. gabriel-greeting-go)
    processes the `SayHello` request and returns the localized greeting text
    (e.g. `"Hello, Jesus!"`).

13. **COAX updates the UI** — Back in `GreetingAppServiceProvider.greet()`, the
    handler sets `holon.greeting = greeting` on `@MainActor`. Because
    `HolonProcess.greeting` is `@Published`, **SwiftUI instantly updates the
    speech bubble** in `ContentView.bubbleColumn` — the greeting appears on
    screen.

14. **COAX returns response to OP** — The `GreetResponse` (containing the
    greeting string) is serialized as binary protobuf and sent back over TCP
    to `op`.

15. **OP receives response** — gRPC unmarshals the binary protobuf into
    `outputMsg` (a `dynamicpb.Message` backed by the synthetic `GreetResponse`
    descriptor).

16. **OP formats output** — `protojson.Marshal` converts the response back to
    JSON. `newCallResult` pretty-prints it for display in the terminal.

> **Key insight**: The COAX call produces **two visible effects simultaneously**:
> the terminal displays the JSON response, and the SwiftUI app updates its UI
> in real time (name field + greeting bubble). This happens because the COAX
> handler mutates the same `@Published` state that the UI observes — the agent
> and the human share a single source of truth.
