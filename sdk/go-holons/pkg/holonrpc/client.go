package holonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
)

// Client is a bidirectional Holon-RPC client.
//
// It can:
//   - invoke methods on a remote Holon-RPC server (client -> server)
//   - receive and handle server-initiated requests (server -> client)
type Client struct {
	stateMu sync.RWMutex
	ws      *websocket.Conn
	rxCtx   context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	closed  bool

	reconnectURL        string
	reconnectCtx        context.Context
	reconnectCancel     context.CancelFunc
	reconnectParentStop func() bool
	reconnectWake       chan struct{}
	reconnectDone       chan struct{}

	sendMu sync.Mutex

	handlersMu sync.RWMutex
	handlers   map[string]Handler

	pendingMu sync.Mutex
	pending   map[string]chan rpcMessage

	nextClientID int64
}

const (
	reconnectMinDelay = 500 * time.Millisecond
	reconnectMaxDelay = 30 * time.Second
	reconnectFactor   = 2.0
	reconnectJitter   = 0.1
)

// NewClient creates an empty Holon-RPC client.
func NewClient() *Client {
	return &Client{
		handlers: make(map[string]Handler),
		pending:  make(map[string]chan rpcMessage),
	}
}

// Register registers a handler for server-initiated requests.
func (c *Client) Register(method string, handler Handler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[method] = handler
}

// Unregister removes a previously registered handler.
func (c *Client) Unregister(method string) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	delete(c.handlers, method)
}

// Connect dials a Holon-RPC endpoint and starts the receive loop.
func (c *Client) Connect(ctx context.Context, url string) error {
	if strings.TrimSpace(url) == "" {
		return errors.New("holon-rpc: url is required")
	}

	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		return errors.New("holon-rpc: client is closed")
	}
	if c.ws != nil {
		c.stateMu.Unlock()
		return errors.New("holon-rpc: client already connected")
	}
	c.stateMu.Unlock()

	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		Subprotocols: []string{"holon-rpc"},
	})
	if err != nil {
		return fmt.Errorf("holon-rpc: dial failed: %w", err)
	}

	if ws.Subprotocol() != "holon-rpc" {
		_ = ws.Close(websocket.StatusProtocolError, "missing holon-rpc subprotocol")
		return errors.New("holon-rpc: server did not negotiate holon-rpc")
	}
	ws.SetReadLimit(1 << 20)

	rxCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		cancel()
		_ = ws.Close(websocket.StatusNormalClosure, "client closed")
		return errors.New("holon-rpc: client is closed")
	}
	c.ws = ws
	c.rxCtx = rxCtx
	c.cancel = cancel
	c.done = done
	c.stateMu.Unlock()

	go c.readLoop(ws, done)
	return nil
}

// ConnectWithReconnect dials a Holon-RPC endpoint and enables automatic
// reconnect with exponential backoff when the connection is dropped.
//
// The provided ctx controls reconnect lifetime after the initial dial.
func (c *Client) ConnectWithReconnect(ctx context.Context, url string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(url) == "" {
		return errors.New("holon-rpc: url is required")
	}

	reconnectCtx, reconnectCancel := context.WithCancel(context.Background())
	reconnectParentStop := context.AfterFunc(ctx, reconnectCancel)
	reconnectWake := make(chan struct{}, 1)
	reconnectDone := make(chan struct{})

	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		reconnectParentStop()
		reconnectCancel()
		return errors.New("holon-rpc: client is closed")
	}
	if c.ws != nil || c.reconnectCtx != nil {
		c.stateMu.Unlock()
		reconnectParentStop()
		reconnectCancel()
		return errors.New("holon-rpc: client already connected")
	}
	c.reconnectURL = url
	c.reconnectCtx = reconnectCtx
	c.reconnectCancel = reconnectCancel
	c.reconnectParentStop = reconnectParentStop
	c.reconnectWake = reconnectWake
	c.reconnectDone = reconnectDone
	c.stateMu.Unlock()

	go c.reconnectLoop(reconnectCtx, reconnectWake, reconnectDone)

	if err := c.Connect(ctx, url); err != nil {
		c.disableReconnect(reconnectDone)
		return err
	}
	return nil
}

