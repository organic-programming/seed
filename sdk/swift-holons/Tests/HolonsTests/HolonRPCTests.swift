import Foundation
import XCTest
@testable import Holons

final class HolonRPCTests: XCTestCase {
    func testHolonRPCGoEchoRoundTrip() async throws {
        try await withGoHolonRPCServer(mode: "echo") { url in
            let client = HolonRPCClient(
                heartbeatInterval: 0.25,
                heartbeatTimeout: 0.25,
                reconnectMinDelay: 0.1,
                reconnectMaxDelay: 0.4
            )

            try await client.connect(url.absoluteString)
            let out = try await client.invoke(method: "echo.v1.Echo/Ping", params: ["message": "hello"])
            XCTAssertEqual(out["message"] as? String, "hello")
            await client.close()
        }
    }

    func testHolonRPCRegisterHandlesServerCalls() async throws {
        try await withGoHolonRPCServer(mode: "echo") { url in
            let client = HolonRPCClient(
                heartbeatInterval: 0.25,
                heartbeatTimeout: 0.25,
                reconnectMinDelay: 0.1,
                reconnectMaxDelay: 0.4
            )

            await client.register(method: "client.v1.Client/Hello") { params in
                ["message": "hello \(params["name"] as? String ?? "")"]
            }

            try await client.connect(url.absoluteString)
            let out = try await client.invoke(method: "echo.v1.Echo/CallClient", params: [:])
            XCTAssertEqual(out["message"] as? String, "hello go")
            await client.close()
        }
    }

    func testHolonRPCReconnectAndHeartbeat() async throws {
        try await withGoHolonRPCServer(mode: "drop-once") { url in
            let client = HolonRPCClient(
                heartbeatInterval: 0.2,
                heartbeatTimeout: 0.2,
                reconnectMinDelay: 0.1,
                reconnectMaxDelay: 0.4
            )

            try await client.connect(url.absoluteString)

            let first = try await client.invoke(method: "echo.v1.Echo/Ping", params: ["message": "first"])
            XCTAssertEqual(first["message"] as? String, "first")

            try await Task.sleep(nanoseconds: 700_000_000)

            let second = try await invokeEventually(client: client, method: "echo.v1.Echo/Ping", params: ["message": "second"])
            XCTAssertEqual(second["message"] as? String, "second")

            let hb = try await invokeEventually(client: client, method: "echo.v1.Echo/HeartbeatCount", params: [:])
            let hbCount = (hb["count"] as? NSNumber)?.intValue ?? 0
            XCTAssertGreaterThanOrEqual(hbCount, 1)

            await client.close()
        }
    }
}

private func invokeEventually(
    client: HolonRPCClient,
    method: String,
    params: [String: Any],
    timeout: TimeInterval = 5.0
) async throws -> [String: Any] {
    let deadline = Date().addingTimeInterval(timeout)
    var lastError: Error?

    while Date() < deadline {
        do {
            return try await client.invoke(method: method, params: params)
        } catch {
            lastError = error
            try await Task.sleep(nanoseconds: 120_000_000)
        }
    }

    throw lastError ?? HolonRPCClientError.timeout
}

