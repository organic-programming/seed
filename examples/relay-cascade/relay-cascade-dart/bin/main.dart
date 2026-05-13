import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart' as describe_pb;
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obs_pb;
import 'package:protobuf/protobuf.dart';

import '../../cascade-node-dart/gen/dart/relay/v1/relay.pbgrpc.dart'
    as relay_pb;

const goSlug = 'cascade-node-go';
const dartSlug = 'cascade-node-dart';
const runTicks = 3;
const runPhases = 4;

const roleOrder = ['D', 'C', 'B', 'A'];

class RoleSpec {
  const RoleSpec({required this.slug, required this.binaryPath});

  final String slug;
  final String binaryPath;
}

class RoleRuntime {
  RoleRuntime({
    required this.role,
    required this.uid,
    required this.slug,
    required this.binaryPath,
    required this.listenUris,
    required this.clientTarget,
    required this.relayAddress,
  });

  final String role;
  final String uid;
  final String slug;
  final String binaryPath;
  final List<String> listenUris;
  final String clientTarget;
  final String relayAddress;
  String memberAddress = '';
  String metricsAddr = '';
  Process? process;
  ClientChannel? conn;
  final stdout = StringBuffer();
  final stderr = StringBuffer();
  StreamSubscription<String>? stdoutSub;
  StreamSubscription<String>? stderrSub;
}

class Cascade {
  Cascade({
    required this.phase,
    required this.transport,
    required this.runRoot,
    required this.roles,
  });

  final int phase;
  final String transport;
  final String runRoot;
  final Map<String, RoleRuntime> roles;

  Future<TickOutcome> runTick(int tick, double previousMetric) {
    final sender = 'phase-$phase-tick-$tick';
    return runTickWithSender(sender, previousMetric);
  }

  Future<TickOutcome> runTickWithSender(
    String sender,
    double previousMetric,
  ) async {
    try {
      final client = relay_pb.RelayServiceClient(roles['D']!.conn!);
      await client
          .tick(relay_pb.TickRequest(sender: sender, note: transport))
          .timeout(const Duration(seconds: 5));
    } catch (error) {
      final result = CheckResult(evidence: '$error');
      return TickOutcome(log: result, event: result, metric: result);
    }

    final logResult = await waitFor(const Duration(seconds: 3), () {
      return checkLog(sender);
    });
    final eventResult = await waitFor(const Duration(seconds: 3), checkEvent);
    var metricValue = previousMetric;
    final metricResult = await waitFor(const Duration(seconds: 3), () async {
      final result = await checkMetric(previousMetric);
      if (result.$1.pass) {
        metricValue = result.$2;
      }
      return result.$1;
    });
    return TickOutcome(
      log: logResult,
      event: eventResult,
      metric: metricResult,
      metricValue: metricValue,
    );
  }

  Future<TickOutcome> runLiveTick(
    LiveStreams? streams,
    Object? streamOpenError,
    int tick,
    double previousMetric,
  ) {
    final sender = 'phase-$phase-tick-$tick';
    return runLiveTickWithSender(
      streams,
      streamOpenError,
      sender,
      previousMetric,
    );
  }

  Future<TickOutcome> runLiveTickWithSender(
    LiveStreams? streams,
    Object? streamOpenError,
    String sender,
    double previousMetric,
  ) async {
    try {
      final client = relay_pb.RelayServiceClient(roles['D']!.conn!);
      await client
          .tick(relay_pb.TickRequest(sender: sender, note: transport))
          .timeout(const Duration(seconds: 5));
    } catch (error) {
      final result = CheckResult(evidence: '$error');
      return TickOutcome(log: result, event: result, metric: result);
    }

    var logResult = CheckResult(
      evidence: 'stream re-open failed: ${errorText(streamOpenError)}',
    );
    var eventResult = logResult;
    if (streamOpenError == null) {
      logResult = await waitForEvery(
        const Duration(seconds: 1),
        const Duration(milliseconds: 50),
        () async => checkLiveLog(streams, sender),
      );
      eventResult = await waitForEvery(
        const Duration(seconds: 1),
        const Duration(milliseconds: 50),
        () async => checkLiveEvent(streams),
      );
    }

    var metricValue = previousMetric;
    final metricResult = await waitForEvery(
      const Duration(seconds: 1),
      const Duration(milliseconds: 50),
      () async {
        final result = await checkMetric(previousMetric);
        if (result.$1.pass) {
          metricValue = result.$2;
        }
        return result.$1;
      },
    );
    return TickOutcome(
      log: logResult,
      event: eventResult,
      metric: metricResult,
      metricValue: metricValue,
    );
  }