// Connected reports whether the client currently has an active WebSocket.
func (c *Client) Connected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return !c.closed && c.ws != nil
}

// Close gracefully closes the client connection and stops runtime goroutines.
func (c *Client) Close() error {
	c.stateMu.Lock()
	if c.closed {
		c.stateMu.Unlock()
		return nil
	}
	c.closed = true
	ws := c.ws
	cancel := c.cancel
	done := c.done
	reconnectCancel := c.reconnectCancel
	reconnectParentStop := c.reconnectParentStop
	reconnectDone := c.reconnectDone
	c.ws = nil
	c.cancel = nil
	c.done = nil
	c.reconnectURL = ""
	c.reconnectCtx = nil
	c.reconnectCancel = nil
	c.reconnectParentStop = nil
	c.reconnectWake = nil
	c.reconnectDone = nil
	c.stateMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if reconnectParentStop != nil {
		reconnectParentStop()
	}
	if reconnectCancel != nil {
		reconnectCancel()
	}

	if ws != nil {
		_ = ws.Close(websocket.StatusNormalClosure, "client close")
	}
	if done != nil {
		<-done
	}
	if reconnectDone != nil {
		<-reconnectDone
	}

	c.failAllPending(errConnectionClosed)
	return nil
}

// Invoke sends a JSON-RPC request and waits for the corresponding response.
func (c *Client) Invoke(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(method) == "" {
		return nil, errors.New("holon-rpc: method is required")
	}

	ws, rxCtx, err := c.currentConn()
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("c%d", atomic.AddInt64(&c.nextClientID, 1))
	idRaw := makeID(id)
	key, _ := idKey(idRaw)

	ch := make(chan rpcMessage, 1)
	c.pendingMu.Lock()
	c.pending[key] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, key)
		c.pendingMu.Unlock()
	}()

	paramsRaw, err := marshalObject(params)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: marshal params: %w", err)
	}

	msg, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      idRaw,
		Method:  method,
		Params:  paramsRaw,
	})
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: marshal request: %w", err)
	}

	if err := c.write(ws, ctx, msg); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		out, err := decodeResult(resp.Result)
		if err != nil {
			return nil, fmt.Errorf("holon-rpc: invalid result: %w", err)
		}
		return out, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-rxCtx.Done():
		return nil, errConnectionClosed
	}
}

func (c *Client) currentConn() (*websocket.Conn, context.Context, error) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()

	if c.closed {
		return nil, nil, errors.New("holon-rpc: client is closed")
	}
	if c.ws == nil || c.cancel == nil || c.rxCtx == nil {
		if c.reconnectCtx != nil {
			return nil, nil, &ResponseError{
				Code:    codeUnavailable,
				Message: errConnectionClosed.Error(),
			}
		}
		return nil, nil, errors.New("holon-rpc: client is not connected")
	}
	return c.ws, c.rxCtx, nil
}

func (c *Client) readLoop(ws *websocket.Conn, done chan struct{}) {
	defer close(done)
	defer c.onDisconnect(ws)

	for {
		kind, data, err := ws.Read(c.rxCtx)
		if err != nil {
			return
		}
		if kind != websocket.MessageText {
			continue
		}

		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			code := classifyDecodeError(data)
			message := "invalid request"
			if code == codeParseError {
				message = "parse error"
			}
			_ = c.sendError(ws, c.rxCtx, json.RawMessage("null"), code, message, nil)
			continue
		}

		if msg.Method != "" {
			go c.handleRequest(ws, c.rxCtx, msg)
			continue
		}

		if len(msg.Result) > 0 || msg.Error != nil {
			c.handleResponse(msg)
			continue
		}

		if hasID(msg.ID) {
			_ = c.sendError(ws, c.rxCtx, msg.ID, codeInvalidRequest, "invalid request", nil)
		}
	}
}

