import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

void main() {
  test('ObservabilityKit gates persist through SettingsStore', () async {
    final settings = MemorySettingsStore();
    final kit = ObservabilityKit.standalone(
      slug: 'gabriel-greeting-app-flutter',
      declaredFamilies: const [
        holons.Family.logs,
        holons.Family.metrics,
        holons.Family.events,
        holons.Family.prom,
      ],
      settings: settings,
      bundledHolons: const [
        ObservabilityMemberRef(slug: 'gabriel-greeting-go', uid: 'go-1'),
      ],
    );
    addTearDown(kit.dispose);

    await kit.gate.setMaster(false);
    await kit.gate.setMemberOverride('go-1', GateOverride.off);

    final restored = RuntimeGate(
      settings: settings,
      members: const [
        ObservabilityMemberRef(slug: 'gabriel-greeting-go', uid: 'go-1'),
      ],
    );
    expect(restored.masterEnabled, isFalse);
    expect(restored.memberOverride('go-1'), GateOverride.off);
  });

  test('LogConsoleController returns entries sorted by timestamp', () async {
    addTearDown(holons.reset);
    final obs = holons.configure(
      const holons.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs'},
    );
    final gate = RuntimeGate(settings: MemorySettingsStore());
    final controller = LogConsoleController(obs, gate);
    addTearDown(controller.dispose);

    final base = DateTime.utc(2026, 5, 12, 12);
    obs.logRing!.push(
      holons.LogEntry(
        timestamp: base.add(const Duration(milliseconds: 200)),
        level: holons.Level.info,
        slug: 'parent',
        instanceUid: 'parent-uid',
        message: 'second',
      ),
    );
    obs.logRing!.push(
      holons.LogEntry(
        timestamp: base.add(const Duration(milliseconds: 100)),
        level: holons.Level.info,
        slug: 'parent',
        instanceUid: 'parent-uid',
        message: 'first',
      ),
    );
    obs.logRing!.push(
      holons.LogEntry(
        timestamp: base.add(const Duration(milliseconds: 300)),
        level: holons.Level.info,
        slug: 'parent',
        instanceUid: 'parent-uid',
        message: 'third',
      ),
    );

    await _waitFor(() => controller.entries.length == 3);

    expect(controller.entries.map((entry) => entry.message), [
      'first',
      'second',
      'third',
    ]);
  });

  test('RelayController starts relay when member is enabled', () async {
    final channels = <ClientChannel>[];
    addTearDown(() async {
      for (final channel in channels) {
        await channel.shutdown();
      }
      holons.reset();
    });
    final gate = RuntimeGate(
      settings: MemorySettingsStore(),
      members: const [
        ObservabilityMemberRef(slug: 'gabriel-greeting-go', uid: 'go-1'),
      ],
    );
    final local = holons.configure(
      const holons.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs,events'},
    );
    final fakes = <_FakeMemberRelay>[];
    final controller = RelayController(
      gate,
      local,
      channelOpener: (member) async => _dummyChannel(channels),
      memberRelayFactory:
          ({
            required childSlug,
            required childUid,
            required channel,
            required observability,
          }) {
            final fake = _FakeMemberRelay();
            fakes.add(fake);
            return fake;
          },
    );
    addTearDown(controller.dispose);

    await _waitFor(() => fakes.length == 1 && fakes.single.startCalls == 1);

    expect(controller.runningRelayCount, equals(1));
    expect(controller.activeMembers.map((member) => member.uid), ['go-1']);
  });

  test('RelayController stops and recreates relay as gate changes', () async {
    final channels = <ClientChannel>[];
    addTearDown(() async {
      for (final channel in channels) {
        await channel.shutdown();
      }
      holons.reset();
    });
    final gate = RuntimeGate(
      settings: MemorySettingsStore(),
      members: const [
        ObservabilityMemberRef(slug: 'gabriel-greeting-go', uid: 'go-1'),
      ],
    );
    final local = holons.configure(
      const holons.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs,events'},
    );
    final fakes = <_FakeMemberRelay>[];
    final controller = RelayController(
      gate,
      local,
      channelOpener: (member) async => _dummyChannel(channels),
      memberRelayFactory:
          ({
            required childSlug,
            required childUid,
            required channel,
            required observability,
          }) {
            final fake = _FakeMemberRelay();
            fakes.add(fake);
            return fake;
          },
    );
    addTearDown(controller.dispose);

    await _waitFor(
      () => fakes.length == 1 && controller.runningRelayCount == 1,
    );
    final first = fakes.single;

    await gate.setMemberOverride('go-1', GateOverride.off);
    await _waitFor(() => controller.runningRelayCount == 0);
    expect(first.stopCalls, equals(1));

    await gate.setMemberOverride('go-1', GateOverride.on);
    await _waitFor(
      () => fakes.length == 2 && controller.runningRelayCount == 1,
    );
    expect(fakes.last, isNot(same(first)));
    expect(fakes.last.startCalls, equals(1));
  });

  test(
    'RelayController logs opener failures and starts other members',
    () async {
      final channels = <ClientChannel>[];
      addTearDown(() async {
        for (final channel in channels) {
          await channel.shutdown();
        }
        holons.reset();
      });
      final gate = RuntimeGate(
        settings: MemorySettingsStore(),
        members: const [
          ObservabilityMemberRef(slug: 'bad-child', uid: 'bad-1'),
          ObservabilityMemberRef(slug: 'good-child', uid: 'good-1'),
        ],
      );
      final local = holons.configure(
        const holons.Config(slug: 'parent', instanceUid: 'parent-uid'),
        env: const {'OP_OBS': 'logs,events'},
      );
      final fakes = <_FakeMemberRelay>[];
      final controller = RelayController(
        gate,
        local,
        channelOpener: (member) async {
          if (member.uid == 'bad-1') {
            throw StateError('boom');
          }
          return _dummyChannel(channels);
        },
        memberRelayFactory:
            ({
              required childSlug,
              required childUid,
              required channel,
              required observability,
            }) {
              final fake = _FakeMemberRelay();
              fakes.add(fake);
              return fake;
            },
      );
      addTearDown(controller.dispose);

      await _waitFor(() => controller.runningRelayCount == 1);
      await _waitFor(
        () => local.logRing!.drain().any(
          (entry) =>
              entry.level == holons.Level.warn &&
              entry.loggerName == 'relay-controller' &&
              entry.fields['uid'] == 'bad-1' &&
              (entry.fields['error'] ?? '').contains('boom'),
        ),
      );

      expect(fakes, hasLength(1));
      expect(controller.activeMembers.map((member) => member.uid), ['good-1']);
    },
  );

  test('RelayController dispose stops all relays once', () async {
    final channels = <ClientChannel>[];
    addTearDown(() async {
      for (final channel in channels) {
        await channel.shutdown();
      }
      holons.reset();
    });
    final gate = RuntimeGate(
      settings: MemorySettingsStore(),
      members: const [
        ObservabilityMemberRef(slug: 'child-a', uid: 'a'),
        ObservabilityMemberRef(slug: 'child-b', uid: 'b'),
        ObservabilityMemberRef(slug: 'child-c', uid: 'c'),
      ],
    );
    final local = holons.configure(
      const holons.Config(slug: 'parent', instanceUid: 'parent-uid'),
      env: const {'OP_OBS': 'logs,events'},
    );
    final fakes = <_FakeMemberRelay>[];
    final controller = RelayController(
      gate,
      local,
      channelOpener: (member) async => _dummyChannel(channels),
      memberRelayFactory:
          ({
            required childSlug,
            required childUid,
            required channel,
            required observability,
          }) {
            final fake = _FakeMemberRelay();
            fakes.add(fake);
            return fake;
          },
    );

    await _waitFor(
      () => fakes.length == 3 && controller.runningRelayCount == 3,
    );

    controller.dispose();
    expect(controller.runningRelayCount, equals(0));
    expect(fakes.map((fake) => fake.stopCalls), everyElement(1));

    controller.dispose();
    expect(fakes.map((fake) => fake.stopCalls), everyElement(1));
  });

  test('controllers expose logs metrics events and export bundle', () async {
    final temp = await Directory.systemTemp.createTemp('obs-kit-flutter-');
    addTearDown(() => temp.delete(recursive: true));
    final kit = ObservabilityKit.standalone(
      slug: 'gabriel-greeting-app-flutter',
      declaredFamilies: const [
        holons.Family.logs,
        holons.Family.metrics,
        holons.Family.events,
      ],
      settings: MemorySettingsStore(),
    );
    addTearDown(kit.dispose);

    kit.obs.logger('test').info('ready');
    kit.obs.counter('requests_total')!.inc();
    kit.obs.gauge('live_gauge')!.set(1.25);
    kit.obs.emit(
      holons.EventType.instanceReady,
      payload: {'listener': 'local'},
    );
    await Future<void>.delayed(const Duration(milliseconds: 20));
    kit.metrics.refresh();

    expect(kit.logs.entries.map((entry) => entry.message), contains('ready'));
    expect(
      kit.metrics.latest?.counters.map((counter) => counter.name),
      contains('requests_total'),
    );
    expect(
      kit.events.events.map((event) => event.type),
      contains(holons.EventType.instanceReady),
    );

    final exported = await kit.export.exportTo(temp);
    expect(File('${exported.path}/logs.jsonl').existsSync(), isTrue);
    expect(
      File('${exported.path}/metrics.prom').readAsStringSync(),
      contains('requests_total'),
    );
  });

  test('standalone uses launcher uid and registry root when present', () async {
    final temp = await Directory.systemTemp.createTemp('obs-kit-run-dir-');
    addTearDown(() => temp.delete(recursive: true));

    final kit = ObservabilityKit.standalone(
      slug: 'gabriel-greeting-app-flutter',
      declaredFamilies: const [holons.Family.logs, holons.Family.events],
      settings: MemorySettingsStore(),
      environment: {'OP_INSTANCE_UID': 'uid-1', 'OP_RUN_DIR': temp.path},
    );
    addTearDown(kit.dispose);

    expect(kit.obs.cfg.instanceUid, equals('uid-1'));
    expect(
      kit.obs.cfg.runDir,
      equals(
        '${temp.path}${Platform.pathSeparator}gabriel-greeting-app-flutter${Platform.pathSeparator}uid-1',
      ),
    );
  });

  testWidgets('ObservabilityPanel opens tabs and reads kit state', (
    tester,
  ) async {
    final kit = ObservabilityKit.standalone(
      slug: 'gabriel-greeting-app-flutter',
      declaredFamilies: const [
        holons.Family.logs,
        holons.Family.metrics,
        holons.Family.events,
      ],
      settings: MemorySettingsStore(),
    );
    kit.obs.logger('test').info('panel-log');
    try {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: ObservabilityPanel(kit: kit)),
        ),
      );
      await tester.pump();

      expect(find.text('Settings'), findsOneWidget);
      await tester.tap(find.widgetWithText(Tab, 'Logs'));
      await tester.pumpAndSettle();
      expect(find.textContaining('panel-log'), findsOneWidget);
    } finally {
      kit.dispose();
    }
  });
}

class _FakeMemberRelay implements RelaySession {
  var startCalls = 0;
  var stopCalls = 0;
  var _isRunning = false;

  @override
  bool get isRunning => _isRunning;

  @override
  Future<void> start() async {
    startCalls += 1;
    _isRunning = true;
  }

  @override
  Future<void> stop() async {
    stopCalls += 1;
    _isRunning = false;
  }
}

ClientChannel _dummyChannel(List<ClientChannel> channels) {
  final channel = ClientChannel(
    '127.0.0.1',
    port: 1,
    options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
  );
  channels.add(channel);
  return channel;
}

Future<void> _waitFor(
  bool Function() condition, {
  Duration timeout = const Duration(seconds: 2),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (condition()) return;
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
  fail('condition was not met within $timeout');
}
