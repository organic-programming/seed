import 'dart:async';

import 'package:flutter_test/flutter_test.dart';

import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/holon_connector.dart';

import 'support/fakes.dart';

void main() {
  group('GreetingController', () {
    test('initializes with preferred holon and greets in English', () async {
      final swiftConnection = FakeGreetingHolonConnection(
        languages: [
          language(code: 'en', name: 'English', native: 'English'),
          language(code: 'fr', name: 'French', native: 'Francais'),
        ],
        greetingBuilder: ({required name, required langCode}) =>
            'Hello $name from Swift',
      );
      final connector = FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-go': (_) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                'Hello $name from Go',
          ),
          'gabriel-greeting-swift': (_) => swiftConnection,
        },
      );
      final controller = GreetingController(
        catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
          holon('gabriel-greeting-go'),
          holon('gabriel-greeting-swift'),
        ]),
        connector: connector,
        initialTransport: 'stdio',
      );

      await controller.initialize();
      await waitForCoaxUpdate();

      expect(controller.selectedHolon?.slug, 'gabriel-greeting-swift');
      expect(controller.selectedLanguageCode, 'en');
      expect(controller.greeting, 'Hello World from Swift');
      expect(connector.connectCalls, [('gabriel-greeting-swift', 'stdio')]);
      expect(swiftConnection.sayHelloCalls.single, ('World', 'en'));
    });

    test('changing the user name refreshes the greeting', () async {
      final connection = FakeGreetingHolonConnection(
        languages: [language(code: 'en', name: 'English', native: 'English')],
        greetingBuilder: ({required name, required langCode}) => 'Hello $name',
      );
      final controller = GreetingController(
        catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
          holon('gabriel-greeting-swift'),
        ]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (_) => connection,
          },
        ),
      );

      await controller.initialize();
      await waitForCoaxUpdate();
      await controller.setUserName('Alice');

      expect(controller.greeting, 'Hello Alice');
      expect(connection.sayHelloCalls.last, ('Alice', 'en'));
    });

    test(
      'rejects unix transport when the platform capability disables it',
      () async {
        final controller = GreetingController(
          catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
            holon('gabriel-greeting-swift'),
          ]),
          connector: FakeHolonConnector(
            factories: <String, FakeGreetingHolonConnection Function(String)>{
              'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
                languages: [
                  language(code: 'en', name: 'English', native: 'English'),
                ],
                greetingBuilder: ({required name, required langCode}) =>
                    'Hello $name',
              ),
            },
          ),
          capabilities: const AppPlatformCapabilities(
            supportsUnixSockets: false,
          ),
        );

        expect(
          () => controller.setTransport('unix'),
          throwsA(isA<StateError>()),
        );
      },
    );

    test('surfaces connector failures as connection errors', () async {
      final controller = GreetingController(
        catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
          holon('gabriel-greeting-swift'),
        ]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
              languages: const [],
              greetingBuilder: ({required name, required langCode}) =>
                  'ignored',
              listLanguagesError: StateError('boot failure'),
            ),
          },
        ),
      );

      await controller.initialize();

      expect(controller.connectionError, isNull);
      expect(controller.error, contains('Failed to load languages'));
      expect(controller.isRunning, isTrue);
    });

    test('transport change invalidates an in-flight start', () async {
      final stdioCompleter = Completer<GreetingHolonConnection>();
      final tcpConnection = FakeGreetingHolonConnection(
        languages: [language(code: 'en', name: 'English', native: 'English')],
        greetingBuilder: ({required name, required langCode}) => 'Hello $name',
      );
      final connector = _DeferredHolonConnector(
        onConnect: (transport) {
          if (transport == 'stdio') {
            return stdioCompleter.future;
          }
          if (transport == 'tcp') {
            return Future<GreetingHolonConnection>.value(tcpConnection);
          }
          throw StateError('Unexpected transport $transport');
        },
      );
      final controller = GreetingController(
        catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
          holon('gabriel-greeting-swift'),
        ]),
        connector: connector,
        initialTransport: 'stdio',
      );

      await controller.refreshHolons();
      final firstStart = controller.ensureStarted();
      await Future<void>.delayed(Duration.zero);

      await controller.setTransport('tcp', reload: false);
      await controller.ensureStarted();

      expect(connector.connectCalls, <(String, String)>[
        ('gabriel-greeting-swift', 'stdio'),
        ('gabriel-greeting-swift', 'tcp'),
      ]);
      expect(controller.transport, 'tcp');
      expect(controller.isRunning, isTrue);

      stdioCompleter.complete(
        FakeGreetingHolonConnection(
          languages: [language(code: 'en', name: 'English', native: 'English')],
          greetingBuilder: ({required name, required langCode}) =>
              'Hello $name from stdio',
        ),
      );
      await firstStart;

      expect(controller.transport, 'tcp');
      expect(controller.isRunning, isTrue);
    });

    test(
      'retries non-stdio connection startup once before surfacing failure',
      () async {
        var tcpAttempts = 0;
        final controller = GreetingController(
          catalog: FakeHolonCatalog(<GabrielHolonIdentity>[
            holon('gabriel-greeting-c'),
          ]),
          connector: FakeHolonConnector(
            factories: <String, FakeGreetingHolonConnection Function(String)>{
              'gabriel-greeting-c': (transport) {
                if (transport == 'tcp' && tcpAttempts++ == 0) {
                  throw StateError('temporary tcp startup race');
                }
                return FakeGreetingHolonConnection(
                  languages: [
                    language(code: 'en', name: 'English', native: 'English'),
                  ],
                  greetingBuilder: ({required name, required langCode}) =>
                      'Hello $name',
                );
              },
            },
          ),
          initialTransport: 'tcp',
        );

        await controller.initialize();
        await waitForCoaxUpdate();

        expect(controller.isRunning, isTrue);
        expect(controller.connectionError, isNull);
        expect(controller.greeting, 'Hello World');
        expect(tcpAttempts, 2);
      },
    );
  });
}

class _DeferredHolonConnector implements HolonConnector {
  _DeferredHolonConnector({required this.onConnect});

  final Future<GreetingHolonConnection> Function(String transport) onConnect;
  final List<(String slug, String transport)> connectCalls =
      <(String, String)>[];

  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) {
    connectCalls.add((holon.slug, transport));
    return onConnect(transport);
  }
}
