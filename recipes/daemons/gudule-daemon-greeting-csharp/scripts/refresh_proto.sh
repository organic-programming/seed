#!/bin/sh
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
CANONICAL_ROOT="$ROOT/../../protos"
DESCRIBE_PROTO="$ROOT/protos/describe/greeting/v1/greeting.proto"
GEN_ROOT="$ROOT/gen/csharp"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

mkdir -p "$(dirname "$DESCRIBE_PROTO")" "$GEN_ROOT"
cp "$CANONICAL_ROOT/greeting/v1/greeting.proto" "$DESCRIBE_PROTO"
cp "$CANONICAL_ROOT/greeting/v1/greeting.proto" "$TMPDIR/greeting.proto"

cat > "$TMPDIR/Generator.csproj" <<'EOF'
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Google.Protobuf" Version="3.31.1" />
    <PackageReference Include="Grpc.Tools" Version="2.76.0">
      <PrivateAssets>all</PrivateAssets>
      <IncludeAssets>runtime; build; native; contentfiles; analyzers; buildtransitive</IncludeAssets>
    </PackageReference>
    <PackageReference Include="Grpc.Core.Api" Version="2.76.0" />
  </ItemGroup>
  <ItemGroup>
    <Protobuf Include="greeting.proto" ProtoRoot="." GrpcServices="Both" />
  </ItemGroup>
</Project>
EOF

dotnet build "$TMPDIR/Generator.csproj" >/dev/null

OBJDIR="$(find "$TMPDIR/obj" -type d -path '*/net8.0' | head -n 1)"
cp "$OBJDIR/Greeting.cs" "$GEN_ROOT/Greeting.cs"
cp "$OBJDIR/GreetingGrpc.cs" "$GEN_ROOT/GreetingGrpc.cs"
