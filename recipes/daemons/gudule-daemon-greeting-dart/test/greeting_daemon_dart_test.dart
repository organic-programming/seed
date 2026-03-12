import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';
import 'package:test/test.dart';

import '../bin/main.dart';
import '../gen/dart/greeting/v1/greeting.pb.dart';
import '../gen/dart/greeting/v1/greeting.pbgrpc.dart';
import '../lib/src/greetings.dart';
import '../lib/src/recipe_paths.dart';

void main() {
  test('greeting table exposes 56 languages', () {
    expect(greetings, hasLength(56));
  });

  test('lookup falls back to English', () {
    expect(lookupGreeting('??').code, equals('en'));
  });

  test('serve round-trip returns Bonjour for French', () async {
    final root = findRecipeRoot();
    final running = await startWithOptions(
      'tcp://127.0.0.1:0',
      <Service>[GreetingService()],
      options: ServeOptions(
        protoDir: '$root/protos',
        holonYamlPath: '$root/holon.yaml',
      ),
    );
    addTearDown(() async => running.stop());

    final port = int.parse(running.publicUri.split(':').last);
    final channel = ClientChannel(
      '127.0.0.1',
      port: port,
      options: const ChannelOptions(
        credentials: ChannelCredentials.insecure(),
      ),
    );
    addTearDown(() async => channel.shutdown());

    final client = GreetingServiceClient(channel);
    final languages = await client.listLanguages(ListLanguagesRequest());
    final greeting = await client.sayHello(
      SayHelloRequest()
        ..langCode = 'fr'
        ..name = 'Ada',
    );

    expect(languages.languages, hasLength(56));
    expect(greeting.greeting, equals('Bonjour, Ada !'));
  });
}
