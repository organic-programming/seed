package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestTransport_TCP_RoundTrip(t *testing.T) {
	lis, err := Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer lis.Close()

	serverConnCh := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := lis.Accept()
		if acceptErr == nil {
			serverConnCh <- conn
		}
	}()

	clientConn, err := net.DialTimeout("tcp", lis.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer clientConn.Close()

	serverConn := <-serverConnCh
	defer serverConn.Close()

	if _, err := clientConn.Write([]byte("hello-tcp")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	buf := make([]byte, 32)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	if got := string(buf[:n]); got != "hello-tcp" {
		t.Fatalf("tcp payload = %q, want %q", got, "hello-tcp")
	}
}

func TestTransport_Unix_RoundTrip(t *testing.T) {
	socketPath := shortUnixSocketPath(t, "transport-roundtrip")
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	lis, err := Listen("unix://" + socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer lis.Close()

	serverConnCh := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := lis.Accept()
		if acceptErr == nil {
			serverConnCh <- conn
		}
	}()

	clientConn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatalf("dial unix: %v", err)
	}
	defer clientConn.Close()

	serverConn := <-serverConnCh
	defer serverConn.Close()

	if _, err := clientConn.Write([]byte("hello-unix")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	buf := make([]byte, 32)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	if got := string(buf[:n]); got != "hello-unix" {
		t.Fatalf("unix payload = %q, want %q", got, "hello-unix")
	}
}

func TestTransport_Stdio_RoundTrip(t *testing.T) {
	clientToServerR, clientToServerW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create clientToServer pipe: %v", err)
	}
	defer clientToServerR.Close()
	defer clientToServerW.Close()

	serverToClientR, serverToClientW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create serverToClient pipe: %v", err)
	}
	defer serverToClientR.Close()
	defer serverToClientW.Close()

	lis := newStdioListenerWithIO(clientToServerR, serverToClientW)
	defer lis.Close()

	serverConn, err := lis.Accept()
	if err != nil {
		t.Fatalf("accept stdio: %v", err)
	}
	defer serverConn.Close()

	writeErrCh := make(chan error, 1)
	go func() {
		_, writeErr := clientToServerW.Write([]byte("hello-stdio"))
		writeErrCh <- writeErr
	}()

	buf := make([]byte, 64)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("server read stdio: %v", err)
	}
	if got := string(buf[:n]); got != "hello-stdio" {
		t.Fatalf("stdio payload = %q, want %q", got, "hello-stdio")
	}
	if writeErr := <-writeErrCh; writeErr != nil {
		t.Fatalf("client write stdio: %v", writeErr)
	}

	serverWriteErrCh := make(chan error, 1)
	go func() {
		_, writeErr := serverConn.Write([]byte("ack-stdio"))
		serverWriteErrCh <- writeErr
	}()

	n, err = serverToClientR.Read(buf)
	if err != nil {
		t.Fatalf("client read stdio: %v", err)
	}
	if got := string(buf[:n]); got != "ack-stdio" {
		t.Fatalf("stdio ack = %q, want %q", got, "ack-stdio")
	}
	if writeErr := <-serverWriteErrCh; writeErr != nil {
		t.Fatalf("server write stdio: %v", writeErr)
	}
}

func TestTransport_WSS_RoundTrip(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)
	listenURI := fmt.Sprintf(
		"wss://127.0.0.1:0/grpc?cert=%s&key=%s",
		url.QueryEscape(certFile),
		url.QueryEscape(keyFile),
	)

	lis, err := Listen(listenURI)
	if err != nil {
		t.Fatalf("listen wss: %v", err)
	}
	defer lis.Close()

	serverConnCh := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := lis.Accept()
		if acceptErr == nil {
			serverConnCh <- conn
		}
	}()

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, lis.Addr().String(), &websocket.DialOptions{
		Subprotocols: []string{"grpc"},
		HTTPClient:   httpClient,
	})
	if err != nil {
		t.Fatalf("dial wss: %v", err)
	}
	defer ws.CloseNow()

	clientConn := websocket.NetConn(ctx, ws, websocket.MessageBinary)
	defer clientConn.Close()

	serverConn := <-serverConnCh
	defer serverConn.Close()

	if _, err := clientConn.Write([]byte("hello-wss")); err != nil {
		t.Fatalf("wss client write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("wss server read: %v", err)
	}
	if got := string(buf[:n]); got != "hello-wss" {
		t.Fatalf("wss payload = %q, want %q", got, "hello-wss")
	}
}

