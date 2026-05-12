/// Smoke test that drives gabriel-greeting-go over each transport
/// (stdio, tcp, unix) and verifies that observability signals
/// (logs/events) flow through a MemberRelay back to the local
/// Observability.
///
/// Run from the repo root:
///   op build gabriel-greeting-go --install
///   dart run sdk/dart-holons/example/host_smoke.dart
///
/// Exit code 0 if all scenarios pass (or skip on unsupported
/// platforms); 1 if any scenario fails.
///
/// This script is consumed by the user as a diagnostic tool and
/// will be wrapped by an ader e2e test in a follow-up brief
/// (Brief G3, ader-dart-go bouquet).
library;

import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

const _childSlug = 'gabriel-greeting-go';
const _childUid = 'smoke-target';
const _hostSlug = 'dart-host-smoke';
const _connectTimeout = Duration(seconds: 5);
const _signalTimeout = Duration(seconds: 10);
const _pollInterval = Duration(milliseconds: 100);

enum SmokeResult { pass, fail, skip }

typedef SmokeOutcome = ({
  SmokeResult result,
  String detail,
  Duration elapsed,
});

Future<int> main(List<String> args) async {
  print('=== host_smoke.dart ===');
  print('');

  if (!await _binaryAvailable()) {
    stderr.writeln(
      'binary not found, run `op build gabriel-greeting-go --install` first',
    );
    return 1;
  }

  final results = <({String label, SmokeOutcome outcome})>[];
  final scenarios = <(String, String)>[
    ('stdio direct', 'stdio'),
    ('tcp direct', 'tcp'),
    ('unix direct', 'unix'),
  ];

  for (var i = 0; i < scenarios.length; i += 1) {
    final (label, transport) = scenarios[i];
    final outcome = await runScenario(transport);
    results.add((label: label, outcome: outcome));
    print(_formatRow(i + 1, 4, label, outcome));
  }

  final cycleOutcome = await runCycle();
  const cycleLabel = 'cycle stdio→tcp→stdio';
  results.add((label: cycleLabel, outcome: cycleOutcome));
  print(_formatRow(4, 4, cycleLabel, cycleOutcome));

  print('');
  final pass =
      results.where((row) => row.outcome.result == SmokeResult.pass).length;
  final fail =
      results.where((row) => row.outcome.result == SmokeResult.fail).length;
  final skip =
      results.where((row) => row.outcome.result == SmokeResult.skip).length;
  print('Summary: $pass PASS / $fail FAIL / $skip SKIP');

  exitCode = fail > 0 ? 1 : 0;
  return exitCode;
}

Future<SmokeOutcome> runScenario(String transport) async {
  final stopwatch = Stopwatch()..start();
  try {
    if (transport == 'unix' && Platform.isWindows) {
      return _outcome(
        SmokeResult.skip,
        'unix socket not supported on this platform',
        stopwatch,
      );
    }
    return await _runScenario(transport, stopwatch);
  } on _ScenarioFailure catch (error) {
    return _outcome(SmokeResult.fail, error.message, stopwatch);
  } on UnsupportedError catch (error) {
    if (transport == 'unix') {
      return _outcome(SmokeResult.skip, '$error', stopwatch);
    }
    return _outcome(SmokeResult.fail, 'uncaught: $error', stopwatch);
  } on Object catch (error) {
    return _outcome(SmokeResult.fail, 'uncaught: $error', stopwatch);
  }
}

Future<SmokeOutcome> _runScenario(
  String transport,
  Stopwatch stopwatch,
) async {
  holons.reset();
  final local = holons.configure(
    holons.Config(slug: _hostSlug, instanceUid: 'smoke-$transport'),
    env: const {'OP_OBS': 'logs,events'},
  );

  ClientChannel? channel;
  holons.MemberRelay? relay;
  try {
    final connected = await _connectChild(transport);
    if (connected is! ClientChannel) {
      return _outcome(
        SmokeResult.fail,
        'unexpected channel type: ${connected.runtimeType}',
        stopwatch,
      );
    }
    channel = connected;

    relay = holons.MemberRelay(
      childSlug: _childSlug,
      childUid: _childUid,
      channel: channel,
      observability: holons.current(),
    );
    await relay.start();
    if (!relay.isRunning) {
      return _outcome(SmokeResult.fail, 'relay did not start', stopwatch);
    }

    final signal = await _waitForSignal(local, _signalTimeout);
    if (signal == null) {
      return _outcome(
        SmokeResult.fail,
        'no relayed signal within 10s; isRunning=${relay.isRunning}',
        stopwatch,
      );
    }

    return _outcome(
      SmokeResult.pass,
      'received ${signal.kind} in ${_formatDuration(signal.elapsed)}',
      stopwatch,
    );
  } finally {
    await _cleanup(relay, channel);
    holons.reset();
  }
}

