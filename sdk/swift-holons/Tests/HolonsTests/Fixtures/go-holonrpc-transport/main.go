package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
	"nhooyr.io/websocket"
)

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	mode := "http"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	certFile := ""
	if len(os.Args) > 2 {
		certFile = os.Args[2]
	}
	keyFile := ""
	if len(os.Args) > 3 {
		keyFile = os.Args[3]
	}

	switch mode {
	case "wss":
		if err := writeSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatal(err)
		}
		runWSS(certFile, keyFile)
	case "http":
		runHTTP("http", "", "")
	case "https":
		if err := writeSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatal(err)
		}
		runHTTP("https", certFile, keyFile)
	default:
		log.Fatalf("unsupported mode %q", mode)
	}
}

func runWSS(certFile, keyFile string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"holon-rpc"},
		})
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer c.CloseNow()

		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}

			var msg rpcMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = writeError(ctx, c, nil, -32700, "parse error")
				continue
			}
			if msg.JSONRPC != "2.0" {
				_ = writeError(ctx, c, msg.ID, -32600, "invalid request")
				continue
			}

			switch msg.Method {
			case "rpc.heartbeat":
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{})
			case "echo.v1.Echo/Ping":
				var params map[string]interface{}
				_ = json.Unmarshal(msg.Params, &params)
				if params == nil {
					params = map[string]interface{}{}
				}
				params["transport"] = "wss"
				_ = writeResult(ctx, c, msg.ID, params)
			default:
				_ = writeError(ctx, c, msg.ID, -32601, fmt.Sprintf("method %q not found", msg.Method))
			}
		}
	})

	srv := &http.Server{Handler: h}
	go func() {
		if err := srv.ServeTLS(ln, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	fmt.Printf("wss://%s/rpc?ca=%s\n", ln.Addr().String(), url.QueryEscape(certFile))
	waitForShutdown(func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})
}

func runHTTP(scheme, certFile, keyFile string) {
	bindURL := fmt.Sprintf("%s://127.0.0.1:0/api/v1/rpc", scheme)
	if scheme == "https" {
		bindURL = fmt.Sprintf(
			"https://127.0.0.1:0/api/v1/rpc?cert=%s&key=%s",
			url.QueryEscape(certFile),
			url.QueryEscape(keyFile),
		)
	}

	server := holonrpc.NewHTTPServer(bindURL)
	server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		out := cloneParams(params)
		out["transport"] = scheme
		return out, nil
	})
	server.RegisterStream("echo.v1.Echo/Watch", func(_ context.Context, params map[string]any, send func(map[string]any) error) error {
		base := cloneParams(params)
		base["transport"] = scheme

		first := cloneParams(base)
		first["step"] = "1"
		if err := send(first); err != nil {
			return err
		}

		time.Sleep(20 * time.Millisecond)

		second := cloneParams(base)
		second["step"] = "2"
		return send(second)
	})

	addr, err := server.Start()
	if err != nil {
		log.Fatal(err)
	}

	if scheme == "https" {
		fmt.Printf("%s?ca=%s\n", addr, url.QueryEscape(certFile))
	} else {
		fmt.Println(addr)
	}

	waitForShutdown(func(ctx context.Context) error {
		return server.Close(ctx)
	})
}

func cloneParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}

func waitForShutdown(closeFn func(context.Context) error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = closeFn(ctx)
}

func writeSelfSignedCert(certFile, keyFile string) error {
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("cert and key paths are required")
	}
	if err := os.MkdirAll(filepath.Dir(certFile), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0o755); err != nil {
		return err
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if certPEM == nil || keyPEM == nil {
		return fmt.Errorf("encode pem failed")
	}

	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return err
	}
	return nil
}

func writeResult(ctx context.Context, c *websocket.Conn, id interface{}, result interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  mustRaw(result),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func writeError(ctx context.Context, c *websocket.Conn, id interface{}, code int, message string) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func mustRaw(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
