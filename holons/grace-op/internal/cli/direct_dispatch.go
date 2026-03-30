package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdkgrpc "github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/grace-op/internal/grpcclient"
)

// isExecutableFile returns true if path points to an existing, non-directory
// file with the executable bit set.
func isExecutableFile(path string) bool {
	// Only consider paths that look like file paths (contain / \ or .).
	if !strings.ContainsAny(path, `/\\.`) {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// cmdDirectBinary launches an executable directly and invokes an RPC via
// stdio — no discovery, no slug resolution.
func cmdDirectBinary(format Format, binaryPath string, args []string) int {
	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op: %v\n", err)
		return 1
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "op: missing method for binary %q\n", binaryPath)
		fmt.Fprintln(os.Stderr, "usage: op <binary-path> <method> [json]")
		return 1
	}

	method := args[0]
	inputJSON := "{}"
	if len(args) > 1 && looksLikeJSON(args[1]) {
		inputJSON = args[1]
	}

	cmd := exec.Command(abs, "serve", "--listen", "stdio://")
	conn, startedCmd, dialErr := sdkgrpc.DialStdioCommand(context.Background(), cmd)
	if dialErr != nil {
		fmt.Fprintf(os.Stderr, "op: %v\n", dialErr)
		return 1
	}
	defer func() {
		_ = conn.Close()
		if startedCmd.Process != nil {
			_ = startedCmd.Process.Kill()
			_ = startedCmd.Wait()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := grpcclient.InvokeConn(ctx, conn, method, inputJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "op: %v\n", err)
		return 1
	}

	fmt.Println(formatRPCOutput(format, method, []byte(result.Output)))
	return 0
}