  Future<CheckResult> checkLog(String sender) async {
    final entries = await readLogs(roles['A']!.conn!);
    for (final entry in entries) {
      if (entry.message != 'tick received') {
        continue;
      }
      if (entry.fields['sender'] != sender ||
          entry.fields['responder_uid'] != roles['D']!.uid) {
        continue;
      }
      final error = checkChain(entry.chain);
      if (error != null) {
        return CheckResult(
          evidence:
              'matching log has bad chain: $error entry=${marshalProto(entry)}',
        );
      }
      return CheckResult(pass: true, evidence: marshalProto(entry));
    }
    return CheckResult(
      evidence:
          'no relayed D tick log for sender=$sender in ${entries.length} A log entries',
    );
  }

  Future<CheckResult> checkEvent() async {
    final events = await readEvents(roles['A']!.conn!);
    for (final event in events) {
      if (event.type != obs_pb.EventType.INSTANCE_READY ||
          event.instanceUid != roles['D']!.uid) {
        continue;
      }
      final error = checkChain(event.chain);
      if (error != null) {
        return CheckResult(
          evidence:
              'matching event has bad chain: $error event=${marshalProto(event)}',
        );
      }
      return CheckResult(pass: true, evidence: marshalProto(event));
    }
    return CheckResult(
      evidence:
          'no relayed D INSTANCE_READY event in ${events.length} A events',
    );
  }

  CheckResult checkLiveLog(LiveStreams? streams, String sender) {
    if (streams == null) {
      return CheckResult(evidence: 'live streams are not open');
    }
    final entries = streams.logEntries();
    for (final entry in entries) {
      if (entry.message != 'tick received') {
        continue;
      }
      if (entry.fields['sender'] != sender ||
          entry.fields['responder_uid'] != roles['D']!.uid) {
        continue;
      }
      final error = checkChain(entry.chain);
      if (error != null) {
        return CheckResult(
          evidence:
              'matching live log has bad chain: $error entry=${marshalProto(entry)}',
        );
      }
      return CheckResult(pass: true, evidence: marshalProto(entry));
    }
    return CheckResult(
      evidence:
          'no live log found for sender=$sender current_d_uid=${roles['D']!.uid} within 1s (buffer=${entries.length}, stream_errors=${streams.streamErrors()})',
    );
  }

  CheckResult checkLiveEvent(LiveStreams? streams) {
    if (streams == null) {
      return CheckResult(evidence: 'live streams are not open');
    }
    final events = streams.eventEntries();
    for (final event in events) {
      if (event.type != obs_pb.EventType.INSTANCE_READY ||
          event.instanceUid != roles['D']!.uid) {
        continue;
      }
      final error = checkChain(event.chain);
      if (error != null) {
        return CheckResult(
          evidence:
              'matching live event has bad chain: $error event=${marshalProto(event)}',
        );
      }
      return CheckResult(pass: true, evidence: marshalProto(event));
    }
    return CheckResult(
      evidence:
          'no live INSTANCE_READY event found for current_d_uid=${roles['D']!.uid} within 1s (buffer=${events.length}, stream_errors=${streams.streamErrors()})',
    );
  }

  Future<(CheckResult, double)> checkMetric(double previous) async {
    final body = await fetchMetrics(roles['D']!.metricsAddr);
    final value = parseCascadeTicks(body, roles['D']!.uid);
    if (value == null) {
      return (CheckResult(evidence: body), previous);
    }
    if (value <= previous) {
      return (
        CheckResult(
          evidence:
              'cascade_ticks_total=$value did not increase beyond $previous\n$body',
        ),
        value,
      );
    }
    return (
      CheckResult(pass: true, evidence: 'cascade_ticks_total=$value'),
      value
    );
  }

  String? checkChain(List<obs_pb.ChainHop> chain) {
    const wantRoles = ['D', 'C', 'B'];
    if (chain.length < wantRoles.length) {
      return 'chain length ${chain.length} < ${wantRoles.length}';
    }
    for (var i = 0; i < wantRoles.length; i++) {
      final want = roles[wantRoles[i]]!;
      if (chain[i].slug != want.slug || chain[i].instanceUid != want.uid) {
        return 'hop $i = ${chain[i].slug}/${chain[i].instanceUid}, want ${want.slug}/${want.uid}';
      }
    }
    return null;
  }

