import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

import '../gen/v1/greeting.pbgrpc.dart';
import '../model/app_model.dart';

const _holonRpcTimeout = Duration(seconds: 20);
final _callOptions = CallOptions(timeout: _holonRpcTimeout);
final _greetingMethodRegistry = holons.UnaryJsonMethodRegistry(
  <holons.UnaryJsonMethodDescriptor<dynamic, dynamic>>[
    holons.UnaryJsonMethodDescriptor<ListLanguagesRequest, ListLanguagesResponse>(
      path: '/greeting.v1.GreetingService/ListLanguages',
      createRequest: ListLanguagesRequest.new,
      createResponse: ListLanguagesResponse.new,
      defaultCallOptions: _callOptions,
    ),
    holons.UnaryJsonMethodDescriptor<SayHelloRequest, SayHelloResponse>(
      path: '/greeting.v1.GreetingService/SayHello',
      createRequest: SayHelloRequest.new,
      createResponse: SayHelloResponse.new,
      defaultCallOptions: _callOptions,
    ),
  ],
);

abstract interface class GreetingHolonConnection {
  Future<List<Language>> listLanguages();
  Future<String> sayHello({required String name, required String langCode});
  Future<Object?> tell({required String method, Object? payload});
  Future<void> close();
}

abstract interface class GreetingHolonConnectionFactory {
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  });
}

class DesktopGreetingHolonConnectionFactory
    implements GreetingHolonConnectionFactory {
  DesktopGreetingHolonConnectionFactory({
    HolonConnector<GabrielHolonIdentity>? connector,
  }) : _connector =
           connector ??
           DesktopHolonConnector<GabrielHolonIdentity>(
             slugOf: (holon) => holon.slug,
             buildRunnerOf: (holon) => holon.buildRunner,
           );

  final HolonConnector<GabrielHolonIdentity> _connector;

  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    final channel = await _connector.connect(holon, transport: transport);
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
  Future<Object?> tell({required String method, Object? payload}) {
    return holons.invokeUnaryJson(
      _channel,
      method,
      payload: payload,
      registry: _greetingMethodRegistry,
      callOptions: _callOptions,
    );
  }

  @override
  Future<void> close() async {
    await holons.disconnectAsync(_channel);
  }
}
