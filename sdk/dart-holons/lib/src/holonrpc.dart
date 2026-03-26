import 'dart:async';
import 'dart:collection';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

typedef HolonRPCHandler = FutureOr<Map<String, dynamic>> Function(
    Map<String, dynamic> params);

const String _jsonRPCVersion = '2.0';
const int _codeParseError = -32700;
const int _codeInvalidRequest = -32600;
const int _codeMethodNotFound = -32601;
const int _codeInvalidParams = -32602;
const int _codeInternalError = -32603;
const int _codeNotFound = 5;
const int _codeUnavailable = 14;

const String _routeModeDefault = '';
const String _routeModeBroadcastResponse = 'broadcast-response';
const String _routeModeFullBroadcast = 'full-broadcast';

class HolonRPCResponseException implements Exception {
  HolonRPCResponseException({
    required this.code,
    required this.message,
    this.data,
  });

  final int code;
  final String message;
  final Object? data;

  @override
  String toString() =>
      'HolonRPCResponseException(code: $code, message: $message)';
}

class HolonRPCClient {
  HolonRPCClient({
    this.heartbeatIntervalMs = 15000,
    this.heartbeatTimeoutMs = 5000,
    this.reconnectMinDelayMs = 500,
    this.reconnectMaxDelayMs = 30000,
    this.reconnectFactor = 2.0,
    this.reconnectJitter = 0.1,
    this.connectTimeoutMs = 10000,
    this.requestTimeoutMs = 10000,
    Random? random,
  }) : _random = random ?? Random();

  final int heartbeatIntervalMs;
  final int heartbeatTimeoutMs;
  final int reconnectMinDelayMs;
  final int reconnectMaxDelayMs;
  final double reconnectFactor;
  final double reconnectJitter;
  final int connectTimeoutMs;
  final int requestTimeoutMs;
  final Random _random;

  final Map<String, HolonRPCHandler> _handlers = <String, HolonRPCHandler>{};
  final Map<String, Completer<Map<String, dynamic>>> _pending =
      <String, Completer<Map<String, dynamic>>>{};

  WebSocket? _socket;
  StreamSubscription<dynamic>? _socketSubscription;
  Timer? _heartbeatTimer;
  Timer? _reconnectTimer;
  Completer<void>? _connectedWaiter;

  String? _url;
  int _nextID = 0;
  int _reconnectAttempt = 0;
  bool _connecting = false;
  bool _closed = false;

  Future<void> connect(String url) async {
    if (url.isEmpty) {
      throw ArgumentError('url is required');
    }

    if (_socket != null && _url == url) {
      return;
    }

    await close();
    _closed = false;
    _url = url;
    _connectedWaiter = Completer<void>();

    await _openSocket(initial: true);
    await _awaitConnected(Duration(milliseconds: connectTimeoutMs));
  }

  void register(String method, HolonRPCHandler handler) {
    if (method.isEmpty) {
      throw ArgumentError('method is required');
    }
    _handlers[method] = handler;
  }

  Future<Map<String, dynamic>> invoke(
    String method, {
    Map<String, dynamic> params = const <String, dynamic>{},
    int? timeoutMs,
  }) async {
    if (method.isEmpty) {
      throw ArgumentError('method is required');
    }

    await _awaitConnected(Duration(milliseconds: connectTimeoutMs));

    final id = 'c${++_nextID}';
    final completer = Completer<Map<String, dynamic>>();
    _pending[id] = completer;

    try {
      await _send(<String, dynamic>{
        'jsonrpc': '2.0',
        'id': id,
        'method': method,
        'params': params,
      });
    } catch (_) {
      _pending.remove(id);
      rethrow;
    }

    final timeout = Duration(milliseconds: timeoutMs ?? requestTimeoutMs);

    try {
      return await completer.future.timeout(timeout);
    } finally {
      _pending.remove(id);
    }
  }

  Future<void> close() async {
    _closed = true;
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;

    final socket = _socket;
    _socket = null;

    final subscription = _socketSubscription;
    _socketSubscription = null;
    await subscription?.cancel();

    if (socket != null) {
      await socket.close(WebSocketStatus.normalClosure, 'client close');
    }

    _failAllPending(StateError('holon-rpc client closed'));
  }

  Future<void> _openSocket({required bool initial}) async {
    if (_connecting || _closed) {
      return;
    }

    final url = _url;
    if (url == null) {
      throw StateError('url is not set');
    }

    _connecting = true;
    try {
      final socket =
          await WebSocket.connect(url, protocols: <String>['holon-rpc']);
      if (socket.protocol != 'holon-rpc') {
        await socket.close(
            WebSocketStatus.protocolError, 'missing holon-rpc subprotocol');
        throw StateError('server did not negotiate holon-rpc subprotocol');
      }

      _socket = socket;
      _reconnectAttempt = 0;
      _connectedWaiter ??= Completer<void>();
      if (!_connectedWaiter!.isCompleted) {
        _connectedWaiter!.complete();
      }

      _socketSubscription = socket.listen(
        _handleIncoming,
        onDone: _handleDisconnect,
        onError: (Object _, StackTrace __) => _handleDisconnect(),
        cancelOnError: true,
      );

      _startHeartbeat();
    } catch (_) {
      if (initial) {
        rethrow;
      }
      _scheduleReconnect();
    } finally {
      _connecting = false;
    }
  }

  void _handleIncoming(dynamic data) {
    final String text;
    if (data is String) {
      text = data;
    } else if (data is List<int>) {
      text = utf8.decode(data);
    } else {
      return;
    }

    late final Object decoded;
    try {
      decoded = jsonDecode(text);
    } catch (_) {
      return;
    }

    if (decoded is! Map<String, dynamic>) {
      return;
    }

    if (decoded['method'] != null) {
      unawaited(_handleRequest(decoded));
      return;
    }

    if (decoded['result'] != null || decoded['error'] != null) {
      _handleResponse(decoded);
    }
  }

  Future<void> _handleRequest(Map<String, dynamic> msg) async {
    final dynamic id = msg['id'];
    final method = msg['method'];
    final jsonrpc = msg['jsonrpc'];

    if (jsonrpc != '2.0' || method is! String || method.isEmpty) {
      if (id != null) {
        await _sendError(id, -32600, 'invalid request');
      }
      return;
    }

    if (method == 'rpc.heartbeat') {
      if (id != null) {
        await _sendResult(id, <String, dynamic>{});
      }
      return;
    }

    if (id != null) {
      if (id is! String || !id.startsWith('s')) {
        await _sendError(id, -32600, "server request id must start with 's'");
        return;
      }
    }

    final handler = _handlers[method];
    if (handler == null) {
      if (id != null) {
        await _sendError(id, -32601, 'method "$method" not found');
      }
      return;
    }

    final dynamic rawParams = msg['params'];
    final params = rawParams is Map<String, dynamic>
        ? rawParams
        : rawParams is Map
            ? rawParams.cast<String, dynamic>()
            : <String, dynamic>{};

    try {
      final result = await handler(params);
      if (id != null) {
        await _sendResult(id, result);
      }
    } on HolonRPCResponseException catch (rpcError) {
      if (id != null) {
        await _sendError(id, rpcError.code, rpcError.message, rpcError.data);
      }
    } catch (error) {
      if (id != null) {
        await _sendError(id, 13, error.toString());
      }
    }
  }