  Future<void> stop() async {
    for (final role in roleOrder.reversed) {
      final runtime = roles[role];
      if (runtime == null) {
        continue;
      }
      try {
        await runtime.conn?.shutdown().timeout(const Duration(seconds: 1));
      } on Object {
        // Continue process teardown even if the client channel is wedged.
      }
      runtime.conn = null;
      runtime.process?.kill(ProcessSignal.sigterm);
    }

    final deadline = DateTime.now().add(const Duration(seconds: 3));
    for (final role in roleOrder) {
      final runtime = roles[role];
      final process = runtime?.process;
      if (runtime == null || process == null) {
        continue;
      }
      final remaining = deadline.difference(DateTime.now());
      try {
        await process.exitCode.timeout(
          remaining.isNegative ? const Duration(milliseconds: 10) : remaining,
        );
      } on TimeoutException {
        process.kill(ProcessSignal.sigkill);
        await process.exitCode.timeout(const Duration(seconds: 1));
      }
      await runtime.stdoutSub?.cancel().timeout(
            const Duration(seconds: 1),
            onTimeout: () {},
          );
      await runtime.stderrSub?.cancel().timeout(
            const Duration(seconds: 1),
            onTimeout: () {},
          );
    }
  }
}

class CheckResult {
  const CheckResult({this.pass = false, this.evidence = ''});

  final bool pass;
  final String evidence;
}

class TickOutcome {
  const TickOutcome({
    required this.log,
    required this.event,
    required this.metric,
    this.metricValue = 0,
  });

  final CheckResult log;
  final CheckResult event;
  final CheckResult metric;
  final double metricValue;
}

class CascadePattern {
  const CascadePattern({required this.name, required this.roles});

  final String name;
  final Map<String, RoleSpec> roles;
}

Future<void> main(List<String> args) async {
  final liveStream = args.contains('--live-stream');
  final multiPattern = args.contains('--multi-pattern');

  try {
    if (multiPattern) {
      await runMultiPattern();
    } else if (liveStream) {
      await runLiveStream();
    } else {
      await run();
    }
  } catch (error) {
    stderr.writeln('\nFAIL: $error');
    exitCode = 1;
  }
}

Future<void> run() async {
  final binaryPath = await findCascadeNodeBinary();
  final runRoot = '${Platform.environment['HOME']}/.op/run';
  final transports = ['tcp', 'unix', 'tcp', 'unix'];

  print('=== relay-cascade-dart ===');
  print('');

  var totalPass = 0;
  var totalFail = 0;
  var previous = '';
  for (var phaseIdx = 0; phaseIdx < transports.length; phaseIdx++) {
    final phaseNo = phaseIdx + 1;
    final transport = transports[phaseIdx];
    if (previous.isEmpty) {
      print('Phase $phaseNo/$runPhases: transport=$transport');
    } else if (phaseNo == runPhases && transport == transports.first) {
      print('Phase $phaseNo/$runPhases: transport=$transport (cycle wrap)');
    } else {
      print(
        'Phase $phaseNo/$runPhases: transport=$transport (switching from $previous)',
      );
    }

    final spawnStart = DateTime.now();
    Cascade? cascade;
    try {
      cascade = await spawnCascade(phaseNo, transport, binaryPath, runRoot);
    } catch (error) {
      totalFail += runTicks;
      print('  spawn FAIL: $error\n');
      previous = transport;
      continue;
    }
    print('  spawned 4 nodes in ${elapsed(spawnStart)}');

    var previousMetric = 0.0;
    for (var tick = 1; tick <= runTicks; tick++) {
      final tickStart = DateTime.now();
      final result = await cascade.runTick(tick, previousMetric);
      if (result.metric.pass) {
        previousMetric = result.metricValue;
      }
      final overall =
          result.log.pass && result.event.pass && result.metric.pass;
      if (overall) {
        totalPass++;
      } else {
        totalFail++;
      }
      final status = overall ? 'PASS' : 'FAIL';
      print(
        '  Tick $tick/$runTicks: log ${passText(result.log.pass)}, event ${passText(result.event.pass)}, metric ${passText(result.metric.pass)} (overall $status in ${elapsed(tickStart)})',
      );
      if (!overall) {
        printFailureEvidence('log', result.log);
        printFailureEvidence('event', result.event);
        printFailureEvidence('metric', result.metric);
      }
    }
    await cascade.stop();
    print('');
    previous = transport;
  }

  print(
      'Summary: ${totalPass + totalFail} ticks, $totalPass PASS, $totalFail FAIL');
  if (totalFail > 0) {
    throw StateError('$totalFail tick(s) failed');
  }
}

