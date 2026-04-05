import 'package:flutter_test/flutter_test.dart';
import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/coax.pbgrpc.dart';

import 'package:gabriel_greeting_app_flutter/src/controller/coax_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/controller/greeting_controller.dart';
import 'package:gabriel_greeting_app_flutter/src/gen/v1/holon.pbgrpc.dart';
import 'package:gabriel_greeting_app_flutter/src/settings_store.dart';

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
    final coaxController = CoaxController(
      greetingController: greetingController,
      settingsStore: MemorySettingsStore(),
    );

    await greetingController.initialize();
    final port = await reserveTcpPort();
    await coaxController.setServerPortText(port.toString());
    await coaxController.setIsEnabled(true);

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

    await expectLater(
      coaxClient.tell(
        TellRequest(
          memberSlug: 'gabriel-greeting-go',
          method: 'greeting.v1.GreetingService/SayHello',
        ),
      ),
      throwsA(isA<GrpcError>()),
    );

    await coaxClient.turnOffCoax(TurnOffCoaxRequest());
    await waitForCoaxUpdate();

    expect(coaxController.isEnabled, isFalse);
  });
}
