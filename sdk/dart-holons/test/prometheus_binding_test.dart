import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' show ServeOptions, startWithOptions;
import 'package:holons/src/observability.dart' as obs;
import 'package:test/test.dart';

void main() {
  setUp(obs.reset);
  tearDown(obs.reset);

  for (final address in ['127.0.0.1:0', ':0', '0.0.0.0:0']) {
    test('explicit Prometheus address $address is preserved without binding', () {
      expect(obs.PromServer(address).addr, address);
      final configured = obs.fromEnv(
        const obs.Config(slug: 'prom-config-test'),
        {'OP_OBS': 'metrics,prom', 'OP_PROM_ADDR': address},
      );
      expect(configured.cfg.promAddr, address);
    });
  }

  for (final address in ['', '127.0.0.1:0', ':0', '0.0.0.0:0']) {
    final wildcard = address == ':0' || address == '0.0.0.0:0';
    test(
      'PromServer binds ${address.isEmpty ? 'default loopback' : address}',
      () async {
        obs.configure(
          const obs.Config(slug: 'prom-bind-test'),
          env: const {'OP_OBS': 'metrics,prom'},
        );
        final server = obs.PromServer(address);
        final endpoint = Uri.parse(await server.start());
        addTearDown(server.close);
        await _checkBinding(endpoint.port, wildcard: wildcard);
      },
      skip: Platform.isWindows && wildcard
          ? 'Wildcard socket binding is skipped on Windows to avoid firewall authorization prompts; configuration is tested without binding.'
          : false,
    );
  }

  for (final address in ['', '127.0.0.1:0', '0.0.0.0:0']) {
    final wildcard = address == '0.0.0.0:0';
    test(
      'serve Prometheus ${address.isEmpty ? 'defaults to loopback' : 'preserves $address'}',
      () async {
        final messages = <String>[];
        final env = {'OP_OBS': 'metrics,prom', 'OP_PROM_ADDR': address};
        obs.fromEnv(const obs.Config(slug: 'prom-serve-test'), env);
        final server = await startWithOptions(
          'tcp://127.0.0.1:0',
          const <Service>[],
          options: ServeOptions(
            describe: false,
            environment: env,
            logger: messages.add,
          ),
        );
        addTearDown(server.stop);
        const prefix = 'Prometheus /metrics listening on ';
        final message = messages.singleWhere(
          (message) => message.startsWith(prefix),
        );
        final endpoint = Uri.parse(message.substring(prefix.length));
        await _checkBinding(endpoint.port, wildcard: wildcard);
      },
      skip: Platform.isWindows && wildcard
          ? 'Wildcard socket binding is skipped on Windows to avoid firewall authorization prompts; configuration is tested without binding.'
          : false,
    );
  }
}

Future<void> _checkBinding(int port, {required bool wildcard}) async {
  final client = HttpClient()..findProxy = (_) => 'DIRECT';
  try {
    final request = await client.getUrl(
      Uri.parse('http://127.0.0.1:$port/metrics'),
    );
    final response = await request.close();
    expect(response.statusCode, HttpStatus.ok);
    await response.drain<void>();
  } finally {
    client.close(force: true);
  }

  // The advertised URL maps wildcard bindings to 127.0.0.1. Test the actual
  // socket using a second loopback address, without relying on a LAN adapter.
  Socket? alternate;
  try {
    alternate = await Socket.connect(
      '127.0.0.2',
      port,
      timeout: const Duration(seconds: 1),
    );
  } on SocketException {
    expect(
      wildcard,
      isFalse,
      reason: 'an explicit wildcard must accept another loopback address',
    );
    return;
  } finally {
    alternate?.destroy();
  }
  expect(
    wildcard,
    isTrue,
    reason: 'the default must bind only 127.0.0.1, not all interfaces',
  );
}