func (c *Client) onDisconnect(ws *websocket.Conn) {
	shouldFailPending := false
	var reconnectWake chan struct{}

	c.stateMu.Lock()
	if c.ws == ws {
		c.ws = nil
		if c.cancel != nil {
			c.cancel()
		}
		c.cancel = nil
		c.rxCtx = nil
		c.done = nil
		shouldFailPending = true
		if !c.closed && c.reconnectCtx != nil && c.reconnectWake != nil {
			reconnectWake = c.reconnectWake
		}
	}
	c.stateMu.Unlock()

	if shouldFailPending {
		c.failAllPending(errConnectionClosed)
	}
	if reconnectWake != nil {
		select {
		case reconnectWake <- struct{}{}:
		default:
		}
	}
}

func (c *Client) reconnectLoop(ctx context.Context, wake <-chan struct{}, done chan struct{}) {
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
		}

		attempt := 0
		for {
			c.stateMu.RLock()
			if c.closed {
				c.stateMu.RUnlock()
				return
			}
			if c.ws != nil {
				c.stateMu.RUnlock()
				break
			}
			url := c.reconnectURL
			c.stateMu.RUnlock()

			ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
				Subprotocols: []string{"holon-rpc"},
			})
			if err == nil {
				if ws.Subprotocol() != "holon-rpc" {
					_ = ws.Close(websocket.StatusProtocolError, "missing holon-rpc subprotocol")
				} else {
					ws.SetReadLimit(1 << 20)

					rxCtx, cancel := context.WithCancel(context.Background())
					readDone := make(chan struct{})

					c.stateMu.Lock()
					switch {
					case c.closed:
						c.stateMu.Unlock()
						cancel()
						_ = ws.Close(websocket.StatusNormalClosure, "client closed")
						return
					case c.ws != nil:
						c.stateMu.Unlock()
						cancel()
						_ = ws.Close(websocket.StatusNormalClosure, "already connected")
						break
					default:
						c.ws = ws
						c.rxCtx = rxCtx
						c.cancel = cancel
						c.done = readDone
						c.stateMu.Unlock()

						go c.readLoop(ws, readDone)
						break
					}

					if c.Connected() {
						break
					}
				}
			}

			delay := reconnectDelay(attempt)
			attempt++

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
			}
		}
	}
}

func (c *Client) disableReconnect(expectedDone chan struct{}) {
	c.stateMu.Lock()
	if c.reconnectDone != expectedDone {
		c.stateMu.Unlock()
		return
	}

	reconnectCancel := c.reconnectCancel
	reconnectParentStop := c.reconnectParentStop
	reconnectDone := c.reconnectDone

	c.reconnectURL = ""
	c.reconnectCtx = nil
	c.reconnectCancel = nil
	c.reconnectParentStop = nil
	c.reconnectWake = nil
	c.reconnectDone = nil
	c.stateMu.Unlock()

	if reconnectParentStop != nil {
		reconnectParentStop()
	}
	if reconnectCancel != nil {
		reconnectCancel()
	}
	if reconnectDone != nil {
		<-reconnectDone
	}

}

func reconnectDelay(attempt int) time.Duration {
	base := float64(reconnectMinDelay) * math.Pow(reconnectFactor, float64(attempt))
	if base > float64(reconnectMaxDelay) {
		base = float64(reconnectMaxDelay)
	}
	jitter := 1 + rand.Float64()*reconnectJitter
	return time.Duration(base * jitter)
}

