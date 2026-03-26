import 'dart:async';
import 'dart:collection';
import 'dart:io';

import 'package:holons/holons.dart';
import 'package:test/test.dart';

void main() {
  group('holon-rpc routing', () {
    test('dispatch routes by holon-name method prefix', () async {
      if (!await _canBindLoopbackTCP()) {
        return;
      }

      final server = HolonRPCServer('ws://127.0.0.1:0/rpc');
      final peers = <_RoutingPeer>[];

      try {
        final url = await server.start();

        final peerA = _RoutingPeer(holonName: 'caller', label: 'A');
        final peerB = _RoutingPeer(holonName: 'compute', label: 'B');
        final peerC = _RoutingPeer(holonName: 'storage', label: 'C');
        peers.addAll(<_RoutingPeer>[peerA, peerB, peerC]);

        for (final peer in peers) {
          await peer.connect(url);
        }

        final response = await peerA.client.invoke(
          'compute.Echo/Ping',
          params: const <String, dynamic>{'message': 'hello-dispatch'},
        );

        expect(response['from'], equals('B'));
        expect(peerB.requestCount, equals(1));
        expect(peerC.requestCount, equals(0));

        final params = await peerB.nextRequest('peer B request');
        _assertRoutingFieldStripped(params);
      } finally {
        for (final peer in peers) {
          await peer.close();
        }
        await server.close();
      }
    });

    test('fan-out aggregates responses from all peers', () async {
      if (!await _canBindLoopbackTCP()) {
        return;
      }

      final server = HolonRPCServer('ws://127.0.0.1:0/rpc');
      final peers = <_RoutingPeer>[];

      try {
        final url = await server.start();

        final peerA = _RoutingPeer(holonName: 'caller', label: 'A');
        final peerB = _RoutingPeer(holonName: 'compute', label: 'B');
        final peerC = _RoutingPeer(holonName: 'ml', label: 'C');
        final peerD = _RoutingPeer(holonName: 'storage', label: 'D');
        peers.addAll(<_RoutingPeer>[peerA, peerB, peerC, peerD]);

        for (final peer in peers) {
          await peer.connect(url);
        }

        final response = await peerA.client.invoke(
          '*.Echo/Ping',
          params: const <String, dynamic>{'message': 'hello-fanout'},
        );

        final entries = _parseFanOutEntries(response);
        expect(entries.length, equals(3));

        final seenPeers = <String>{};
        for (final entry in entries) {
          final peerID = entry['peer']?.toString();
          expect(peerID, isNotNull);
          expect(peerID, isNotEmpty);
          seenPeers.add(peerID!);
          expect(entry['result'], isA<Map>());
        }

        expect(
          seenPeers,
          equals(<String>{peerB.peerID, peerC.peerID, peerD.peerID}),
        );
      } finally {
        for (final peer in peers) {
          await peer.close();
        }
        await server.close();
      }
    });

    test('broadcast-response forwards target response to other peers',
        () async {
      if (!await _canBindLoopbackTCP()) {
        return;
      }

      final server = HolonRPCServer('ws://127.0.0.1:0/rpc');
      final peers = <_RoutingPeer>[];

      try {
        final url = await server.start();

        final peerA = _RoutingPeer(holonName: 'caller', label: 'A');
        final peerB = _RoutingPeer(holonName: 'compute', label: 'B');
        final peerC = _RoutingPeer(holonName: 'storage', label: 'C');
        final peerD = _RoutingPeer(holonName: 'ml', label: 'D');
        peers.addAll(<_RoutingPeer>[peerA, peerB, peerC, peerD]);

        for (final peer in peers) {
          await peer.connect(url);
        }

        final response = await peerA.client.invoke(
          'storage.Echo/Ping',
          params: const <String, dynamic>{
            '_routing': 'broadcast-response',
            'message': 'hello-broadcast-response',
          },
        );

        expect(response['from'], equals('C'));

        final targetParams = await peerC.nextRequest('peer C request');
        _assertRoutingFieldStripped(targetParams);

        final notifB = await peerB.nextNotification('peer B notification');
        final notifD = await peerD.nextNotification('peer D notification');

        for (final notification in <Map<String, dynamic>>[notifB, notifD]) {
          expect(notification['peer'], equals(peerC.peerID));
          expect(notification['result'], isA<Map>());
        }

        await Future<void>.delayed(const Duration(milliseconds: 100));
        expect(peerC.notificationCount, equals(0));
      } finally {
        for (final peer in peers) {
          await peer.close();
        }
        await server.close();
      }
    });

    test('full-broadcast aggregates and broadcasts each response', () async {
      if (!await _canBindLoopbackTCP()) {
        return;
      }

      final server = HolonRPCServer('ws://127.0.0.1:0/rpc');
      final peers = <_RoutingPeer>[];

      try {
        final url = await server.start();

        final peerA = _RoutingPeer(holonName: 'caller', label: 'A');
        final peerB = _RoutingPeer(holonName: 'compute', label: 'B');
        final peerC = _RoutingPeer(holonName: 'storage', label: 'C');
        final peerD = _RoutingPeer(holonName: 'ml', label: 'D');
        peers.addAll(<_RoutingPeer>[peerA, peerB, peerC, peerD]);

        for (final peer in peers) {
          await peer.connect(url);
        }

        final response = await peerA.client.invoke(
          '*.Echo/Ping',
          params: const <String, dynamic>{
            '_routing': 'full-broadcast',
            'message': 'hello-full-broadcast',
          },
        );

        final entries = _parseFanOutEntries(response);
        expect(entries.length, equals(3));

        for (final peer in <_RoutingPeer>[peerB, peerC, peerD]) {
          final requestParams =
              await peer.nextRequest('peer ${peer.label} request');
          _assertRoutingFieldStripped(requestParams);
        }

        for (final peer in <_RoutingPeer>[peerB, peerC, peerD]) {
          final seenFrom = <String>{};
          for (var i = 0; i < 2; i++) {
            final notification =
                await peer.nextNotification('peer ${peer.label} notification');
            final fromPeer = notification['peer']?.toString() ?? '';
            expect(fromPeer, isNotEmpty);
            expect(fromPeer, isNot(equals(peer.peerID)));
            seenFrom.add(fromPeer);
            expect(notification['result'], isA<Map>());
          }
          expect(seenFrom.length, equals(2));
        }
      } finally {
        for (final peer in peers) {
          await peer.close();
        }
        await server.close();
      }
    });
  });
}

