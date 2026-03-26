package holonrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	defaultHTTPRPCPath = "/api/v1/rpc"
	codeHTTPNotFound   = 5
	codeHTTPInternal   = 13
)

// StreamHandler handles one server-streaming HTTP+SSE request.
type StreamHandler func(ctx context.Context, params map[string]any, send func(map[string]any) error) error

// SSEEvent is one decoded server-sent event from the HTTP+SSE transport.
type SSEEvent struct {
	Event  string
	ID     string
	Result map[string]any
	Error  *ResponseError
}

// HTTPServer is a standalone Holon-RPC server using HTTP request/response
// for unary calls and Server-Sent Events for server-streaming calls.
type HTTPServer struct {
	address string
	path    string

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	closed   bool

	handlersMu     sync.RWMutex
	handlers       map[string]Handler
	streamHandlers map[string]StreamHandler

	nextRequestID int64
}

// NewHTTPServer creates an HTTP+SSE Holon-RPC server bound to the given base URL.
//
// Example URL: http://127.0.0.1:0/api/v1/rpc
func NewHTTPServer(bindURL string) *HTTPServer {
	return &HTTPServer{
		address:        bindURL,
		handlers:       make(map[string]Handler),
		streamHandlers: make(map[string]StreamHandler),
	}
}

// Address returns the current HTTP+SSE endpoint. Before Start, it returns the
// configured bind URL. After Start, it returns the resolved URL with bound port.
func (s *HTTPServer) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.address
}

// Register registers a unary HTTP handler for client-originated requests.
func (s *HTTPServer) Register(method string, handler Handler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[method] = handler
}

// RegisterStream registers a server-streaming HTTP+SSE handler.
func (s *HTTPServer) RegisterStream(method string, handler StreamHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.streamHandlers[method] = handler
}

