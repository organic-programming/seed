import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import '../gen/v1/greeting.pbgrpc.dart';
import '../model/app_model.dart';

final _callOptions = CallOptions(timeout: const Duration(seconds: 2));

abstract interface class GreetingHolonConnection {
  Future<List<Language>> listLanguages();
  Future<String> sayHello({required String name, required String langCode});
  Future<void> close();
}

abstract interface class HolonConnector {
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  });
}

class DesktopHolonConnector implements HolonConnector {
  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    final channel =
        await holons.connect(
              holon.slug,
              holons.ConnectOptions(
                transport: normalizedTransportSelection(transport),
                timeout: const Duration(seconds: 5),
              ),
            )
            as ClientChannel;
    return DesktopGreetingHolonConnection(channel);
  }
}

class DesktopGreetingHolonConnection implements GreetingHolonConnection {
  DesktopGreetingHolonConnection(this._channel)
    : _client = GreetingServiceClient(_channel);

  final ClientChannel _channel;
  final GreetingServiceClient _client;

  @override
  Future<List<Language>> listLanguages() async {
    final response = await _client.listLanguages(
      ListLanguagesRequest(),
      options: _callOptions,
    );
    return response.languages;
  }

  @override
  Future<String> sayHello({
    required String name,
    required String langCode,
  }) async {
    final response = await _client.sayHello(
      SayHelloRequest(name: name, langCode: langCode),
      options: _callOptions,
    );
    return response.greeting;
  }

  @override
  Future<void> close() async {
    holons.disconnect(_channel);
  }
}
