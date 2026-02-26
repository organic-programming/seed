#!/bin/bash
# Generate Python gRPC stubs from hello.proto
set -e
python3 -m grpc_tools.protoc \
  -I api \
  --python_out=. \
  --grpc_python_out=. \
  api/hello.proto
echo "Generated hello_pb2.py and hello_pb2_grpc.py"