  void _handleResponse(Map<String, dynamic> msg) {
    final rawID = msg['id'];
    final id = rawID is String ? rawID : rawID?.toString();
    if (id == null) {
      return;
    }

    final completer = _pending.remove(id);
    if (completer == null || completer.isCompleted) {
      return;
    }

    final dynamic rawError = msg['error'];
    if (rawError is Map<String, dynamic>) {
      final rawCode = rawError['code'];
      final code = rawCode is int
          ? rawCode
          : (rawCode is num ? rawCode.toInt() : -32603);
      final message = rawError['message']?.toString() ?? 'internal error';
      completer.completeError(
        HolonRPCResponseException(
          code: code,
          message: message,
          data: rawError['data'],
        ),
      );
      return;
    }

    final dynamic rawResult = msg['result'];
    if (rawResult is Map<String, dynamic>) {
      completer.complete(rawResult);
      return;
    }
    if (rawResult is Map) {
      completer.complete(rawResult.cast<String, dynamic>());
      return;
    }

    if (rawResult == null) {
      completer.complete(<String, dynamic>{});
      return;
    }

    completer.complete(<String, dynamic>{'value': rawResult});
  }

  void _startHeartbeat() {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = Timer.periodic(
      Duration(milliseconds: heartbeatIntervalMs),
      (_) async {
        if (_closed || _socket == null) {
          return;
        }

        try {
          await invoke(
            'rpc.heartbeat',
            params: const <String, dynamic>{},
            timeoutMs: heartbeatTimeoutMs,
          );
        } catch (_) {
          await _socket?.close(WebSocketStatus.goingAway, 'heartbeat timeout');
        }
      },
    );
  }

  void _handleDisconnect() {
    _socket = null;
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;

    _connectedWaiter = Completer<void>();
    _failAllPending(StateError('holon-rpc connection closed'));

    if (_closed) {
      return;
    }

    _scheduleReconnect();
  }

  void _scheduleReconnect() {
    if (_closed || _reconnectTimer != null) {
      return;
    }

    final baseDelay = min(
      reconnectMinDelayMs * pow(reconnectFactor, _reconnectAttempt),
      reconnectMaxDelayMs.toDouble(),
    );
    final jitter = baseDelay * reconnectJitter * _random.nextDouble();
    final delayMs = (baseDelay + jitter).round();
    _reconnectAttempt += 1;

    _reconnectTimer = Timer(Duration(milliseconds: delayMs), () async {
      _reconnectTimer = null;
      await _openSocket(initial: false);
    });
  }

  Future<void> _awaitConnected(Duration timeout) async {
    if (_socket != null) {
      return;
    }
    if (_closed) {
      throw StateError('holon-rpc client closed');
    }

    _connectedWaiter ??= Completer<void>();
    await _connectedWaiter!.future.timeout(timeout);
  }

  Future<void> _send(Map<String, dynamic> payload) async {
    final socket = _socket;
    if (socket == null) {
      throw StateError('websocket is not connected');
    }
    socket.add(jsonEncode(payload));
  }

  Future<void> _sendResult(dynamic id, Map<String, dynamic> result) async {
    await _send(<String, dynamic>{
      'jsonrpc': '2.0',
      'id': id,
      'result': result,
    });
  }

  Future<void> _sendError(
    dynamic id,
    int code,
    String message, [
    Object? data,
  ]) async {
    await _send(<String, dynamic>{
      'jsonrpc': '2.0',
      'id': id,
      'error': <String, dynamic>{
        'code': code,
        'message': message,
        if (data != null) 'data': data,
      },
    });
  }

  void _failAllPending(Object error) {
    if (_pending.isEmpty) {
      return;
    }
    final values = _pending.values.toList(growable: false);
    _pending.clear();
    for (final completer in values) {
      if (!completer.isCompleted) {
        completer.completeError(error);
      }
    }
  }
}

class HolonRPCDispatchRoute {
  const HolonRPCDispatchRoute({
    required this.holonName,
    required this.method,
  });

  final String holonName;
  final String method;
}

class HolonRPCRouter {
  final Map<String, String> _peerToName = <String, String>{};
  final Map<String, LinkedHashSet<String>> _nameToPeers =
      <String, LinkedHashSet<String>>{};

  void registerHolon({
    required String peerID,
    required String name,
  }) {
    final trimmedPeerID = peerID.trim();
    final trimmedName = name.trim();
    if (trimmedPeerID.isEmpty) {
      throw ArgumentError('peerID is required');
    }
    if (trimmedName.isEmpty) {
      throw ArgumentError('name is required');
    }

    final previousName = _peerToName[trimmedPeerID];
    if (previousName != null && previousName != trimmedName) {
      final previousSet = _nameToPeers[previousName];
      previousSet?.remove(trimmedPeerID);
      if (previousSet != null && previousSet.isEmpty) {
        _nameToPeers.remove(previousName);
      }
    }

    _peerToName[trimmedPeerID] = trimmedName;
    final peers = _nameToPeers.putIfAbsent(
      trimmedName,
      () => LinkedHashSet<String>(),
    );
    peers.add(trimmedPeerID);
  }

  void deregisterHolon(String peerID) {
    final trimmedPeerID = peerID.trim();
    if (trimmedPeerID.isEmpty) {
      return;
    }

    final name = _peerToName.remove(trimmedPeerID);
    if (name == null) {
      return;
    }

    final peers = _nameToPeers[name];
    peers?.remove(trimmedPeerID);
    if (peers != null && peers.isEmpty) {
      _nameToPeers.remove(name);
    }
  }

  bool hasHolon(String name) {
    final trimmedName = name.trim();
    if (trimmedName.isEmpty) {
      return false;
    }
    final peers = _nameToPeers[trimmedName];
    return peers != null && peers.isNotEmpty;
  }

  String? resolveHolon(String name, {String? excludePeerID}) {
    final trimmedName = name.trim();
    if (trimmedName.isEmpty) {
      return null;
    }

    final peers = _nameToPeers[trimmedName];
    if (peers == null || peers.isEmpty) {
      return null;
    }

    for (final peerID in peers) {
      if (peerID == excludePeerID) {
        continue;
      }
      return peerID;
    }
    return null;
  }

  String? holonNameOf(String peerID) => _peerToName[peerID];

