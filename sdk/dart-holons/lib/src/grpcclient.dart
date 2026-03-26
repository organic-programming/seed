import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:http2/transport.dart';

/// gRPC transport connector backed by a child process's stdin/stdout pipes.
///
/// Mirrors go-holons/pkg/grpcclient.DialStdio.
/// The child process must speak gRPC (HTTP/2) on its stdin/stdout — this is
/// the standard behavior of any holon started with
/// `serve --listen stdio://`.
///
/// Unlike TCP sockets, process pipes are single-use: stdout is a
/// single-subscription stream. The connector caches the transport
/// connection and returns the same instance on every call to [connect],
/// matching Go's `sync.Once` pattern in `pipeConn`.
class StdioTransportConnector implements ClientTransportConnector {
  StdioTransportConnector.fromProcess(this._process) {
    _process.exitCode.then((_) {
      if (!_done.isCompleted) {
        _done.complete();
      }
    });
  }

  final Process _process;
  final Completer<void> _done = Completer<void>();
  ClientTransportConnection? _cachedConnection;

  /// Spawn [binaryPath] with the given [args] and return a connector.
  ///
  /// Default args: `['serve', '--listen', 'stdio://']`.
  static Future<StdioTransportConnector> spawn(
    String binaryPath, {
    List<String> args = const <String>['serve', '--listen', 'stdio://'],
  }) async {
    final process = await Process.start(binaryPath, args);
    return StdioTransportConnector.fromProcess(process);
  }

  @override
  Future<ClientTransportConnection> connect() async {
    // Process pipes are single-subscription: stdout can only be listened
    // to once. Cache the connection like Go's sync.Once dialer.
    if (_cachedConnection != null) {
      return _cachedConnection!;
    }
    // process.stdout = server → client (incoming)
    // process.stdin  = client → server (outgoing)
    _cachedConnection = ClientTransportConnection.viaStreams(
      _process.stdout,
      _process.stdin,
    );
    return _cachedConnection!;
  }

  @override
  Future<void> get done => _done.future;

  @override
  void shutdown() {
    _process.kill(ProcessSignal.sigterm);
  }

  @override
  String get authority => 'localhost';

  /// The underlying process, for lifecycle management by the caller.
  Process get process => _process;
}

/// Spawn a holon binary and return a gRPC channel backed by its stdio pipes.
///
/// Returns both the channel and the process so the caller can manage
/// the child process lifecycle (kill on shutdown, etc.).
///
/// Idle timeout is disabled by default because stdio pipes are
/// single-connection transports that cannot be re-established.
Future<(ClientTransportConnectorChannel, Process)> dialStdio(
  String binaryPath, {
  List<String>? args,
  ChannelOptions options = const ChannelOptions(
    credentials: ChannelCredentials.insecure(),
    idleTimeout: null,
  ),
}) async {
  final connector = await StdioTransportConnector.spawn(
    binaryPath,
    args: args ?? const <String>['serve', '--listen', 'stdio://'],
  );
  unawaited(connector.process.stderr.drain<void>());
  final channel = ClientTransportConnectorChannel(connector, options: options);
  return (channel, connector.process);
}