// Start starts the HTTP+SSE server.
func (s *HTTPServer) Start() (string, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", errors.New("holon-rpc: http server is closed")
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
		return "", fmt.Errorf("holon-rpc: invalid http server URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("holon-rpc: unsupported scheme %q (expected http:// or https://)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}
	port := parsed.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	path := parsed.Path
	if path == "" {
		path = defaultHTTPRPCPath
	}

	certFile, keyFile, err := httpsCertFiles(parsed, scheme)
	if err != nil {
		return "", err
	}

	lis, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return "", fmt.Errorf("holon-rpc: listen failed: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, s.handleHTTP)
	mux.HandleFunc(strings.TrimRight(path, "/")+"/", s.handleHTTP)

	srv := &http.Server{Handler: mux}
	go func() {
		if scheme == "https" {
			_ = srv.ServeTLS(lis, certFile, keyFile)
			return
		}
		_ = srv.Serve(lis)
	}()

	actual := fmt.Sprintf("%s://%s%s", scheme, lis.Addr().String(), path)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = lis.Close()
		_ = srv.Close()
		return "", errors.New("holon-rpc: http server is closed")
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

// Close gracefully stops the HTTP+SSE server.
func (s *HTTPServer) Close(ctx context.Context) error {
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
	return shutdownErr
}

func httpsCertFiles(parsed *url.URL, scheme string) (string, string, error) {
	if scheme != "https" {
		return "", "", nil
	}

	query := parsed.Query()
	certFile := query.Get("cert")
	keyFile := query.Get("key")
	if certFile == "" {
		certFile = os.Getenv("HOLONS_HTTPS_CERT_FILE")
	}
	if keyFile == "" {
		keyFile = os.Getenv("HOLONS_HTTPS_KEY_FILE")
	}
	if certFile == "" || keyFile == "" {
		return "", "", fmt.Errorf("holon-rpc: https:// requires cert and key (query params cert/key or HOLONS_HTTPS_CERT_FILE/HOLONS_HTTPS_KEY_FILE)")
	}
	return certFile, keyFile, nil
}

func (s *HTTPServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	setHTTPCORSHeaders(w, r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	method, ok := s.methodFromPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if acceptsSSE(r) {
		switch r.Method {
		case http.MethodGet:
			params := queryParams(r.URL.Query())
			s.handleStream(w, r, method, params)
			return
		case http.MethodPost:
			params, rpcErr := decodeHTTPRequestParams(r.Body)
			if rpcErr != nil {
				writeHTTPRPCError(w, httpStatusForRPCError(rpcErr), "h0", rpcErr)
				return
			}
			s.handleStream(w, r, method, params)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	params, rpcErr := decodeHTTPRequestParams(r.Body)
	if rpcErr != nil {
		writeHTTPRPCError(w, httpStatusForRPCError(rpcErr), "h0", rpcErr)
		return
	}

	s.handlersMu.RLock()
	handler := s.handlers[method]
	s.handlersMu.RUnlock()
	if handler == nil {
		writeHTTPRPCError(w, http.StatusNotFound, "h0", &ResponseError{
			Code:    codeHTTPNotFound,
			Message: fmt.Sprintf("method %q not found", method),
		})
		return
	}

	result, err := handler(r.Context(), params)
	if err != nil {
		rpcErr := responseErrorFromErr(err)
		writeHTTPRPCError(w, httpStatusForRPCError(rpcErr), s.nextHTTPID(), rpcErr)
		return
	}

	writeHTTPRPCResult(w, s.nextHTTPID(), result)
}

func (s *HTTPServer) handleStream(w http.ResponseWriter, r *http.Request, method string, params map[string]any) {
	s.handlersMu.RLock()
	handler := s.streamHandlers[method]
	s.handlersMu.RUnlock()
	if handler == nil {
		writeHTTPRPCError(w, http.StatusNotFound, "h0", &ResponseError{
			Code:    codeHTTPNotFound,
			Message: fmt.Sprintf("method %q not found", method),
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeHTTPRPCError(w, http.StatusInternalServerError, "h0", &ResponseError{
			Code:    codeHTTPInternal,
			Message: "streaming not supported by response writer",
		})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	requestID := s.nextHTTPID()
	var eventID int64
	send := func(result map[string]any) error {
		payload, err := marshalMessage(rpcMessage{
			JSONRPC: jsonRPCVersion,
			ID:      makeID(requestID),
			Result:  mustMarshalObject(result),
		})
		if err != nil {
			return err
		}
		return writeSSEEvent(w, flusher, "message", strconv.FormatInt(atomic.AddInt64(&eventID, 1), 10), payload)
	}

	if err := handler(r.Context(), params, send); err != nil {
		rpcErr := responseErrorFromErr(err)
		payload, marshalErr := marshalMessage(rpcMessage{
			JSONRPC: jsonRPCVersion,
			ID:      makeID(requestID),
			Error:   rpcErr,
		})
		if marshalErr == nil {
			_ = writeSSEEvent(w, flusher, "error", strconv.FormatInt(atomic.AddInt64(&eventID, 1), 10), payload)
		}
	}

	_ = writeSSEEvent(w, flusher, "done", "", nil)
}

func (s *HTTPServer) methodFromPath(path string) (string, bool) {
	base := strings.TrimRight(s.path, "/")
	prefix := base + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	method := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	return method, method != ""
}

func (s *HTTPServer) nextHTTPID() string {
	return fmt.Sprintf("h%d", atomic.AddInt64(&s.nextRequestID, 1))
}

// HTTPClient is a Holon-RPC client for the HTTP+SSE transport.
type HTTPClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPClient creates an HTTP+SSE client for the given base URL.
func NewHTTPClient(baseURL string) *HTTPClient {
	return NewHTTPClientWithClient(baseURL, nil)
}

// NewHTTPClientWithClient creates an HTTP+SSE client with an explicit http.Client.
func NewHTTPClientWithClient(baseURL string, client *http.Client) *HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  client,
	}
}

// Invoke sends one unary HTTP request and decodes the JSON-RPC result.
func (c *HTTPClient) Invoke(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	body := mustMarshalObject(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: build http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: http invoke failed: %w", err)
	}
	defer resp.Body.Close()

	return decodeHTTPRPCResponse(resp)
}

// Stream sends a server-streaming HTTP request using POST and returns every SSE event.
func (c *HTTPClient) Stream(ctx context.Context, method string, params map[string]any) ([]SSEEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	body := mustMarshalObject(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: build http stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: http stream failed: %w", err)
	}
	defer resp.Body.Close()

	return readSSEEvents(resp)
}

// StreamQuery sends a server-streaming GET request with query parameters.
func (c *HTTPClient) StreamQuery(ctx context.Context, method string, params map[string]string) ([]SSEEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	endpoint, err := url.Parse(c.methodURL(method))
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: parse method URL: %w", err)
	}

	query := endpoint.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: build http query stream request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: http query stream failed: %w", err)
	}
	defer resp.Body.Close()

	return readSSEEvents(resp)
}

func (c *HTTPClient) methodURL(method string) string {
	return c.baseURL + "/" + strings.Trim(strings.TrimSpace(method), "/")
}

func decodeHTTPRPCResponse(resp *http.Response) (map[string]any, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("holon-rpc: read http response: %w", err)
	}

	var msg rpcMessage
	if err := json.Unmarshal(data, &msg); err == nil && (msg.JSONRPC == jsonRPCVersion || msg.Error != nil || len(msg.Result) > 0) {
		if msg.Error != nil {
			return nil, msg.Error
		}
		return decodeResult(msg.Result)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("holon-rpc: http status %d", resp.StatusCode)
	}
	return decodeResult(json.RawMessage(data))
}

func readSSEEvents(resp *http.Response) ([]SSEEvent, error) {
	if resp.StatusCode >= 400 {
		_, err := decodeHTTPRPCResponse(resp)
		return nil, err
	}

	events := make([]SSEEvent, 0)
	scanner := bufio.NewScanner(resp.Body)

	type rawEvent struct {
		event string
		id    string
		data  string
	}

	current := rawEvent{}
	flush := func() ([]SSEEvent, error, bool) {
		if current.event == "" && current.id == "" && current.data == "" {
			return nil, nil, false
		}

		event := SSEEvent{
			Event: current.event,
			ID:    current.id,
		}
		switch current.event {
		case "message", "error":
			var msg rpcMessage
			if err := json.Unmarshal([]byte(current.data), &msg); err != nil {
				return nil, fmt.Errorf("holon-rpc: decode SSE payload: %w", err), false
			}
			if msg.Error != nil {
				event.Error = msg.Error
			} else {
				result, err := decodeResult(msg.Result)
				if err != nil {
					return nil, fmt.Errorf("holon-rpc: decode SSE result: %w", err), false
				}
				event.Result = result
			}
		case "done":
			events = append(events, event)
			return events, nil, true
		}

		events = append(events, event)
		if event.Error != nil {
			return events, event.Error, true
		}
		return events, nil, false
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flushed, err, stop := flush()
			if flushed != nil {
				events = flushed
			}
			if err != nil || stop {
				return events, err
			}
			current = rawEvent{}
			continue
		}

		switch {
		case strings.HasPrefix(line, "event:"):
			current.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "id:"):
			current.id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "data:"):
			current.data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("holon-rpc: read SSE stream: %w", err)
	}

	flushed, err, _ := flush()
	if flushed != nil {
		events = flushed
	}
	return events, err
}

func setHTTPCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Last-Event-ID")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Vary", "Origin")
}

func acceptsSSE(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

func decodeHTTPRequestParams(body io.Reader) (map[string]any, *ResponseError) {
	if body == nil {
		return map[string]any{}, nil
	}

	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, &ResponseError{
			Code:    codeHTTPInternal,
			Message: "read request body failed",
		}
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]any{}, nil
	}

	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return nil, &ResponseError{
			Code:    codeParseError,
			Message: "parse error",
		}
	}

	if decoded == nil {
		return map[string]any{}, nil
	}

	params, ok := decoded.(map[string]any)
	if !ok {
		return nil, &ResponseError{
			Code:    codeInvalidRequest,
			Message: "invalid request",
		}
	}

	return params, nil
}