Future<void> runLiveStream() async {
  final binaryPath = await findCascadeNodeBinary();
  final runRoot = '${Platform.environment['HOME']}/.op/run';
  final transports = ['tcp', 'unix', 'tcp', 'unix'];

  print('=== relay-cascade-dart --live-stream ===');
  print('');
  print('Setup: opening long-lived Follow:true streams on A');
  print('       (initial transport: tcp, port 9090)');
  print('');

  var totalPass = 0;
  var totalFail = 0;
  Cascade? cascade;
  LiveStreams? streams;
  Object? streamOpenError;
  try {
    for (var phaseIdx = 0; phaseIdx < transports.length; phaseIdx++) {
      final phaseNo = phaseIdx + 1;
      final transport = transports[phaseIdx];
      if (phaseNo == 1) {
        print('Phase $phaseNo/$runPhases: initial chain ($transport)');
      } else {
        print('Phase $phaseNo/$runPhases: respawn on $transport');
        final killStart = DateTime.now();
        await streams?.stop();
        await cascade?.stop();
        print('  killed 4 nodes in ${elapsed(killStart)}');
      }

      final spawnStart = DateTime.now();
      try {
        cascade = await spawnCascade(phaseNo, transport, binaryPath, runRoot);
      } catch (error) {
        totalFail += runTicks;
        print('  spawn FAIL: $error\n');
        streams = null;
        streamOpenError = error;
        continue;
      }
      print('  spawned 4 nodes in ${elapsed(spawnStart)}');
      if (phaseNo > 1) {
        print('  re-opening Follow:true streams on new A');
      }
      try {
        streams = startLiveStreams(cascade.roles['A']!.conn!);
        streamOpenError = null;
      } catch (error) {
        streams = null;
        streamOpenError = error;
        print('  stream re-open failed: $error');
      }

      var previousMetric = 0.0;
      for (var tick = 1; tick <= runTicks; tick++) {
        final tickStart = DateTime.now();
        final result = await cascade.runLiveTick(
          streams,
          streamOpenError,
          tick,
          previousMetric,
        );
        if (result.metric.pass) {
          previousMetric = result.metricValue;
        }
        final overall =
            result.log.pass && result.event.pass && result.metric.pass;
        if (overall) {
          totalPass++;
        } else {
          totalFail++;
        }
        print(
          '  Tick $tick/$runTicks: log ${passText(result.log.pass)}, event ${passText(result.event.pass)}, metric ${passText(result.metric.pass)} (overall ${passText(overall)} in ${elapsed(tickStart)})',
        );
        if (!overall) {
          printFailureEvidence('log', result.log);
          printFailureEvidence('event', result.event);
          printFailureEvidence('metric', result.metric);
        }
      }
      print('');
    }
  } finally {
    await streams?.stop();
    await cascade?.stop();
  }

  print(
      'Summary: $totalPass PASS / $totalFail FAIL across ${totalPass + totalFail} ticks');
  if (totalFail > 0) {
    throw StateError('$totalFail tick(s) failed');
  }
}

