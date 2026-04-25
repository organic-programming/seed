# python-holons

Python SDK for holons.

## serve

```python
from __future__ import annotations

import sys

from holons import describe
from holons.serve import parse_options, run_with_options
from gen import describe_generated
from my_holon.v1 import service_pb2_grpc


describe.use_static_response(describe_generated.static_describe_response())


class MyService(service_pb2_grpc.MyServiceServicer):
    pass


def register(server) -> None:
    service_pb2_grpc.add_MyServiceServicer_to_server(MyService(), server)


if __name__ == "__main__":
    options = parse_options(sys.argv[1:])
    run_with_options(options.listen_uri, register, reflect=options.reflect)
```

## transport

Choose the listener with `--listen` or `run_with_options(listen_uri, ...)`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`.

Python `serve` supports `tcp://`, `unix://`, and `stdio://`. Client helpers also dial `ws://`, `wss://`, and `rest+sse://`.

## identity / describe

Wire the generated Incode Description with one line:

```python
describe.use_static_response(describe_generated.static_describe_response())
```

At build time, `op build` generates `gen/describe_generated.py`. At runtime, `serve` fails fast with `no Incode Description registered — run op build` if that static response is missing. For build and dev tooling, `holons.identity.resolve(".")` reads `holon.proto`.

## discover

```python
from holons.discover import find_by_slug

entry = find_by_slug("gabriel-greeting-python")
```

## connect

```python
from holons.connect import connect

channel = connect("gabriel-greeting-python")
```

## Build and test

```sh
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -e '.[dev]'
python -m pytest
```
