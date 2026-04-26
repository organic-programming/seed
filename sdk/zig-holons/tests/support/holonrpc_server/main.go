package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
	"nhooyr.io/websocket"
)

type ready struct {
	URL    string `json:"url"`
	CAFile string `json:"ca_file,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

var httpID int64

func main() {
	mode := flag.String("mode", "ws", "ws, wss, rest, or hub")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		info       ready
		stopServer func(context.Context) error
		err        error
	)
	switch *mode {
	case "ws", "hub":
		info, stopServer, err = startWebSocket(ctx, false)
	case "wss":
		info, stopServer, err = startWebSocket(ctx, true)
	case "rest":
		info, stopServer, err = startHTTP(ctx)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		log.Fatal(err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
		log.Fatal(err)
	}
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = stopServer(shutdownCtx)
}

func startWebSocket(ctx context.Context, tls bool) (ready, func(context.Context) error, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"holon-rpc"},
		})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			response, err := handleRPC(data)
			if err != nil {
				return
			}
			if err := conn.Write(r.Context(), websocket.MessageText, response); err != nil {
				return
			}
		}
	})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return ready{}, nil, err
	}
	srv := &http.Server{Handler: mux}

	if tls {
		certFile, keyFile, err := writeSelfSignedCert()
		if err != nil {
			_ = lis.Close()
			return ready{}, nil, err
		}
		go func() { _ = srv.ServeTLS(lis, certFile, keyFile) }()
		return ready{
			URL:    fmt.Sprintf("wss://localhost:%d/rpc", lis.Addr().(*net.TCPAddr).Port),
			CAFile: certFile,
		}, srv.Shutdown, nil
	}

	go func() { _ = srv.Serve(lis) }()
	return ready{URL: fmt.Sprintf("ws://%s/rpc", lis.Addr().String())}, srv.Shutdown, nil
}

func startHTTP(ctx context.Context) (ready, func(context.Context) error, error) {
	_ = ctx
	server := holonrpc.NewHTTPServer("http://127.0.0.1:0/api/v1/rpc")
	server.Register("echo.v1.Echo/Ping", func(ctx context.Context, params map[string]any) (map[string]any, error) {
		return params, nil
	})
	server.Register("hub.v1.Hub/ListPeers", func(ctx context.Context, params map[string]any) (map[string]any, error) {
		return map[string]any{"peers": []map[string]any{{"id": "go-helper", "methods": []string{"echo.v1.Echo/Ping"}}}}, nil
	})
	server.RegisterStream("echo.v1.Echo/Stream", func(ctx context.Context, params map[string]any, send func(map[string]any) error) error {
		message, _ := params["message"].(string)
		if message == "" {
			message = "stream"
		}
		if err := send(map[string]any{"message": message + ":1"}); err != nil {
			return err
		}
		return send(map[string]any{"message": message + ":2"})
	})
	address, err := server.Start()
	if err != nil {
		return ready{}, nil, err
	}
	return ready{URL: strings.Replace(address, "http://", "rest+sse://", 1)}, server.Close, nil
}

func handleRPC(data []byte) ([]byte, error) {
	var message rpcMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, err
	}
	result := json.RawMessage(`{}`)
	switch message.Method {
	case "echo.v1.Echo/Ping":
		if len(message.Params) > 0 {
			result = message.Params
		}
	case "hub.v1.Hub/ListPeers":
		result = json.RawMessage(`{"peers":[{"id":"go-helper","methods":["echo.v1.Echo/Ping"]}]}`)
	default:
		errorPayload := map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(message.ID),
			"error": map[string]any{
				"code":    -32601,
				"message": "method not found",
			},
		}
		return json.Marshal(errorPayload)
	}
	response := rpcMessage{
		JSONRPC: "2.0",
		ID:      message.ID,
		Result:  result,
	}
	if len(response.ID) == 0 {
		response.ID = json.RawMessage(fmt.Sprintf("%q", fmt.Sprintf("h%d", atomic.AddInt64(&httpID, 1))))
	}
	return json.Marshal(response)
}

func writeSelfSignedCert() (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}
	dir, err := os.MkdirTemp("", "zig-holonrpc-wss-*")
	if err != nil {
		return "", "", err
	}
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return "", "", err
	}
	return certFile, keyFile, nil
}
