import 'dart:io';

import 'coax_configuration.dart';

class AppPlatformCapabilities {
  const AppPlatformCapabilities({required this.supportsUnixSockets});

  factory AppPlatformCapabilities.desktopCurrent() {
    return AppPlatformCapabilities(supportsUnixSockets: !Platform.isWindows);
  }

  final bool supportsUnixSockets;

  List<String> get appTransports => supportsUnixSockets
      ? const ['stdio', 'unix', 'tcp']
      : const ['stdio', 'tcp'];

  List<CoaxServerTransport> get coaxServerTransports => supportsUnixSockets
      ? CoaxServerTransport.values
      : const [CoaxServerTransport.tcp];
}

String normalizedTransportSelection(String? value) {
  switch ((value ?? '').trim().toLowerCase()) {
    case '':
    case 'auto':
    case 'stdio':
    case 'stdio://':
      return 'stdio';
    case 'unix':
    case 'unix://':
      return 'unix';
    case 'tcp':
    case 'tcp://':
      return 'tcp';
    default:
      return 'stdio';
  }
}

String? canonicalTransportName(String value) {
  switch (value.trim().toLowerCase()) {
    case 'stdio':
      return 'stdio';
    case 'unix':
      return 'unix';
    case 'tcp':
      return 'tcp';
    default:
      return null;
  }
}

String transportTitle(String value) {
  switch (normalizedTransportSelection(value)) {
    case 'unix':
      return 'unix';
    case 'tcp':
      return 'tcp';
    default:
      return 'stdio';
  }
}
