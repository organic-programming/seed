package holonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"nhooyr.io/websocket"
)

// Server is a standalone Holon-RPC server (JSON-RPC 2.0 over WebSocket).
// It owns its own TCP listener and HTTP server.
//
// For embedding Holon-RPC in an existing HTTP server (e.g. alongside
// static files in a browser-facing application), use
// [transport.WebBridge] from pkg/transport instead.
//
// It supports bidirectional calls:
//   - client -> server via Register handlers
//   - server -> client via Invoke(clientID, ...)
type Server struct {
	address string
	path    string

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	closed   bool

	handlersMu sync.RWMutex
	handlers   map[string]Handler

	peersMu sync.RWMutex
	peers   map[string]*serverPeer

	// connectQ buffers up to 32 client-connection events for WaitForClient.
	// When it is full, new events are dropped to keep accept-path backpressure-free.
	connectQ chan string

	nextClientID int64
	nextServerID int64
}

type serverPeer struct {
	id string
	ws *websocket.Conn

	ctx    context.Context
	cancel context.CancelFunc

	sendMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan rpcMessage
}

// NewServer creates a Holon-RPC server bound to the given URL.
//
// Example URL: ws://127.0.0.1:0/rpc
func NewServer(bindURL string) *Server {
	return &Server{
		address:  bindURL,
		handlers: make(map[string]Handler),
		peers:    make(map[string]*serverPeer),
		// 32 is intentionally small: connection events are best-effort signals.
		// Callers that need exact client state should inspect ClientIDs().
		connectQ: make(chan string, 32),
	}
}

// Address returns the current server endpoint. Before Start, it returns the
// configured bind URL. After Start, it returns the resolved URL with bound port.
func (s *Server) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.address
}

// Register registers a server-side handler for client-originated requests.
func (s *Server) Register(method string, handler Handler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[method] = handler
}

// Unregister removes a previously registered server-side handler.
func (s *Server) Unregister(method string) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	delete(s.handlers, method)
}

// ClientIDs returns currently connected client IDs.
func (s *Server) ClientIDs() []string {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	out := make([]string, 0, len(s.peers))
	for id := range s.peers {
		out = append(out, id)
	}
	return out
}

func (s *Server) peersResult() []map[string]any {
	s.peersMu.RLock()
	ids := make([]string, 0, len(s.peers))
	for id := range s.peers {
		ids = append(ids, id)
	}
	s.peersMu.RUnlock()

	sort.Strings(ids)

	peers := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		peers = append(peers, map[string]any{
			"id":      id,
			"methods": []string{},
		})
	}
	return peers
}

// WaitForClient blocks until a client is connected or ctx is cancelled.
func (s *Server) WaitForClient(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case id := <-s.connectQ:
		return id, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Start starts the Holon-RPC server.
func (s *Server) Start() (string, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", errors.New("holon-rpc: server is closed")
	}
	if s.server != nil {
		addr := s.address
		s.mu.Unlock()
		return addr, nil
	}
	bindURL := s.address
	s.mu.Unlock()

	parsed, err := url.Parse(bindURL)
	if err != nil {
		return "", fmt.Errorf("holon-rpc: invalid server URL: %w", err)
	}
	if parsed.Scheme != "ws" {
		return "", fmt.Errorf("holon-rpc: unsupported scheme %q (expected ws://)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := parsed.Port()
	if port == "" {
		port = "80"
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/rpc"
	}

	lis, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return "", fmt.Errorf("holon-rpc: listen failed: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handleWebSocket)

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(lis)
	}()

	addr := lis.Addr().String()
	actual := fmt.Sprintf("ws://%s%s", addr, path)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = lis.Close()
		_ = srv.Close()
		return "", errors.New("holon-rpc: server is closed")
	}
	if s.server != nil {
		_ = lis.Close()
		_ = srv.Close()
		return s.address, nil
	}
	s.server = srv
	s.listener = lis
	s.path = path
	s.address = actual

	return s.address, nil
}

// Close gracefully stops the server and disconnects all clients.
func (s *Server) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	srv := s.server
	s.server = nil
	lis := s.listener
	s.listener = nil
	s.mu.Unlock()

	var shutdownErr error
	if srv != nil {
		if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErr = err
		}
	}
	if lis != nil {
		_ = lis.Close()
	}

	s.peersMu.Lock()
	peers := make([]*serverPeer, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	s.peers = make(map[string]*serverPeer)
	s.peersMu.Unlock()

	for _, peer := range peers {
		peer.cancel()
		s.failPeerPending(peer, errConnectionClosed)
		_ = peer.ws.Close(websocket.StatusGoingAway, "server shutdown")
	}

	if shutdownErr != nil {
		return shutdownErr
	}
	return nil
}