func TestTransport_URIParsingEdgeCases(t *testing.T) {
	testCases := []struct {
		name          string
		uri           string
		wantErr       bool
		wantAddrSufix string
	}{
		{
			name:    "tcp-missing-port",
			uri:     "tcp://127.0.0.1",
			wantErr: true,
		},
		{
			name:    "unix-empty-path",
			uri:     "unix://",
			wantErr: true,
		},
		{
			name:    "ws-missing-port",
			uri:     "ws://127.0.0.1",
			wantErr: true,
		},
		{
			name:          "ws-empty-host-valid",
			uri:           "ws://:0",
			wantAddrSufix: "/grpc",
		},
		{
			name:          "ws-trailing-slash",
			uri:           "ws://127.0.0.1:0/",
			wantAddrSufix: "/",
		},
		{
			name:          "ws-query-ignored-in-listener-path",
			uri:           "ws://127.0.0.1:0/grpc?token=abc",
			wantAddrSufix: "/grpc",
		},
		{
			name:    "wss-missing-key",
			uri:     "wss://127.0.0.1:0/grpc?cert=/tmp/cert.pem",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			lis, err := Listen(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.uri)
				}
				return
			}
			if err != nil {
				t.Fatalf("listen %q: %v", tc.uri, err)
			}
			defer lis.Close()

			addr := lis.Addr().String()
			if tc.wantAddrSufix != "" && !strings.HasSuffix(addr, tc.wantAddrSufix) {
				t.Fatalf("listener addr %q does not end with %q", addr, tc.wantAddrSufix)
			}
			if strings.Contains(addr, "?") {
				t.Fatalf("listener addr should not include query: %q", addr)
			}
		})
	}
}

func TestTransport_UnixSocketCleanupOnClose(t *testing.T) {
	socketPath := shortUnixSocketPath(t, "cleanup")
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	lis, err := Listen("unix://" + socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	if _, err := os.Stat(socketPath); err != nil {
		t.Fatalf("expected unix socket to exist: %v", err)
	}

	if err := lis.Close(); err != nil {
		t.Fatalf("close unix listener: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(socketPath)
		if os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("unix socket %q still exists after listener close", socketPath)
}

func TestTransport_ConcurrentDialSameAddress(t *testing.T) {
	lis, err := Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer lis.Close()

	const clients = 10
	var acceptWG sync.WaitGroup
	acceptWG.Add(clients)

	go func() {
		for i := 0; i < clients; i++ {
			conn, acceptErr := lis.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer acceptWG.Done()
				defer c.Close()

				buf := make([]byte, 32)
				n, readErr := c.Read(buf)
				if readErr != nil {
					return
				}
				_, _ = c.Write(buf[:n])
			}(conn)
		}
	}()

	var clientWG sync.WaitGroup
	clientWG.Add(clients)
	errCh := make(chan error, clients)

	for i := 0; i < clients; i++ {
		i := i
		go func() {
			defer clientWG.Done()

			conn, dialErr := net.DialTimeout("tcp", lis.Addr().String(), time.Second)
			if dialErr != nil {
				errCh <- dialErr
				return
			}
			defer conn.Close()

			msg := fmt.Sprintf("c-%d", i)
			if _, writeErr := conn.Write([]byte(msg)); writeErr != nil {
				errCh <- writeErr
				return
			}
			buf := make([]byte, 16)
			n, readErr := conn.Read(buf)
			if readErr != nil {
				errCh <- readErr
				return
			}
			if got := string(buf[:n]); got != msg {
				errCh <- fmt.Errorf("echo mismatch: got %q want %q", got, msg)
				return
			}
		}()
	}

	clientWG.Wait()
	acceptWG.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestTransport_ListenerCloseWithActiveConnections(t *testing.T) {
	lis, err := Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}

	serverConnCh := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := lis.Accept()
		if acceptErr == nil {
			serverConnCh <- conn
		}
	}()

	clientConn, err := net.DialTimeout("tcp", lis.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer clientConn.Close()

	serverConn := <-serverConnCh
	defer serverConn.Close()

	if err := lis.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	if _, err := clientConn.Write([]byte("still-open")); err != nil {
		t.Fatalf("write on active conn after listener close: %v", err)
	}
	buf := make([]byte, 32)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("read on active conn after listener close: %v", err)
	}
	if got := string(buf[:n]); got != "still-open" {
		t.Fatalf("active conn payload = %q, want %q", got, "still-open")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	dialer := net.Dialer{}
	if _, err := dialer.DialContext(ctx, "tcp", lis.Addr().String()); err == nil {
		t.Fatal("expected new dial to fail after listener close")
	}
}

func writeSelfSignedCert(t *testing.T) (string, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath := filepath.Join(t.TempDir(), "cert.pem")
	keyPath := filepath.Join(t.TempDir(), "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}

	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	return certPath, keyPath
}

func shortUnixSocketPath(t *testing.T, prefix string) string {
	t.Helper()
	name := fmt.Sprintf("%s-%d.sock", prefix, time.Now().UnixNano())
	return filepath.Join(os.TempDir(), name)
}