Future<void> runMultiPattern() async {
  final dartBinary = await findHolonBinary(dartSlug);
  final goBinary = await findHolonBinary(goSlug);
  final patterns = [
    CascadePattern(
      name: 'dart-dart-dart-dart',
      roles: {
        'A': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'B': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'C': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'D': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
      },
    ),
    CascadePattern(
      name: 'dart-dart-go-dart',
      roles: {
        'A': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'B': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'C': RoleSpec(slug: goSlug, binaryPath: goBinary),
        'D': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
      },
    ),
    CascadePattern(
      name: 'dart-dart-go-go',
      roles: {
        'A': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'B': RoleSpec(slug: dartSlug, binaryPath: dartBinary),
        'C': RoleSpec(slug: goSlug, binaryPath: goBinary),
        'D': RoleSpec(slug: goSlug, binaryPath: goBinary),
      },
    ),
  ];
  final runRoot = '${Platform.environment['HOME']}/.op/run';
  final transports = ['tcp', 'unix', 'tcp', 'unix'];

  print('=== relay-cascade-dart (multi-pattern) ===');
  print('');

  var totalPass = 0;
  var totalFail = 0;
  for (var patternIdx = 0; patternIdx < patterns.length; patternIdx++) {
    final pattern = patterns[patternIdx];
    print('Pattern ${patternIdx + 1}/${patterns.length}: ${pattern.name}');
    var patternPass = 0;
    for (var phaseIdx = 0; phaseIdx < transports.length; phaseIdx++) {
      final phaseNo = phaseIdx + 1;
      final transport = transports[phaseIdx];
      final spawnStart = DateTime.now();
      Cascade cascade;
      try {
        cascade = await spawnPatternCascade(
          phaseNo,
          transport,
          pattern.roles,
          runRoot,
        );
      } catch (error) {
        totalFail += runTicks;
        print('  Phase $phaseNo/$runPhases ($transport): spawn FAIL ($error)');
        continue;
      }

      LiveStreams? streams;
      Object? streamOpenError;
      try {
        streams = startLiveStreams(cascade.roles['A']!.conn!);
        final ready = await waitForEvery(
          const Duration(seconds: 5),
          const Duration(milliseconds: 50),
          () async => cascade.checkLiveEvent(streams),
        );
        if (!ready.pass) {
          streamOpenError = 'live relay readiness: ${ready.evidence}';
        }
      } catch (error) {
        streamOpenError = error;
      }

      var previousMetric = 0.0;
      final results = <String>[];
      final evidence = <String>[];
      for (var tick = 1; tick <= runTicks; tick++) {
        final sender = '${pattern.name}-phase-$phaseNo-tick-$tick';
        final result = await cascade.runLiveTickWithSender(
          streams,
          streamOpenError,
          sender,
          previousMetric,
        );
        if (result.metric.pass) {
          previousMetric = result.metricValue;
        }
        final overall =
            result.log.pass && result.event.pass && result.metric.pass;
        if (overall) {
          patternPass++;
          totalPass++;
          results.add('Tick $tick PASS');
        } else {
          totalFail++;
          results.add('Tick $tick FAIL (${failureSummary(result)})');
          evidence.add(
            '      Tick $tick evidence: ${compactEvidence(result)}',
          );
        }
      }
      print(
        '  Phase $phaseNo/$runPhases ($transport): ${results.join(', ')} (spawned in ${elapsed(spawnStart)})',
      );
      for (final line in evidence) {
        print(line);
      }
      await streams?.stop();
      await cascade.stop();
    }
    print('  Subtotal: $patternPass/12 PASS');
    print('');
  }

  print(
      'Summary: $totalPass PASS / $totalFail FAIL across ${totalPass + totalFail} ticks');
  if (totalFail > 0) {
    throw StateError('$totalFail tick(s) failed');
  }
}

Future<Cascade> spawnCascade(
  int phase,
  String transport,
  String binaryPath,
  String runRoot,
) {
  final specs = {
    'A': RoleSpec(slug: dartSlug, binaryPath: binaryPath),
    'B': RoleSpec(slug: dartSlug, binaryPath: binaryPath),
    'C': RoleSpec(slug: dartSlug, binaryPath: binaryPath),
    'D': RoleSpec(slug: dartSlug, binaryPath: binaryPath),
  };
  return spawnPatternCascade(phase, transport, specs, runRoot);
}

Future<Cascade> spawnPatternCascade(
  int phase,
  String transport,
  Map<String, RoleSpec> specs,
  String runRoot,
) async {
  final cascade = Cascade(
    phase: phase,
    transport: transport,
    runRoot: runRoot,
    roles: {},
  );
  for (final role in roleOrder) {
    final spec = specs[role];
    if (spec == null || spec.slug.isEmpty || spec.binaryPath.isEmpty) {
      throw StateError('missing role spec for $role');
    }
    final runtime = newRoleRuntime(phase, transport, role, spec);
    cascade.roles[role] = runtime;
    await deletePath('${runRootPath(runRoot, runtime.slug, runtime.uid)}');
    if (transport == 'unix') {
      for (final uri in runtime.listenUris) {
        if (uri.startsWith('unix://')) {
          await deletePath(uri.substring('unix://'.length));
        }
      }
    }
  }
  for (var i = 0; i < roleOrder.length; i++) {
    final role = roleOrder[i];
    final runtime = cascade.roles[role]!;
    if (i > 0) {
      runtime.memberAddress = cascade.roles[roleOrder[i - 1]]!.relayAddress;
    }
    try {
      await cascade.startRole(runtime);
    } catch (_) {
      await cascade.stop();
      rethrow;
    }
  }
  await Future<void>.delayed(const Duration(milliseconds: 150));
  return cascade;
}