// Invoke initiates a server-originated request toward a connected client.
//
// Server-originated IDs are prefixed with "s" as required by COMMUNICATION.md §4.6.
func (s *Server) Invoke(ctx context.Context, clientID, method string, params map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(method) == "" {
		return nil, errors.New("holon-rpc: method is required")
	}

	s.peersMu.RLock()
	peer := s.peers[clientID]
	s.peersMu.RUnlock()
	if peer == nil {
		return nil, fmt.Errorf("holon-rpc: unknown client %q", clientID)
	}

	id := fmt.Sprintf("s%d", atomic.AddInt64(&s.nextServerID, 1))
	idRaw := makeID(id)
	key, _ := idKey(idRaw)

	ch := make(chan rpcMessage, 1)
	peer.pendingMu.Lock()
	peer.pending[key] = ch
	peer.pendingMu.Unlock()
	defer func() {
		peer.pendingMu.Lock()
		delete(peer.pending, key)
		peer.pendingMu.Unlock()
	}()

	paramsRaw, err := marshalObject(params)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: marshal params: %w", err)
	}

	req, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      idRaw,
		Method:  method,
		Params:  paramsRaw,
	})
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: marshal request: %w", err)
	}

	if err := s.writePeer(peer, req); err != nil {
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
	case <-peer.ctx.Done():
		return nil, errConnectionClosed
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:       []string{"holon-rpc"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer c.CloseNow()

	if c.Subprotocol() != "holon-rpc" {
		_ = c.Close(websocket.StatusProtocolError, "missing holon-rpc subprotocol")
		return
	}
	c.SetReadLimit(1 << 20)

	clientID := fmt.Sprintf("c%d", atomic.AddInt64(&s.nextClientID, 1))
	ctx, cancel := context.WithCancel(r.Context())

	peer := &serverPeer{
		id:      clientID,
		ws:      c,
		ctx:     ctx,
		cancel:  cancel,
		pending: make(map[string]chan rpcMessage),
	}

	s.peersMu.Lock()
	s.peers[clientID] = peer
	s.peersMu.Unlock()

	select {
	case s.connectQ <- clientID:
	default:
		log.Printf("holon-rpc: dropping connect event for %s: WaitForClient queue is full", clientID)
	}

	defer func() {
		cancel()

		s.peersMu.Lock()
		delete(s.peers, clientID)
		s.peersMu.Unlock()

		s.failPeerPending(peer, errConnectionClosed)
	}()

	for {
		kind, data, err := c.Read(ctx)
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
			_ = s.sendPeerError(peer, json.RawMessage("null"), code, message, nil)
			continue
		}

		if msg.Method != "" {
			go s.handlePeerRequest(peer, msg)
			continue
		}

		if len(msg.Result) > 0 || msg.Error != nil {
			s.handlePeerResponse(peer, msg)
			continue
		}

		if hasID(msg.ID) {
			_ = s.sendPeerError(peer, msg.ID, codeInvalidRequest, "invalid request", nil)
		}
	}
}

