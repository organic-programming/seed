import 'dart:convert';
import 'dart:io';

import 'package:collection/collection.dart';

enum CoaxSurfaceState {
  off('OFF'),
  saved('SAVED'),
  announced('ANNOUNCED'),
  live('LIVE'),
  error('ERROR');

  const CoaxSurfaceState(this.badgeTitle);

  final String badgeTitle;
}

enum CoaxServerTransport {
  tcp('TCP', 60000),
  unix('Unix socket', 0);

  const CoaxServerTransport(this.title, this.defaultPort);

  final String title;
  final int defaultPort;

  static CoaxServerTransport fromRaw(String value) {
    switch (value.trim().toLowerCase()) {
      case 'unix':
        return CoaxServerTransport.unix;
      case 'tcp':
      case 'websocket':
      case 'restsse':
      default:
        return CoaxServerTransport.tcp;
    }
  }
}

class CoaxSettingsSnapshot {
  const CoaxSettingsSnapshot({
    required this.serverEnabled,
    required this.serverTransport,
    required this.serverHost,
    required this.serverPortText,
    required this.serverUnixPath,
  });

  static const defaultHost = '127.0.0.1';
  static const defaultUnixPath = '/tmp/gabriel-greeting-coax.sock';
  static const defaults = CoaxSettingsSnapshot(
    serverEnabled: true,
    serverTransport: CoaxServerTransport.tcp,
    serverHost: defaultHost,
    serverPortText: '60000',
    serverUnixPath: defaultUnixPath,
  );

  final bool serverEnabled;
  final CoaxServerTransport serverTransport;
  final String serverHost;
  final String serverPortText;
  final String serverUnixPath;

  Map<String, Object?> toJson() => <String, Object?>{
    'serverEnabled': serverEnabled,
    'serverTransport': serverTransport.name,
    'serverHost': serverHost,
    'serverPortText': serverPortText,
    'serverUnixPath': serverUnixPath,
  };

  factory CoaxSettingsSnapshot.fromJson(Map<String, dynamic> json) {
    return CoaxSettingsSnapshot(
      serverEnabled: json['serverEnabled'] == true,
      serverTransport: CoaxServerTransport.fromRaw(
        (json['serverTransport'] as String?) ?? '',
      ),
      serverHost: (json['serverHost'] as String?) ?? defaultHost,
      serverPortText: (json['serverPortText'] as String?) ?? '60000',
      serverUnixPath: (json['serverUnixPath'] as String?) ?? defaultUnixPath,
    );
  }

  String encode() => jsonEncode(toJson());

  static CoaxSettingsSnapshot decode(String value) {
    if (value.trim().isEmpty) {
      return defaults;
    }
    final decoded = jsonDecode(value);
    if (decoded is! Map<String, dynamic>) {
      return defaults;
    }
    return CoaxSettingsSnapshot.fromJson(decoded);
  }
}

class CoaxSurfaceStatus {
  const CoaxSurfaceStatus({
    required this.id,
    required this.title,
    required this.endpoint,
    required this.state,
  });

  final String id;
  final String title;
  final String? endpoint;
  final CoaxSurfaceState state;
}

class GabrielHolonIdentity {
  const GabrielHolonIdentity({
    required this.slug,
    required this.familyName,
    required this.binaryName,
    required this.buildRunner,
    required this.displayName,
    required this.sortRank,
    required this.holonUuid,
    required this.born,
    required this.sourceKind,
    required this.discoveryPath,
    required this.hasSource,
  });

  final String slug;
  final String familyName;
  final String binaryName;
  final String buildRunner;
  final String displayName;
  final int sortRank;
  final String holonUuid;
  final String born;
  final String sourceKind;
  final String discoveryPath;
  final bool hasSource;

  String get id => slug;
  String get variant => slug.replaceFirst('gabriel-greeting-', '');

  static GabrielHolonIdentity? fromDiscovered(dynamic ref) {
    final info = ref.info;
    if (info == null) {
      return null;
    }
    final slug = info.slug.trim();
    if (!slug.startsWith('gabriel-greeting-') ||
        slug == 'gabriel-greeting-app-swiftui' ||
        slug == 'gabriel-greeting-app-flutter') {
      return null;
    }
    final entrypoint = info.entrypoint.trim();
    return GabrielHolonIdentity(
      slug: slug,
      familyName: info.identity.familyName.trim(),
      binaryName: entrypoint.isEmpty
          ? slug
          : entrypoint.split(Platform.pathSeparator).last,
      buildRunner: info.runner.trim(),
      displayName: displayNameFor(slug),
      sortRank: sortRankFor(slug),
      holonUuid: info.uuid.trim(),
      born: '',
      sourceKind: _sourceKindForUrl(ref.url as String),
      discoveryPath: _discoveryPathFromUrl(ref.url as String),
      hasSource: info.hasSource,
    );
  }

  static String displayNameFor(String slug) {
    switch (slug.replaceFirst('gabriel-greeting-', '')) {
      case 'cpp':
        return 'Gabriel (C++)';
      case 'csharp':
        return 'Gabriel (C#)';
      case 'node':
        return 'Gabriel (Node.js)';
      default:
        final variant = slug
            .replaceFirst('gabriel-greeting-', '')
            .split('-')
            .where((part) => part.trim().isNotEmpty)
            .map(_capitalize)
            .join(' ');
        return 'Gabriel ($variant)';
    }
  }

  static int sortRankFor(String slug) {
    return _sortOrder[slug] ?? 999;
  }

  static String _capitalize(String value) {
    if (value.isEmpty) {
      return value;
    }
    return '${value[0].toUpperCase()}${value.substring(1)}';
  }

  static String _discoveryPathFromUrl(String url) {
    final uri = Uri.tryParse(url);
    if (uri != null && uri.scheme == 'file') {
      return uri.toFilePath();
    }
    return url;
  }

  static String _sourceKindForUrl(String url) {
    final path = _discoveryPathFromUrl(url);
    if (path.contains('.op${Platform.pathSeparator}build')) {
      return 'built';
    }
    if (path.contains(
      '${Platform.pathSeparator}Holons${Platform.pathSeparator}',
    )) {
      return 'siblings';
    }
    return 'source';
  }

  @override
  bool operator ==(Object other) {
    return other is GabrielHolonIdentity && other.slug == slug;
  }

  @override
  int get hashCode => slug.hashCode;

  static final _sortOrder = <String, int>{
    'gabriel-greeting-swift': 0,
    'gabriel-greeting-go': 1,
    'gabriel-greeting-rust': 2,
    'gabriel-greeting-python': 3,
    'gabriel-greeting-c': 4,
    'gabriel-greeting-cpp': 5,
    'gabriel-greeting-csharp': 6,
    'gabriel-greeting-dart': 7,
    'gabriel-greeting-java': 8,
    'gabriel-greeting-kotlin': 9,
    'gabriel-greeting-node': 10,
    'gabriel-greeting-ruby': 11,
  };
}

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

String coaxTransportTitle(CoaxServerTransport value) => value.title;

String formatHolonListTitle(List<GabrielHolonIdentity> holons) {
  return holons.map((item) => item.displayName).join(', ');
}

int sanitizedPort(String value, int fallback) {
  final parsed = int.tryParse(value.trim());
  if (parsed == null || parsed < 1 || parsed > 65535) {
    return fallback;
  }
  return parsed;
}

GabrielHolonIdentity? preferredHolon(Iterable<GabrielHolonIdentity> holons) {
  return holons.sortedBy<num>((item) => item.sortRank).firstOrNull;
}
