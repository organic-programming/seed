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

    expect(find.text('Host'), findsOneWidget);
    expect(find.text('Port'), findsOneWidget);
    expect(find.text('Endpoint'), findsOneWidget);

    await coaxManager.setServerTransport(CoaxServerTransport.unix);
    await tester.pump();

    expect(find.text('Socket path'), findsOneWidget);
  });
}
