package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

const (
	defaultSDK       = "rust-holons"
	defaultServerSDK = "go-holons"
	defaultMethod    = "echo.v1.Echo/Ping"
	defaultMessage   = "cert"
	defaultTimeoutMS = 5000

	connectOnlyHeartbeatInterval = 200 * time.Millisecond
	connectOnlyHeartbeatTimeout  = 400 * time.Millisecond
)

type options struct {
	url              string
	sdk              string
	serverSDK        string
	method           string
	params           map[string]any
	expectedErrorIDs []int
	timeoutMS        int
	connectOnly      bool
	bidirectional    bool
}

func main() {
	args, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	timeout := time.Duration(args.timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultTimeoutMS * time.Millisecond
	}

	startedAt := time.Now()

	client := holonrpc.NewClient()
	if args.bidirectional {
		client.Register("client.v1.Client/Hello", func(_ context.Context, params map[string]any) (map[string]any, error) {
			name := "unknown"
			if rawName, ok := params["name"].(string); ok {
				trimmed := strings.TrimSpace(rawName)
				if trimmed != "" {
					name = trimmed
				}
			}
			return map[string]any{"message": "pong:" + name}, nil
		})
	}

	if args.connectOnly {
		runConnectOnly(client, args, startedAt, timeout)
		return
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), timeout)
	err = client.Connect(connectCtx, args.url)
	connectCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = client.Close()
	}()

	if args.bidirectional {
		runBidirectional(client, args, startedAt, timeout)
		return
	}

	invokeCtx, invokeCancel := context.WithTimeout(context.Background(), timeout)
	out, err := client.Invoke(invokeCtx, args.method, args.params)
	invokeCancel()
	if err != nil {
		if code, ok := rpcErrorCode(err); ok && containsInt(args.expectedErrorIDs, code) {
			mustWriteJSON(map[string]any{
				"status":     "pass",
				"sdk":        args.sdk,
				"server_sdk": args.serverSDK,
				"latency_ms": time.Since(startedAt).Milliseconds(),
				"method":     args.method,
				"error_code": code,
			})
			return
		}
		fmt.Fprintf(os.Stderr, "invoke failed: %v\n", err)
		os.Exit(1)
	}

	if len(args.expectedErrorIDs) > 0 {
		fmt.Fprintf(os.Stderr, "expected one of error codes %v, but call succeeded\n", args.expectedErrorIDs)
		os.Exit(1)
	}

	if args.method == defaultMethod {
		expected := fmt.Sprint(args.params["message"])
		actual := fmt.Sprint(out["message"])
		if actual != expected {
			fmt.Fprintf(os.Stderr, "unexpected echo response: %v\n", out)
			os.Exit(1)
		}
	}

	mustWriteJSON(map[string]any{
		"status":     "pass",
		"sdk":        args.sdk,
		"server_sdk": args.serverSDK,
		"latency_ms": time.Since(startedAt).Milliseconds(),
		"method":     args.method,
	})
}

func runConnectOnly(client *holonrpc.Client, args options, startedAt time.Time, timeout time.Duration) {
	probeCtx, probeCancel := context.WithTimeout(context.Background(), timeout)
	defer probeCancel()

	if err := client.ConnectWithReconnect(probeCtx, args.url); err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = client.Close()
	}()

	deadline, hasDeadline := probeCtx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(timeout)
	}

	connected := client.Connected()
	connectedAtLeastOnce := connected
	sawDisconnect := false
	sawReconnect := false

	for time.Now().Before(deadline) {
		invokeTimeout := connectOnlyHeartbeatTimeout
		if remaining := time.Until(deadline); remaining < invokeTimeout {
			invokeTimeout = remaining
		}
		if invokeTimeout <= 0 {
			break
		}

		invokeCtx, invokeCancel := context.WithTimeout(probeCtx, invokeTimeout)
		_, err := client.Invoke(invokeCtx, "rpc.heartbeat", map[string]any{})
		invokeCancel()
		if err != nil {
			if code, ok := rpcErrorCode(err); ok && code == 14 {
				sawDisconnect = true
			}
		} else {
			connectedAtLeastOnce = true
		}

		connectedNow := client.Connected()
		if connected && !connectedNow {
			sawDisconnect = true
		}
		if !connected && connectedNow {
			connectedAtLeastOnce = true
			if sawDisconnect {
				sawReconnect = true
			}
		}
		if sawDisconnect && connectedNow {
			sawReconnect = true
		}
		connected = connectedNow

		sleepFor := connectOnlyHeartbeatInterval
		if remaining := time.Until(deadline); remaining < sleepFor {
			sleepFor = remaining
		}
		if sleepFor <= 0 {
			break
		}

		timer := time.NewTimer(sleepFor)
		select {
		case <-probeCtx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
		if probeCtx.Err() != nil {
			break
		}
	}

	if !connectedAtLeastOnce {
		fmt.Fprintln(os.Stderr, "connect-only probe did not establish a healthy connection")
		os.Exit(1)
	}

	mustWriteJSON(map[string]any{
		"status":           "pass",
		"sdk":              args.sdk,
		"server_sdk":       args.serverSDK,
		"latency_ms":       time.Since(startedAt).Milliseconds(),
		"check":            "connect",
		"saw_disconnect":   sawDisconnect,
		"recovered":        !sawDisconnect || sawReconnect,
		"reconnect_active": true,
	})
}

