package grpcclient

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"nhooyr.io/websocket"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WebSocketDialOptions controls the underlying websocket dial behavior.
type WebSocketDialOptions struct {
	HTTPClient *http.Client
}

// DialWebSocket connects to a gRPC server via WebSocket.
// uri is "ws://host:port/path" or "wss://host:port/path".
// The path defaults to "/grpc" if omitted.
//
// It intentionally negotiates the WebSocket subprotocol "grpc", not
// "holon-rpc": this transport carries raw gRPC HTTP/2 bytes inside binary
// WebSocket frames, while holon-rpc is JSON-RPC 2.0 over text frames.
func DialWebSocket(ctx context.Context, uri string) (*grpc.ClientConn, error) {
	return DialWebSocketWithOptions(ctx, uri, WebSocketDialOptions{})
}

// DialWebSocketWithOptions connects to a gRPC server via WebSocket with
// explicit transport options.
func DialWebSocketWithOptions(ctx context.Context, uri string, options WebSocketDialOptions) (*grpc.ClientConn, error) {
	// Each gRPC transport dial gets its own WebSocket connection.
	// This keeps reconnect paths functional instead of failing on
	// "connection already consumed" after the first attempt.
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		dialOptions := &websocket.DialOptions{
			Subprotocols: []string{"grpc"},
		}
		if options.HTTPClient != nil {
			dialOptions.HTTPClient = options.HTTPClient
		}
		c, _, err := websocket.Dial(ctx, uri, dialOptions)
		if err != nil {
			return nil, fmt.Errorf("websocket dial %s: %w", uri, err)
		}
		return websocket.NetConn(context.Background(), c, websocket.MessageBinary), nil
	}

	//nolint:staticcheck // DialContext is deprecated but needed for single-connection transports.
	conn, err := grpc.DialContext(ctx,
		"passthrough:///ws",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc handshake over ws: %w", err)
	}

	return conn, nil
}