RoleRuntime newRoleRuntime(
  int phase,
  String transport,
  String role,
  RoleSpec spec,
) {
  final lower = role.toLowerCase();
  final uid = 'relay-p${phase.toString().padLeft(2, '0')}-$lower';
  switch (transport) {
    case 'tcp':
      final port = {'A': 9090, 'B': 9091, 'C': 9092, 'D': 9093}[role]!;
      final uri = 'tcp://127.0.0.1:$port';
      return RoleRuntime(
        role: role,
        uid: uid,
        slug: spec.slug,
        binaryPath: spec.binaryPath,
        listenUris: [uri],
        clientTarget: '127.0.0.1:$port',
        relayAddress: uri,
      );
    case 'unix':
      final path = '/tmp/relay-cascade-$lower.sock';
      return RoleRuntime(
        role: role,
        uid: uid,
        slug: spec.slug,
        binaryPath: spec.binaryPath,
        listenUris: ['unix://$path'],
        clientTarget: 'unix://$path',
        relayAddress: 'unix://$path',
      );
    default:
      throw StateError('unknown transport $transport');
  }
}

extension CascadeStart on Cascade {
  Future<void> startRole(RoleRuntime runtime) async {
    final args = <String>['serve'];
    for (final uri in runtime.listenUris) {
      args.addAll(['--listen', uri]);
    }
    if (runtime.memberAddress.isNotEmpty) {
      final child = roles[childRole(runtime.role)]!;
      args.addAll(['--member', '${child.slug}=${runtime.memberAddress}']);
    }

    final env = Map<String, String>.from(Platform.environment)
      ..addAll({
        'OP_OBS': 'logs,events,metrics,prom',
        'OP_RUN_DIR': runRoot,
        'OP_INSTANCE_UID': runtime.uid,
        'OP_ORGANISM_UID': roles['A']!.uid,
        'OP_ORGANISM_SLUG': roles['A']!.slug,
        'OP_PROM_ADDR': '127.0.0.1:0',
      });

    final process = await Process.start(
      runtime.binaryPath,
      args,
      environment: env,
      runInShell: false,
    );
    runtime.process = process;
    runtime.stdoutSub =
        process.stdout.transform(utf8.decoder).listen(runtime.stdout.write);
    runtime.stderrSub =
        process.stderr.transform(utf8.decoder).listen(runtime.stderr.write);

    final meta = await waitMeta(
      runRoot,
      runtime.slug,
      runtime.uid,
      const Duration(seconds: 10),
    );
    runtime.metricsAddr = meta.metricsAddr;
    runtime.conn =
        await dialReady(runtime.clientTarget, const Duration(seconds: 10));
  }
}

String childRole(String role) {
  switch (role) {
    case 'A':
      return 'B';
    case 'B':
      return 'C';
    case 'C':
      return 'D';
    default:
      return '';
  }
}

class LiveStreams {
  LiveStreams(this.logSub, this.eventSub);

  final StreamSubscription<obs_pb.LogEntry> logSub;
  final StreamSubscription<obs_pb.EventInfo> eventSub;
  final List<obs_pb.LogEntry> _logs = [];
  final List<obs_pb.EventInfo> _events = [];
  final List<String> _errs = [];
  bool _stopping = false;

  List<obs_pb.LogEntry> logEntries() => List.of(_logs);
  List<obs_pb.EventInfo> eventEntries() => List.of(_events);
  List<String> streamErrors() => List.of(_errs);

  void addLog(obs_pb.LogEntry entry) {
    _logs.add(entry);
  }

  void addEvent(obs_pb.EventInfo event) {
    _events.add(event);
  }

  void addErr(String message) {
    if (!_stopping) {
      _errs.add(message);
    }
  }

  Future<void> stop() async {
    _stopping = true;
    await Future.wait<void>([
      logSub.cancel().timeout(const Duration(seconds: 1), onTimeout: () {}),
      eventSub.cancel().timeout(const Duration(seconds: 1), onTimeout: () {}),
    ]);
  }
}

