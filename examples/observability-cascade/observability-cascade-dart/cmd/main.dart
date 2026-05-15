import 'dart:io' as io;

import 'package:fixnum/fixnum.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as sdk;

import '../../observability-cascade-dart-node/gen/dart/relay/v1/relay.pbgrpc.dart'
    as relay_pb;
import '../gen/dart/observability_cascade/v1/service.pbgrpc.dart' as cascade_pb;
import '../gen/describe_generated.dart';

const goSlug = 'observability-cascade-go-node';
const dartSlug = 'observability-cascade-dart-node';
const runTicks = 3;

class LanguageMember {
  const LanguageMember({
    required this.lang,
    required this.slug,
    required this.binary,
  });

  final String lang;
  final String slug;
  final String binary;
}

class NamedPattern {
  const NamedPattern({required this.name, required this.members});

  final String name;
  final List<LanguageMember> members;
}

class ObservabilityCascadeRpc
    extends cascade_pb.ObservabilityCascadeServiceBase {
  @override
  Future<cascade_pb.CascadeReport> runDefault(
    ServiceCall call,
    cascade_pb.RunRequest request,
  ) async {
    return runReport('default', ownLanguageMembers(), live: false);
  }

  @override
  Future<cascade_pb.CascadeReport> runLiveStream(
    ServiceCall call,
    cascade_pb.RunRequest request,
  ) async {
    return runReport('live-stream', ownLanguageMembers(), live: true);
  }

  @override
  Future<cascade_pb.MultiPatternReport> runMultiPattern(
    ServiceCall call,
    cascade_pb.RunRequest request,
  ) async {
    return runMultiPatternReport();
  }
}

Future<void> main(List<String> args) async {
  try {
    if (args.isNotEmpty && canonicalCommand(args.first) == 'serve') {
      await serveComposite(args.sublist(1));
      return;
    }
    var failed = 0;
    if (args.contains('--multi-pattern')) {
      final report = await runMultiPatternReport(emit: true);
      failed = report.totalFail;
    } else {
      final live = args.contains('--live-stream');
      final report = await runReport(
        live ? 'live-stream' : 'default',
        ownLanguageMembers(),
        live: live,
        emit: true,
      );
      failed = report.fail;
    }
    if (failed > 0) {
      io.exitCode = 1;
    }
  } catch (error) {
    io.stderr.writeln('\nFAIL: $error');
    io.exitCode = 1;
  }
}

Future<void> serveComposite(List<String> args) {
  sdk.useStaticResponse(staticDescribeResponse());
  final parsed = sdk.parseOptions(args);
  return sdk.runWithOptions(
    parsed.listenUri,
    <Service>[ObservabilityCascadeRpc()],
    options: sdk.ServeOptions(reflect: parsed.reflect),
  );
}

Future<cascade_pb.MultiPatternReport> runMultiPatternReport({
  bool emit = false,
}) async {
  final totalWatch = Stopwatch()..start();
  final patterns = dartPatterns();
  final out = cascade_pb.MultiPatternReport();
  if (emit) {
    io.stdout.writeln('=== observability-cascade-dart --multi-pattern ===');
    io.stdout.writeln();
  }
  for (var i = 0; i < patterns.length; i++) {
    final pattern = patterns[i];
    if (emit) {
      io.stdout.writeln('Pattern ${i + 1}/${patterns.length}: ${pattern.name}');
    }
    final report =
        await runReport(pattern.name, pattern.members, live: true, emit: emit);
    out.patterns.add(report);
    out.totalPass += report.pass;
    out.totalFail += report.fail;
    if (emit) {
      final status = report.fail > 0 ? 'FAIL' : 'PASS';
      io.stdout.writeln(
        'Pattern ${pattern.name}: ${report.pass}/${report.ticks} $status (elapsed=${elapsedText(report.elapsedUs.toInt())})',
      );
      io.stdout.writeln();
    }
  }
  totalWatch.stop();
  out.totalElapsedUs = Int64(totalWatch.elapsedMicroseconds);
  if (emit) {
    io.stdout.writeln(
      'Summary: ${out.totalPass} PASS / ${out.totalFail} FAIL across ${out.totalPass + out.totalFail} ticks (total elapsed=${elapsedText(out.totalElapsedUs.toInt())})',
    );
  }
  return out;
}

