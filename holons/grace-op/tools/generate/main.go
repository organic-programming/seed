// Package main regenerates grace-op's Go protobuf bindings from
// api/v1/holon.proto.
//
// Invoked automatically by `op build op` via the holon manifest's
// before_commands hook. Can also be run directly:
//
//     cd holons/grace-op && go run ./tools/generate
//
// In organic programming the .proto IS the source code. This generator
// keeps the .pb.go bindings under gen/go/op/v1/ in sync with the
// hand-edited proto, the same way a compiler keeps object code in sync
// with sources.
//
// Mirrors the pattern of holons/mody-media/tools/generate but stays
// minimal: grace-op authors its proto by hand (no OpenAPI mirror) and
// only needs the protoc invocation.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

const (
	holonProto      = "api/v1/holon.proto"
	holonGoPackage  = "github.com/organic-programming/grace-op/gen/go/op/v1"
	moduleRoot      = "github.com/organic-programming/grace-op/gen/go"
	manifestProto   = "holons/v1/manifest.proto"
	manifestGoPackg = "github.com/organic-programming/go-holons/gen/go/holons/v1"
)

func main() {
	for _, tool := range []string{"protoc", "protoc-gen-go", "protoc-gen-go-grpc"} {
		if _, err := exec.LookPath(tool); err != nil {
			log.Fatalf("missing %s on PATH: %s", tool, installHint(tool))
		}
	}

	if err := os.MkdirAll("gen/go", 0o755); err != nil {
		log.Fatalf("mkdir gen/go: %v", err)
	}

	args := []string{
		"-I", ".",
		"-I", "_protos",
		"--go_out=gen/go",
		"--go_opt=module=" + moduleRoot,
		"--go_opt=M" + holonProto + "=" + holonGoPackage,
		"--go_opt=M" + manifestProto + "=" + manifestGoPackg,
		"--go-grpc_out=gen/go",
		"--go-grpc_opt=module=" + moduleRoot,
		"--go-grpc_opt=M" + holonProto + "=" + holonGoPackage,
		"--go-grpc_opt=M" + manifestProto + "=" + manifestGoPackg,
		holonProto,
	}

	cmd := exec.Command("protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("protoc failed: %v", err)
	}
}

func installHint(tool string) string {
	switch tool {
	case "protoc":
		return "install via `brew install protobuf` (macOS) or `apt install protobuf-compiler` (Linux)"
	case "protoc-gen-go":
		return "install via `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`"
	case "protoc-gen-go-grpc":
		return "install via `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`"
	}
	return fmt.Sprintf("install %s and ensure it is on PATH", tool)
}
