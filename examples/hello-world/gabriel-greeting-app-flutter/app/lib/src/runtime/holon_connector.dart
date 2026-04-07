import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import '../gen/v1/greeting.pbgrpc.dart';
import '../model/app_model.dart';

const _holonConnectTimeout = Duration(seconds: 7);
const _holonRpcTimeout = Duration(seconds: 20);
final _callOptions = CallOptions(timeout: _holonRpcTimeout);

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

String effectiveHolonTransport(
  GabrielHolonIdentity holon,
  String requestedTransport,
) {
  final normalized = normalizedTransportSelection(requestedTransport);
  if (!_isBundledMacOSHost()) {
    return normalized;
  }
  if (normalized == 'unix') {
    return 'tcp';
  }
  if (normalized == 'stdio' && holon.buildRunner == 'cmake') {
    return 'tcp';
  }
  return normalized;
}

bool _isBundledMacOSHost() {
  if (!Platform.isMacOS) {
    return false;
  }
  return Platform.resolvedExecutable.contains('.app/Contents/');
}

class DesktopHolonConnector implements HolonConnector {
  @override
  Future<GreetingHolonConnection> connect(
    GabrielHolonIdentity holon, {
    required String transport,
  }) async {
    final effectiveTransport = effectiveHolonTransport(holon, transport);
    final channel =
        await holons.connect(
              holon.slug,
              holons.ConnectOptions(
                transport: effectiveTransport,
                timeout: _holonConnectTimeout,
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
