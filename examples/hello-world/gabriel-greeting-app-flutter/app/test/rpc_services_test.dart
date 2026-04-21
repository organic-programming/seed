import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/coax.pbgrpc.dart';
import 'package:holons_app/holons_app.dart';

import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/gen/v1/holon.pbgrpc.dart';
import 'package:gabriel_greeting_app_flutter/src/model/app_model.dart';
import 'package:gabriel_greeting_app_flutter/src/runtime/greeting_holon_connection.dart';

import 'support/fakes.dart';

void main() {
  test('COAX and GreetingApp RPC services drive the shared state', () async {
    final connection = FakeGreetingHolonConnection(
      languages: [
        language(code: 'en', name: 'English', native: 'English'),
        language(code: 'fr', name: 'French', native: 'Francais'),
      ],
      greetingBuilder: ({required name, required langCode}) {
        if (langCode == 'fr') {
          return 'Bonjour $name from Gabriel';
        }
        return 'Hello $name from Gabriel';
      },
    );
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([
        holon('gabriel-greeting-go'),
        holon('gabriel-greeting-swift'),
      ]),
      connector: FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-go': (_) => connection,
          'gabriel-greeting-swift': (_) => connection,
        },
      ),
    );
    final coaxController = buildCoaxManager(
      greetingController: greetingController,
    );

    await greetingController.initialize();
    final port = await reserveTcpPort();
    await coaxController.setServerPortText(port.toString());
    await coaxController.setEnabled(true);

    final channel = clientChannelFromListenUri(coaxController.listenUri!);
    addTearDown(() async {
      await channel.shutdown();
      await coaxController.shutdown();
      await greetingController.shutdown();
    });

    final coaxClient = CoaxServiceClient(channel);
    final appClient = GreetingAppServiceClient(channel);

    final members = await coaxClient.listMembers(ListMembersRequest());
    expect(
      members.members.map((member) => member.slug),
      containsAll(<String>['gabriel-greeting-go', 'gabriel-greeting-swift']),
    );

    final selectHolon = await appClient.selectHolon(
      SelectHolonRequest(slug: 'gabriel-greeting-go'),
    );
    expect(selectHolon.slug, 'gabriel-greeting-go');

    final selectLanguage = await appClient.selectLanguage(
      SelectLanguageRequest(code: 'fr'),
    );
    expect(selectLanguage.code, 'fr');

    final greeting = await appClient.greet(
      GreetRequest(name: 'Alice', langCode: 'fr'),
    );
    expect(greeting.greeting, 'Bonjour Alice from Gabriel');
    expect(greetingController.greeting, 'Bonjour Alice from Gabriel');

    final status = await coaxClient.memberStatus(
      MemberStatusRequest(slug: 'gabriel-greeting-go'),
    );
    expect(status.member.state, MemberState.MEMBER_STATE_CONNECTED);

    final tell = await coaxClient.tell(
      TellRequest(
        memberSlug: 'gabriel-greeting-go',
        method: 'greeting.v1.GreetingService/SayHello',
        payload: utf8.encode('{"name":"Alice","langCode":"fr"}'),
      ),
    );
    expect(jsonDecode(utf8.decode(tell.payload)), <String, Object?>{
      'greeting': 'Bonjour Alice from Gabriel',
    });
    expect(greetingController.userName, 'Alice');
    expect(greetingController.selectedLanguageCode, 'fr');
    expect(greetingController.greeting, 'Bonjour Alice from Gabriel');

    await coaxClient.turnOffCoax(TurnOffCoaxRequest());
    await waitForCoaxUpdate();

    expect(coaxController.isEnabled, isFalse);
  });

  test(
    'ConnectMember retries transient tcp startup and leaves the member ready',
    () async {
      var tcpAttempts = 0;
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([
          holon('gabriel-greeting-swift'),
          holon('gabriel-greeting-c'),
        ]),
        connector: _ScriptedHolonConnector((holon, transport) {
          if (holon.slug == 'gabriel-greeting-c' &&
              transport == 'tcp' &&
              tcpAttempts++ == 0) {
            throw StateError('temporary tcp startup race');
          }
          return FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
              language(code: 'fr', name: 'French', native: 'Francais'),
            ],
            greetingBuilder: ({required name, required langCode}) {
              return langCode == 'fr' ? 'Bonjour $name' : 'Hello $name';
            },
          );
        }),
        initialTransport: 'stdio',
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final coaxClient = CoaxServiceClient(channel);
      final appClient = GreetingAppServiceClient(channel);

      final member = await coaxClient.connectMember(
        ConnectMemberRequest(slug: 'gabriel-greeting-c', transport: 'tcp'),
      );
      expect(member.member.state, MemberState.MEMBER_STATE_CONNECTED);
      expect(greetingController.selectedHolon?.slug, 'gabriel-greeting-c');
      expect(greetingController.transport, 'tcp');
      expect(greetingController.availableLanguages, isNotEmpty);
      expect(tcpAttempts, 2);

      final greeting = await appClient.greet(
        GreetRequest(name: 'Bob', langCode: 'fr'),
      );
      expect(greeting.greeting, 'Bonjour Bob');
    },
  );

  test(
    'SelectTransport switches the active transport and reloads the holon',
    () async {
      final connector = FakeHolonConnector(
        factories: <String, FakeGreetingHolonConnection Function(String)>{
          'gabriel-greeting-swift': (transport) => FakeGreetingHolonConnection(
            languages: [
              language(code: 'en', name: 'English', native: 'English'),
              language(code: 'fr', name: 'French', native: 'Francais'),
            ],
            greetingBuilder: ({required name, required langCode}) =>
                '$transport:$langCode:$name',
          ),
        },
      );
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: connector,
        initialTransport: 'stdio',
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      final response = await appClient.selectTransport(
        SelectTransportRequest(transport: 'tcp'),
      );

      expect(response.transport, 'tcp');
      expect(greetingController.transport, 'tcp');
      expect(greetingController.isRunning, isTrue);
      expect(greetingController.availableLanguages, isNotEmpty);
      expect(
        connector.connectCalls,
        containsAllInOrder(<(String, String)>[
          ('gabriel-greeting-swift', 'stdio'),
          ('gabriel-greeting-swift', 'tcp'),
        ]),
      );
    },
  );

  test('SelectTransport rejects invalid transport names', () async {
    final greetingController = GreetingController(
      catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
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
    );
    final coaxController = buildCoaxManager(
      greetingController: greetingController,
    );

    await greetingController.initialize();
    final port = await reserveTcpPort();
    await coaxController.setServerPortText(port.toString());
    await coaxController.setEnabled(true);

    final channel = clientChannelFromListenUri(coaxController.listenUri!);
    addTearDown(() async {
      await channel.shutdown();
      await coaxController.shutdown();
      await greetingController.shutdown();
    });

    final appClient = GreetingAppServiceClient(channel);
    await expectLater(
      appClient.selectTransport(SelectTransportRequest(transport: '2')),
      throwsA(
        isA<GrpcError>().having(
          (error) => error.code,
          'code',
          StatusCode.invalidArgument,
        ),
      ),
    );
  });

  test(
    'SelectLanguage rejects invalid codes and preserves the current selection',
    () async {
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (_) => FakeGreetingHolonConnection(
              languages: [
                language(code: 'en', name: 'English', native: 'English'),
                language(code: 'fr', name: 'French', native: 'Francais'),
              ],
              greetingBuilder: ({required name, required langCode}) =>
                  '$langCode:$name',
            ),
          },
        ),
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      final selected = await appClient.selectLanguage(
        SelectLanguageRequest(code: 'fr'),
      );

      expect(selected.code, 'fr');
      expect(greetingController.selectedLanguageCode, 'fr');

      await expectLater(
        appClient.selectLanguage(SelectLanguageRequest(code: 'zz')),
        throwsA(
          isA<GrpcError>().having(
            (error) => error.code,
            'code',
            StatusCode.invalidArgument,
          ),
        ),
      );

      expect(greetingController.selectedLanguageCode, 'fr');

      final greeting = await appClient.greet(GreetRequest(name: 'Alice'));
      expect(greeting.greeting, 'fr:Alice');
    },
  );

  test(
    'SelectTransport rejects unix when the platform does not support it',
    () async {
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
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
        capabilities: const AppPlatformCapabilities(supportsUnixSockets: false),
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      await expectLater(
        appClient.selectTransport(SelectTransportRequest(transport: 'unix')),
        throwsA(
          isA<GrpcError>().having(
            (error) => error.code,
            'code',
            StatusCode.invalidArgument,
          ),
        ),
      );
    },
  );

  test(
    'SelectTransport returns the connection error when reconnect fails',
    () async {
      final greetingController = GreetingController(
        catalog: FakeHolonCatalog([holon('gabriel-greeting-swift')]),
        connector: FakeHolonConnector(
          factories: <String, FakeGreetingHolonConnection Function(String)>{
            'gabriel-greeting-swift': (transport) {
              if (transport == 'tcp') {
                throw StateError('tcp dial failed');
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
        initialTransport: 'stdio',
      );
      final coaxController = buildCoaxManager(
        greetingController: greetingController,
      );

      await greetingController.initialize();
      final port = await reserveTcpPort();
      await coaxController.setServerPortText(port.toString());
      await coaxController.setEnabled(true);

      final channel = clientChannelFromListenUri(coaxController.listenUri!);
      addTearDown(() async {
        await channel.shutdown();
        await coaxController.shutdown();
        await greetingController.shutdown();
      });

      final appClient = GreetingAppServiceClient(channel);
      await expectLater(
        appClient.selectTransport(SelectTransportRequest(transport: 'tcp')),
        throwsA(
          isA<GrpcError>()
              .having((error) => error.code, 'code', StatusCode.unavailable)
              .having(
                (error) => error.message,
                'message',
                contains('Failed to start Gabriel holon'),
              ),
        ),
      );
    },
  );
}

class _ScriptedHolonConnector implements GreetingHolonConnectionFactory {
  _ScriptedHolonConnector(this._onConnect);

  final GreetingHolonConnection Function(
    GabrielHolonIdentity holon,
    String transport,
  )
  _onConnect;

  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    return _onConnect(holon, transport);
  }
}