  HolonRPCDispatchRoute? parseDispatchRoute(String method) {
    final trimmedMethod = method.trim();
    if (trimmedMethod.isEmpty) {
      return null;
    }

    final separator = trimmedMethod.indexOf('.');
    if (separator <= 0 || separator >= trimmedMethod.length - 1) {
      return null;
    }

    final holonName = trimmedMethod.substring(0, separator).trim();
    final routedMethod = trimmedMethod.substring(separator + 1).trim();
    if (holonName.isEmpty || routedMethod.isEmpty) {
      return null;
    }

    return HolonRPCDispatchRoute(
      holonName: holonName,
      method: routedMethod,
    );
  }
}

class HolonRPCServer {
  HolonRPCServer(
    this.bindURL, {
    HolonRPCRouter? router,
  }) : _router = router ?? HolonRPCRouter();

  final String bindURL;
  final HolonRPCRouter _router;

  final Map<String, HolonRPCHandler> _handlers = <String, HolonRPCHandler>{};
  final Map<String, _HolonRPCServerPeer> _peers =
      <String, _HolonRPCServerPeer>{};
  final Queue<String> _connectQueue = Queue<String>();
  final Queue<Completer<String>> _waiters = Queue<Completer<String>>();

  HttpServer? _server;
  String? _address;
  String _path = '/rpc';
  bool _closed = false;
  int _nextClientID = 0;
  int _nextServerID = 0;

  HolonRPCRouter get router => _router;

  String get address => _address ?? bindURL;

  void register(String method, HolonRPCHandler handler) {
    final trimmedMethod = method.trim();
    if (trimmedMethod.isEmpty) {
      throw ArgumentError('method is required');
    }
    _handlers[trimmedMethod] = handler;
  }

  void unregister(String method) {
    _handlers.remove(method.trim());
  }

  List<String> clientIDs() => _peers.keys.toList(growable: false);

  String? holonNameForPeer(String peerID) => _router.holonNameOf(peerID);

  Future<String> start() async {
    if (_closed) {
      throw StateError('holon-rpc server is closed');
    }
    if (_server != null) {
      return address;
    }

    final parsed = Uri.parse(bindURL);
    if (parsed.scheme != 'ws') {
      throw ArgumentError(
        'unsupported scheme "${parsed.scheme}" (expected ws://)',
      );
    }

    final host = parsed.host.isEmpty
        ? InternetAddress.loopbackIPv4.address
        : parsed.host;
    final hasExplicitPort = _hasExplicitPortInURL(bindURL);
    final port = hasExplicitPort ? parsed.port : 80;
    _path = parsed.path.isEmpty ? '/rpc' : parsed.path;

    final server = await HttpServer.bind(host, port);
    _server = server;
    server.listen(
      (request) {
        unawaited(_handleUpgrade(request));
      },
      onDone: () {
        _closed = true;
      },
    );

    final boundHost = _formatHostForURL(server.address.address);
    _address = 'ws://$boundHost:${server.port}$_path';
    return _address!;
  }

  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;

    final server = _server;
    _server = null;
    await server?.close(force: true);

    final peers = _peers.values.toList(growable: false);
    _peers.clear();

    final closeError = HolonRPCResponseException(
      code: _codeUnavailable,
      message: 'holon-rpc connection closed',
    );

    for (final peer in peers) {
      _router.deregisterHolon(peer.id);
      _failPending(peer, closeError);
      await peer.dispose(closeSocket: true);
    }

