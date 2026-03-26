package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
)

// WebBridge provides an embeddable bidirectional Holon-RPC gateway
// (JSON-RPC 2.0 over WebSocket) that mounts on an existing [http.ServeMux].
//
// Use WebBridge when the holon serves both static files and Holon-RPC
// from the same HTTP server (e.g. browser-facing applications).
// For a standalone Holon-RPC server that owns its own listener, use
// [holonrpc.Server] from pkg/holonrpc instead.
//
// Both sides can act as client and server simultaneously:
//   - Browser calls Go:  browser sends a request, Go handler responds.
//   - Go calls browser:  Go sends a request via [WebConn.Invoke], browser handler responds.
//
// Wire protocol (symmetric — either direction):
//
//	Request:  { "jsonrpc":"2.0", "id":"1", "method":"pkg.Service/Method", "params":{...} }
//	Response: { "jsonrpc":"2.0", "id":"1", "result":{...} }
//	Error:    { "jsonrpc":"2.0", "id":"1", "error":{"code":-32601, "message":"..."} }
//
// A message is a request if it has "method". A message is a response if
// it has "result" or "error". The "id" field correlates responses to requests.
// Server-initiated IDs are prefixed with "s" to avoid collision.
type WebBridge struct {
	mu        sync.RWMutex
	handlers  map[string]WebHandler // Go-side handlers for browser→Go calls
	origins   map[string]bool
	onConnect func(*WebConn) // called when a browser connects
}

// WebHandler processes a single RPC call. It receives the JSON params
// and returns the JSON result or an error.
type WebHandler func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)

// WebError is returned by handlers to communicate a specific error code.
type WebError struct {
	Code    int
	Message string
}

// Error returns the string form of a WebError.
func (e *WebError) Error() string {
	return fmt.Sprintf("code %d: %s", e.Code, e.Message)
}

// WebConn represents an active browser WebSocket connection.
//
// Thread safety: Invoke is safe for concurrent callers. Writes are serialized
// with an internal mutex and pending responses are correlated by request ID.
//
// Note: wsMsg uses string IDs for browser compatibility, while holonrpc's
// rpcMessage uses json.RawMessage IDs to preserve raw JSON typing.
type WebConn struct {
	ws      *websocket.Conn
	mu      sync.Mutex
	nextID  int64
	pending sync.Map // id → chan wsMsg
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewWebBridge creates an empty bridge. Register handlers before mounting.
func NewWebBridge() *WebBridge {
	return &WebBridge{
		handlers: make(map[string]WebHandler),
	}
}

// Register adds a Go-side handler for browser→Go calls. The method string
// must follow the "package.Service/Method" convention.
func (b *WebBridge) Register(method string, handler WebHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[method] = handler
}

// OnConnect sets a callback invoked each time a browser client connects.
// The callback receives a WebConn that can be used to invoke methods on
// the browser. The callback runs in its own goroutine.
func (b *WebBridge) OnConnect(fn func(*WebConn)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onConnect = fn
}

// AllowOrigins restricts CORS to the given origins.
// Pass nothing to allow all origins (development mode).
func (b *WebBridge) AllowOrigins(origins ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(origins) == 0 {
		b.origins = nil
		return
	}
	b.origins = make(map[string]bool, len(origins))
	for _, o := range origins {
		b.origins[o] = true
	}
}

// Methods returns all registered method names.
func (b *WebBridge) Methods() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	methods := make([]string, 0, len(b.handlers))
	for m := range b.handlers {
		methods = append(methods, m)
	}
	return methods
}

// ServeHTTP implements http.Handler, delegating to HandleWebSocket.
func (b *WebBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.HandleWebSocket(w, r)
}

// HandleWebSocket upgrades the HTTP connection to WebSocket and runs the
// bidirectional message loop until the client disconnects.
func (b *WebBridge) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{
		Subprotocols: []string{"holon-rpc"},
	}

	b.mu.RLock()
	if b.origins == nil {
		opts.InsecureSkipVerify = true
	} else {
		origin := r.Header.Get("Origin")
		if !b.origins[origin] {
			b.mu.RUnlock()
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		opts.OriginPatterns = make([]string, 0, len(b.origins))
		for o := range b.origins {
			opts.OriginPatterns = append(opts.OriginPatterns, o)
		}
	}
	b.mu.RUnlock()

	c, err := websocket.Accept(w, r, opts)
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	conn := &WebConn{
		ws:     c,
		ctx:    ctx,
		cancel: cancel,
	}

	// Notify the bridge owner about this new connection
	b.mu.RLock()
	onConnect := b.onConnect
	b.mu.RUnlock()
	if onConnect != nil {
		go onConnect(conn)
	}

	// Message loop: read messages, dispatch based on type
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return // client disconnected
		}

		var msg wsMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			code := -32700
			message := "parse error"
			if json.Valid(data) {
				code = -32600
				message = "invalid request"
			}
			resp := marshalWsResp(wsMsg{
				JSONRPC: "2.0",
				Error:   &wsErr{Code: code, Message: message},
			})
			c.Write(ctx, websocket.MessageText, resp)
			continue
		}

		if msg.Method != "" {
			if msg.JSONRPC != "2.0" {
				if strings.TrimSpace(msg.ID) == "" {
					continue
				}
				resp := marshalWsResp(wsMsg{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   &wsErr{Code: -32600, Message: "invalid request"},
				})
				conn.mu.Lock()
				_ = c.Write(ctx, websocket.MessageText, resp)
				conn.mu.Unlock()
				continue
			}
			// Incoming request from browser → dispatch to Go handler
			go b.handleRequest(ctx, conn, msg)
		} else if len(msg.Result) > 0 || msg.Error != nil {
			// Incoming response from browser → route to pending Invoke()
			conn.handleResponse(msg)
		} else if strings.TrimSpace(msg.ID) != "" {
			resp := marshalWsResp(wsMsg{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error:   &wsErr{Code: -32600, Message: "invalid request"},
			})
			conn.mu.Lock()
			_ = c.Write(ctx, websocket.MessageText, resp)
			conn.mu.Unlock()
		}
	}
}

