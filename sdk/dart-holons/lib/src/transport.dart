import 'dart:collection';
import 'dart:async';
import 'dart:io';

/// Default transport URI when --listen is omitted.
const defaultUri = 'tcp://:9090';

class ParsedUri {
  const ParsedUri({
    required this.raw,
    required this.scheme,
    this.host,
    this.port,
    this.path,
    this.secure = false,
  });

  final String raw;
  final String scheme;
  final String? host;
  final int? port;
  final String? path;
  final bool secure;
}

sealed class TransportListener {
  const TransportListener();
}

class TcpTransportListener extends TransportListener {
  const TcpTransportListener(this.socket);
  final ServerSocket socket;
}

class UnixTransportListener extends TransportListener {
  const UnixTransportListener(this.socket, this.path);
  final ServerSocket socket;
  final String path;
}

class StdioTransportListener extends TransportListener {
  const StdioTransportListener({this.address = 'stdio://'});
  final String address;
}

class WsTransportListener extends TransportListener {
  const WsTransportListener({
    required this.host,
    required this.port,
    required this.path,
    required this.secure,
  });

  final String host;
  final int port;
  final String path;
  final bool secure;
}

abstract class RuntimeConnection {
  Future<List<int>> read({int maxBytes = 65536});
  Future<void> write(List<int> data);
  Future<void> close();
}

abstract class RuntimeTransportListener {
  String get boundUri;
  Future<RuntimeConnection> accept();
  Future<void> close();
}

class SocketRuntimeConnection implements RuntimeConnection {
  SocketRuntimeConnection(this._socket) : _iterator = StreamIterator<List<int>>(_socket);

  final Socket _socket;
  final StreamIterator<List<int>> _iterator;
  bool _closed = false;

  @override
  Future<List<int>> read({int maxBytes = 65536}) async {
    if (_closed) {
      throw StateError('connection is closed');
    }
    if (maxBytes <= 0) {
      return const <int>[];
    }

    final hasChunk = await _iterator.moveNext();
    if (!hasChunk) {
      return const <int>[];
    }

    final chunk = _iterator.current;
    if (chunk.length <= maxBytes) {
      return List<int>.from(chunk);
    }
    return List<int>.from(chunk.take(maxBytes));
  }

  @override
  Future<void> write(List<int> data) async {
    if (_closed) {
      throw StateError('connection is closed');
    }
    if (data.isEmpty) {
      return;
    }

    _socket.add(data);
    await _socket.flush();
  }

  @override
  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;

    try {
      await _iterator.cancel();
    } catch (_) {}
    _socket.destroy();
  }
}

class StdioRuntimeConnection implements RuntimeConnection {
  StdioRuntimeConnection() : _iterator = StreamIterator<List<int>>(stdin);

  final StreamIterator<List<int>> _iterator;
  bool _closed = false;

  @override
  Future<List<int>> read({int maxBytes = 65536}) async {
    if (_closed) {
      throw StateError('connection is closed');
    }
    if (maxBytes <= 0) {
      return const <int>[];
    }

    final hasChunk = await _iterator.moveNext();
    if (!hasChunk) {
      return const <int>[];
    }

    final chunk = _iterator.current;
    if (chunk.length <= maxBytes) {
      return List<int>.from(chunk);
    }
    return List<int>.from(chunk.take(maxBytes));
  }

  @override
  Future<void> write(List<int> data) async {
    if (_closed) {
      throw StateError('connection is closed');
    }
    if (data.isEmpty) {
      return;
    }

    stdout.add(data);
    await stdout.flush();
  }

  @override
  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;
    try {
      await _iterator.cancel();
    } catch (_) {}
  }
}

class TcpRuntimeListener implements RuntimeTransportListener {
  TcpRuntimeListener(this.socket)
      : _subscription = socket.listen(
          (_) {},
        ) {
    _subscription
      ..onData(_onSocket)
      ..onDone(_onDone)
      ..onError(_onError);
  }

  final ServerSocket socket;
  final Queue<Socket> _pending = Queue<Socket>();
  final Queue<Completer<Socket>> _waiters = Queue<Completer<Socket>>();
  late final StreamSubscription<Socket> _subscription;
  bool _closed = false;

  @override
  String get boundUri {
    final host = _formatHostForUri(socket.address.address);
    return 'tcp://$host:${socket.port}';
  }

  @override
  Future<RuntimeConnection> accept() async {
    if (_pending.isNotEmpty) {
      return SocketRuntimeConnection(_pending.removeFirst());
    }
    if (_closed) {
      throw StateError('listener closed: $boundUri');
    }

    final waiter = Completer<Socket>();
    _waiters.add(waiter);
    final socket = await waiter.future;
    return SocketRuntimeConnection(socket);
  }

