import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;
import 'package:holons/holons.dart' show HolonTransportName;
import 'package:protobuf/protobuf.dart';

abstract interface class HolonConnector<T> {
  Future<ClientChannel> connect(T holon, {required String transport});
}

class BundledHolonConnector<T> implements HolonConnector<T> {
  BundledHolonConnector({
    required this.slugOf,
    this.buildRunnerOf,
    this.timeout = const Duration(seconds: 7),
    this.withTransitiveObservability = true,
  });

  final String Function(T holon) slugOf;
  final String Function(T holon)? buildRunnerOf;
  final Duration timeout;
  final bool withTransitiveObservability;

  @override
  Future<ClientChannel> connect(T holon, {required String transport}) async {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: buildRunnerOf?.call(holon),
    );
    return await holons.connect(
          slugOf(holon),
          holons.ConnectOptions(
            transport: effectiveTransport,
            timeout: timeout,
            withTransitiveObservability: withTransitiveObservability,
          ),
        )
        as ClientChannel;
  }
}

class SharedHolonChannels<T> {
  SharedHolonChannels(
    this.connector, {
    required this.slugOf,
    this.buildRunnerOf,
  });

  final HolonConnector<T> connector;
  final String Function(T holon) slugOf;
  final String Function(T holon)? buildRunnerOf;
  final Map<String, Future<ClientChannel>> _channels = {};

  Future<ClientChannel> open(T holon, {required String transport}) {
    final key = _cacheKey(holon, transport);
    return _channels.putIfAbsent(key, () async {
      try {
        return await connector.connect(holon, transport: transport);
      } on Object {
        _channels.remove(key);
        rethrow;
      }
    });
  }

  Future<void> close(T holon, {required String transport}) async {
    final future = _channels.remove(_cacheKey(holon, transport));
    if (future == null) return;
    await holons.disconnectAsync(await future);
  }

  String _cacheKey(T holon, String transport) {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: buildRunnerOf?.call(holon),
    );
    return '${slugOf(holon)}\u0000$effectiveTransport';
  }
}

class UnaryJsonMethodRegistryBuilder {
  UnaryJsonMethodRegistryBuilder({CallOptions? defaultCallOptions})
    : _defaultCallOptions = defaultCallOptions;

  final CallOptions? _defaultCallOptions;
  final List<holons.JsonUnaryMethodDescriptor> _methods =
      <holons.JsonUnaryMethodDescriptor>[];

  UnaryJsonMethodRegistryBuilder
  add<Request extends GeneratedMessage, Response extends GeneratedMessage>({
    required String path,
    required Request Function() createRequest,
    required Response Function() createResponse,
    CallOptions? defaultCallOptions,
  }) {
    _methods.add(
      holons.UnaryJsonMethodDescriptor<Request, Response>(
        path: path,
        createRequest: createRequest,
        createResponse: createResponse,
        defaultCallOptions: defaultCallOptions ?? _defaultCallOptions,
      ),
    );
    return this;
  }

  holons.UnaryJsonMethodRegistry build() {
    return holons.UnaryJsonMethodRegistry(_methods);
  }
}

UnaryJsonMethodRegistryBuilder unaryJsonMethodRegistryBuilder({
  CallOptions? defaultCallOptions,
}) {
  return UnaryJsonMethodRegistryBuilder(defaultCallOptions: defaultCallOptions);
}

String effectiveHolonTransport({
  required String requestedTransport,
  String? buildRunner,
}) {
  final normalized = HolonTransportName.normalize(requestedTransport).rawValue;
  if (!_isBundledMacOSHost()) {
    return normalized;
  }
  if (normalized == 'unix') {
    return 'tcp';
  }
  if (normalized == 'stdio' && buildRunner == 'cmake') {
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

@Deprecated('Use BundledHolonConnector<T>')
typedef DesktopHolonConnector<T> = BundledHolonConnector<T>;
