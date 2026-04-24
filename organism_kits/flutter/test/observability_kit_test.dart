import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
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
      expect(find.text('panel-log'), findsOneWidget);
    } finally {
      kit.dispose();
    }
  });
}