LiveStreams startLiveStreams(ClientChannel conn) {
  final client = obs_pb.HolonObservabilityClient(conn);
  late final LiveStreams streams;
  final logSub = client
      .logs(obs_pb.LogsRequest(minLevel: obs_pb.LogLevel.INFO, follow: true))
      .listen(
        (entry) => streams.addLog(entry),
        onError: (Object error) => streams.addErr('logs stream ended: $error'),
        onDone: () => streams.addErr('logs stream completed'),
      );
  final eventSub = client.events(obs_pb.EventsRequest(follow: true)).listen(
        (event) => streams.addEvent(event),
        onError: (Object error) =>
            streams.addErr('events stream ended: $error'),
        onDone: () => streams.addErr('events stream completed'),
      );
  streams = LiveStreams(logSub, eventSub);
  return streams;
}

Future<List<obs_pb.LogEntry>> readLogs(ClientChannel conn) async {
  final client = obs_pb.HolonObservabilityClient(conn);
  final stream = client.logs(
    obs_pb.LogsRequest(minLevel: obs_pb.LogLevel.INFO, follow: false),
  );
  return stream.toList().timeout(const Duration(seconds: 2));
}

Future<List<obs_pb.EventInfo>> readEvents(ClientChannel conn) async {
  final client = obs_pb.HolonObservabilityClient(conn);
  final stream = client.events(obs_pb.EventsRequest(follow: false));
  return stream.toList().timeout(const Duration(seconds: 2));
}

Future<CheckResult> waitFor(
  Duration timeout,
  FutureOr<CheckResult> Function() fn,
) {
  return waitForEvery(timeout, const Duration(milliseconds: 100), fn);
}

Future<CheckResult> waitForEvery(
  Duration timeout,
  Duration interval,
  FutureOr<CheckResult> Function() fn,
) async {
  final deadline = DateTime.now().add(timeout);
  var last = const CheckResult();
  while (true) {
    last = await fn();
    if (last.pass || DateTime.now().isAfter(deadline)) {
      return last;
    }
    await Future<void>.delayed(interval);
  }
}

class MetaJson {
  const MetaJson({required this.metricsAddr});

  final String metricsAddr;
}