  @override
  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;

    while (_pending.isNotEmpty) {
      _pending.removeFirst().destroy();
    }

    await _subscription.cancel();
    await socket.close();
    _failWaiters(StateError('listener closed: $boundUri'));
  }

  void _onSocket(Socket socket) {
    if (_closed) {
      socket.destroy();
      return;
    }

    if (_waiters.isNotEmpty) {
      _waiters.removeFirst().complete(socket);
      return;
    }

    _pending.add(socket);
  }

  void _onDone() {
    _closed = true;
    _failWaiters(StateError('listener closed: $boundUri'));
  }

  void _onError(Object error, StackTrace stackTrace) {
    if (_waiters.isNotEmpty) {
      _waiters.removeFirst().completeError(error, stackTrace);
    }
  }

  void _failWaiters(Object error) {
    while (_waiters.isNotEmpty) {
      final waiter = _waiters.removeFirst();
      if (!waiter.isCompleted) {
        waiter.completeError(error);
      }
    }
  }
}

class UnixRuntimeListener implements RuntimeTransportListener {
  UnixRuntimeListener(this.socket, this.path)
      : _subscription = socket.listen(
          (_) {},
        ) {
    _subscription
      ..onData(_onSocket)
      ..onDone(_onDone)
      ..onError(_onError);
  }

  final ServerSocket socket;
  final String path;
  final Queue<Socket> _pending = Queue<Socket>();
  final Queue<Completer<Socket>> _waiters = Queue<Completer<Socket>>();
  late final StreamSubscription<Socket> _subscription;
  bool _closed = false;

  @override
  String get boundUri => 'unix://$path';

  @override
  Future<RuntimeConnection> accept() async {
    if (_pending.isNotEmpty) {
      return SocketRuntimeConnection(_pending.removeFirst());
    }
    if (_closed) {
      throw StateError('listener closed: $boundUri');
    }

    final waiter = Completer<Socket>();
    _waiters.add(waiter);
    final socket = await waiter.future;
    return SocketRuntimeConnection(socket);
  }

  @override
  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;

    while (_pending.isNotEmpty) {
      _pending.removeFirst().destroy();
    }

    await _subscription.cancel();
    await socket.close();

    try {
      final stale = File(path);
      if (stale.existsSync()) {
        stale.deleteSync();
      }
    } catch (_) {}

    _failWaiters(StateError('listener closed: $boundUri'));
  }

  void _onSocket(Socket socket) {
    if (_closed) {
      socket.destroy();
      return;
    }

    if (_waiters.isNotEmpty) {
      _waiters.removeFirst().complete(socket);
      return;
    }

    _pending.add(socket);
  }

  void _onDone() {
    _closed = true;
    _failWaiters(StateError('listener closed: $boundUri'));
  }

  void _onError(Object error, StackTrace stackTrace) {
    if (_waiters.isNotEmpty) {
      _waiters.removeFirst().completeError(error, stackTrace);
    }
  }

  void _failWaiters(Object error) {
    while (_waiters.isNotEmpty) {
      final waiter = _waiters.removeFirst();
      if (!waiter.isCompleted) {
        waiter.completeError(error);
      }
    }
  }
}

class StdioRuntimeListener implements RuntimeTransportListener {
  bool _closed = false;
  bool _consumed = false;

  @override
  String get boundUri => 'stdio://';

  @override
  Future<RuntimeConnection> accept() async {
    if (_closed) {
      throw StateError('listener closed: $boundUri');
    }
    if (_consumed) {
      throw StateError('stdio:// accepts exactly one connection');
    }

    _consumed = true;
    return StdioRuntimeConnection();
  }

  @override
  Future<void> close() async {
    _closed = true;
  }
}

/// Extract the scheme from a transport URI.
String scheme(String uri) {
  final idx = uri.indexOf('://');
  return idx >= 0 ? uri.substring(0, idx) : uri;
}