func (c *Client) handleRequest(ws *websocket.Conn, ctx context.Context, msg rpcMessage) {
	reqID := msg.ID

	if msg.JSONRPC != jsonRPCVersion {
		if hasID(reqID) {
			_ = c.sendError(ws, ctx, reqID, codeInvalidRequest, "invalid request", nil)
		}
		return
	}

	method := strings.TrimSpace(msg.Method)
	if method == "" {
		if hasID(reqID) {
			_ = c.sendError(ws, ctx, reqID, codeInvalidRequest, "invalid request", nil)
		}
		return
	}

	if method == "rpc.heartbeat" {
		if hasID(reqID) {
			_ = c.sendResult(ws, ctx, reqID, map[string]any{})
		}
		return
	}

	if hasID(reqID) {
		sid, err := decodeStringID(reqID)
		if err != nil || !strings.HasPrefix(sid, "s") {
			_ = c.sendError(ws, ctx, reqID, codeInvalidRequest, "server request id must start with 's'", nil)
			return
		}
	}

	params, err := decodeParams(msg.Params)
	if err != nil {
		if hasID(reqID) {
			_ = c.sendError(ws, ctx, reqID, codeInvalidParams, err.Error(), nil)
		}
		return
	}

	c.handlersMu.RLock()
	handler, ok := c.handlers[method]
	c.handlersMu.RUnlock()
	if !ok {
		if hasID(reqID) {
			_ = c.sendError(ws, ctx, reqID, codeMethodNotFound, fmt.Sprintf("method %q not found", method), nil)
		}
		return
	}

	result, err := handler(ctx, params)
	if err != nil {
		if !hasID(reqID) {
			return
		}
		var rpcErr *ResponseError
		if errors.As(err, &rpcErr) {
			_ = c.sendError(ws, ctx, reqID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			return
		}
		_ = c.sendError(ws, ctx, reqID, codeInternalError, "internal error", nil)
		return
	}

	if hasID(reqID) {
		_ = c.sendResult(ws, ctx, reqID, result)
	}
}

func (c *Client) handleResponse(msg rpcMessage) {
	key, ok := idKey(msg.ID)
	if !ok {
		return
	}

	c.pendingMu.Lock()
	ch, exists := c.pending[key]
	c.pendingMu.Unlock()
	if !exists {
		return
	}

	if msg.JSONRPC != jsonRPCVersion {
		msg.Error = &ResponseError{
			Code:    codeInvalidRequest,
			Message: "invalid response",
		}
		msg.Result = nil
	}

	select {
	case ch <- msg:
	default:
	}
}

func (c *Client) sendResult(ws *websocket.Conn, ctx context.Context, id json.RawMessage, result map[string]any) error {
	resultRaw, err := marshalObject(result)
	if err != nil {
		// Framework-level marshal failure → JSON-RPC internal error (§5.2).
		return c.sendError(ws, ctx, id, codeInternalError, "internal error", nil)
	}

	data, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  resultRaw,
	})
	if err != nil {
		return c.sendError(ws, ctx, id, codeInternalError, "internal error", nil)
	}
	return c.write(ws, ctx, data)
}

func (c *Client) sendError(ws *websocket.Conn, ctx context.Context, id json.RawMessage, code int, message string, data any) error {
	errBody := &ResponseError{
		Code:    code,
		Message: message,
		Data:    data,
	}
	dataJSON, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   errBody,
	})
	if err != nil {
		return err
	}
	return c.write(ws, ctx, dataJSON)
}

func (c *Client) write(ws *websocket.Conn, ctx context.Context, payload []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if err := ws.Write(ctx, websocket.MessageText, payload); err != nil {
		return fmt.Errorf("holon-rpc: write failed: %w", err)
	}
	return nil
}

func (c *Client) failAllPending(err error) {
	c.pendingMu.Lock()
	pending := c.pending
	c.pending = make(map[string]chan rpcMessage)
	c.pendingMu.Unlock()

	for _, ch := range pending {
		select {
		case ch <- rpcMessage{
			JSONRPC: jsonRPCVersion,
			Error: &ResponseError{
				Code:    codeUnavailable,
				Message: err.Error(),
			},
		}:
		default:
		}
	}
}
