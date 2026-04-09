import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:holons_app/holons_app.dart';
import 'package:shadcn_flutter/shadcn_flutter.dart';

void main() {
  testWidgets('CoaxControlBar renders the saved preview endpoint', (
    tester,
  ) async {
    final controller = CoaxController(
      settingsStore: MemorySettingsStore(),
      defaults: CoaxSettingsDefaults.standard(socketName: 'henri-nobody.sock'),
      serviceFactory: () => <Service>[],
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      ShadcnApp(
        home: Scaffold(
          child: CoaxControlBar(
            controller: controller,
            onOpenSettings: () {},
          ),
        ),
      ),
    );

    expect(find.text('COAX'), findsOneWidget);
    expect(find.text('Server:'), findsOneWidget);
    expect(find.text('tcp://127.0.0.1:60000'), findsOneWidget);
    expect(find.text('OFF'), findsOneWidget);
  });

  testWidgets('CoaxSettingsDialog switches between tcp and unix fields', (
    tester,
  ) async {
    final controller = CoaxController(
      settingsStore: MemorySettingsStore(),
      defaults: CoaxSettingsDefaults.standard(socketName: 'henri-nobody.sock'),
      serviceFactory: () => <Service>[],
    );
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      ShadcnApp(
        home: Scaffold(
          child: CoaxSettingsDialog(controller: controller),
        ),
      ),
    );

    expect(find.text('Transport'), findsOneWidget);
    expect(find.text('Host'), findsOneWidget);
    expect(find.text('Port'), findsOneWidget);
    expect(find.text('Endpoint'), findsOneWidget);

    await controller.setServerTransport(CoaxServerTransport.unix);
    await tester.pump();

    expect(find.text('Socket path'), findsOneWidget);
  });
}
