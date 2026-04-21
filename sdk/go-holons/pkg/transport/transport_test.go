package transport_test

import (
	"context"
	"net"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/organic-programming/go-holons/pkg/transport"
)

func TestWSListenViaURI(t *testing.T) {
	lis, err := transport.Listen("ws://127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	if lis.Addr().Network() != "ws" {
		t.Errorf("network = %q", lis.Addr().Network())
	}
}

func TestWSListenerRoundTrip(t *testing.T) {
	lis, err := transport.Listen("ws://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ws://127.0.0.1:0: %v", err)
	}
	defer lis.Close()

	// Accept in background
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	// Give the HTTP server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Dial via WebSocket
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wsURL := lis.Addr().String()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"grpc"},
	})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}

	clientConn := websocket.NetConn(ctx, c, websocket.MessageBinary)
	defer clientConn.Close()

	serverConn := <-accepted
	defer serverConn.Close()

	// Write from client, read from server
	msg := []byte("hello ws")
	if _, err := clientConn.Write(msg); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 64)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "hello ws" {
		t.Errorf("got %q", buf[:n])
	}
}