class _RoutingPeer {
  _RoutingPeer({
    required this.holonName,
    required this.label,
  });

  final String holonName;
  final String label;

  late final HolonRPCClient client;
  String peerID = '';

  int requestCount = 0;
  int notificationCount = 0;

  final _requestQueue = _AsyncQueue<Map<String, dynamic>>();
  final _notificationQueue = _AsyncQueue<Map<String, dynamic>>();

  Future<void> connect(String url) async {
    client = HolonRPCClient(
      heartbeatIntervalMs: 250,
      heartbeatTimeoutMs: 250,
      reconnectMinDelayMs: 100,
      reconnectMaxDelayMs: 400,
      requestTimeoutMs: 3000,
    );

    client.register('Echo/Ping', (params) async {
      final cloned = Map<String, dynamic>.from(params);
      if (_isBridgeNotification(cloned)) {
        notificationCount += 1;
        _notificationQueue.add(cloned);
        return <String, dynamic>{};
      }

      requestCount += 1;
      _requestQueue.add(cloned);
      return <String, dynamic>{
        'from': label,
        'message': cloned['message'],
      };
    });

    await client.connect(url);
    final registration = await client.invoke(
      'rpc.register',
      params: <String, dynamic>{'name': holonName},
    );

    peerID = registration['peer']?.toString() ?? '';
    if (peerID.isEmpty) {
      throw StateError('missing peer id for $label');
    }
  }

  Future<Map<String, dynamic>> nextRequest(String label) {
    return _requestQueue.next(
      timeout: const Duration(seconds: 2),
      label: label,
    );
  }

  Future<Map<String, dynamic>> nextNotification(String label) {
    return _notificationQueue.next(
      timeout: const Duration(seconds: 3),
      label: label,
    );
  }

  Future<void> close() async {
    await client.close();
  }
}

class _AsyncQueue<T> {
  final Queue<T> _items = Queue<T>();
  Completer<T>? _waiter;

  void add(T value) {
    final waiter = _waiter;
    if (waiter != null && !waiter.isCompleted) {
      _waiter = null;
      waiter.complete(value);
      return;
    }
    _items.addLast(value);
  }

  Future<T> next({
    required Duration timeout,
    required String label,
  }) async {
    if (_items.isNotEmpty) {
      return _items.removeFirst();
    }

    final waiter = Completer<T>();
    _waiter = waiter;
    try {
      return await waiter.future.timeout(timeout);
    } on TimeoutException {
      if (identical(_waiter, waiter)) {
        _waiter = null;
      }
      throw TimeoutException('timeout waiting for $label');
    }
  }
}

bool _isBridgeNotification(Map<String, dynamic> params) {
  return params.containsKey('peer') &&
      (params.containsKey('result') || params.containsKey('error'));
}

List<Map<String, dynamic>> _parseFanOutEntries(Map<String, dynamic> response) {
  final rawEntries = response['value'];
  if (rawEntries is! List) {
    throw StateError('missing fan-out array: $response');
  }

  return rawEntries.map<Map<String, dynamic>>((entry) {
    if (entry is Map<String, dynamic>) {
      return entry;
    }
    if (entry is Map) {
      return entry.cast<String, dynamic>();
    }
    throw StateError('invalid fan-out entry: $entry');
  }).toList(growable: false);
}

void _assertRoutingFieldStripped(Map<String, dynamic> params) {
  expect(params.containsKey('_routing'), isFalse);
  expect(params.containsKey('_peer'), isFalse);
}

Future<bool> _canBindLoopbackTCP() async {
  try {
    final probe = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    await probe.close();
    return true;
  } on SocketException catch (error) {
    if (_isLocalBindDenied(error)) {
      return false;
    }
    rethrow;
  }
}

bool _isLocalBindDenied(Object error) {
  final text = error.toString().toLowerCase();
  return text.contains('operation not permitted') ||
      text.contains('permission denied') ||
      text.contains('errno = 1');
}