Future<MetaJson> waitMeta(
  String runRoot,
  String slug,
  String uid,
  Duration timeout,
) async {
  final deadline = DateTime.now().add(timeout);
  final file = File('${runRootPath(runRoot, slug, uid)}/meta.json');
  Object? lastError;
  while (DateTime.now().isBefore(deadline)) {
    try {
      final payload =
          jsonDecode(await file.readAsString()) as Map<String, dynamic>;
      if (payload['uid'] == uid &&
          (payload['metrics_addr'] as String? ?? '').isNotEmpty) {
        return MetaJson(metricsAddr: payload['metrics_addr'] as String);
      }
    } catch (error) {
      lastError = error;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('meta not ready for $uid: ${lastError ?? 'not found'}');
}

Future<ClientChannel> dialReady(String target, Duration timeout) async {
  final deadline = DateTime.now().add(timeout);
  Object? lastError;
  while (DateTime.now().isBefore(deadline)) {
    final conn = dialTarget(target);
    try {
      await describeReady(conn, const Duration(seconds: 1));
      return conn;
    } catch (error) {
      lastError = error;
      await conn.shutdown();
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('${lastError ?? 'dial timeout'}');
}

ClientChannel dialTarget(String target) {
  if (target.startsWith('unix://')) {
    final socketPath = target.substring('unix://'.length);
    return ClientChannel(
      InternetAddress(socketPath, type: InternetAddressType.unix),
      port: 0,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
  }
  final index = target.lastIndexOf(':');
  if (index <= 0 || index == target.length - 1) {
    throw ArgumentError('invalid host:port target: $target');
  }
  return ClientChannel(
    target.substring(0, index),
    port: int.parse(target.substring(index + 1)),
    options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
  );
}

Future<void> describeReady(ClientChannel conn, Duration timeout) async {
  final deadline = DateTime.now().add(timeout);
  Object? lastError;
  while (DateTime.now().isBefore(deadline)) {
    try {
      await describe_pb.HolonMetaClient(conn)
          .describe(describe_pb.DescribeRequest())
          .timeout(const Duration(milliseconds: 500));
      return;
    } catch (error) {
      lastError = error;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('${lastError ?? 'describe timeout'}');
}

Future<String> fetchMetrics(String addr) async {
  final client = HttpClient()..connectionTimeout = const Duration(seconds: 2);
  try {
    final request = await client.getUrl(Uri.parse(addr));
    final response = await request.close().timeout(const Duration(seconds: 2));
    final body = await utf8.decodeStream(response);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw StateError('metrics HTTP ${response.statusCode}: $body');
    }
    return body;
  } finally {
    client.close(force: true);
  }
}

double? parseCascadeTicks(String body, String uid) {
  for (final line in const LineSplitter().convert(body)) {
    if (!line.startsWith('cascade_ticks_total{') ||
        !line.contains('responder_uid="$uid"')) {
      continue;
    }
    final parts = line.split(RegExp(r'\s+'));
    if (parts.length < 2) {
      continue;
    }
    final value = double.tryParse(parts.last);
    if (value != null) {
      return value;
    }
  }
  return null;
}

Future<String> findCascadeNodeBinary() {
  return findHolonBinary(dartSlug);
}

Future<String> findHolonBinary(String slug) async {
  final envName =
      'CASCADE_NODE_${slug.replaceFirst('cascade-node-', '').toUpperCase()}_BIN';
  final override = Platform.environment[envName]?.trim() ?? '';
  if (override.isNotEmpty) {
    return override;
  }
  final home = Platform.environment['HOME'];
  if (home == null || home.isEmpty) {
    throw StateError('HOME is not set');
  }
  final root = Directory('$home/.op/bin/$slug.holon/bin');
  if (!await root.exists()) {
    throw StateError(
        '$slug binary not found under ${root.path}; run op build $slug --install');
  }
  final osToken =
      Platform.operatingSystem == 'macos' ? 'darwin' : Platform.operatingSystem;
  String? found;
  await for (final entity in root.list(recursive: true, followLinks: false)) {
    if (entity is! File || entity.uri.pathSegments.last != slug) {
      continue;
    }
    if (entity.path.contains(osToken) || found == null) {
      found = entity.path;
    }
  }
  if (found == null) {
    throw StateError(
        '$slug binary not found under ${root.path}; run op build $slug --install');
  }
  return found;
}

String passText(bool pass) => pass ? 'PASS' : 'FAIL';

void printFailureEvidence(String family, CheckResult result) {
  if (result.pass) {
    return;
  }
  final evidence =
      result.evidence.trim().isEmpty ? '<empty>' : result.evidence.trim();
  print('    $family evidence: $evidence');
}

String failureSummary(TickOutcome result) {
  final families = <String>[];
  if (!result.log.pass) {
    families.add('log family');
  }
  if (!result.event.pass) {
    families.add('event family');
  }
  if (!result.metric.pass) {
    families.add('metric family');
  }
  return families.isEmpty ? 'unknown' : families.join(', ');
}

String compactEvidence(TickOutcome result) {
  final parts = <String>[];
  if (!result.log.pass) {
    parts.add('log=${truncateEvidence(result.log.evidence)}');
  }
  if (!result.event.pass) {
    parts.add('event=${truncateEvidence(result.event.evidence)}');
  }
  if (!result.metric.pass) {
    parts.add('metric=${truncateEvidence(result.metric.evidence)}');
  }
  return parts.join('; ');
}

String truncateEvidence(String value) {
  final compact =
      value.trim().split(RegExp(r'\s+')).where((s) => s.isNotEmpty).join(' ');
  if (compact.isEmpty) {
    return '<empty>';
  }
  if (compact.length <= 240) {
    return compact;
  }
  return '${compact.substring(0, 240)}...';
}

String errorText(Object? error) => error == null ? '<nil>' : '$error';

String marshalProto(GeneratedMessage message) {
  return jsonEncode(message.toProto3Json());
}

String elapsed(DateTime start) {
  final ms = DateTime.now().difference(start).inMilliseconds;
  if (ms < 1000) {
    return '${ms}ms';
  }
  if (ms % 1000 == 0) {
    return '${ms ~/ 1000}s';
  }
  var seconds = (ms / 1000).toStringAsFixed(3);
  seconds = seconds.replaceFirst(RegExp(r'0+$'), '');
  seconds = seconds.replaceFirst(RegExp(r'\.$'), '');
  return '${seconds}s';
}

String runRootPath(String runRoot, String slug, String uid) {
  return '$runRoot/$slug/$uid';
}

Future<void> deletePath(String path) async {
  final type = await FileSystemEntity.type(path);
  switch (type) {
    case FileSystemEntityType.notFound:
      return;
    case FileSystemEntityType.directory:
      await Directory(path).delete(recursive: true);
      return;
    default:
      await File(path).delete();
  }
}
