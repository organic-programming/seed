// Package grpcclient provides client-side gRPC helpers for inter-holon
// communication. It supports TCP, Unix sockets, and stdio pipe transports.
//
// Dial connects to an existing gRPC server:
//
//	conn, err := grpcclient.Dial(ctx, "localhost:9090")     // TCP
//	conn, err := grpcclient.Dial(ctx, "unix:///tmp/h.sock") // Unix
//
// DialStdio launches a holon binary, communicates via stdin/stdout pipes,
// and returns the gRPC connection:
//
//	conn, cmd, err := grpcclient.DialStdio(ctx, "/path/to/holon")
//	defer cmd.Process.Kill()
//	defer conn.Close()
package grpcclient

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial connects to a gRPC server at the given address.
// For TCP: "host:port". For Unix: "unix:///path".
func Dial(ctx context.Context, address string) (*grpc.ClientConn, error) {
	if isWebSocketAddress(address) {
		return DialWebSocket(ctx, address)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if isUnixAddress(address) {
		path := address
		if len(path) > 7 && path[:7] == "unix://" {
			path = path[7:]
		}
		opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", path, 5*time.Second)
		}))
		address = "passthrough:///unix"
	}

	return grpc.NewClient(address, opts...)
}

// DialStdio launches a holon binary with `serve --listen stdio://`.
func DialStdio(ctx context.Context, binaryPath string) (*grpc.ClientConn, *exec.Cmd, error) {
	cmd := exec.Command(binaryPath, "serve", "--listen", "stdio://")
	return DialStdioCommand(ctx, cmd)
}

// DialStdioCommand launches an explicit command that serves gRPC over stdio
// and returns a gRPC connection backed by the process's stdin/stdout pipes.
//
// The caller must kill the process and close the connection when done.
func DialStdioCommand(ctx context.Context, cmd *exec.Cmd) (*grpc.ClientConn, *exec.Cmd, error) {
	if cmd == nil {
		return nil, nil, fmt.Errorf("stdio command is required")
	}
	if cmd.Path == "" && len(cmd.Args) == 0 {
		return nil, nil, fmt.Errorf("stdio command is empty")
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		label := cmd.Path
		if label == "" && len(cmd.Args) > 0 {
			label = cmd.Args[0]
		}
		return nil, nil, fmt.Errorf("start %s: %w", label, err)
	}

	pConn := &pipeConn{
		reader: stdoutPipe,
		writer: stdinPipe,
	}

	// Single-connection dialer: the pipe can only be used once.
	var dialOnce sync.Once
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		var conn net.Conn
		dialOnce.Do(func() { conn = pConn })
		if conn == nil {
			return nil, fmt.Errorf("stdio pipe already consumed")
		}
		return conn, nil
	}

	// DialContext+WithBlock forces immediate HTTP/2 handshake,
	// which is required for single-connection transports.
	//nolint:staticcheck // DialContext is deprecated but needed for pipes.
	conn, err := grpc.DialContext(ctx,
		"passthrough:///stdio",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
	if err != nil {
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("server startup timeout")
		}
		return nil, nil, fmt.Errorf("grpc handshake over stdio: %w", err)
	}

	return conn, cmd, nil
}

func isUnixAddress(addr string) bool {
	return len(addr) > 7 && addr[:7] == "unix://"
}

func isWebSocketAddress(addr string) bool {
	trimmed := strings.TrimSpace(addr)
	return strings.HasPrefix(trimmed, "ws://") || strings.HasPrefix(trimmed, "wss://")
}

// --- pipeConn: wraps stdin/stdout pipes as a net.Conn ---

type pipeConn struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (c *pipeConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *pipeConn) Write(p []byte) (int, error) { return c.writer.Write(p) }
func (c *pipeConn) Close() error {
	writeErr := c.writer.Close()
	readErr := c.reader.Close()
	if writeErr != nil {
		return writeErr
	}
	return readErr
}
func (c *pipeConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (c *pipeConn) SetDeadline(_ time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(_ time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "stdio://" }