func (s *Server) handlePeerRequest(peer *serverPeer, msg rpcMessage) {
	reqID := msg.ID

	if msg.JSONRPC != jsonRPCVersion {
		if hasID(reqID) {
			_ = s.sendPeerError(peer, reqID, codeInvalidRequest, "invalid request", nil)
		}
		return
	}

	method := strings.TrimSpace(msg.Method)
	if method == "" {
		if hasID(reqID) {
			_ = s.sendPeerError(peer, reqID, codeInvalidRequest, "invalid request", nil)
		}
		return
	}

	if method == "rpc.heartbeat" {
		if hasID(reqID) {
			_ = s.sendPeerResult(peer, reqID, map[string]any{})
		}
		return
	}

	if method == "rpc.peers" {
		if hasID(reqID) {
			_ = s.sendPeerResult(peer, reqID, map[string]any{
				"peers": s.peersResult(),
			})
		}
		return
	}

	params, err := decodeParams(msg.Params)
	if err != nil {
		if hasID(reqID) {
			_ = s.sendPeerError(peer, reqID, codeInvalidParams, err.Error(), nil)
		}
		return
	}

	dispatchMethod, fanOut, cleanedParams, routingHints, err := parseRouteHints(method, params)
	if err != nil {
		if hasID(reqID) {
			_ = s.sendRPCError(peer, reqID, err)
		}
		return
	}
	method = dispatchMethod
	params = cleanedParams

	routed, _ := s.routePeerRequest(peer, reqID, method, params, routingHints, fanOut)
	if routed {
		return
	}

	s.handlersMu.RLock()
	handler, ok := s.handlers[method]
	s.handlersMu.RUnlock()
	if !ok {
		if hasID(reqID) {
			_ = s.sendPeerError(peer, reqID, codeMethodNotFound, fmt.Sprintf("method %q not found", method), nil)
		}
		return
	}

	// The request context is peer-scoped: disconnecting the peer cancels
	// the handler context and allows in-flight work to terminate promptly.
	result, err := s.callPeerHandler(peer.ctx, handler, params)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if !hasID(reqID) {
			return
		}
		var rpcErr *ResponseError
		if errors.As(err, &rpcErr) {
			_ = s.sendPeerError(peer, reqID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			return
		}
		_ = s.sendPeerError(peer, reqID, codeInternalError, "internal error", nil)
		return
	}

	if hasID(reqID) {
		_ = s.sendPeerResult(peer, reqID, result)
	}
}

func (s *Server) callPeerHandler(ctx context.Context, handler Handler, params map[string]any) (map[string]any, error) {
	type handlerResult struct {
		result map[string]any
		err    error
	}

	done := make(chan handlerResult, 1)
	go func() {
		res, err := handler(ctx, params)
		done <- handlerResult{result: res, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case out := <-done:
		return out.result, out.err
	}
}

func (s *Server) handlePeerResponse(peer *serverPeer, msg rpcMessage) {
	key, ok := idKey(msg.ID)
	if !ok {
		return
	}

	peer.pendingMu.Lock()
	ch, exists := peer.pending[key]
	peer.pendingMu.Unlock()
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

func (s *Server) sendPeerResult(peer *serverPeer, id json.RawMessage, result map[string]any) error {
	return s.sendPeerResultAny(peer, id, result)
}

func (s *Server) sendPeerError(peer *serverPeer, id json.RawMessage, code int, message string, data any) error {
	msg, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
	if err != nil {
		return err
	}
	return s.writePeer(peer, msg)
}

func (s *Server) writePeer(peer *serverPeer, payload []byte) error {
	peer.sendMu.Lock()
	defer peer.sendMu.Unlock()

	if err := peer.ws.Write(peer.ctx, websocket.MessageText, payload); err != nil {
		return fmt.Errorf("holon-rpc: write failed: %w", err)
	}
	return nil
}

func (s *Server) failPeerPending(peer *serverPeer, err error) {
	peer.pendingMu.Lock()
	pending := peer.pending
	peer.pending = make(map[string]chan rpcMessage)
	peer.pendingMu.Unlock()

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
