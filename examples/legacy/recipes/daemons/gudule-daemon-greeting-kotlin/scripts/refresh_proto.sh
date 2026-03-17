#!/bin/sh
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
CANONICAL_ROOT="$ROOT/../../protos"
DESCRIBE_PROTO="$ROOT/protos/describe/greeting/v1/greeting.proto"
GEN_ROOT="$ROOT/gen/kotlin"
GRPC_KOTLIN_JAR="$HOME/.gradle/caches/modules-2/files-2.1/io.grpc/protoc-gen-grpc-kotlin/1.4.3/7cbde5c64967023bfcb2ea1e5414a4b6ee3298f8/protoc-gen-grpc-kotlin-1.4.3-jdk8.jar"
GRPC_JAVA_PLUGIN="$HOME/.gradle/caches/modules-2/files-2.1/io.grpc/protoc-gen-grpc-java/1.76.0/acc9bb0fcb143ce54188fb82de77885073fbfcf4/protoc-gen-grpc-java-1.76.0-osx-aarch_64.exe"
GRPC_KOTLIN_WRAPPER="$ROOT/.protoc-gen-grpc-kotlin"

mkdir -p "$(dirname "$DESCRIBE_PROTO")" "$GEN_ROOT"
cp "$CANONICAL_ROOT/greeting/v1/greeting.proto" "$DESCRIBE_PROTO"

cat > "$GRPC_KOTLIN_WRAPPER" <<EOF
#!/bin/sh
exec java -jar "$GRPC_KOTLIN_JAR" "\$@"
EOF
chmod +x "$GRPC_KOTLIN_WRAPPER"

protoc \
  -I "$CANONICAL_ROOT" \
  --plugin=protoc-gen-grpckt="$GRPC_KOTLIN_WRAPPER" \
  --plugin=protoc-gen-grpc-java="$GRPC_JAVA_PLUGIN" \
  --java_out="$GEN_ROOT" \
  --kotlin_out="$GEN_ROOT" \
  --grpc-java_out="$GEN_ROOT" \
  --grpckt_out="$GEN_ROOT" \
  "$CANONICAL_ROOT/greeting/v1/greeting.proto"
