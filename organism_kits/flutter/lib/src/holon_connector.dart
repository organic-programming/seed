import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import 'platform_capabilities.dart';

abstract interface class HolonConnector<T> {
  Future<ClientChannel> connect(
    T holon, {
    required String transport,
  });
}

class DesktopHolonConnector<T> implements HolonConnector<T> {
  DesktopHolonConnector({
    required this.slugOf,
    this.buildRunnerOf,
  });

  final String Function(T holon) slugOf;
  final String Function(T holon)? buildRunnerOf;

  @override
  Future<ClientChannel> connect(
    T holon, {
    required String transport,
  }) async {
    final effectiveTransport = effectiveHolonTransport(
      requestedTransport: transport,
      buildRunner: buildRunnerOf?.call(holon),
    );
    return await holons.connect(
          slugOf(holon),
          holons.ConnectOptions(
            transport: effectiveTransport,
            timeout: const Duration(seconds: 7),
          ),
        )
        as ClientChannel;
  }
}

String effectiveHolonTransport({
  required String requestedTransport,
  String? buildRunner,
}) {
  final normalized = normalizedTransportSelection(requestedTransport);
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
