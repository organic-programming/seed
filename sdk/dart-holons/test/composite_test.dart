import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/describe.pbgrpc.dart';
import 'package:holons/gen/holons/v1/observability.pbgrpc.dart' as obs_pb;
import 'package:holons/src/composite.dart';
import 'package:holons/src/observability.dart' as observability;
import 'package:test/test.dart';

void main() {
  test('member resolves embedded binary', () {
    final root = Directory.systemTemp.createTempSync('holons-dart-composite-');
    addTearDown(() => root.deleteSync(recursive: true));
    final binDir = Directory('${root.path}/composite.holon/bin/darwin_arm64')
      ..createSync(recursive: true);
    final self = File('${binDir.path}/composite')..writeAsStringSync('self');
    final memberDir = Directory('${binDir.path}/holons/node-a')
      ..createSync(recursive: true);
    File('${memberDir.path}/README.txt').writeAsStringSync('not executable');
    final member = File('${memberDir.path}/node-a-bin')
      ..writeAsStringSync('node');
    if (!Platform.isWindows) {
      Process.runSync('chmod', ['755', member.path]);
    }

    expect(memberFromExecutable(self.path, 'node-a'),
        equals(member.absolute.path));
  });

  test('member rejects empty id', () {
    expect(
      () => memberFromExecutable('/tmp/composite', ' '),
      throwsA(isA<ArgumentError>()),
    );
  });

  group('tcp member startup', () {
    test(
      'reports an early child exit immediately with bounded stderr',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-exit-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        final script = _writeExecutableScript(root, 'exit-before-meta', '''
printf 'START-OF-STDERR\\n' >&2
i=0
while [ "\$i" -lt 3000 ]; do
  printf '0123456789abcdef\\n' >&2
  i=\$((i + 1))
done
printf 'FINAL-STARTUP-DIAGNOSTIC\\n' >&2
exit 23
''');
        final stopwatch = Stopwatch()..start();

        late MemberStartupException failure;
        try {
          await spawnMember(
            binaryPath: script.path,
            slug: 'early-exit',
            transport: 'tcp',
            instanceUid: 'early-exit-uid',
            extraEnv: <String, String>{'OP_RUN_DIR': root.path},
            withTransitiveObservability: false,
            startupTimeout: const Duration(seconds: 5),
          );
          fail('spawnMember unexpectedly succeeded');
        } on MemberStartupException catch (error) {
          failure = error;
        }
        stopwatch.stop();

        expect(stopwatch.elapsed, lessThan(const Duration(seconds: 2)));
        expect(failure.slug, 'early-exit');
        expect(failure.uid, 'early-exit-uid');
        expect(failure.processExited, isTrue);
        expect(failure.cancelled, isFalse);
        expect(failure.kind, MemberStartupFailureKind.processExited);
        expect(failure.retryable, isTrue);
        expect(failure.exitCode, 23);
        expect(failure.diagnostic, contains('FINAL-STARTUP-DIAGNOSTIC'));
        expect(failure.diagnostic, isNot(contains('START-OF-STDERR')));
        expect(failure.diagnostic.length, lessThanOrEqualTo(16 * 1024));
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );

    test(
      'accepts delayed metadata within the configured startup timeout',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-delayed-meta-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        final server = Server.create(services: <Service>[_TestHolonMeta()]);
        await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
        addTearDown(server.shutdown);
        final script = _writeExecutableScript(root, 'delayed-meta', '''
sleep 0.2
instance_dir="\$OP_RUN_DIR/delayed-meta/\$OP_INSTANCE_UID"
mkdir -p "\$instance_dir"
printf '{"uid":"%s","address":"tcp://127.0.0.1:${server.port}"}\\n' "\$OP_INSTANCE_UID" > "\$instance_dir/meta.json"
trap 'exit 0' TERM INT
while :; do
  sleep 1
done
''');
        final stopwatch = Stopwatch()..start();

        final spawned = await spawnMember(
          binaryPath: script.path,
          slug: 'delayed-meta',
          transport: 'tcp',
          instanceUid: 'delayed-meta-uid',
          extraEnv: <String, String>{'OP_RUN_DIR': root.path},
          withTransitiveObservability: false,
          startupTimeout: const Duration(seconds: 2),
        );
        stopwatch.stop();
        try {
          expect(
            stopwatch.elapsed,
            greaterThanOrEqualTo(const Duration(milliseconds: 150)),
          );
          expect(spawned.listenUri, 'tcp://127.0.0.1:${server.port}');
        } finally {
          await spawned.stop(timeout: const Duration(seconds: 2));
        }
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );

    test(
      'classifies metadata timeout as retryable readiness failure',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-meta-timeout-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        final script = _writeExecutableScript(root, 'missing-meta', '''
trap 'exit 0' TERM INT
while :; do
  sleep 1
done
''');

        late MemberStartupException failure;
        try {
          await spawnMember(
            binaryPath: script.path,
            slug: 'missing-meta',
            transport: 'tcp',
            instanceUid: 'missing-meta-uid',
            extraEnv: <String, String>{'OP_RUN_DIR': root.path},
            withTransitiveObservability: false,
            startupTimeout: const Duration(milliseconds: 150),
          );
          fail('spawnMember unexpectedly succeeded');
        } on MemberStartupException catch (error) {
          failure = error;
        }

        expect(failure.kind, MemberStartupFailureKind.readiness);
        expect(failure.retryable, isTrue);
        expect(failure.processExited, isFalse);
        expect(failure.cancelled, isFalse);
        expect(failure.diagnostic, contains('meta not ready'));
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );

    test(
      'cancellation wins metadata wait and terminates the child',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-cancel-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        final started = File('${root.path}/started');
        final stopped = File('${root.path}/stopped');
        final script = _writeExecutableScript(root, 'cancel-startup', '''
trap 'printf stopped > "\$STOP_FILE"; exit 0' TERM INT
printf started > "\$START_FILE"
while :; do
  sleep 0.05
done
''');
        final cancellation = Completer<void>();
        unawaited(
          () async {
            final deadline = DateTime.now().add(const Duration(seconds: 2));
            while (!started.existsSync() && DateTime.now().isBefore(deadline)) {
              await Future<void>.delayed(const Duration(milliseconds: 10));
            }
            cancellation.complete();
          }(),
        );

        late MemberStartupException failure;
        try {
          await spawnMember(
            binaryPath: script.path,
            slug: 'cancelled-member',
            transport: 'tcp',
            instanceUid: 'cancelled-member-uid',
            extraEnv: <String, String>{
              'OP_RUN_DIR': root.path,
              'START_FILE': started.path,
              'STOP_FILE': stopped.path,
            },
            withTransitiveObservability: false,
            startupTimeout: const Duration(seconds: 5),
            startupCancellation: cancellation.future,
          );
          fail('spawnMember unexpectedly succeeded');
        } on MemberStartupException catch (error) {
          failure = error;
        }

        expect(started.existsSync(), isTrue);
        expect(stopped.existsSync(), isTrue);
        expect(failure.slug, 'cancelled-member');
        expect(failure.uid, 'cancelled-member-uid');
        expect(failure.cancelled, isTrue);
        expect(failure.processExited, isFalse);
        expect(failure.kind, MemberStartupFailureKind.cancelled);
        expect(failure.retryable, isFalse);
        expect(failure.exitCode, isNull);
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );

    test(
      'configured startup termination timeout bounds forced cleanup',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-termination-timeout-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        final pidFile = File('${root.path}/pid');
        final script = _writeExecutableScript(root, 'termination-timeout', '''
trap '' TERM INT
printf '%s' "\$\$" > "\$PID_FILE"
while :; do
  sleep 0.02
done
''');
        addTearDown(() {
          if (pidFile.existsSync()) {
            Process.runSync('kill', <String>[
              '-KILL',
              pidFile.readAsStringSync(),
            ]);
          }
        });
        final stopwatch = Stopwatch()..start();

        await expectLater(
          spawnMember(
            binaryPath: script.path,
            slug: 'termination-timeout',
            transport: 'tcp',
            instanceUid: 'termination-timeout-uid',
            extraEnv: <String, String>{
              'OP_RUN_DIR': root.path,
              'PID_FILE': pidFile.path,
            },
            withTransitiveObservability: false,
            startupTimeout: const Duration(milliseconds: 300),
            startupTerminationTimeout: const Duration(milliseconds: 300),
          ),
          throwsA(
            isA<MemberStartupException>().having(
              (error) => error.kind,
              'kind',
              MemberStartupFailureKind.readiness,
            ),
          ),
        );
        stopwatch.stop();

        expect(
          stopwatch.elapsed,
          greaterThanOrEqualTo(const Duration(milliseconds: 550)),
          reason: 'startup timeout plus configured SIGTERM grace must elapse',
        );
        expect(stopwatch.elapsed, lessThan(const Duration(milliseconds: 1200)));
        expect(pidFile.existsSync(), isTrue);
        final probe = Process.runSync(
          'kill',
          <String>['-0', pidFile.readAsStringSync()],
        );
        expect(probe.exitCode, isNot(0), reason: 'child process leaked');
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );

    test(
      'cancellation during relay initialization closes streams and child',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-relay-cancel-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        observability.configure(
          const observability.Config(
            slug: 'relay-parent',
            instanceUid: 'relay-parent-uid',
          ),
          env: const <String, String>{'OP_OBS': 'events'},
        );
        addTearDown(observability.reset);
        final relayService = _BlockingObservabilityService();
        final server = Server.create(
          services: <Service>[_TestHolonMeta(), relayService],
        );
        await server.serve(address: InternetAddress.loopbackIPv4, port: 0);
        addTearDown(server.shutdown);
        final pidFile = File('${root.path}/pid');
        final stopped = File('${root.path}/stopped');
        final script = _writeExecutableScript(root, 'relay-cancel', '''
trap 'printf stopped > "\$STOP_FILE"; exit 0' TERM INT
printf '%s' "\$\$" > "\$PID_FILE"
instance_dir="\$OP_RUN_DIR/relay-cancel/\$OP_INSTANCE_UID"
mkdir -p "\$instance_dir"
printf '{"uid":"%s","address":"tcp://127.0.0.1:${server.port}"}\\n' "\$OP_INSTANCE_UID" > "\$instance_dir/meta.json"
while :; do
  sleep 0.02
done
''');
        addTearDown(() {
          if (pidFile.existsSync()) {
            Process.runSync('kill', <String>[
              '-KILL',
              pidFile.readAsStringSync(),
            ]);
          }
        });
        final cancellation = Completer<void>();
        final spawning = spawnMember(
          binaryPath: script.path,
          slug: 'relay-cancel',
          transport: 'tcp',
          instanceUid: 'relay-cancel-uid',
          extraEnv: <String, String>{
            'OP_RUN_DIR': root.path,
            'PID_FILE': pidFile.path,
            'STOP_FILE': stopped.path,
          },
          startupTimeout: const Duration(seconds: 2),
          startupTerminationTimeout: const Duration(seconds: 1),
          startupCancellation: cancellation.future,
        );

        await _waitForCondition(() => relayService.eventsOpened == 1);
        cancellation.complete();

        await expectLater(
          spawning,
          throwsA(
            isA<MemberStartupException>()
                .having((error) => error.cancelled, 'cancelled', isTrue)
                .having((error) => error.retryable, 'retryable', isFalse),
          ),
        );
        await relayService.eventsClosed.timeout(const Duration(seconds: 2));
        expect(stopped.existsSync(), isTrue);
        final probe = Process.runSync(
          'kill',
          <String>['-0', pidFile.readAsStringSync()],
        );
        expect(probe.exitCode, isNot(0), reason: 'child process leaked');
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );

    test(
      'normal stop still terminates the child when relay stop fails',
      () async {
        final root = Directory.systemTemp.createTempSync(
          'holons-dart-composite-stop-relay-failure-',
        );
        addTearDown(() => root.deleteSync(recursive: true));
        final pidFile = File('${root.path}/pid');
        final stopped = File('${root.path}/stopped');
        final script = _writeExecutableScript(root, 'stop-relay-failure', '''
trap 'printf stopped > "\$STOP_FILE"; exit 0' TERM INT
printf '%s' "\$\$" > "\$PID_FILE"
while :; do
  sleep 0.02
done
''');
        final process = await Process.start(
          script.path,
          const <String>[],
          environment: <String, String>{
            'PID_FILE': pidFile.path,
            'STOP_FILE': stopped.path,
          },
        );
        addTearDown(() {
          if (pidFile.existsSync()) {
            Process.runSync('kill', <String>[
              '-KILL',
              pidFile.readAsStringSync(),
            ]);
          }
        });
        await _waitForCondition(pidFile.existsSync);
        final channel = ClientChannel(
          InternetAddress.loopbackIPv4,
          port: 1,
          options: const ChannelOptions(
            credentials: ChannelCredentials.insecure(),
          ),
        );
        final relay = _FailingStopMemberRelay(channel);
        final member = SpawnedMember(
          slug: 'stop-relay-failure',
          uid: 'stop-relay-failure-uid',
          listenUri: 'tcp://127.0.0.1:1',
          conn: channel,
          process: process,
          relay: relay,
        );

        final firstStop = member.stop(timeout: const Duration(seconds: 1));
        final concurrentStop = member.stop(timeout: Duration.zero);
        expect(identical(firstStop, concurrentStop), isTrue);
        await Future.wait<void>(<Future<void>>[firstStop, concurrentStop]);
        await member.stop(timeout: Duration.zero);

        expect(relay.stopCalls, 1,
            reason: 'stop must share one teardown future');
        expect(stopped.existsSync(), isTrue);
        final probe = Process.runSync(
          'kill',
          <String>['-0', pidFile.readAsStringSync()],
        );
        expect(probe.exitCode, isNot(0), reason: 'child process leaked');
      },
      skip: Platform.isWindows ? 'requires a POSIX shell' : false,
    );
  });
}

