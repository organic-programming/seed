# js-hello-world

A minimal Node.js holon implementing `hello.v1.HelloService/Greet`, now using
`@organic-programming/holons` for standard serve/transport behavior.

## Setup

```bash
npm install
```

## Run

Default (`tcp://:9090`):

```bash
npm start
```

Custom transport URI:

```bash
node server.mjs --listen tcp://0.0.0.0:9090
node server.mjs --listen unix:///tmp/js-hello.sock
node server.mjs --listen ws://0.0.0.0:8080/grpc
node server.mjs --listen stdio://
```

Legacy `--port` compatibility:

```bash
node server.mjs --port 8080
```

## Test

```bash
npm test
```

## Invoke via stdio (zero config)

```bash
op grpc+stdio://"node server.mjs" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

## Notes

- `wss://` requires TLS key/cert configuration in the SDK (`HOLONS_TLS_KEY_FILE`, `HOLONS_TLS_CERT_FILE`, or serve options).