    _connectQueue.clear();
    _failWaiters(StateError('holon-rpc server closed'));
  }

  Future<String> waitForClient({Duration? timeout}) async {
    if (_connectQueue.isNotEmpty) {
      return _connectQueue.removeFirst();
    }
    if (_closed) {
      throw StateError('holon-rpc server is closed');
    }

    final waiter = Completer<String>();
    _waiters.add(waiter);

    Future<String> result = waiter.future;
    if (timeout != null) {
      result = result.timeout(timeout);
    }

    return result.whenComplete(() {
      _waiters.remove(waiter);
    });
  }

  Future<Map<String, dynamic>> invoke(
    String clientID,
    String method, {
    Map<String, dynamic> params = const <String, dynamic>{},
    Duration timeout = const Duration(seconds: 10),
  }) async {
    final trimmedMethod = method.trim();
    if (trimmedMethod.isEmpty) {
      throw ArgumentError('method is required');
    }

    final peer = _peers[clientID];
    if (peer == null) {
      throw HolonRPCResponseException(
        code: _codeUnavailable,
        message: 'unknown client "$clientID"',
      );
    }

    final id = 's${++_nextServerID}';
    final pending = Completer<Map<String, dynamic>>();
    peer.pending[id] = pending;

    try {
      await _writePeer(
        peer,
        <String, dynamic>{
          'jsonrpc': _jsonRPCVersion,
          'id': id,
          'method': trimmedMethod,
          'params': params,
        },
      );
    } catch (error) {
      peer.pending.remove(id);
      rethrow;
    }

    try {
      return await pending.future.timeout(timeout);
    } on TimeoutException {
      throw HolonRPCResponseException(
        code: 4,
        message: 'deadline exceeded',
      );
    } finally {
      peer.pending.remove(id);
    }
  }

  Future<void> _handleUpgrade(HttpRequest request) async {
    if (_closed) {
      request.response.statusCode = HttpStatus.serviceUnavailable;
      await request.response.close();
      return;
    }

    if (request.uri.path != _path) {
      request.response.statusCode = HttpStatus.notFound;
      await request.response.close();
      return;
    }

    WebSocket socket;
    try {
      socket = await WebSocketTransformer.upgrade(
        request,
        protocolSelector: (protocols) {
          if (protocols.contains('holon-rpc')) {
            return 'holon-rpc';
          }
          return null;
        },
      );
    } catch (_) {
      return;
    }

    if (socket.protocol != 'holon-rpc') {
      await socket.close(
        WebSocketStatus.protocolError,
        'missing holon-rpc subprotocol',
      );
      return;
    }

    final clientID = 'c${++_nextClientID}';
    final peer = _HolonRPCServerPeer(clientID, socket);
    _peers[clientID] = peer;
    _onClientConnected(clientID);

    peer.subscription = socket.listen(
      (data) {
        _handleIncoming(peer, data);
      },
      onDone: () {
        unawaited(_removePeer(peer.id));
      },
      onError: (Object _, StackTrace __) {
        unawaited(_removePeer(peer.id));
      },
      cancelOnError: true,
    );
  }

  void _onClientConnected(String clientID) {
    if (_waiters.isNotEmpty) {
      final waiter = _waiters.removeFirst();
      if (!waiter.isCompleted) {
        waiter.complete(clientID);
      }
      return;
    }
    _connectQueue.add(clientID);
  }

  void _failWaiters(Object error, [StackTrace? stackTrace]) {
    while (_waiters.isNotEmpty) {
      final waiter = _waiters.removeFirst();
      if (!waiter.isCompleted) {
        waiter.completeError(error, stackTrace);
      }
    }
  }

  Future<void> _removePeer(String peerID) async {
    final peer = _peers.remove(peerID);
    if (peer == null) {
      return;
    }

    _router.deregisterHolon(peerID);
    _failPending(
      peer,
      HolonRPCResponseException(
        code: _codeUnavailable,
        message: 'holon-rpc connection closed',
      ),
    );
    await peer.dispose(closeSocket: false);
  }

  void _handleIncoming(_HolonRPCServerPeer peer, dynamic data) {
    final String text;
    if (data is String) {
      text = data;
    } else if (data is List<int>) {
      text = utf8.decode(data);
    } else {
      return;
    }

    late final Object decoded;
    try {
      decoded = jsonDecode(text);
    } catch (_) {
      unawaited(_sendPeerError(
        peer,
        null,
        _codeParseError,
        'parse error',
        null,
        true,
      ));
      return;
    }

    if (decoded is! Map) {
      unawaited(_sendPeerError(
        peer,
        null,
        _codeInvalidRequest,
        'invalid request',
        null,
        true,
      ));
      return;
    }

    final msg = decoded is Map<String, dynamic>
        ? decoded
        : decoded.cast<String, dynamic>();

    if (msg['method'] != null) {
      unawaited(_handlePeerRequest(peer, msg));
      return;
    }

    if (msg.containsKey('result') || msg.containsKey('error')) {
      _handlePeerResponse(peer, msg);
      return;
    }

    if (_hasID(msg['id'])) {
      unawaited(
        _sendPeerError(peer, msg['id'], _codeInvalidRequest, 'invalid request'),
      );
    }
  }

  Future<void> _handlePeerRequest(
    _HolonRPCServerPeer peer,
    Map<String, dynamic> msg,
  ) async {
    final reqID = msg['id'];
    final rawMethod = msg['method'];

    if (msg['jsonrpc'] != _jsonRPCVersion ||
        rawMethod is! String ||
        rawMethod.trim().isEmpty) {
      if (_hasID(reqID)) {
        await _sendPeerError(
            peer, reqID, _codeInvalidRequest, 'invalid request');
      }
      return;
    }

    var method = rawMethod.trim();
    if (method == 'rpc.heartbeat') {
      if (_hasID(reqID)) {
        await _sendPeerResult(peer, reqID, <String, dynamic>{});
      }
      return;
    }

    Map<String, dynamic> params;
    try {
      params = _decodeParams(msg['params']);
    } on HolonRPCResponseException catch (error) {
      if (_hasID(reqID)) {
        await _sendPeerError(
            peer, reqID, error.code, error.message, error.data);
      }
      return;
    }

    if (method == 'rpc.register') {
      await _handleRegister(peer, reqID, params);
      return;
    }

    if (method == 'rpc.unregister') {
      await _handleUnregister(peer, reqID);
      return;
    }

    _ParsedRoute parsedRoute;
    try {
      parsedRoute = _parseRouteHints(method, params);
    } on HolonRPCResponseException catch (error) {
      if (_hasID(reqID)) {
        await _sendPeerError(
            peer, reqID, error.code, error.message, error.data);
      }
      return;
    }

    method = parsedRoute.method;
    params = parsedRoute.params;

    final routed = await _routePeerRequest(
      peer,
      reqID,
      method,
      params,
      parsedRoute.hints,
      parsedRoute.fanOut,
    );
    if (routed) {
      return;
    }

    final handler = _handlers[method];
    if (handler == null) {
      if (_hasID(reqID)) {
        await _sendPeerError(
          peer,
          reqID,
          _codeMethodNotFound,
          'method "$method" not found',
        );
      }
      return;
    }

    try {
      final result = await handler(params);
      if (_hasID(reqID)) {
        await _sendPeerResult(peer, reqID, result);
      }
    } on HolonRPCResponseException catch (error) {
      if (_hasID(reqID)) {
        await _sendPeerError(
            peer, reqID, error.code, error.message, error.data);
      }
    } catch (error) {
      if (_hasID(reqID)) {
        await _sendPeerError(peer, reqID, _codeInternalError, 'internal error');
      }
    }
  }

  void _handlePeerResponse(
    _HolonRPCServerPeer peer,
    Map<String, dynamic> msg,
  ) {
    final rawID = msg['id'];
    if (rawID is! String) {
      return;
    }

    final pending = peer.pending.remove(rawID);
    if (pending == null || pending.isCompleted) {
      return;
    }

    final rawError = msg['error'];
    if (rawError is Map<String, dynamic>) {
      final rawCode = rawError['code'];
      final code = rawCode is int
          ? rawCode
          : (rawCode is num ? rawCode.toInt() : _codeInternalError);
      pending.completeError(
        HolonRPCResponseException(
          code: code,
          message: rawError['message']?.toString() ?? 'internal error',
          data: rawError['data'],
        ),
      );
      return;
    }
    if (rawError is Map) {
      final casted = rawError.cast<String, dynamic>();
      final rawCode = casted['code'];
      final code = rawCode is int
          ? rawCode
          : (rawCode is num ? rawCode.toInt() : _codeInternalError);
      pending.completeError(
        HolonRPCResponseException(
          code: code,
          message: casted['message']?.toString() ?? 'internal error',
          data: casted['data'],
        ),
      );
      return;
    }

    pending.complete(_normalizeResult(msg['result']));
  }

  Future<bool> _routePeerRequest(
    _HolonRPCServerPeer caller,
    dynamic reqID,
    String method,
    Map<String, dynamic> params,
    _RouteHints hints,
    bool fanOut,
  ) async {
    if (fanOut) {
      List<Map<String, dynamic>> entries;
      try {
        entries = await _dispatchFanOut(caller, method, params);
      } on HolonRPCResponseException catch (error) {
        if (_hasID(reqID)) {
          await _sendPeerError(
            caller,
            reqID,
            error.code,
            error.message,
            error.data,
          );
        }
        return true;
      }

      if (hints.mode == _routeModeFullBroadcast) {
        for (final entry in entries) {
          final sourcePeer = entry['peer']?.toString() ?? '';
          final payload = <String, dynamic>{'peer': sourcePeer};
          if (entry.containsKey('error')) {
            payload['error'] = entry['error'];
          } else {
            payload['result'] = entry['result'];
          }

          final excluded = <String>{caller.id};
          if (sourcePeer.isNotEmpty) {
            excluded.add(sourcePeer);
          }
          await _broadcastNotificationMany(excluded, method, payload);
        }
      }

      if (_hasID(reqID)) {
        await _sendPeerResultAny(caller, reqID, entries);
      }
      return true;
    }

    var dispatchMethod = method;
    var targetPeerID = hints.targetPeerID;

    if (targetPeerID.isEmpty) {
      final dispatchRoute = _router.parseDispatchRoute(method);
      if (dispatchRoute != null && _router.hasHolon(dispatchRoute.holonName)) {
        final resolvedPeer = _router.resolveHolon(
          dispatchRoute.holonName,
          excludePeerID: caller.id,
        );
        if (resolvedPeer == null) {
          if (_hasID(reqID)) {
            await _sendPeerError(
              caller,
              reqID,
              _codeNotFound,
              'holon "${dispatchRoute.holonName}" not found',
            );
          }
          return true;
        }
        targetPeerID = resolvedPeer;
        dispatchMethod = dispatchRoute.method;
      }
    }

    if (targetPeerID.isEmpty) {
      return false;
    }

    if (!_peers.containsKey(targetPeerID)) {
      if (_hasID(reqID)) {
        await _sendPeerError(
          caller,
          reqID,
          _codeNotFound,
          'peer "$targetPeerID" not found',
        );
      }
      return true;
    }

    Map<String, dynamic> result;
    try {
      result = await invoke(targetPeerID, dispatchMethod, params: params);
    } on HolonRPCResponseException catch (error) {
      if (_hasID(reqID)) {
        await _sendPeerError(
          caller,
          reqID,
          error.code,
          error.message,
          error.data,
        );
      }
      return true;
    } catch (error) {
      if (_hasID(reqID)) {
        await _sendPeerError(caller, reqID, _codeUnavailable, error.toString());
      }
      return true;
    }

    if (hints.mode == _routeModeBroadcastResponse) {
      await _broadcastNotificationMany(
        <String>{caller.id, targetPeerID},
        dispatchMethod,
        <String, dynamic>{
          'peer': targetPeerID,
          'result': result,
        },
      );
    }

    if (_hasID(reqID)) {
      await _sendPeerResult(caller, reqID, result);
    }
    return true;
  }

  Future<List<Map<String, dynamic>>> _dispatchFanOut(
    _HolonRPCServerPeer caller,
    String method,
    Map<String, dynamic> params,
  ) async {
    final targets = _peers.keys
        .where((peerID) => peerID != caller.id)
        .toList(growable: false);
    if (targets.isEmpty) {
      throw HolonRPCResponseException(
        code: _codeNotFound,
        message: 'no connected peers',
      );
    }

    final entries = <Map<String, dynamic>>[];
    final jobs = <Future<void>>[];

    for (final targetPeerID in targets) {
      jobs.add(() async {
        try {
          final result = await invoke(targetPeerID, method, params: params);
          entries.add(<String, dynamic>{
            'peer': targetPeerID,
            'result': result,
          });
        } on HolonRPCResponseException catch (error) {
          entries.add(<String, dynamic>{
            'peer': targetPeerID,
            'error': _responseErrorToMap(error),
          });
        } catch (error) {
          entries.add(<String, dynamic>{
            'peer': targetPeerID,
            'error': <String, dynamic>{
              'code': _codeUnavailable,
              'message': error.toString(),
            },
          });
        }
      }());
    }

    await Future.wait(jobs);
    return entries;
  }

  Future<void> _broadcastNotificationMany(
    Set<String> excludedPeerIDs,
    String method,
    Map<String, dynamic> payload,
  ) async {
    final peers = _peers.entries
        .where((entry) => !excludedPeerIDs.contains(entry.key))
        .map((entry) => entry.value)
        .toList(growable: false);

    for (final peer in peers) {
      try {
        await _writePeer(
          peer,
          <String, dynamic>{
            'jsonrpc': _jsonRPCVersion,
            'method': method,
            'params': payload,
          },
        );
      } catch (_) {
        // Best-effort notifications.
      }
    }
  }

  Future<void> _handleRegister(
    _HolonRPCServerPeer peer,
    dynamic reqID,
    Map<String, dynamic> params,
  ) async {
    final rawName = params['name'];
    if (rawName is! String || rawName.trim().isEmpty) {
      if (_hasID(reqID)) {
        await _sendPeerError(
          peer,
          reqID,
          _codeInvalidParams,
          'name must be a non-empty string',
        );
      }
      return;
    }

    final name = rawName.trim();
    _router.registerHolon(peerID: peer.id, name: name);
    if (_hasID(reqID)) {
      await _sendPeerResult(
        peer,
        reqID,
        <String, dynamic>{'peer': peer.id, 'name': name},
      );
    }
  }

  Future<void> _handleUnregister(
    _HolonRPCServerPeer peer,
    dynamic reqID,
  ) async {
    _router.deregisterHolon(peer.id);
    if (_hasID(reqID)) {
      await _sendPeerResult(peer, reqID, <String, dynamic>{});
    }
  }

  Future<void> _sendPeerResult(
    _HolonRPCServerPeer peer,
    dynamic id,
    Map<String, dynamic> result,
  ) async {
    await _sendPeerResultAny(peer, id, result);
  }

  Future<void> _sendPeerResultAny(
    _HolonRPCServerPeer peer,
    dynamic id,
    Object? result,
  ) async {
    await _writePeer(
      peer,
      <String, dynamic>{
        'jsonrpc': _jsonRPCVersion,
        'id': id,
        'result': result ?? <String, dynamic>{},
      },
    );
  }

  Future<void> _sendPeerError(
    _HolonRPCServerPeer peer,
    dynamic id,
    int code,
    String message, [
    Object? data,
    bool includeNullID = false,
  ]) async {
    await _writePeer(
      peer,
      <String, dynamic>{
        'jsonrpc': _jsonRPCVersion,
        if (id != null || includeNullID) 'id': id,
        'error': <String, dynamic>{
          'code': code,
          'message': message,
          if (data != null) 'data': data,
        },
      },
    );
  }

  Future<void> _writePeer(
    _HolonRPCServerPeer peer,
    Map<String, dynamic> payload,
  ) async {
    if (peer.closed) {
      throw StateError('holon-rpc connection closed');
    }
    peer.socket.add(jsonEncode(payload));
  }

  void _failPending(
    _HolonRPCServerPeer peer,
    HolonRPCResponseException error,
  ) {
    if (peer.pending.isEmpty) {
      return;
    }

    final pending = peer.pending.values.toList(growable: false);
    peer.pending.clear();
    for (final completer in pending) {
      if (!completer.isCompleted) {
        completer.completeError(error);
      }
    }
  }

  _ParsedRoute _parseRouteHints(
    String method,
    Map<String, dynamic> params,
  ) {
    var dispatchMethod = method.trim();
    if (dispatchMethod.isEmpty) {
      throw HolonRPCResponseException(
        code: _codeInvalidRequest,
        message: 'invalid request',
      );
    }

    final cleaned = Map<String, dynamic>.from(params);
    var mode = _routeModeDefault;

    if (cleaned.containsKey('_routing')) {
      final rawMode = cleaned.remove('_routing');
      if (rawMode is! String) {
        throw HolonRPCResponseException(
          code: _codeInvalidParams,
          message: '_routing must be a string',
        );
      }
      final trimmedMode = rawMode.trim();
      if (trimmedMode != _routeModeDefault &&
          trimmedMode != _routeModeBroadcastResponse &&
          trimmedMode != _routeModeFullBroadcast) {
        throw HolonRPCResponseException(
          code: _codeInvalidParams,
          message: 'unsupported _routing "$trimmedMode"',
        );
      }
      mode = trimmedMode;
    }

    var targetPeerID = '';
    if (cleaned.containsKey('_peer')) {
      final rawPeerID = cleaned.remove('_peer');
      if (rawPeerID is! String) {
        throw HolonRPCResponseException(
          code: _codeInvalidParams,
          message: '_peer must be a string',
        );
      }
      targetPeerID = rawPeerID.trim();
      if (targetPeerID.isEmpty) {
        throw HolonRPCResponseException(
          code: _codeInvalidParams,
          message: '_peer must be non-empty',
        );
      }
    }

    var fanOut = false;
    if (dispatchMethod.startsWith('*.')) {
      fanOut = true;
      dispatchMethod = dispatchMethod.substring(2).trim();
      if (dispatchMethod.isEmpty) {
        throw HolonRPCResponseException(
          code: _codeInvalidRequest,
          message: 'invalid fan-out method',
        );
      }
    }

    if (mode == _routeModeFullBroadcast && !fanOut) {
      throw HolonRPCResponseException(
        code: _codeInvalidParams,
        message: 'full-broadcast requires a fan-out method',
      );
    }

    return _ParsedRoute(
      method: dispatchMethod,
      fanOut: fanOut,
      params: cleaned,
      hints: _RouteHints(targetPeerID: targetPeerID, mode: mode),
    );
  }

  Map<String, dynamic> _decodeParams(Object? rawParams) {
    if (rawParams == null) {
      return <String, dynamic>{};
    }
    if (rawParams is Map<String, dynamic>) {
      return Map<String, dynamic>.from(rawParams);
    }
    if (rawParams is Map) {
      return rawParams.cast<String, dynamic>();
    }

    throw HolonRPCResponseException(
      code: _codeInvalidParams,
      message: 'params must be an object',
    );
  }
}

