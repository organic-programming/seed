package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	internalgrpc "github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
	"google.golang.org/grpc"
)

func TestInvokeConnectedHolonCallPrefersLocalCatalog(t *testing.T) {
	remoteCalled := false
	localCalled := false

	previousLocal := invokeLocalCatalogRPC
	previousRemote := invokeRemoteRPC
	t.Cleanup(func() {
		invokeLocalCatalogRPC = previousLocal
		invokeRemoteRPC = previousRemote
	})

	invokeLocalCatalogRPC = func(_ context.Context, _ *grpc.ClientConn, holonName string, method string, inputJSON string) (*internalgrpc.CallResult, error) {
		localCalled = true
		if holonName != "just-asrligned" || method != "TranscribeAlign" || inputJSON != `{"audio":{}}` {
			t.Fatalf("unexpected local invoke args: holon=%q method=%q input=%q", holonName, method, inputJSON)
		}
		return &internalgrpc.CallResult{Output: `{"alignment":{"onnxGpu":true}}`}, nil
	}
	invokeRemoteRPC = func(_ context.Context, _ *grpc.ClientConn, _ string, _ string) (*internalgrpc.CallResult, error) {
		remoteCalled = true
		return &internalgrpc.CallResult{Output: `{"alignment":{"onnxGpu":false}}`}, nil
	}

	result, err := invokeConnectedHolonCall(context.Background(), nil, "just-asrligned", invokeCall{
		method:    "TranscribeAlign",
		inputJSON: `{"audio":{}}`,
	})
	if err != nil {
		t.Fatalf("invokeConnectedHolonCall returned error: %v", err)
	}
	if !localCalled {
		t.Fatal("local catalog invoke was not attempted")
	}
	if remoteCalled {
		t.Fatal("remote invoke should not run when local catalog succeeds")
	}
	if got, want := result.Output, `{"alignment":{"onnxGpu":true}}`; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestInvokeConnectedHolonCallFallsBackToRemote(t *testing.T) {
	previousLocal := invokeLocalCatalogRPC
	previousRemote := invokeRemoteRPC
	t.Cleanup(func() {
		invokeLocalCatalogRPC = previousLocal
		invokeRemoteRPC = previousRemote
	})

	invokeLocalCatalogRPC = func(_ context.Context, _ *grpc.ClientConn, _ string, _ string, _ string) (*internalgrpc.CallResult, error) {
		return nil, errors.New("no local descriptor")
	}
	invokeRemoteRPC = func(_ context.Context, _ *grpc.ClientConn, method string, inputJSON string) (*internalgrpc.CallResult, error) {
		if method != "TranscribeAlign" || inputJSON != `{"audio":{}}` {
			t.Fatalf("unexpected remote invoke args: method=%q input=%q", method, inputJSON)
		}
		return &internalgrpc.CallResult{Output: `{"detectedLanguage":"fr"}`}, nil
	}

	result, err := invokeConnectedHolonCall(context.Background(), nil, "just-asrligned", invokeCall{
		method:    "TranscribeAlign",
		inputJSON: `{"audio":{}}`,
	})
	if err != nil {
		t.Fatalf("invokeConnectedHolonCall returned error: %v", err)
	}
	if got, want := result.Output, `{"detectedLanguage":"fr"}`; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestCompositeServeCommandLaunchesMacAppExecutableDirectly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific")
	}

	root := t.TempDir()
	bundle := filepath.Join(root, "Example.app")
	macosDir := filepath.Join(bundle, "Contents", "MacOS")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		t.Fatal(err)
	}

	executablePath := filepath.Join(macosDir, "example")
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>org.organicprogramming.example</string>
	<key>CFBundleExecutable</key>
	<string>example</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &holons.LoadedManifest{
		Dir:  root,
		Name: "gabriel-greeting-app-flutter",
		Manifest: holons.Manifest{
			Kind:       holons.KindComposite,
			FamilyName: "Greeting-App-Flutter",
			Transport:  "stdio",
		},
	}

	cmd, cleanup, err := compositeServeCommand(
		manifest,
		bundle,
		"tcp://127.0.0.1:60000",
	)
	if cleanup != nil {
		defer func() { _ = cleanup() }()
	}
	if err != nil {
		t.Fatalf("compositeServeCommand returned error: %v", err)
	}
	if cmd.Path != executablePath {
		t.Fatalf("cmd.Path = %q, want %q", cmd.Path, executablePath)
	}
	if got := envValue(cmd.Env, "OP_COAX_SERVER_ENABLED"); got != "1" {
		t.Fatalf("OP_COAX_SERVER_ENABLED = %q, want 1", got)
	}
	if got := envValue(cmd.Env, "OP_COAX_SERVER_LISTEN_URI"); got != "tcp://127.0.0.1:60000" {
		t.Fatalf("OP_COAX_SERVER_LISTEN_URI = %q", got)
	}
	if got := envValue(cmd.Env, "CFFIXED_USER_HOME"); got == "" {
		t.Fatal("CFFIXED_USER_HOME missing")
	}
}
