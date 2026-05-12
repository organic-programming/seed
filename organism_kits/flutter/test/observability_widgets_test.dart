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
    testWidgets('renders adaptive duration fields', (tester) async {
      final kit = _kit('panel-duration-log', const [holons.Family.logs]);
      try {
        for (final entry in <holons.LogEntry>[
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'micro',
            fields: const {'duration_ns': '215000', 'count': '42'},
          ),
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'milli',
            fields: const {'duration_ns': '3200000'},
          ),
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'seconds',
            fields: const {'duration_ns': '1400000000'},
          ),
        ]) {
          kit.obs.logRing!.push(entry);
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
      } finally {
        kit.dispose();
      }
    });

    testWidgets('renders log title message in bold', (tester) async {
      final kit = _kit('panel-title-log', const [holons.Family.logs]);
      try {
        kit.obs.logRing!.push(
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            loggerName: 'rpc',
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'rpc handled',
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
        kit.obs.logRing!.push(
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'relayed',
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
        kit.obs.logRing!.push(
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'self-chain',
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
        kit.obs.logRing!.push(
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'child-chain',
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
        kit.obs.logRing!.push(
          holons.LogEntry(
            timestamp: DateTime.now(),
            level: holons.Level.info,
            slug: 'parent',
            instanceUid: 'parent-uid',
            message: 'no-chain',
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
        kit.obs.eventBus!.emit(
          holons.Event(
            timestamp: DateTime.now(),
            type: holons.EventType.instanceReady,
            slug: 'parent',
            instanceUid: 'parent-uid',
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
        kit.obs.eventBus!.emit(
          holons.Event(
            timestamp: DateTime.now(),
            type: holons.EventType.instanceReady,
            slug: 'parent',
            instanceUid: 'parent-uid',
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