class _HolonRPCServerPeer {
  _HolonRPCServerPeer(this.id, this.socket);

  final String id;
  final WebSocket socket;
  final Map<String, Completer<Map<String, dynamic>>> pending =
      <String, Completer<Map<String, dynamic>>>{};
  StreamSubscription<dynamic>? subscription;
  bool closed = false;

  Future<void> dispose({required bool closeSocket}) async {
    if (closed) {
      return;
    }
    closed = true;

    final sub = subscription;
    subscription = null;
    await sub?.cancel();

    if (closeSocket) {
      try {
        await socket.close(WebSocketStatus.goingAway, 'server shutdown');
      } catch (_) {}
    }
  }
}

class _RouteHints {
  const _RouteHints({
    required this.targetPeerID,
    required this.mode,
  });

  final String targetPeerID;
  final String mode;
}

class _ParsedRoute {
  const _ParsedRoute({
    required this.method,
    required this.fanOut,
    required this.params,
    required this.hints,
  });

  final String method;
  final bool fanOut;
  final Map<String, dynamic> params;
  final _RouteHints hints;
}

Map<String, dynamic> _normalizeResult(Object? rawResult) {
  if (rawResult == null) {
    return <String, dynamic>{};
  }
  if (rawResult is Map<String, dynamic>) {
    return rawResult;
  }
  if (rawResult is Map) {
    return rawResult.cast<String, dynamic>();
  }
  return <String, dynamic>{'value': rawResult};
}