Future<dynamic> _connectChild(String transport) async {
  try {
    return await holons.connect(
      _childSlug,
      holons.ConnectOptions(transport: transport, timeout: _connectTimeout),
    );
  } on Object catch (error) {
    throw _ScenarioFailure('connect failed: $error');
  }
}

Future<SmokeOutcome> runCycle() async {
  final stopwatch = Stopwatch()..start();
  try {
    final first = await runScenario('stdio');
    if (first.result != SmokeResult.pass) {
      return _outcome(
        SmokeResult.fail,
        'stdio init failed: ${first.detail}',
        stopwatch,
      );
    }

    holons.reset();
    final tcp = await runScenario('tcp');
    holons.reset();
    final second = await runScenario('stdio');

    final failed = <String>[
      if (tcp.result != SmokeResult.pass) 'tcp phase ${tcp.result.name}',
      if (second.result != SmokeResult.pass)
        'final stdio phase ${second.result.name}',
    ];
    if (failed.isNotEmpty) {
      return _outcome(
        SmokeResult.fail,
        '${failed.join(', ')}; tcp=${tcp.detail}; stdio=${second.detail}',
        stopwatch,
      );
    }

    return _outcome(SmokeResult.pass, 'all phases passed', stopwatch);
  } on Object catch (error) {
    return _outcome(SmokeResult.fail, 'uncaught: $error', stopwatch);
  } finally {
    holons.reset();
  }
}

Future<({String kind, Duration elapsed})?> _waitForSignal(
  holons.Observability observability,
  Duration timeout,
) async {
  final stopwatch = Stopwatch()..start();
  while (stopwatch.elapsed < timeout) {
    final logs = observability.logRing?.drain() ?? const <holons.LogEntry>[];
    if (logs.any((entry) => entry.slug == _childSlug)) {
      return (kind: 'log', elapsed: stopwatch.elapsed);
    }

    final events = observability.eventBus?.drain() ?? const <holons.Event>[];
    if (events.any((event) => event.slug == _childSlug)) {
      return (kind: 'event', elapsed: stopwatch.elapsed);
    }

    await Future<void>.delayed(_pollInterval);
  }
  return null;
}

Future<void> _cleanup(
  holons.MemberRelay? relay,
  ClientChannel? channel,
) async {
  if (relay != null) {
    try {
      await relay.stop();
    } on Object catch (error) {
      stderr.writeln('cleanup: relay stop failed: $error');
    }
  }
  if (channel != null) {
    try {
      await holons.disconnectAsync(channel);
    } on Object catch (error) {
      stderr.writeln('cleanup: channel disconnect failed: $error');
    }
  }
}

Future<bool> _binaryAvailable() async {
  if (await _commandExists(_childSlug)) {
    return true;
  }

  try {
    final installed = holons.Discover(
      holons.LOCAL,
      _childSlug,
      null,
      holons.INSTALLED,
      1,
      5000,
    );
    return installed.error == null &&
        installed.found.any((ref) => ref.error == null);
  } on Object {
    return false;
  }
}

Future<bool> _commandExists(String command) async {
  final executable = Platform.isWindows ? 'where' : 'which';
  try {
    final result = await Process.run(executable, <String>[command]);
    return result.exitCode == 0;
  } on Object {
    return false;
  }
}

SmokeOutcome _outcome(
  SmokeResult result,
  String detail,
  Stopwatch stopwatch,
) {
  stopwatch.stop();
  return (result: result, detail: detail, elapsed: stopwatch.elapsed);
}

String _formatRow(
  int index,
  int total,
  String label,
  SmokeOutcome outcome,
) {
  final prefix = '[$index/$total] ${label.padRight(28, '.')}';
  switch (outcome.result) {
    case SmokeResult.pass:
      return '$prefix PASS (${outcome.detail})';
    case SmokeResult.fail:
      return '$prefix FAIL: ${outcome.detail}';
    case SmokeResult.skip:
      return '$prefix SKIP (${outcome.detail})';
  }
}

String _formatDuration(Duration duration) {
  if (duration.inMilliseconds < 1000) {
    return '${duration.inMilliseconds}ms';
  }
  return '${(duration.inMilliseconds / 1000).toStringAsFixed(2)}s';
}

class _ScenarioFailure implements Exception {
  _ScenarioFailure(this.message);

  final String message;

  @override
  String toString() => message;
}
