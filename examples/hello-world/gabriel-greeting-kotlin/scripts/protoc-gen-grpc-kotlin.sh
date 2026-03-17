#!/bin/zsh
set -euo pipefail

exec java -jar "$HOME/.gradle/caches/modules-2/files-2.1/io.grpc/protoc-gen-grpc-kotlin/1.4.3/7cbde5c64967023bfcb2ea1e5414a4b6ee3298f8/protoc-gen-grpc-kotlin-1.4.3-jdk8.jar" "$@"
