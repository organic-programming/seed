---
# Cartouche v1
title: "python-hello-world — Hello World Holon in Python"
author:
  name: "B. ALTER"
  copyright: "© 2026 Benoit Pereira da Silva"
created: 2026-02-12
revised: 2026-02-12
lang: en-US
access:
  humans: true
  agents: false
status: draft
---
# python-hello-world

A minimal holon implementing `HelloService.Greet` in Python.

## Setup

```bash
pip3 install grpcio grpcio-tools grpcio-reflection
bash generate.sh
```

## Run

```bash
python3 server.py
# or with a custom port:
python3 server.py --port 8080
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