func runBidirectional(client *holonrpc.Client, args options, startedAt time.Time, timeout time.Duration) {
	echoMessage := fmt.Sprint(args.params["message"])
	if strings.TrimSpace(echoMessage) == "" || echoMessage == "<nil>" {
		echoMessage = "interop-bidi"
	}

	echoCtx, echoCancel := context.WithTimeout(context.Background(), timeout)
	echoOut, err := client.Invoke(echoCtx, "echo.v1.Echo/Ping", map[string]any{"message": echoMessage})
	echoCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bidi echo invoke failed: %v\n", err)
		os.Exit(1)
	}
	if got := fmt.Sprint(echoOut["message"]); got != echoMessage {
		fmt.Fprintf(os.Stderr, "bidi echo mismatch: got=%q want=%q\n", got, echoMessage)
		os.Exit(1)
	}

	callbackCtx, callbackCancel := context.WithTimeout(context.Background(), timeout)
	callbackOut, err := client.Invoke(callbackCtx, "echo.v1.Echo/CallClient", map[string]any{"name": "from-go"})
	callbackCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bidi callback invoke failed: %v\n", err)
		os.Exit(1)
	}

	callbackMessage := fmt.Sprint(callbackOut["message"])
	if callbackMessage != "pong:from-go" {
		fmt.Fprintf(os.Stderr, "bidi callback mismatch: got=%q want=%q\n", callbackMessage, "pong:from-go")
		os.Exit(1)
	}

	mustWriteJSON(map[string]any{
		"status":           "pass",
		"sdk":              args.sdk,
		"server_sdk":       args.serverSDK,
		"latency_ms":       time.Since(startedAt).Milliseconds(),
		"check":            "bidi",
		"echo_message":     echoMessage,
		"callback_message": callbackMessage,
	})
}

func parseFlags() (options, error) {
	var out options
	var paramsJSON string
	var expectError string

	out.sdk = defaultSDK
	out.serverSDK = defaultServerSDK
	out.method = defaultMethod
	out.timeoutMS = defaultTimeoutMS

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		token := args[i]

		switch token {
		case "--connect-only":
			out.connectOnly = true
			continue
		case "--bidirectional":
			out.bidirectional = true
			continue
		case "--sdk":
			value, err := requireValue(args, i, "--sdk")
			if err != nil {
				return options{}, err
			}
			out.sdk = value
			i++
			continue
		case "--server-sdk":
			value, err := requireValue(args, i, "--server-sdk")
			if err != nil {
				return options{}, err
			}
			out.serverSDK = value
			i++
			continue
		case "--method":
			value, err := requireValue(args, i, "--method")
			if err != nil {
				return options{}, err
			}
			out.method = value
			i++
			continue
		case "--message":
			value, err := requireValue(args, i, "--message")
			if err != nil {
				return options{}, err
			}
			out.params = map[string]any{"message": value}
			i++
			continue
		case "--params-json":
			value, err := requireValue(args, i, "--params-json")
			if err != nil {
				return options{}, err
			}
			paramsJSON = value
			i++
			continue
		case "--expect-error":
			value, err := requireValue(args, i, "--expect-error")
			if err != nil {
				return options{}, err
			}
			expectError = value
			i++
			continue
		case "--timeout-ms":
			value, err := requireValue(args, i, "--timeout-ms")
			if err != nil {
				return options{}, err
			}
			timeout, err := strconv.Atoi(value)
			if err != nil || timeout <= 0 {
				return options{}, fmt.Errorf("--timeout-ms must be a positive integer")
			}
			out.timeoutMS = timeout
			i++
			continue
		}

		if strings.HasPrefix(token, "--") {
			return options{}, fmt.Errorf("unknown flag: %s", token)
		}
		if out.url != "" {
			return options{}, fmt.Errorf("unexpected argument: %s", token)
		}
		out.url = token
	}

	if out.url == "" {
		return options{}, fmt.Errorf("usage: go_holonrpc_client.go <ws://host:port/rpc> [flags]")
	}

	if out.params == nil {
		out.params = map[string]any{}
	}

	params, err := parseParams(paramsJSON, out.method, fmt.Sprint(out.params["message"]))
	if err != nil {
		return options{}, err
	}

	codes, err := parseExpectedCodes(expectError)
	if err != nil {
		return options{}, err
	}

	out.params = params
	out.expectedErrorIDs = codes
	return out, nil
}

func parseParams(raw, method, message string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		if method == defaultMethod {
			if message == "" || message == "<nil>" {
				message = defaultMessage
			}
			return map[string]any{"message": message}, nil
		}
		return map[string]any{}, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("--params-json must be valid JSON object: %w", err)
	}
	if parsed == nil {
		return nil, fmt.Errorf("--params-json must decode to a JSON object")
	}
	return parsed, nil
}

func parseExpectedCodes(raw string) ([]int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	tokens := strings.Split(trimmed, ",")
	codes := make([]int, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		code, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid --expect-error code: %s", token)
		}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("--expect-error requires at least one numeric code")
	}
	return codes, nil
}

func requireValue(args []string, index int, flagName string) (string, error) {
	if index+1 >= len(args) {
		return "", fmt.Errorf("missing value for %s", flagName)
	}
	return args[index+1], nil
}

func containsInt(list []int, value int) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func rpcErrorCode(err error) (int, bool) {
	var responseErr *holonrpc.ResponseError
	if errors.As(err, &responseErr) {
		return responseErr.Code, true
	}
	return 0, false
}

func mustWriteJSON(payload map[string]any) {
	if err := json.NewEncoder(os.Stdout).Encode(payload); err != nil {
		fmt.Fprintf(os.Stderr, "encode failed: %v\n", err)
		os.Exit(1)
	}
}