Map<String, dynamic> _responseErrorToMap(HolonRPCResponseException error) {
  return <String, dynamic>{
    'code': error.code,
    'message': error.message,
    if (error.data != null) 'data': error.data,
  };
}

String _formatHostForURL(String host) {
  if (host.contains(':') && !host.startsWith('[') && !host.endsWith(']')) {
    return '[$host]';
  }
  return host;
}

bool _hasExplicitPortInURL(String rawURL) {
  final schemeSeparator = rawURL.indexOf('://');
  if (schemeSeparator < 0 || schemeSeparator + 3 >= rawURL.length) {
    return false;
  }

  final afterScheme = rawURL.substring(schemeSeparator + 3);
  final slashIndex = afterScheme.indexOf('/');
  final authority =
      slashIndex >= 0 ? afterScheme.substring(0, slashIndex) : afterScheme;

  if (authority.isEmpty) {
    return false;
  }
  if (authority.startsWith('[')) {
    final closing = authority.indexOf(']');
    if (closing < 0 || closing >= authority.length - 1) {
      return false;
    }
    return authority[closing + 1] == ':';
  }
  return authority.lastIndexOf(':') > 0;
}

bool _hasID(Object? id) {
  return id != null;
}

const String _defaultHTTPRPCPath = '/api/v1/rpc';

typedef HolonRPCStreamHandler = Future<void> Function(
  Map<String, dynamic> params,
  Future<void> Function(Map<String, dynamic> result) send,
);

class HolonRPCSSEEvent {
  const HolonRPCSSEEvent({
    required this.event,
    required this.id,
    this.result = const <String, dynamic>{},
    this.error,
  });

  final String event;
  final String id;
  final Map<String, dynamic> result;
  final HolonRPCResponseException? error;
}

class HolonRPCHTTPServer {
  HolonRPCHTTPServer(this.bindURL);

  final String bindURL;

  final Map<String, HolonRPCHandler> _handlers = <String, HolonRPCHandler>{};
  final Map<String, HolonRPCStreamHandler> _streamHandlers =
      <String, HolonRPCStreamHandler>{};

  HttpServer? _server;
  String? _address;
  String _path = _defaultHTTPRPCPath;
  bool _closed = false;
  int _nextRequestID = 0;

  String get address => _address ?? bindURL;

  void register(String method, HolonRPCHandler handler) {
    final trimmed = method.trim();
    if (trimmed.isEmpty) {
      throw ArgumentError('method is required');
    }
    _handlers[trimmed] = handler;
  }

  void registerStream(String method, HolonRPCStreamHandler handler) {
    final trimmed = method.trim();
    if (trimmed.isEmpty) {
      throw ArgumentError('method is required');
    }
    _streamHandlers[trimmed] = handler;
  }

