package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	internalgrpc "github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/progress"
	"google.golang.org/grpc"
)

const connectDispatchTimeout = 10 * time.Second

func oneShotConnectOptions(transport string) sdkconnect.ConnectOptions {
	return sdkconnect.ConnectOptions{
		Timeout:   connectDispatchTimeout,
		Transport: transport,
		Lifecycle: sdkconnect.LifecycleEphemeral,
		Start:     true,
	}
}

func runConnectedRPC(
	format Format,
	errPrefix string,
	holonName string,
	method string,
	inputJSON string,
	opts sdkconnect.ConnectOptions,
	noBuild bool,
) int {
	conn, err := sdkconnect.ConnectWithOpts(holonName, opts)
	if err != nil {
		if !noBuild && errors.Is(err, sdkconnect.ErrBinaryNotFound) {
			conn, err = autoBuildAndConnect(holonName, opts)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", errPrefix, err)
		return 1
	}
	defer func() { _ = sdkconnect.Disconnect(conn) }()

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	result, err := internalgrpc.InvokeConn(ctx, conn, method, inputJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", errPrefix, err)
		return 1
	}

	fmt.Println(formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}

func parseConnectedRPCArgs(args []string) (method string, inputJSON string, noBuild bool, err error) {
	if len(args) < 1 {
		return "", "", false, fmt.Errorf("method required")
	}

	method = args[0]
	remaining := args[1:]
	inputJSON = "{}"

	if len(remaining) > 0 && remaining[0] == "--no-build" {
		noBuild = true
		remaining = remaining[1:]
	}

	if len(remaining) > 0 {
		inputJSON = remaining[0]
	}

	for _, arg := range remaining[1:] {
		if strings.TrimSpace(arg) == "--no-build" {
			return "", "", false, fmt.Errorf("--no-build must come immediately after the method")
		}
	}

	return method, inputJSON, noBuild, nil
}

func autoBuildAndConnect(holonName string, opts sdkconnect.ConnectOptions) (*grpc.ClientConn, error) {
	target, err := holons.ResolveTarget(holonName)
	if err != nil {
		return nil, err
	}
	if target.ManifestErr != nil {
		return nil, target.ManifestErr
	}
	if target.Manifest == nil {
		return nil, fmt.Errorf("no %s found in %s", identity.ProtoManifestFileName, target.RelativePath)
	}

	pw := progress.New(os.Stderr)
	if !pw.IsTTY() {
		pw = progress.Silence()
	}
	defer pw.Close()

	if _, err := holons.ExecuteLifecycle(holons.OperationBuild, target.Dir, holons.BuildOptions{Progress: pw}); err != nil {
		pw.Keep()
		return nil, err
	}
	pw.Clear()

	return sdkconnect.ConnectWithOpts(holonName, opts)
}

func cmdGRPCConnected(format Format, uri string, holonName string, args []string, transport string) int {
	method, inputJSON, noBuild, err := parseConnectedRPCArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op grpc: %v\n", err)
		fmt.Fprintf(os.Stderr, "usage: op %s <method> [--no-build] [json]\n", uri)
		return 1
	}
	return runConnectedRPC(format, "op grpc", holonName, method, inputJSON, oneShotConnectOptions(transport), noBuild)
}
