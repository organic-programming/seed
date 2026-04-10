import 'dart:io';

import 'coax_configuration.dart';

enum HolonTransportName {
  stdio('stdio'),
  tcp('tcp'),
  unix('unix');

  const HolonTransportName(this.rawValue);

  final String rawValue;

  String get title => rawValue;

  static HolonTransportName normalize(String? value) {
    switch ((value ?? '').trim().toLowerCase()) {
      case '':
      case 'auto':
      case 'stdio':
      case 'stdio://':
        return HolonTransportName.stdio;
      case 'unix':
      case 'unix://':
        return HolonTransportName.unix;
      case 'tcp':
      case 'tcp://':
        return HolonTransportName.tcp;
      default:
        return HolonTransportName.stdio;
    }
  }

  static HolonTransportName? parseCanonical(String value) {
    switch (value.trim().toLowerCase()) {
      case 'stdio':
        return HolonTransportName.stdio;
      case 'unix':
        return HolonTransportName.unix;
      case 'tcp':
        return HolonTransportName.tcp;
      default:
        return null;
    }
  }
}

class AppPlatformCapabilities {
  const AppPlatformCapabilities({required this.supportsUnixSockets});

  factory AppPlatformCapabilities.desktopCurrent() {
    return AppPlatformCapabilities(supportsUnixSockets: !Platform.isWindows);
  }

  final bool supportsUnixSockets;

  List<HolonTransportName> get holonTransportNames => supportsUnixSockets
      ? const [
          HolonTransportName.stdio,
          HolonTransportName.unix,
          HolonTransportName.tcp,
        ]
      : const [
          HolonTransportName.stdio,
          HolonTransportName.tcp,
        ];

  List<String> get appTransports => holonTransportNames
      .map((transport) => transport.rawValue)
      .toList(growable: false);

  List<CoaxServerTransport> get coaxServerTransports => supportsUnixSockets
      ? CoaxServerTransport.values
      : const [CoaxServerTransport.tcp];
}

@Deprecated('Use HolonTransportName.normalize(value).rawValue')
String normalizedTransportSelection(String? value) {
  return HolonTransportName.normalize(value).rawValue;
}

@Deprecated('Use HolonTransportName.parseCanonical(value)?.rawValue')
String? canonicalTransportName(String value) {
  return HolonTransportName.parseCanonical(value)?.rawValue;
}

@Deprecated('Use HolonTransportName.normalize(value).title')
String transportTitle(String value) {
  return HolonTransportName.normalize(value).title;
}