Future<cascade_pb.CascadeReport> runReport(
  String name,
  List<LanguageMember> members, {
  required bool live,
  bool emit = false,
}) async {
  ensureCascadeObservability();
  final reportWatch = Stopwatch()..start();
  final report = cascade_pb.CascadeReport(name: name);
  final poll = live
      ? const Duration(milliseconds: 50)
      : const Duration(milliseconds: 100);
  final timeout =
      live ? const Duration(seconds: 1) : const Duration(seconds: 3);
  if (emit) {
    io.stdout.writeln('=== observability-cascade-dart ${modeSuffix(name)}===');
    io.stdout.writeln();
  }

  for (var phaseIdx = 0;
      phaseIdx < sdk.transportCoverageSequence.length;
      phaseIdx++) {
    final phaseWatch = Stopwatch()..start();
    final transport = sdk.transportCoverageSequence[phaseIdx];
    final from =
        phaseIdx == 0 ? transport : sdk.transportCoverageSequence[phaseIdx - 1];
    final phase = cascade_pb.PhaseResult(
      name: '${(phaseIdx + 1).toString().padLeft(2, '0')}-$from→$transport',
    );
    if (emit) {
      io.stdout.writeln(
        'Phase ${phaseIdx + 1}/${sdk.transportCoverageSequence.length}: ${phase.name}',
      );
    }

    sdk.Cascade? cascade;
    try {
      cascade = await sdk.buildCascade(sdk.CascadeOptions(
        transport: transport,
        members: childSpecs(members),
        extraEnv: const {
          'OP_OBS': 'logs,events,metrics,prom',
          'OP_PROM_ADDR': '127.0.0.1:0',
        },
      ));
    } catch (error) {
      phase.fail += runTicks;
      for (var tick = 1; tick <= runTicks; tick++) {
        phase.failures.add(
          'tick=$tick log=spawn event=spawn hops=${compactEvidence('$error')}',
        );
      }
      phaseWatch.stop();
      phase.elapsedUs = Int64(phaseWatch.elapsedMicroseconds);
      addPhase(report, phase);
      if (emit) {
        printPhaseSummary(phase);
      }
      continue;
    }

    final previous = <String, int>{};
    for (var tick = 1; tick <= runTicks; tick++) {
      final sender =
          '$name-phase-${(phaseIdx + 1).toString().padLeft(2, '0')}-tick-$tick';
      final result = await runTick(
        cascade,
        sender,
        transport,
        members,
        previous,
        timeout,
        poll,
        live,
      );
      if (result.pass) {
        phase.pass += 1;
      } else {
        phase.fail += 1;
        phase.failures.add(result.evidenceLine(tick));
      }
      if (emit) {
        io.stdout.writeln('  Tick $tick/$runTicks: ${passText(result.pass)}');
        if (!result.pass) {
          io.stderr.writeln('    ${result.evidenceLine(tick)}');
        }
      }
    }
    await cascade.stop();
    phaseWatch.stop();
    phase.elapsedUs = Int64(phaseWatch.elapsedMicroseconds);
    addPhase(report, phase);
    if (emit) {
      printPhaseSummary(phase);
    }
  }
  reportWatch.stop();
  report.elapsedUs = Int64(reportWatch.elapsedMicroseconds);
  if (emit) {
    io.stdout.writeln();
    io.stdout.writeln(
      'Summary: ${report.ticks} ticks, ${report.pass} PASS, ${report.fail} FAIL (total elapsed=${elapsedText(report.elapsedUs.toInt())})',
    );
  }
  return report;
}

class TickResult {
  const TickResult({
    required this.pass,
    required this.log,
    required this.event,
    required this.hops,
  });

  final bool pass;
  final sdk.CheckOutcome log;
  final sdk.CheckOutcome event;
  final sdk.CheckOutcome hops;

  String evidenceLine(int tick) {
    return 'tick=$tick log=${evidenceText(log)} event=${evidenceText(event)} hops=${evidenceText(hops)}';
  }
}

Future<TickResult> runTick(
  sdk.Cascade cascade,
  String sender,
  String note,
  List<LanguageMember> members,
  Map<String, int> previous,
  Duration timeout,
  Duration poll,
  bool live,
) async {
  relay_pb.TickResponse response;
  try {
    response = await relay_pb.RelayServiceClient(cascade.top.conn)
        .tick(relay_pb.TickRequest(sender: sender, note: note))
        .timeout(const Duration(seconds: 5));
  } catch (error) {
    final out = sdk.CheckOutcome(evidence: compactEvidence('$error'));
    return TickResult(pass: false, log: out, event: out, hops: out);
  }

  final hops = checkHops(response.hops, members, previous);
  if (!hops.pass) {
    return TickResult(
      pass: false,
      log: const sdk.CheckOutcome(evidence: 'skipped'),
      event: const sdk.CheckOutcome(evidence: 'skipped'),
      hops: hops,
    );
  }
  final expected = response.hops
      .map((hop) => sdk.Hop(slug: hop.slug, instanceUid: hop.uid))
      .toList();
  final leafUid = response.hops.first.uid;
  final log = await sdk.checkRelayedLog(sdk.LogCheckOptions(
    sender: sender,
    leafUid: leafUid,
    expectedChain: expected,
    timeout: timeout,
    pollInterval: poll,
    live: live,
  ));
  final event = await sdk.checkRelayedEvent(sdk.EventCheckOptions(
    leafUid: leafUid,
    expectedChain: expected,
    timeout: timeout,
    pollInterval: poll,
    live: live,
  ));
  return TickResult(
    pass: hops.pass && log.pass && event.pass,
    log: log,
    event: event,
    hops: hops,
  );
}

