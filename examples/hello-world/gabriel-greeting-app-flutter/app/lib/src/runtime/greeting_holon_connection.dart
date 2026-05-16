import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons_app/holons_app.dart';

import '../gen/v1/greeting.pbgrpc.dart';
import '../model/app_model.dart';

const _holonRpcTimeout = Duration(seconds: 20);
final _callOptions = CallOptions(timeout: _holonRpcTimeout);
final _greetingMethodRegistry = holons.UnaryJsonMethodRegistry(<
  holons.UnaryJsonMethodDescriptor<dynamic, dynamic>
>[
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
]);

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

class BundledGreetingHolonConnectionFactory
    implements GreetingHolonConnectionFactory {
  BundledGreetingHolonConnectionFactory({
    HolonConnector<GabrielHolonIdentity>? connector,
    bool withTransitiveObservability = true,
  }) : _channels = _SharedGreetingHolonChannels(
         connector ??
             _GreetingHolonConnector(
               withTransitiveObservability: withTransitiveObservability,
             ),
       );

  final _SharedGreetingHolonChannels _channels;

  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    final channel = await openChannel(holon, transport: transport);
    return DesktopGreetingHolonConnection(
      channel,
      onClose: () => _channels.close(holon, transport: transport),
    );
  }

  Future<ClientChannel> openChannel(
    GabrielHolonIdentity holon, {
    required String transport,
  }) {
    return _channels.open(holon, transport: transport);
  }
}

@Deprecated('Use BundledGreetingHolonConnectionFactory')
typedef DesktopGreetingHolonConnectionFactory =
    BundledGreetingHolonConnectionFactory;

class DesktopGreetingHolonConnection implements GreetingHolonConnection {
  DesktopGreetingHolonConnection(
    this._channel, {
    Future<void> Function()? onClose,
  }) : _onClose = onClose,
       _client = GreetingServiceClient(_channel);

  final ClientChannel _channel;
  final Future<void> Function()? _onClose;
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
    final close = _onClose;
    if (close != null) {
      await close();
      return;
    }
    await holons.disconnectAsync(_channel);
  }
}

class _SharedGreetingHolonChannels {
  _SharedGreetingHolonChannels(this._connector);

  final HolonConnector<GabrielHolonIdentity> _connector;
  final Map<String, Future<ClientChannel>> _channels = {};

  Future<ClientChannel> open(
    GabrielHolonIdentity holon, {
    required String transport,
  }) {
    final key = _cacheKey(holon, transport);
    return _channels.putIfAbsent(key, () async {
      try {
        return await _connector.connect(holon, transport: transport);
      } on Object {
        _channels.remove(key);
        rethrow;
      }
    });
  }

  Future<void> close(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    final future = _channels.remove(_cacheKey(holon, transport));
    if (future == null) return;
    await holons.disconnectAsync(await future);
  }

  String _cacheKey(GabrielHolonIdentity holon, String transport) {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: holon.buildRunner,
    );
    return '${holon.slug}\u0000$effectiveTransport';
  }
}

class _GreetingHolonConnector implements HolonConnector<GabrielHolonIdentity> {
  _GreetingHolonConnector({required this.withTransitiveObservability});

  final bool withTransitiveObservability;

  @override
  Future<ClientChannel> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: holon.buildRunner,
    );
    return await holons.connect(
          holon.slug,
          holons.ConnectOptions(
            transport: effectiveTransport,
            timeout: const Duration(seconds: 7),
            withTransitiveObservability: withTransitiveObservability,
          ),
        )
        as ClientChannel;
  }
}
