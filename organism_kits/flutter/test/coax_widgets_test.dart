import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:holons_app/holons_app.dart';

void main() {
  testWidgets('CoaxControlsView renders the saved preview endpoint', (
    tester,
  ) async {
    final coaxManager = CoaxManager(
      settingsStore: MemorySettingsStore(),
      defaults: CoaxSettingsDefaults.standard(socketName: 'henri-nobody.sock'),
      serviceFactory: () => <Service>[],
    );
    addTearDown(coaxManager.dispose);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: CoaxControlsView(
            coaxManager: coaxManager,
            onOpenSettings: () {},
          ),
        ),
      ),
    );

    expect(find.text('COAX'), findsOneWidget);
    expect(find.text('tcp://127.0.0.1:60000'), findsOneWidget);
    expect(find.text('OFF'), findsOneWidget);

    final titleRect = tester.getRect(find.text('COAX'));
    final endpointRect = tester.getRect(find.text('tcp://127.0.0.1:60000'));
    final badgeRect = tester.getRect(find.text('OFF'));

    expect(endpointRect.top, greaterThan(titleRect.bottom));
    expect((badgeRect.center.dy - endpointRect.center.dy).abs(), lessThan(1));
  });

  testWidgets('CoaxSettingsView switches between tcp and unix fields', (
    tester,
  ) async {
    final coaxManager = CoaxManager(
      settingsStore: MemorySettingsStore(),
      defaults: CoaxSettingsDefaults.standard(socketName: 'henri-nobody.sock'),
      serviceFactory: () => <Service>[],
    );
    addTearDown(coaxManager.dispose);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: CoaxSettingsView(coaxManager: coaxManager)),
      ),
    );
    final tcpDialogHeight = tester.getSize(find.byType(AlertDialog)).height;

    expect(find.text('Host'), findsOneWidget);
    expect(find.text('Port'), findsOneWidget);
    expect(find.text('Endpoint'), findsOneWidget);
    expect(find.byKey(const ValueKey<String>('coax-tcp-host')), findsOneWidget);

    await coaxManager.setServerTransport(CoaxServerTransport.unix);
    await tester.pump();
    final unixDialogHeight = tester.getSize(find.byType(AlertDialog)).height;

    expect(find.text('Socket path'), findsOneWidget);
    expect(find.byKey(const ValueKey<String>('coax-tcp-host')), findsNothing);
    final unixPathField = tester.widget<TextFormField>(
      find.byKey(const ValueKey<String>('coax-unix-path')),
    );
    expect(unixPathField.initialValue, coaxManager.defaultUnixPath);
    expect(unixDialogHeight, tcpDialogHeight);
  });
}
