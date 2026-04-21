package transport

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// newWSListener starts an HTTP server that upgrades WebSocket connections
// and presents them as net.Conn to the gRPC server via a net.Listener.
//
// URI format: ws://<host>:<port>[/path] or wss://<host>:<port>[/path]
// The path defaults to "/grpc" if omitted.
func newWSListener(uri string) (net.Listener, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("parse websocket URI %q: %w", uri, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	isTLS := scheme == "wss"
	if scheme != "ws" && scheme != "wss" {
		return nil, fmt.Errorf("unsupported websocket scheme %q", parsed.Scheme)
	}

	addr := parsed.Host
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("websocket URI %q is missing host:port", uri)
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, fmt.Errorf("invalid websocket host:port %q: %w", addr, err)
	}

	path := parsed.Path
	if path == "" {
		path = "/grpc"
	}

	certFile := parsed.Query().Get("cert")
	keyFile := parsed.Query().Get("key")
	if isTLS {
		if certFile == "" {
			certFile = os.Getenv("HOLONS_WSS_CERT_FILE")
		}
		if keyFile == "" {
			keyFile = os.Getenv("HOLONS_WSS_KEY_FILE")
		}
		if certFile == "" || keyFile == "" {
			return nil, fmt.Errorf("wss:// requires cert and key (query params cert/key or HOLONS_WSS_CERT_FILE/HOLONS_WSS_KEY_FILE)")
		}
	}

	// Bind the TCP listener first so we fail fast on port conflicts
	tcpLis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ws listen %s: %w", addr, err)
	}

	// Use the actual bound address (resolves :0 → actual port)
	boundAddr := tcpLis.Addr().String()

	wsl := &wsListener{
		addr:   boundAddr,
		path:   path,
		isTLS:  isTLS,
		connCh: make(chan net.Conn, 16),
		done:   make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, wsl.handleUpgrade)

	wsl.server = &http.Server{Handler: mux}

	go func() {
		var err error
		if isTLS {
			err = wsl.server.ServeTLS(tcpLis, certFile, keyFile)
		} else {
			err = wsl.server.Serve(tcpLis)
		}
		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("ws server error: %v\n", err)
		}
	}()

	return wsl, nil
}

// wsListener adapts WebSocket connections into a net.Listener.
type wsListener struct {
	addr   string
	path   string
	isTLS  bool
	server *http.Server
	connCh chan net.Conn
	done   chan struct{}
	once   sync.Once
}

func (l *wsListener) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow any origin — holons are not browsers, CORS is irrelevant.
		// For browser clients, the holon operator configures a reverse proxy.
		InsecureSkipVerify: true,
		Subprotocols:       []string{"grpc"},
	})
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}

	// Wrap the WebSocket as a net.Conn and send it to Accept().
	conn := websocket.NetConn(context.Background(), c, websocket.MessageBinary)

	select {
	case l.connCh <- conn:
	case <-l.done:
		conn.Close()
	}
}

func (l *wsListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case <-l.done:
		return nil, io.EOF
	}
}

func (l *wsListener) Close() error {
	l.once.Do(func() {
		close(l.done)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		l.server.Shutdown(ctx) //nolint:errcheck
	})
	return nil
}

func (l *wsListener) Addr() net.Addr {
	scheme := "ws"
	if l.isTLS {
		scheme = "wss"
	}
	return wsAddr{scheme: scheme, addr: l.addr, path: l.path}
}

type wsAddr struct {
	scheme string
	addr   string
	path   string
}

func (a wsAddr) Network() string { return a.scheme }
func (a wsAddr) String() string  { return a.scheme + "://" + a.addr + a.path }