File _writeExecutableScript(Directory root, String name, String body) {
  final script = File('${root.path}/$name.sh')
    ..writeAsStringSync('#!/bin/sh\nset -eu\n$body');
  final chmod = Process.runSync('chmod', <String>['755', script.path]);
  if (chmod.exitCode != 0) {
    throw StateError('chmod failed for ${script.path}: ${chmod.stderr}');
  }
  return script;
}

final class _FailingStopMemberRelay extends observability.MemberRelay {
  _FailingStopMemberRelay(ClientChannel channel)
      : super(
          childSlug: 'stop-relay-failure',
          childUid: 'stop-relay-failure-uid',
          channel: channel,
          observability: observability.current(),
        );

  int stopCalls = 0;

  @override
  Future<void> stop() async {
    stopCalls++;
    throw StateError('injected relay stop failure');
  }
}

final class _TestHolonMeta extends HolonMetaServiceBase {
  @override
  Future<DescribeResponse> describe(
    ServiceCall call,
    DescribeRequest request,
  ) async =>
      DescribeResponse();
}

final class _BlockingObservabilityService
    extends obs_pb.HolonObservabilityServiceBase {
  final Completer<void> _eventsClosed = Completer<void>();
  var eventsOpened = 0;

  Future<void> get eventsClosed => _eventsClosed.future;

  @override
  Stream<obs_pb.LogRecord> logs(
    ServiceCall call,
    obs_pb.LogsRequest request,
  ) async* {}

  @override
  Stream<obs_pb.Metric> metrics(
    ServiceCall call,
    obs_pb.MetricsRequest request,
  ) async* {}

  @override
  Stream<obs_pb.LogRecord> events(
    ServiceCall call,
    obs_pb.EventsRequest request,
  ) async* {
    eventsOpened += 1;
    try {
      while (!call.isCanceled) {
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
    } finally {
      if (!_eventsClosed.isCompleted) {
        _eventsClosed.complete();
      }
    }
  }
}

Future<void> _waitForCondition(
  bool Function() condition, {
  Duration timeout = const Duration(seconds: 2),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (!condition() && DateTime.now().isBefore(deadline)) {
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  if (!condition()) {
    fail('condition was not met within $timeout');
  }
}
