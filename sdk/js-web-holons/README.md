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
