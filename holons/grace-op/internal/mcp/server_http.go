package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

// ServeHTTP starts the MCP server as a Streamable HTTP endpoint per
// https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http
//
// Clients POST JSON-RPC requests to /mcp and receive JSON responses.
// The server binds to listenAddr (e.g. "127.0.0.1:8080").
func (s *Server) ServeHTTP(listenAddr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp", s.handleHTTPPost)
	mux.HandleFunc("GET /mcp", s.handleHTTPGet)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	host := listenAddr
	if h, _, splitErr := net.SplitHostPort(listenAddr); splitErr == nil {
		host = h
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	fmt.Fprintf(os.Stderr, "op mcp: listening on http://%s:%s/mcp\n", host, port)

	return http.Serve(listener, mux)
}

// handleHTTPPost processes a single JSON-RPC request via POST /mcp.
func (s *Server) handleHTTPPost(w http.ResponseWriter, r *http.Request) {
	// Origin validation per spec: servers SHOULD validate the Origin header.
	if origin := r.Header.Get("Origin"); origin != "" {
		if !isAllowedOrigin(origin) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	response, hasResponse := s.handle(r.Context(), body)

	if !hasResponse {
		// Notification — no response body needed.
		w.WriteHeader(http.StatusAccepted)
		return
	}

	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

// handleHTTPGet handles GET /mcp for SSE server-to-client streaming.
// Not yet implemented — op mcp doesn't initiate server-side events.
func (s *Server) handleHTTPGet(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "SSE not implemented", http.StatusNotImplemented)
}

// ParseHTTPListenAddr extracts a host:port from an http:// URI or
// returns the value as-is if it's already a bare address.
func ParseHTTPListenAddr(value string) string {
	for _, prefix := range []string{"http://", "https://"} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return value
}

// isAllowedOrigin checks the Origin header against localhost variants.
func isAllowedOrigin(origin string) bool {
	lower := strings.ToLower(origin)
	for _, allowed := range []string{
		"http://localhost", "https://localhost",
		"http://127.0.0.1", "https://127.0.0.1",
		"http://[::1]", "https://[::1]",
	} {
		if lower == allowed || strings.HasPrefix(lower, allowed+":") {
			return true
		}
	}
	return false
}