// Invoke calls a method registered on the browser side and waits for the
// response. This is the Go→Browser direction of the bidirectional RPC.
func (c *WebConn) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	id := fmt.Sprintf("s%d", atomic.AddInt64(&c.nextID, 1))

	if payload == nil {
		payload = json.RawMessage("{}")
	}

	ch := make(chan wsMsg, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	// Send request to browser
	reqData, err := json.Marshal(wsMsg{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  payload,
	})
	if err != nil {
		return nil, fmt.Errorf("wsweb: marshal request: %w", err)
	}

	c.mu.Lock()
	err = c.ws.Write(ctx, websocket.MessageText, reqData)
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("wsweb: write: %w", err)
	}

	// Wait for response
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, &WebError{Code: resp.Error.Code, Message: resp.Error.Message}
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, fmt.Errorf("wsweb: connection closed")
	}
}

// InvokeWithTimeout is a convenience wrapper around Invoke with a deadline.
func (c *WebConn) InvokeWithTimeout(method string, payload json.RawMessage, timeout time.Duration) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.Invoke(ctx, method, payload)
}

// --- wire types ---

// wsMsg is the unified wire envelope. If Method is set, it's a request.
// If Result or Error is set, it's a response.
type wsMsg struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *wsErr          `json:"error,omitempty"`
}

type wsErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// handleRequest dispatches an incoming browser request to a Go handler.
func (b *WebBridge) handleRequest(ctx context.Context, conn *WebConn, msg wsMsg) {
	isNotification := strings.TrimSpace(msg.ID) == ""
	method := strings.TrimSpace(msg.Method)

	b.mu.RLock()
	handler, ok := b.handlers[method]
	b.mu.RUnlock()

	resp := wsMsg{
		JSONRPC: "2.0",
		ID:      msg.ID,
	}

	if msg.JSONRPC != "2.0" {
		if isNotification {
			return
		}
		resp.Error = &wsErr{Code: -32600, Message: "invalid request"}
		data := marshalWsResp(resp)
		conn.mu.Lock()
		_ = conn.ws.Write(ctx, websocket.MessageText, data)
		conn.mu.Unlock()
		return
	}

	if method == "" {
		if isNotification {
			return
		}
		resp.Error = &wsErr{Code: -32600, Message: "invalid request"}
		data := marshalWsResp(resp)
		conn.mu.Lock()
		_ = conn.ws.Write(ctx, websocket.MessageText, data)
		conn.mu.Unlock()
		return
	}

	if method == "rpc.heartbeat" {
		if isNotification {
			return
		}
		resp.Result = json.RawMessage("{}")
		data := marshalWsResp(resp)
		conn.mu.Lock()
		_ = conn.ws.Write(ctx, websocket.MessageText, data)
		conn.mu.Unlock()
		return
	}

	if !ok {
		if isNotification {
			return
		}
		resp.Error = &wsErr{Code: -32601, Message: fmt.Sprintf("method %q not registered", method)}
	} else {
		params := msg.Params
		if params == nil {
			params = json.RawMessage("{}")
		}
		result, err := handler(ctx, params)
		if err != nil {
			if isNotification {
				return
			}
			code := 13
			errMsg := err.Error()
			if we, ok := err.(*WebError); ok {
				code = we.Code
				errMsg = we.Message
			} else {
				code = -32603
				errMsg = "internal error"
			}
			resp.Error = &wsErr{Code: code, Message: errMsg}
		} else {
			if isNotification {
				return
			}
			resp.Result = result
		}
	}

	data := marshalWsResp(resp)
	conn.mu.Lock()
	_ = conn.ws.Write(ctx, websocket.MessageText, data)
	conn.mu.Unlock()
}

// handleResponse routes an incoming browser response to the pending Invoke().
func (c *WebConn) handleResponse(msg wsMsg) {
	if msg.JSONRPC != "2.0" {
		msg.Error = &wsErr{Code: -32600, Message: "invalid response"}
		msg.Result = nil
	}

	val, ok := c.pending.Load(msg.ID)
	if !ok {
		return // stale or unknown id
	}
	ch := val.(chan wsMsg)
	select {
	case ch <- msg:
	default:
	}
}

func marshalWsResp(r wsMsg) []byte {
	data, err := json.Marshal(r)
	if err != nil {
		log.Printf("wsweb: marshal response id=%q: %v", r.ID, err)
		return []byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"internal error"}}`)
	}
	return data
}