  Future<String> start() async {
    if (_closed) {
      throw StateError('holon-rpc http server is closed');
    }
    if (_server != null) {
      return address;
    }

    final parsed = Uri.parse(bindURL);
    final scheme = parsed.scheme.toLowerCase();
    if (scheme != 'http' && scheme != 'https') {
      throw ArgumentError(
        'unsupported scheme "${parsed.scheme}" (expected http:// or https://)',
      );
    }

    final host = parsed.host.isEmpty
        ? InternetAddress.loopbackIPv4.address
        : parsed.host;
    final port = _hasExplicitPortInURL(bindURL)
        ? parsed.port
        : (scheme == 'https' ? 443 : 80);
    _path = parsed.path.isEmpty ? _defaultHTTPRPCPath : parsed.path;

    late final HttpServer server;
    if (scheme == 'https') {
      final certFile = parsed.queryParameters['cert'] ??
          Platform.environment['HOLONS_HTTPS_CERT_FILE'];
      final keyFile = parsed.queryParameters['key'] ??
          Platform.environment['HOLONS_HTTPS_KEY_FILE'];
      if ((certFile ?? '').trim().isEmpty || (keyFile ?? '').trim().isEmpty) {
        throw ArgumentError(
          'https:// requires cert and key (query params cert/key or HOLONS_HTTPS_CERT_FILE/HOLONS_HTTPS_KEY_FILE)',
        );
      }

      final context = SecurityContext()
        ..useCertificateChain(certFile!)
        ..usePrivateKey(keyFile!);
      server = await HttpServer.bindSecure(host, port, context);
    } else {
      server = await HttpServer.bind(host, port);
    }

    _server = server;
    server.listen(
      (request) {
        unawaited(_handleHTTP(request));
      },
      onDone: () {
        _closed = true;
      },
    );

    final boundHost = _formatHostForURL(server.address.address);
    _address = '$scheme://$boundHost:${server.port}$_path';
    return _address!;
  }

  Future<void> close() async {
    if (_closed) {
      return;
    }
    _closed = true;
    final server = _server;
    _server = null;
    await server?.close(force: true);
  }

  Future<void> _handleHTTP(HttpRequest request) async {
    _setHTTPCorsHeaders(request.response, request);

    if (request.method == 'OPTIONS') {
      request.response.statusCode = HttpStatus.noContent;
      await request.response.close();
      return;
    }

    final method = _methodFromPath(request.uri.path);
    if (method == null) {
      await _writeHTTPRPCError(
        request.response,
        HttpStatus.notFound,
        'h0',
        HolonRPCResponseException(
          code: _codeNotFound,
          message: 'method not found',
        ),
      );
      return;
    }

    if (_acceptsSSE(request)) {
      switch (request.method) {
        case 'GET':
          await _handleStream(
            request,
            method,
            request.uri.queryParameters.map(
              (key, value) => MapEntry(key, value),
            ),
          );
          return;
        case 'POST':
          try {
            final params = await _decodeRequestParams(request);
            await _handleStream(request, method, params);
          } on HolonRPCResponseException catch (error) {
            await _writeHTTPRPCError(
              request.response,
              _httpStatusForRPCError(error),
              'h0',
              error,
            );
          }
          return;
        default:
          request.response.statusCode = HttpStatus.methodNotAllowed;
          await request.response.close();
          return;
      }
    }

    if (request.method != 'POST') {
      request.response.statusCode = HttpStatus.methodNotAllowed;
      await request.response.close();
      return;
    }

    Map<String, dynamic> params;
    try {
      params = await _decodeRequestParams(request);
    } on HolonRPCResponseException catch (error) {
      await _writeHTTPRPCError(
        request.response,
        _httpStatusForRPCError(error),
        'h0',
        error,
      );
      return;
    }

    final handler = _handlers[method];
    if (handler == null) {
      await _writeHTTPRPCError(
        request.response,
        HttpStatus.notFound,
        'h0',
        HolonRPCResponseException(
          code: _codeNotFound,
          message: 'method "$method" not found',
        ),
      );
      return;
    }

    try {
      final result = _normalizeResult(await handler(params));
      await _writeHTTPRPCResult(request.response, _nextHTTPID(), result);
    } on HolonRPCResponseException catch (error) {
      await _writeHTTPRPCError(
        request.response,
        _httpStatusForRPCError(error),
        _nextHTTPID(),
        error,
      );
    } catch (error) {
      await _writeHTTPRPCError(
        request.response,
        HttpStatus.internalServerError,
        _nextHTTPID(),
        HolonRPCResponseException(
          code: _codeInternalError,
          message: error.toString(),
        ),
      );
    }
  }

  Future<void> _handleStream(
    HttpRequest request,
    String method,
    Map<String, dynamic> params,
  ) async {
    final handler = _streamHandlers[method];
    if (handler == null) {
      await _writeHTTPRPCError(
        request.response,
        HttpStatus.notFound,
        'h0',
        HolonRPCResponseException(
          code: _codeNotFound,
          message: 'method "$method" not found',
        ),
      );
      return;
    }

    final response = request.response
      ..bufferOutput = false
      ..statusCode = HttpStatus.ok;
    response.headers.contentType = ContentType('text', 'event-stream');
    response.headers.set(HttpHeaders.cacheControlHeader, 'no-cache');
    await response.flush();

    final requestID = _nextHTTPID();
    var eventID = 0;

    Future<void> send(Map<String, dynamic> result) async {
      eventID += 1;
      await _writeSSEEvent(
        response,
        event: 'message',
        id: '$eventID',
        data: jsonEncode(<String, dynamic>{
          'jsonrpc': _jsonRPCVersion,
          'id': requestID,
          'result': result,
        }),
      );
    }

    try {
      await handler(params, send);
    } on HolonRPCResponseException catch (error) {
      eventID += 1;
      await _writeSSEEvent(
        response,
        event: 'error',
        id: '$eventID',
        data: jsonEncode(<String, dynamic>{
          'jsonrpc': _jsonRPCVersion,
          'id': requestID,
          'error': _responseErrorToMap(error),
        }),
      );
    } catch (error) {
      eventID += 1;
      await _writeSSEEvent(
        response,
        event: 'error',
        id: '$eventID',
        data: jsonEncode(<String, dynamic>{
          'jsonrpc': _jsonRPCVersion,
          'id': requestID,
          'error': <String, dynamic>{
            'code': _codeInternalError,
            'message': error.toString(),
          },
        }),
      );
    }

    await _writeSSEEvent(
      response,
      event: 'done',
      id: '',
      data: '',
    );
    await response.close();
  }

  String? _methodFromPath(String path) {
    final base = _path.endsWith('/') && _path.length > 1
        ? _path.substring(0, _path.length - 1)
        : _path;
    final prefix = '$base/';
    if (!path.startsWith(prefix)) {
      return null;
    }
    final method =
        path.substring(prefix.length).replaceAll(RegExp(r'^/+|/+$'), '');
    return method.isEmpty ? null : method;
  }

  String _nextHTTPID() {
    _nextRequestID += 1;
    return 'h$_nextRequestID';
  }
}

class HolonRPCHTTPClient {
  HolonRPCHTTPClient(
    String baseURL, {
    HttpClient? client,
  })  : baseURL = baseURL.trim().replaceFirst(RegExp(r'/+$'), ''),
        _client = client ?? HttpClient(),
        _ownsClient = client == null;

  final String baseURL;
  final HttpClient _client;
  final bool _ownsClient;

  Future<Map<String, dynamic>> invoke(
    String method, {
    Map<String, dynamic> params = const <String, dynamic>{},
  }) async {
    final request = await _client.postUrl(Uri.parse(_methodURL(method)));
    request.headers.contentType = ContentType.json;
    request.headers.set(HttpHeaders.acceptHeader, 'application/json');
    request.add(utf8.encode(jsonEncode(params)));

    final response = await request.close();
    return _decodeHTTPRPCResponse(response);
  }