/// Parse a transport URI into a normalized structure.
ParsedUri parseUri(String uri) {
  final s = scheme(uri);
  switch (s) {
    case 'tcp':
      if (!uri.startsWith('tcp://')) {
        throw ArgumentError('invalid tcp URI: $uri');
      }
      final (host, port) = _splitHostPort(uri.substring(6), 9090);
      return ParsedUri(raw: uri, scheme: 'tcp', host: host, port: port);
    case 'unix':
      if (!uri.startsWith('unix://')) {
        throw ArgumentError('invalid unix URI: $uri');
      }
      final path = uri.substring(7);
      if (path.isEmpty) {
        throw ArgumentError('invalid unix URI: $uri');
      }
      return ParsedUri(raw: uri, scheme: 'unix', path: path);
    case 'stdio':
      return const ParsedUri(raw: 'stdio://', scheme: 'stdio');
    case 'ws':
    case 'wss':
      final secure = s == 'wss';
      final prefix = secure ? 'wss://' : 'ws://';
      if (!uri.startsWith(prefix)) {
        throw ArgumentError('invalid ws URI: $uri');
      }
      final trimmed = uri.substring(prefix.length);
      final slash = trimmed.indexOf('/');
      final addr = slash >= 0 ? trimmed.substring(0, slash) : trimmed;
      final path = slash >= 0 ? trimmed.substring(slash) : '/grpc';
      final (host, port) = _splitHostPort(addr, secure ? 443 : 80);
      return ParsedUri(
        raw: uri,
        scheme: s,
        host: host,
        port: port,
        path: path.isEmpty ? '/grpc' : path,
        secure: secure,
      );
    default:
      throw ArgumentError('unsupported transport URI: $uri');
  }
}

/// Parse a transport URI and create a listener variant.
Future<TransportListener> listen(String uri) async {
  final parsed = parseUri(uri);
  switch (parsed.scheme) {
    case 'tcp':
      return TcpTransportListener(await _listenTcp(parsed));
    case 'unix':
      final path = parsed.path ?? '';
      return UnixTransportListener(await _listenUnix(path), path);
    case 'stdio':
      return const StdioTransportListener();
    case 'ws':
    case 'wss':
      return WsTransportListener(
        host: parsed.host ?? '0.0.0.0',
        port: parsed.port ?? (parsed.secure ? 443 : 80),
        path: parsed.path ?? '/grpc',
        secure: parsed.secure,
      );
    default:
      throw ArgumentError('unsupported transport URI: $uri');
  }
}

/// Parse a transport URI and return a native runtime listener.
Future<RuntimeTransportListener> listenRuntime(String uri) async {
  final parsed = parseUri(uri);
  switch (parsed.scheme) {
    case 'tcp':
      return TcpRuntimeListener(await _listenTcp(parsed));
    case 'unix':
      final path = parsed.path ?? '';
      return UnixRuntimeListener(await _listenUnix(path), path);
    case 'stdio':
      return StdioRuntimeListener();
    case 'ws':
    case 'wss':
      throw UnsupportedError(
        'runtime ws/wss listeners are unavailable: grpc-dart has no official WebSocket server transport for HTTP/2 gRPC framing',
      );
    default:
      throw ArgumentError('unsupported transport URI: $uri');
  }
}

Future<ServerSocket> _listenTcp(ParsedUri parsed) async {
  final host = parsed.host ?? '0.0.0.0';
  final port = parsed.port ?? 9090;
  return ServerSocket.bind(host, port);
}

Future<ServerSocket> _listenUnix(String path) async {
  // Clean stale socket.
  try {
    final stale = File(path);
    if (stale.existsSync()) {
      stale.deleteSync();
    }
  } catch (_) {}

  return ServerSocket.bind(InternetAddress(path, type: InternetAddressType.unix), 0);
}

(String, int) _splitHostPort(String addr, int defaultPort) {
  if (addr.isEmpty) {
    return ('0.0.0.0', defaultPort);
  }

  if (addr.startsWith('[')) {
    final endBracket = addr.indexOf(']');
    if (endBracket < 0) {
      throw ArgumentError('invalid host in URI: $addr');
    }

    final host = addr.substring(1, endBracket);
    final rest = addr.substring(endBracket + 1);
    if (rest.isEmpty) {
      return (host.isEmpty ? '::' : host, defaultPort);
    }
    if (!rest.startsWith(':')) {
      throw ArgumentError('invalid host/port in URI: $addr');
    }

    final portText = rest.substring(1);
    final port = portText.isEmpty ? defaultPort : int.parse(portText);
    return (host.isEmpty ? '::' : host, port);
  }

  final firstColon = addr.indexOf(':');
  final lastColon = addr.lastIndexOf(':');
  if (firstColon != lastColon) {
    // Likely an IPv6 literal without brackets.
    return (addr, defaultPort);
  }

  if (lastColon < 0) {
    return (addr, defaultPort);
  }

  final host = lastColon > 0 ? addr.substring(0, lastColon) : '0.0.0.0';
  final portText = addr.substring(lastColon + 1);
  final port = portText.isEmpty ? defaultPort : int.parse(portText);
  return (host, port);
}

String _formatHostForUri(String host) {
  if (host.contains(':') && !host.startsWith('[')) {
    return '[$host]';
  }
  return host;
}