func queryParams(values url.Values) map[string]any {
	params := make(map[string]any, len(values))
	for key, entries := range values {
		switch len(entries) {
		case 0:
			params[key] = ""
		case 1:
			params[key] = entries[0]
		default:
			out := make([]string, len(entries))
			copy(out, entries)
			params[key] = out
		}
	}
	return params
}

func responseErrorFromErr(err error) *ResponseError {
	if err == nil {
		return nil
	}

	var rpcErr *ResponseError
	if errors.As(err, &rpcErr) {
		return rpcErr
	}

	return &ResponseError{
		Code:    codeHTTPInternal,
		Message: "internal error",
	}
}

func httpStatusForRPCError(err *ResponseError) int {
	if err == nil {
		return http.StatusOK
	}

	switch err.Code {
	case codeParseError, codeInvalidRequest, codeInvalidParams:
		return http.StatusBadRequest
	case codeHTTPNotFound, codeMethodNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeHTTPRPCResult(w http.ResponseWriter, requestID string, result map[string]any) {
	data, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      makeID(requestID),
		Result:  mustMarshalObject(result),
	})
	if err != nil {
		writeHTTPRPCError(w, http.StatusInternalServerError, requestID, &ResponseError{
			Code:    codeHTTPInternal,
			Message: "internal error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeHTTPRPCError(w http.ResponseWriter, statusCode int, requestID string, rpcErr *ResponseError) {
	if rpcErr == nil {
		rpcErr = &ResponseError{Code: codeHTTPInternal, Message: "internal error"}
	}
	data, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      makeID(requestID),
		Error:   rpcErr,
	})
	if err != nil {
		data = []byte(`{"jsonrpc":"2.0","error":{"code":13,"message":"internal error"}}`)
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

func writeSSEEvent(w io.Writer, flusher http.Flusher, eventName string, id string, data []byte) error {
	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}
	if strings.TrimSpace(id) != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if data == nil {
		if _, err := io.WriteString(w, "data:\n\n"); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func mustMarshalObject(obj map[string]any) json.RawMessage {
	out, err := marshalObject(obj)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}
