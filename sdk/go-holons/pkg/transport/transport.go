// Package transport provides URI-based listener and dialer factories for
// gRPC servers and clients. Every Go holon imports this package to implement
// the standard `serve --listen <URI>` convention (Constitution Article 11).
//
// Supported transports:
//   - tcp://<host>:<port>  — TCP socket (default: tcp://:9090)
//   - unix://<path>        — Unix domain socket
//   - stdio://             — stdin/stdout pipe (single connection)
//   - ws://<host>:<port>   — WebSocket (browser, NAT traversal)
//   - wss://<host>:<port>  — WebSocket over TLS
package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// DefaultURI is the transport used when --listen is omitted.
const DefaultURI = "tcp://:9090"

// Listen parses a transport URI and returns a net.Listener.
// This is the server-side function — call it in your ListenAndServe.
func Listen(uri string) (net.Listener, error) {
	switch {
	case strings.HasPrefix(uri, "tcp://"):
		addr := strings.TrimPrefix(uri, "tcp://")
		return net.Listen("tcp", addr)

	case strings.HasPrefix(uri, "unix://"):
		path := strings.TrimPrefix(uri, "unix://")
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("unix transport URI %q is missing socket path", uri)
		}
		// Clean up stale socket files
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale unix socket %q: %w", path, err)
		}
		lis, err := net.Listen("unix", path)
		if err != nil {
			return nil, err
		}
		return &unixListener{
			Listener: lis,
			path:     path,
		}, nil

	case uri == "stdio://" || uri == "stdio":
		return newStdioListener(), nil

	case strings.HasPrefix(uri, "ws://") || strings.HasPrefix(uri, "wss://"):
		return newWSListener(uri)

	default:
		return nil, fmt.Errorf("unsupported transport URI: %q (expected tcp://, unix://, stdio://, or ws://)", uri)
	}
}

// Scheme extracts the transport scheme name from a URI for logging.
func Scheme(uri string) string {
	if i := strings.Index(uri, "://"); i >= 0 {
		return uri[:i]
	}
	return uri
}

// --- stdio transport ---
// Wraps stdin/stdout as a single-connection net.Listener.
// This is how LSP works — parent pipes directly to child process.

type stdioListener struct {
	once   sync.Once
	connCh chan net.Conn
	done   chan struct{}
}

func newStdioListener() *stdioListener {
	return newStdioListenerWithIO(os.Stdin, os.Stdout)
}

func newStdioListenerWithIO(reader io.Reader, writer io.Writer) *stdioListener {
	l := &stdioListener{
		connCh: make(chan net.Conn, 1),
		done:   make(chan struct{}),
	}
	// Deliver exactly one connection wrapping stdin/stdout.
	l.connCh <- &stdioConn{
		reader:        reader,
		writer:        writer,
		closeListener: l.shutdown,
	}
	return l
}

func (l *stdioListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.connCh:
		if !ok {
			return nil, io.EOF
		}
		return conn, nil
	case <-l.done:
		// Single-connection transport: after the connection closes or
		// the listener is closed, Accept returns an error so Serve stops.
		return nil, io.EOF
	}
}

func (l *stdioListener) Close() error {
	l.shutdown()
	return nil
}

func (l *stdioListener) shutdown() {
	l.once.Do(func() {
		close(l.done)
		close(l.connCh)
	})
}

func (l *stdioListener) Addr() net.Addr {
	return stdioAddr{}
}

// stdioConn wraps stdin/stdout as a net.Conn.
type stdioConn struct {
	reader        io.Reader
	writer        io.Writer
	closeListener func()
}

func (c *stdioConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *stdioConn) Write(p []byte) (int, error) { return c.writer.Write(p) }

func (c *stdioConn) Close() error {
	if c.closeListener != nil {
		c.closeListener()
	}
	return nil
}

func (c *stdioConn) LocalAddr() net.Addr                { return stdioAddr{} }
func (c *stdioConn) RemoteAddr() net.Addr               { return stdioAddr{} }
func (c *stdioConn) SetDeadline(_ time.Time) error      { return nil }
func (c *stdioConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *stdioConn) SetWriteDeadline(_ time.Time) error { return nil }

type stdioAddr struct{}

func (stdioAddr) Network() string { return "stdio" }
func (stdioAddr) String() string  { return "stdio://" }

type unixListener struct {
	net.Listener
	path string
	once sync.Once
}

func (l *unixListener) Close() error {
	var closeErr error
	l.once.Do(func() {
		closeErr = l.Listener.Close()
		if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if closeErr == nil {
				closeErr = err
			}
		}
	})
	return closeErr
}