private let goHolonRPCServerSource = #"""
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

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
	mode := "echo"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	var heartbeatCount int64
	var dropped int32

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
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
			if msg.Method == "" {
				continue
			}

			switch msg.Method {
			case "rpc.heartbeat":
				atomic.AddInt64(&heartbeatCount, 1)
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{})
			case "echo.v1.Echo/Ping":
				var params map[string]interface{}
				_ = json.Unmarshal(msg.Params, &params)
				if params == nil {
					params = map[string]interface{}{}
				}
				_ = writeResult(ctx, c, msg.ID, params)
				if mode == "drop-once" && atomic.CompareAndSwapInt32(&dropped, 0, 1) {
					_ = c.Close(websocket.StatusNormalClosure, "drop once")
					return
				}
			case "echo.v1.Echo/HeartbeatCount":
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{"count": atomic.LoadInt64(&heartbeatCount)})
			case "echo.v1.Echo/CallClient":
				callID := "s1"
				if err := writeRequest(ctx, c, callID, "client.v1.Client/Hello", map[string]interface{}{"name": "go"}); err != nil {
					_ = writeError(ctx, c, msg.ID, 13, err.Error())
					continue
				}

				innerResult, callErr := waitForResponse(ctx, c, callID)
				if callErr != nil {
					_ = writeError(ctx, c, msg.ID, 13, callErr.Error())
					continue
				}
				_ = writeResult(ctx, c, msg.ID, innerResult)
			default:
				_ = writeError(ctx, c, msg.ID, -32601, fmt.Sprintf("method %q not found", msg.Method))
			}
		}
	})

	srv := &http.Server{Handler: h}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	fmt.Printf("ws://%s/rpc\n", ln.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func writeRequest(ctx context.Context, c *websocket.Conn, id interface{}, method string, params map[string]interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  mustRaw(params),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
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

func waitForResponse(ctx context.Context, c *websocket.Conn, expectedID string) (map[string]interface{}, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		_, data, err := c.Read(deadlineCtx)
		if err != nil {
			return nil, err
		}

		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		id, _ := msg.ID.(string)
		if id != expectedID {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("client error: %d %s", msg.Error.Code, msg.Error.Message)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(msg.Result, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func mustRaw(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
"""#

private struct GoHolonRPCServer {
    let process: Process
    let helperPath: URL
    let url: URL

    func stop() {
        if process.isRunning {
            process.terminate()
            process.waitUntilExit()
        }
        try? FileManager.default.removeItem(at: helperPath)
    }
}

private func withGoHolonRPCServer(mode: String, _ body: (URL) async throws -> Void) async throws {
    do {
        let server = try startGoHolonRPCServer(mode: mode)
        defer { server.stop() }
        try await body(server.url)
    } catch let error as GoHolonRPCHelperError {
        throw XCTSkip(error.description)
    }
}

private func startGoHolonRPCServer(mode: String) throws -> GoHolonRPCServer {
    let thisFile = URL(fileURLWithPath: #filePath)
    let swiftRepo = thisFile
        .deletingLastPathComponent() // HolonsTests
        .deletingLastPathComponent() // Tests
        .deletingLastPathComponent() // swift-holons
    let sdkDir = swiftRepo.deletingLastPathComponent()
    let goRepo = sdkDir.appendingPathComponent("go-holons")

    let helperPath = goRepo.appendingPathComponent("tmp-holonrpc-\(UUID().uuidString).go")
    try goHolonRPCServerSource.write(to: helperPath, atomically: true, encoding: .utf8)

    let stdout = Pipe()
    let stderr = Pipe()

    let process = Process()
    process.currentDirectoryURL = goRepo
    process.executableURL = URL(fileURLWithPath: resolveGoBinary())
    process.arguments = ["run", helperPath.path, mode]
    process.standardOutput = stdout
    process.standardError = stderr
    do {
        try process.run()
    } catch {
        throw GoHolonRPCHelperError.unavailable("unable to start go helper: \(error)")
    }

    let firstLine: String
    do {
        firstLine = try readFirstLine(from: stdout.fileHandleForReading)
    } catch {
        let stderrText = drainStderr(process: process, stderrHandle: stderr.fileHandleForReading)
        let details = buildHelperFailureDetails(base: String(describing: error), stderrText: stderrText)
        if isInfrastructureFailure(details) {
            throw GoHolonRPCHelperError.unavailable(details)
        }
        throw NSError(
            domain: "HolonRPCTests",
            code: 2,
            userInfo: [NSLocalizedDescriptionKey: details]
        )
    }
    guard let url = URL(string: firstLine) else {
        let stderrText = drainStderr(process: process, stderrHandle: stderr.fileHandleForReading)
        throw NSError(
            domain: "HolonRPCTests",
            code: 1,
            userInfo: [NSLocalizedDescriptionKey: "invalid helper URL: \(firstLine)\n\(stderrText)"]
        )
    }

    return GoHolonRPCServer(process: process, helperPath: helperPath, url: url)
}

private func resolveGoBinary() -> String {
    let preferred = "/Users/bpds/go/go1.25.1/bin/go"
    if FileManager.default.isExecutableFile(atPath: preferred) {
        return preferred
    }
    return "go"
}

private func readFirstLine(from handle: FileHandle) throws -> String {
    var bytes = Data()

    while true {
        guard let chunk = try handle.read(upToCount: 1), !chunk.isEmpty else {
            break
        }
        bytes.append(chunk)
        if chunk.first == 0x0A {
            break
        }
    }

    guard let line = String(data: bytes, encoding: .utf8)?
        .trimmingCharacters(in: .whitespacesAndNewlines), !line.isEmpty else {
        throw NSError(domain: "HolonRPCTests", code: 2, userInfo: [NSLocalizedDescriptionKey: "helper did not output URL"]) }

    return line
}

private enum GoHolonRPCHelperError: Error, CustomStringConvertible {
    case unavailable(String)

    var description: String {
        switch self {
        case let .unavailable(message):
            return "Go Holon-RPC helper unavailable in this environment: \(message)"
        }
    }
}

private func isInfrastructureFailure(_ details: String) -> Bool {
    let lower = details.lowercased()
    return lower.contains("operation not permitted")
        || lower.contains("permission denied")
        || lower.contains("unable to start go helper")
        || lower.contains("no such file or directory")
        || lower.contains("no such host")
        || lower.contains("proxy.golang.org")
        || lower.contains("executable file not found")
        || lower.contains("command not found")
}

private func drainStderr(process: Process, stderrHandle: FileHandle) -> String {
    if process.isRunning {
        process.terminate()
    }
    process.waitUntilExit()

    let data = stderrHandle.readDataToEndOfFile()
    return String(data: data, encoding: .utf8)?
        .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
}

private func buildHelperFailureDetails(base: String, stderrText: String) -> String {
    if stderrText.isEmpty {
        return base
    }
    return "\(base)\n\(stderrText)"
}
