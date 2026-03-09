# python-hello-world

A minimal holon implementing `HelloService.Greet` in Python with the
`python-holons` serve runner and `connect()`.

## Setup

```bash
pip3 install grpcio grpcio-tools grpcio-reflection
bash generate.sh
```

## Run

```bash
python3 server.py
# or with a custom port:
python3 server.py serve --listen tcp://127.0.0.1:8080
```

## Test

```bash
python3 -m pytest test_server.py -v
```

## Invoke via stdio (zero config)

```bash
op grpc+stdio://"python3 server.py" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```

No server to start, no port to allocate. OP launches the process,
communicates over stdin/stdout via gRPC, and tears everything down.

## Connect example

```bash
python3 connect_example.py
# → {"message":"hello-from-python","sdk":"python-holons","version":"0.1.0"}
```
