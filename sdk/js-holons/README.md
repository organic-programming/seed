# js-holons

JavaScript SDK for holons on Node.js.

## serve

```js
const { describe, serve } = require('@organic-programming/holons');
const describeGenerated = require('./gen/describe_generated');
const grpcPb = require('./gen/node/greeting/v1/greeting_grpc_pb');
const publicApi = require('./api/public');

describe.useStaticResponse(describeGenerated.staticDescribeResponse());

await serve.runWithOptions('tcp://127.0.0.1:9090', (server) => {
    server.addService(grpcPb.GreetingServiceService, {
        ListLanguages(call, callback) {
            callback(null, publicApi.listLanguages(call.request));
        },
        SayHello(call, callback) {
            callback(null, publicApi.sayHello(call.request));
        },
    });
}, {
    reflect: false,
    logger: console,
});
```

## transport

Pass the listener URI to `serve.runWithOptions()` directly, or parse it from CLI flags with `serve.parseOptions(process.argv.slice(2))`.

`js-holons` can serve on `tcp://`, `unix://`, `stdio://`, `ws://`, and `wss://`. For example: `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, `stdio://`, `ws://127.0.0.1:8080/grpc`, or `wss://127.0.0.1:8443/grpc?cert=/path/cert.pem&key=/path/key.pem`.

For dial-only HTTP+SSE JSON-RPC, use `http://`, `https://`, or `rest+sse://` with `new holonrpc.HolonRPCClient()`.

## identity / describe

```js
describe.useStaticResponse(require('./gen/describe_generated').staticDescribeResponse());
```

`op build` generates `gen/describe_generated.js`; if that static response is not registered, startup fails with `no Incode Description registered — run op build`.

## discover

```js
const { Discover, LOCAL, CWD, NO_LIMIT, NO_TIMEOUT } = require('@organic-programming/holons');

const result = await Discover(LOCAL, 'gabriel-greeting-node', null, CWD, NO_LIMIT, NO_TIMEOUT);
```

## connect

```js
const { connect, LOCAL, INSTALLED, NO_TIMEOUT } = require('@organic-programming/holons');

const result = await connect(LOCAL, 'gabriel-greeting-node', null, INSTALLED, NO_TIMEOUT);
```
