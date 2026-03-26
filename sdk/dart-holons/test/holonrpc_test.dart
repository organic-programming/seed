import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  group('holon-rpc', () {
    test('echo roundtrip against Go helper', () async {
      if (!await canBindLoopbackTCP()) {
        return;
      }

      await withGoHolonRPCServer('echo', (url) async {
        final client = HolonRPCClient(
          heartbeatIntervalMs: 250,
          heartbeatTimeoutMs: 250,
          reconnectMinDelayMs: 100,
          reconnectMaxDelayMs: 400,
        );

        await client.connect(url);
        final out = await client.invoke(
          'echo.v1.Echo/Ping',
          params: <String, dynamic>{'message': 'hello'},
        );
        expect(out['message'], equals('hello'));
        await client.close();
      });
    });

    test('register handles server-initiated calls', () async {
      if (!await canBindLoopbackTCP()) {
        return;
      }

      await withGoHolonRPCServer('echo', (url) async {
        final client = HolonRPCClient(
          heartbeatIntervalMs: 250,
          heartbeatTimeoutMs: 250,
          reconnectMinDelayMs: 100,
          reconnectMaxDelayMs: 400,
        );

        client.register('client.v1.Client/Hello', (params) async {
          final name = params['name']?.toString() ?? '';
          return <String, dynamic>{'message': 'hello $name'};
        });

        await client.connect(url);
        final out = await client.invoke('echo.v1.Echo/CallClient');
        expect(out['message'], equals('hello go'));
        await client.close();
      });
    });

    test('reconnect and heartbeat after server drop', () async {
      if (!await canBindLoopbackTCP()) {
        return;
      }

      await withGoHolonRPCServer('drop-once', (url) async {
        final client = HolonRPCClient(
          heartbeatIntervalMs: 200,
          heartbeatTimeoutMs: 200,
          reconnectMinDelayMs: 100,
          reconnectMaxDelayMs: 400,
        );

        await client.connect(url);
        final first = await client.invoke(
          'echo.v1.Echo/Ping',
          params: <String, dynamic>{'message': 'first'},
        );
        expect(first['message'], equals('first'));

        await Future<void>.delayed(const Duration(milliseconds: 700));

        final second = await invokeEventually(
          client,
          'echo.v1.Echo/Ping',
          const <String, dynamic>{'message': 'second'},
        );
        expect(second['message'], equals('second'));

        final hb = await invokeEventually(
          client,
          'echo.v1.Echo/HeartbeatCount',
          const <String, dynamic>{},
        );
        final count = hb['count'] is num ? (hb['count'] as num).toInt() : 0;
        expect(count, greaterThanOrEqualTo(1));

        await client.close();
      });
    });
  });
}

Future<Map<String, dynamic>> invokeEventually(
  HolonRPCClient client,
  String method,
  Map<String, dynamic> params,
) async {
  Object? lastError;
  for (var i = 0; i < 40; i++) {
    try {
      return await client.invoke(method, params: params);
    } catch (error) {
      lastError = error;
      await Future<void>.delayed(const Duration(milliseconds: 120));
    }
  }

  throw lastError ?? StateError('invoke eventually failed');
}

Future<void> withGoHolonRPCServer(
  String mode,
  Future<void> Function(String url) body,
) async {
  final sdkDir = Directory.current.parent;
  final goHolonsDir = Directory('${sdkDir.path}/go-holons');
  final helperFile = File(
    '${goHolonsDir.path}/tmp-holonrpc-${DateTime.now().microsecondsSinceEpoch}.go',
  );
  await helperFile.writeAsString(_goHolonRPCServerSource);

  final stderrBuffer = StringBuffer();
  final process = await Process.start(
    resolveGoBinary(),
    <String>['run', helperFile.path, mode],
    workingDirectory: goHolonsDir.path,
    runInShell: false,
    environment: _withGoCache(),
  );

  final stderrSubscription =
      process.stderr.transform(utf8.decoder).listen(stderrBuffer.write);

  final stdoutLines =
      process.stdout.transform(utf8.decoder).transform(const LineSplitter());

  try {
    String url;
    try {
      url = await stdoutLines.first.timeout(const Duration(seconds: 20));
    } on TimeoutException {
      final details = stderrBuffer.toString();
      if (_isBindDenied(details) || _isGoCacheDenied(details)) {
        return;
      }
      throw StateError('Go holon-rpc helper timed out: $details');
    } on StateError {
      final details = stderrBuffer.toString();
      if (_isBindDenied(details) || _isGoCacheDenied(details)) {
        return;
      }
      throw StateError('Go holon-rpc helper exited without URL: $details');
    }

    await body(url);
  } finally {
    process.kill(ProcessSignal.sigterm);
    try {
      await process.exitCode.timeout(const Duration(seconds: 5));
    } on TimeoutException {
      process.kill(ProcessSignal.sigkill);
      await process.exitCode.timeout(const Duration(seconds: 5));
    }
    await stderrSubscription.cancel();
    if (await helperFile.exists()) {
      await helperFile.delete();
    }
  }
}

String resolveGoBinary() {
  final preferred = File('/Users/bpds/go/go1.25.1/bin/go');
  if (preferred.existsSync()) {
    return preferred.path;
  }
  return 'go';
}

Future<bool> canBindLoopbackTCP() async {
  try {
    final probe = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    await probe.close();
    return true;
  } on SocketException catch (error) {
    if (_isBindDenied(error)) {
      return false;
    }
    rethrow;
  }
}

Map<String, String> _withGoCache() {
  final environment = Map<String, String>.from(Platform.environment);
  if ((environment['GOCACHE'] ?? '').trim().isEmpty) {
    environment['GOCACHE'] = '/tmp/go-cache';
  }
  return environment;
}

bool _isBindDenied(Object value) {
  final text = value.toString().toLowerCase();
  return text.contains('operation not permitted') ||
      text.contains('permission denied') ||
      text.contains('errno = 1');
}

bool _isGoCacheDenied(String value) {
  final text = value.toLowerCase();
  return text.contains('failed to trim cache') &&
      text.contains('operation not permitted');
}

const String _goHolonRPCServerSource = r'''
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
					time.Sleep(100 * time.Millisecond)
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
''';
