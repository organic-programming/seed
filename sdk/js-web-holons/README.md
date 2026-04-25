# js-web-holons

Browser-only Holon-RPC client SDK for holons.

`js-web-holons` is dial-only. It connects outward to remote holons over JSON-RPC, but it does not expose `serve` or `listen` helpers.

## connect

WebSocket JSON-RPC:

```js
import { connect } from "js-web-holons";

const client = connect("ws://127.0.0.1:8080/api/v1/rpc", {
  reconnect: false,
  heartbeat: false,
});

const response = await client.invoke("greeting.v1.GreetingService/SayHello", {
  name: "Gabriel",
});
```

HTTP+SSE JSON-RPC:

```js
import { connect } from "js-web-holons";

const client = connect("https://example.com/api/v1/rpc");

const response = await client.invoke("greeting.v1.GreetingService/SayHello", {
  name: "Gabriel",
});
```

For server-streaming methods over HTTP+SSE:

```js
for await (const event of client.stream("build.v1.BuildService/WatchBuild", {
  project: "hello-world",
})) {
  if (event.event === "message") console.log(event.result);
}
```

## describe

Call `HolonMeta/Describe` on a remote holon over either WebSocket JSON-RPC or HTTP POST:

```js
import { describe } from "js-web-holons";

const meta = await describe("https://example.com/api/v1/rpc");
```

Or against a WebSocket endpoint:

```js
import { describe } from "js-web-holons";

const meta = await describe("wss://example.com/api/v1/rpc", {
  connectOptions: {
    reconnect: false,
    heartbeat: false,
  },
});
```

## observability

Browser holons expose observability over the existing bidirectional WebSocket
channel. They do not open a local listener or write run directories.

```js
import { configure, registerObservabilityService } from "js-web-holons";

globalThis.__HOLON_ENV__ = { OP_OBS: "logs,metrics,events" };
const obs = configure({ slug: "browser-greeter", instanceUid: "web-1" });

registerObservabilityService(client, obs);
obs.logger("ui").info("ready");
```

The registered handlers are:

- `holons.v1.HolonObservability/Logs`
- `holons.v1.HolonObservability/Metrics`
- `holons.v1.HolonObservability/Events`

`OP_OBS=otel`, `OP_OBS=sessions`, and `OP_SESSIONS` are rejected in v1.

## build and test

```sh
npm install
npm test
```

## transport

Supported dial URIs:

- `ws://host[:port][/api/v1/rpc]`
- `wss://host[:port][/api/v1/rpc]`
- `http://host[:port][/api/v1/rpc]`
- `https://host[:port][/api/v1/rpc]`

Unsupported in the browser SDK:

- `tcp://`
- `unix://`
- `stdio://`

## note

This SDK is browser-only and dial-only:

- no `serve`
- no `listen`
- no local holon build or static Describe registration