  Future<List<HolonRPCSSEEvent>> stream(
    String method, {
    Map<String, dynamic> params = const <String, dynamic>{},
  }) async {
    final request = await _client.postUrl(Uri.parse(_methodURL(method)));
    request.headers.contentType = ContentType.json;
    request.headers.set(HttpHeaders.acceptHeader, 'text/event-stream');
    request.add(utf8.encode(jsonEncode(params)));

    final response = await request.close();
    return _readSSEEvents(response);
  }

  Future<List<HolonRPCSSEEvent>> streamQuery(
    String method, {
    Map<String, String> params = const <String, String>{},
  }) async {
    final endpoint = Uri.parse(_methodURL(method)).replace(
      queryParameters: params.isEmpty ? null : params,
    );
    final request = await _client.getUrl(endpoint);
    request.headers.set(HttpHeaders.acceptHeader, 'text/event-stream');

    final response = await request.close();
    return _readSSEEvents(response);
  }

  void close({bool force = true}) {
    if (_ownsClient) {
      _client.close(force: force);
    }
  }

  String _methodURL(String method) {
    return '$baseURL/${method.trim().replaceAll(RegExp(r'^/+|/+$'), '')}';
  }
}

bool _acceptsSSE(HttpRequest request) {
  return request.headers[HttpHeaders.acceptHeader]
          ?.any((value) => value.contains('text/event-stream')) ??
      false;
}

void _setHTTPCorsHeaders(HttpResponse response, HttpRequest request) {
  final origin = request.headers.value('origin');
  response.headers.set(
    'Access-Control-Allow-Origin',
    (origin ?? '').trim().isEmpty ? '*' : origin!,
  );
  response.headers.set('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  response.headers.set(
    'Access-Control-Allow-Headers',
    'Content-Type, Accept, Last-Event-ID',
  );
  response.headers.set('Access-Control-Max-Age', '86400');
}

Future<Map<String, dynamic>> _decodeRequestParams(HttpRequest request) async {
  try {
    final body = await utf8.decoder.bind(request).join();
    if (body.trim().isEmpty) {
      return <String, dynamic>{};
    }

    final decoded = jsonDecode(body);
    if (decoded is Map<String, dynamic>) {
      return decoded;
    }
    if (decoded is Map) {
      return decoded.cast<String, dynamic>();
    }

    throw HolonRPCResponseException(
      code: _codeInvalidRequest,
      message: 'invalid request',
    );
  } on FormatException {
    throw HolonRPCResponseException(
      code: _codeParseError,
      message: 'invalid json',
    );
  }
}

Future<void> _writeHTTPRPCResult(
  HttpResponse response,
  String id,
  Map<String, dynamic> result,
) async {
  response.statusCode = HttpStatus.ok;
  response.headers.contentType = ContentType.json;
  response.write(
    jsonEncode(<String, dynamic>{
      'jsonrpc': _jsonRPCVersion,
      'id': id,
      'result': result,
    }),
  );
  await response.close();
}

Future<void> _writeHTTPRPCError(
  HttpResponse response,
  int statusCode,
  String id,
  HolonRPCResponseException error,
) async {
  response.statusCode = statusCode;
  response.headers.contentType = ContentType.json;
  response.write(
    jsonEncode(<String, dynamic>{
      'jsonrpc': _jsonRPCVersion,
      'id': id,
      'error': _responseErrorToMap(error),
    }),
  );
  await response.close();
}

int _httpStatusForRPCError(HolonRPCResponseException error) {
  switch (error.code) {
    case _codeNotFound:
      return HttpStatus.notFound;
    case _codeParseError:
    case _codeInvalidRequest:
      return HttpStatus.badRequest;
    default:
      return HttpStatus.internalServerError;
  }
}

Future<void> _writeSSEEvent(
  HttpResponse response, {
  required String event,
  required String id,
  required String data,
}) async {
  response.write('event: $event\n');
  if (id.isNotEmpty) {
    response.write('id: $id\n');
  }
  response.write('data: $data\n\n');
  await response.flush();
}

Future<Map<String, dynamic>> _decodeHTTPRPCResponse(
  HttpClientResponse response,
) async {
  final body = await utf8.decoder.bind(response).join();
  final decoded = jsonDecode(body);
  if (decoded is! Map) {
    throw HolonRPCResponseException(
      code: _codeInternalError,
      message: 'invalid http response',
    );
  }

  final message = decoded.cast<String, dynamic>();
  final error = message['error'];
  if (error is Map) {
    throw HolonRPCResponseException(
      code: (error['code'] as num?)?.toInt() ?? _codeInternalError,
      message: error['message']?.toString() ?? 'internal error',
      data: error['data'],
    );
  }

  return _normalizeResult(message['result']);
}

Future<List<HolonRPCSSEEvent>> _readSSEEvents(
  HttpClientResponse response,
) async {
  if (response.statusCode >= 400) {
    await _decodeHTTPRPCResponse(response);
  }

  final events = <HolonRPCSSEEvent>[];
  String currentEvent = '';
  String currentID = '';
  final dataLines = <String>[];

  Future<void> flush() async {
    if (currentEvent.isEmpty && currentID.isEmpty && dataLines.isEmpty) {
      return;
    }

    final data = dataLines.join('\n');
    switch (currentEvent) {
      case 'message':
      case 'error':
        final decoded = jsonDecode(data) as Map<String, dynamic>;
        final error = decoded['error'];
        if (error is Map) {
          events.add(
            HolonRPCSSEEvent(
              event: currentEvent,
              id: currentID,
              error: HolonRPCResponseException(
                code: (error['code'] as num?)?.toInt() ?? _codeInternalError,
                message: error['message']?.toString() ?? 'internal error',
                data: error['data'],
              ),
            ),
          );
        } else {
          events.add(
            HolonRPCSSEEvent(
              event: currentEvent,
              id: currentID,
              result: _normalizeResult(decoded['result']),
            ),
          );
        }
        break;
      case 'done':
        events.add(const HolonRPCSSEEvent(event: 'done', id: ''));
        break;
      default:
        events.add(
          HolonRPCSSEEvent(
            event: currentEvent,
            id: currentID,
          ),
        );
        break;
    }

    currentEvent = '';
    currentID = '';
    dataLines.clear();
  }

  await for (final line
      in utf8.decoder.bind(response).transform(const LineSplitter())) {
    if (line.isEmpty) {
      await flush();
      continue;
    }
    if (line.startsWith('event:')) {
      currentEvent = line.substring('event:'.length).trim();
      continue;
    }
    if (line.startsWith('id:')) {
      currentID = line.substring('id:'.length).trim();
      continue;
    }
    if (line.startsWith('data:')) {
      dataLines.add(line.substring('data:'.length).trimLeft());
    }
  }

  await flush();
  return events;
}
