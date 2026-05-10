package main

import (
	"errors"
	"strings"
	"testing"
)

func TestJSAdapterPinsSiblingProtocGenJS(t *testing.T) {
	original := findOptionalSiblingExecutable
	t.Cleanup(func() { findOptionalSiblingExecutable = original })
	findOptionalSiblingExecutable = func(name string) (string, error) {
		switch name {
		case "protoc-gen-js":
			return "/sdk/bin/protoc-gen-js", nil
		case "grpc_tools_node_protoc_plugin":
			return "/sdk/bin/grpc_tools_node_protoc_plugin", nil
		default:
			return "", errors.New("missing")
		}
	}

	args, err := protocArgs("js", "/tmp/request.pb", "/tmp/out")
	if err != nil {
		t.Fatalf("protocArgs(js) failed: %v", err)
	}
	if !hasArg(args, "--plugin=protoc-gen-js=/sdk/bin/protoc-gen-js") {
		t.Fatalf("js args missing explicit protoc-gen-js plugin: %v", args)
	}
	if !hasArg(args, "--plugin=protoc-gen-grpc=/sdk/bin/grpc_tools_node_protoc_plugin") {
		t.Fatalf("js args missing explicit grpc plugin: %v", args)
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == want {
			return true
		}
	}
	return false
}
