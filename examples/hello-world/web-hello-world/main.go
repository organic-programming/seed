// web-hello-world serves a browser-accessible hello holon with
// bidirectional RPC over WebSocket.
//
// Demonstrates [transport.WebBridge]:
//   - Browser → Go:  "Greet" handler (browser invokes, Go responds)
//   - Go → Browser:  "GetViewport" call (Go invokes, browser responds)
//
// Usage:
//
//	go run main.go
//	open http://localhost:8080
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/transport"
)

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	bridge := transport.NewWebBridge()

	// Browser → Go: register a Greet handler
	bridge.Register("hello.v1.HelloService/Greet", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &transport.WebError{Code: 3, Message: "invalid payload"}
		}
		name := req.Name
		if name == "" {
			name = "World"
		}
		return json.Marshal(map[string]string{
			"message": fmt.Sprintf("Hello, %s!", name),
		})
	})

	// Go → Browser: when a browser connects, ask for its viewport size
	bridge.OnConnect(func(conn *transport.WebConn) {
		// Small delay to let the browser register its handlers
		time.Sleep(500 * time.Millisecond)

		result, err := conn.InvokeWithTimeout("ui.v1.UIService/GetViewport", nil, 5*time.Second)
		if err != nil {
			log.Printf("go→browser GetViewport failed: %v", err)
			return
		}

		var viewport struct {
			Width  int     `json:"width"`
			Height int     `json:"height"`
			DPR    float64 `json:"devicePixelRatio"`
		}
		json.Unmarshal(result, &viewport)
		log.Printf("go→browser GetViewport: %dx%d @%.1fx", viewport.Width, viewport.Height, viewport.DPR)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", bridge.HandleWebSocket)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	server := &http.Server{Addr: ":" + port, Handler: mux}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		server.Close()
	}()

	log.Printf("web-hello-world listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
