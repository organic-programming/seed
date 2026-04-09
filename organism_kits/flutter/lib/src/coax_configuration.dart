import 'dart:convert';
import 'dart:io';

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
    required this.serverTransport,
    required this.serverHost,
    required this.serverPortText,
    required this.serverUnixPath,
  });

  static const defaultHost = '127.0.0.1';
  static final String defaultUnixPath = defaultCoaxUnixPath();
  static final defaults = CoaxSettingsDefaults.standard().snapshot();

  final CoaxServerTransport serverTransport;
  final String serverHost;
  final String serverPortText;
  final String serverUnixPath;

  Map<String, Object?> toJson() => <String, Object?>{
    'serverTransport': serverTransport.name,
    'serverHost': serverHost,
    'serverPortText': serverPortText,
    'serverUnixPath': serverUnixPath,
  };

  factory CoaxSettingsSnapshot.fromJson(Map<String, dynamic> json) {
    return CoaxSettingsSnapshot(
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

class CoaxSettingsDefaults {
  const CoaxSettingsDefaults({
    this.serverHost = CoaxSettingsSnapshot.defaultHost,
    this.serverPortText = '60000',
    required this.serverUnixPath,
  });

  factory CoaxSettingsDefaults.standard({
    String socketName = 'organism-holon-coax.sock',
  }) {
    return CoaxSettingsDefaults(
      serverUnixPath: defaultCoaxUnixPath(socketName: socketName),
    );
  }

  final String serverHost;
  final String serverPortText;
  final String serverUnixPath;

  CoaxSettingsSnapshot snapshot({
    CoaxServerTransport serverTransport = CoaxServerTransport.tcp,
  }) {
    return CoaxSettingsSnapshot(
      serverTransport: serverTransport,
      serverHost: serverHost,
      serverPortText: serverPortText,
      serverUnixPath: serverUnixPath,
    );
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

const int _unixSocketPathMaxBytes = 100;

String coaxTransportTitle(CoaxServerTransport value) => value.title;

int sanitizedPort(String value, int fallback) {
  final parsed = int.tryParse(value.trim());
  if (parsed == null || parsed < 1 || parsed > 65535) {
    return fallback;
  }
  return parsed;
}

String defaultCoaxUnixPath({
  String socketName = 'organism-holon-coax.sock',
}) {
  var current = Directory.systemTemp.absolute.path;
  final seen = <String>{};
  while (seen.add(current)) {
    final candidate = '$current${Platform.pathSeparator}$socketName';
    if (_isWritableDirectory(current) &&
        candidate.codeUnits.length <= _unixSocketPathMaxBytes) {
      return candidate;
    }
    final parent = Directory(current).parent.absolute.path;
    if (parent == current) {
      break;
    }
    current = parent;
  }
  return '${Directory.systemTemp.path}${Platform.pathSeparator}$socketName';
}

bool _isWritableDirectory(String path) {
  final directory = Directory(path);
  if (!directory.existsSync()) {
    return false;
  }

  final probe = File(
    '${directory.path}${Platform.pathSeparator}.coax-write-probe-$pid-${DateTime.now().microsecondsSinceEpoch}',
  );
  try {
    probe.writeAsStringSync('probe');
    probe.deleteSync();
    return true;
  } on FileSystemException {
    return false;
  }
}