sdk.CheckOutcome checkHops(
  List<relay_pb.HopReceipt> hops,
  List<LanguageMember> members,
  Map<String, int> previous,
) {
  if (hops.length != members.length) {
    return sdk.CheckOutcome(
      evidence: 'hops length ${hops.length} want ${members.length}',
    );
  }
  for (var i = 0; i < hops.length; i++) {
    final hop = hops[i];
    final want = members[members.length - 1 - i];
    if (hop.slug != want.slug) {
      return sdk.CheckOutcome(
          evidence: 'hop $i slug=${hop.slug} want ${want.slug}');
    }
    if (hop.uid.isEmpty) {
      return sdk.CheckOutcome(evidence: 'hop $i uid empty');
    }
    final received = hop.received.toInt();
    final prev = previous[hop.uid] ?? 0;
    if (received <= prev) {
      return sdk.CheckOutcome(
        evidence: 'hop $i received=$received previous=$prev',
      );
    }
    previous[hop.uid] = received;
  }
  return const sdk.CheckOutcome(pass: true);
}

List<LanguageMember> ownLanguageMembers() {
  final binary = sdk.member('dart-node');
  return [
    LanguageMember(lang: 'dart', slug: dartSlug, binary: binary),
    LanguageMember(lang: 'dart', slug: dartSlug, binary: binary),
    LanguageMember(lang: 'dart', slug: dartSlug, binary: binary),
  ];
}

List<NamedPattern> dartPatterns() {
  final dartBin = sdk.member('dart-node');
  final goBin = sdk.member('go-node');
  final bins = {
    'dart': LanguageMember(lang: 'dart', slug: dartSlug, binary: dartBin),
    'go': LanguageMember(lang: 'go', slug: goSlug, binary: goBin),
  };
  const names = [
    'dart-dart-dart',
    'dart-dart-go',
    'dart-go-dart',
    'dart-go-go',
    'go-dart-dart',
    'go-dart-go',
    'go-go-dart',
    'go-go-go',
  ];
  return [
    for (final name in names)
      NamedPattern(
        name: name,
        members: [
          for (final part in name.split('-')) bins[part]!,
        ],
      ),
  ];
}

List<sdk.ChildSpec> childSpecs(List<LanguageMember> members) {
  return [
    for (final member in members)
      sdk.ChildSpec(slug: member.slug, binary: member.binary),
  ];
}

void addPhase(cascade_pb.CascadeReport report, cascade_pb.PhaseResult phase) {
  report.phases.add(phase);
  report.pass += phase.pass;
  report.fail += phase.fail;
  report.ticks += phase.pass + phase.fail;
}

void ensureCascadeObservability() {
  final current = sdk.current();
  if (current.enabled(sdk.Family.logs) && current.enabled(sdk.Family.events)) {
    return;
  }
  final env = Map<String, String>.from(io.Platform.environment);
  env['OP_OBS'] = 'logs,events,metrics,prom';
  sdk.fromEnv(const sdk.Config(slug: 'observability-cascade-dart'), env);
}

String evidenceText(sdk.CheckOutcome outcome) {
  if (outcome.pass) {
    return 'ok';
  }
  return compactEvidence(outcome.evidence);
}

String compactEvidence(String value) {
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

String passText(bool pass) => pass ? 'PASS' : 'FAIL';

void printPhaseSummary(cascade_pb.PhaseResult phase) {
  final status = phase.fail > 0 ? 'FAIL' : 'PASS';
  io.stdout.writeln(
    'Phase ${phase.name}: ${phase.pass}/${phase.pass + phase.fail} $status (elapsed=${elapsedText(phase.elapsedUs.toInt())})',
  );
}

String elapsedText(int elapsedUs) {
  final duration = Duration(microseconds: elapsedUs);
  if (duration < const Duration(seconds: 1)) {
    return '${duration.inMilliseconds}ms';
  }
  if (duration < const Duration(minutes: 1)) {
    return '${(elapsedUs / Duration.microsecondsPerSecond).toStringAsFixed(2)}s';
  }
  return '${(elapsedUs / Duration.microsecondsPerMinute).toStringAsFixed(1)}m';
}

String modeSuffix(String name) => name == 'default' ? '' : '--$name ';

String canonicalCommand(String raw) {
  final normalized = raw.trim().toLowerCase();
  return normalized.replaceAll('-', '').replaceAll('_', '').replaceAll(' ', '');
}
