import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

ObservabilityKit _kit(String slug, List<holons.Family> families) {
  return ObservabilityKit.standalone(
    slug: slug,
    declaredFamilies: families,
    settings: MemorySettingsStore(),
  );
}

void main() {
  group('LogConsoleView relay chain', () {
    testWidgets('renders typed AnyValue fields', (tester) async {
      final kit = _kit('panel-duration-log', const [holons.Family.logs]);
      try {
        for (final entry in <ObservabilityLogRecord>[
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('micro'),
            attributes: const [
              ObservabilityKeyValue(
                key: 'duration_ns',
                value: ObservabilityAnyValue.int(215000),
              ),
              ObservabilityKeyValue(
                key: 'count',
                value: ObservabilityAnyValue.int(42),
              ),
              ObservabilityKeyValue(
                key: 'cache_hit',
                value: ObservabilityAnyValue.bool(true),
              ),
              ObservabilityKeyValue(
                key: 'ratio',
                value: ObservabilityAnyValue.double(0.75),
              ),
              ObservabilityKeyValue(
                key: 'note',
                value: ObservabilityAnyValue.string('has space'),
              ),
            ],
          ),
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('milli'),
            attributes: const [
              ObservabilityKeyValue(
                key: 'duration_ns',
                value: ObservabilityAnyValue.int(3200000),
              ),
            ],
          ),
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('seconds'),
            attributes: const [
              ObservabilityKeyValue(
                key: 'duration_ns',
                value: ObservabilityAnyValue.int(1400000000),
              ),
              ObservabilityKeyValue(
                key: 'legacy_ns',
                value: ObservabilityAnyValue.string('215000'),
              ),
            ],
          ),
        ]) {
          kit.logs.addRecord(entry);
        }

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: LogConsoleView(controller: kit.logs)),
          ),
        );
        await tester.pump();

        expect(find.textContaining('duration=215.0µs'), findsOneWidget);
        expect(find.textContaining('duration=3.2ms'), findsOneWidget);
        expect(find.textContaining('duration=1.4s'), findsOneWidget);
        expect(find.textContaining('count=42'), findsOneWidget);
        expect(find.textContaining('cache_hit=true'), findsOneWidget);
        expect(find.textContaining('ratio=0.75'), findsOneWidget);
        expect(find.textContaining('note=\"has space\"'), findsOneWidget);
        expect(find.textContaining('legacy=215000'), findsOneWidget);
      } finally {
        kit.dispose();
      }
    });

    testWidgets('renders log title message in bold', (tester) async {
      final kit = _kit('panel-title-log', const [holons.Family.logs]);
      try {
        kit.logs.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            loggerName: 'rpc',
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('rpc handled'),
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: LogConsoleView(controller: kit.logs)),
          ),
        );
        await tester.pump();

        final richText = tester.widget<RichText>(
          find.byWidgetPredicate(
            (widget) =>
                widget is RichText &&
                widget.text.toPlainText() == '[rpc]  rpc handled',
          ),
        );
        final span = richText.text as TextSpan;
        final messageSpan = span.children!.last as TextSpan;
        expect(messageSpan.style?.fontWeight, FontWeight.w600);
      } finally {
        kit.dispose();
      }
    });

    testWidgets('renders chain text when LogEntry has hops', (tester) async {
      final kit = _kit('panel-chain-log', const [holons.Family.logs]);
      try {
        kit.logs.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('relayed'),
            chain: const [
              holons.Hop(slug: 'gabriel', instanceUid: 'g1'),
              holons.Hop(slug: 'clem', instanceUid: 'c1'),
            ],
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: LogConsoleView(controller: kit.logs)),
          ),
        );
        await tester.pump();

        expect(find.text('← gabriel:g1 > clem:c1'), findsOneWidget);
      } finally {
        kit.dispose();
      }
    });

    testWidgets('hides redundant single-hop chain', (tester) async {
      final kit = _kit('panel-chain-redundant-log', const [holons.Family.logs]);
      try {
        kit.logs.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('self-chain'),
            chain: const [holons.Hop(slug: 'parent', instanceUid: 'p1')],
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: LogConsoleView(controller: kit.logs)),
          ),
        );
        await tester.pump();

        expect(find.textContaining('← parent:p1'), findsNothing);
      } finally {
        kit.dispose();
      }
    });

    testWidgets('renders non-redundant single-hop chain', (tester) async {
      final kit = _kit('panel-chain-source-log', const [holons.Family.logs]);
      try {
        kit.logs.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('child-chain'),
            chain: const [holons.Hop(slug: 'child', instanceUid: 'c1')],
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: LogConsoleView(controller: kit.logs)),
          ),
        );
        await tester.pump();

        expect(find.text('← child:c1'), findsOneWidget);
      } finally {
        kit.dispose();
      }
    });

    testWidgets('omits chain row when LogEntry chain is empty', (tester) async {
      final kit = _kit('panel-chain-empty-log', const [holons.Family.logs]);
      try {
        kit.logs.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('no-chain'),
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: LogConsoleView(controller: kit.logs)),
          ),
        );
        await tester.pump();

        expect(find.text('no-chain'), findsOneWidget);
        expect(find.textContaining(' > '), findsNothing);
      } finally {
        kit.dispose();
      }
    });
  });

  group('EventsView relay chain', () {
    testWidgets('renders chain text when Event has hops', (tester) async {
      final kit = _kit('panel-chain-event', const [holons.Family.events]);
      try {
        kit.events.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('instanceReady'),
            eventName: 'instanceReady',
            chain: const [
              holons.Hop(slug: 'gabriel', instanceUid: 'g1'),
              holons.Hop(slug: 'clem', instanceUid: 'c1'),
            ],
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: EventsView(controller: kit.events)),
          ),
        );
        await tester.pump();

        expect(find.text('gabriel:g1 > clem:c1'), findsOneWidget);
      } finally {
        kit.dispose();
      }
    });

    testWidgets('omits chain row when Event chain is empty', (tester) async {
      final kit = _kit('panel-chain-empty-event', const [holons.Family.events]);
      try {
        kit.events.addRecord(
          ObservabilityLogRecord(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            body: const ObservabilityAnyValue.string('instanceReady'),
            eventName: 'instanceReady',
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(body: EventsView(controller: kit.events)),
          ),
        );
        await tester.pump();

        expect(find.textContaining(' > '), findsNothing);
      } finally {
        kit.dispose();
      }
    });
  });

  group('ObservabilityPanel export button', () {
    testWidgets('renders an enabled icon button with the export tooltip', (
      tester,
    ) async {
      final kit = _kit('export-button', const [holons.Family.logs]);
      try {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ObservabilityPanel(
                kit: kit,
                exportDestination: () async =>
                    Directory.systemTemp.createTempSync('unused-'),
              ),
            ),
          ),
        );
        await tester.pump();

        expect(find.byTooltip('Export observability bundle'), findsOneWidget);
        final iconButton = find.widgetWithIcon(
          IconButton,
          Icons.file_download_outlined,
        );
        expect(iconButton, findsOneWidget);
        expect(tester.widget<IconButton>(iconButton).onPressed, isNotNull);
        // The actual bundle writing path is covered by the kit-level test
        // `controllers expose logs metrics events and export bundle`.
      } finally {
        kit.dispose();
      }
    });

    testWidgets(
      'silently resets state when destination resolver returns null (cancel)',
      (tester) async {
        final kit = _kit('export-cancel', const [holons.Family.logs]);
        try {
          await tester.pumpWidget(
            MaterialApp(
              home: Scaffold(
                body: ObservabilityPanel(
                  kit: kit,
                  exportDestination: () async => null,
                ),
              ),
            ),
          );
          await tester.pump();

          await tester.tap(find.byTooltip('Export observability bundle'));
          await tester.pump();
          await tester.pump();

          expect(find.textContaining('Exported'), findsNothing);
          expect(find.textContaining('Export failed'), findsNothing);

          // The button is re-enabled after cancel — ready for another attempt.
          final iconButton = find.widgetWithIcon(
            IconButton,
            Icons.file_download_outlined,
          );
          expect(tester.widget<IconButton>(iconButton).onPressed, isNotNull);
        } finally {
          kit.dispose();
        }
      },
    );

    testWidgets('shows failure status when destination resolver throws', (
      tester,
    ) async {
      final kit = _kit('export-fail', const [holons.Family.logs]);
      try {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: ObservabilityPanel(
                kit: kit,
                exportDestination: () async =>
                    throw const FileSystemException('no docs dir'),
              ),
            ),
          ),
        );
        await tester.pump();

        await tester.tap(find.byTooltip('Export observability bundle'));
        await tester.pump();
        await tester.pump();

        expect(find.textContaining('Export failed'), findsOneWidget);
      } finally {
        kit.dispose();
      }
    });
  });
}
