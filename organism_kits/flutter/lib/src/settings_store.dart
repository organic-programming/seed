import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;

import 'coax_configuration.dart';

const _coaxEnabledKey = 'coax.server.enabled';
const _coaxSettingsKey = 'coax.server.settings';
const _coaxServerEnabledEnv = 'OP_COAX_SERVER_ENABLED';
const _coaxServerListenUriEnv = 'OP_COAX_SERVER_LISTEN_URI';

abstract interface class SettingsStore {
  bool readBool(String key, {bool defaultValue = false});
  String readString(String key, {String defaultValue = ''});
  Future<void> writeBool(String key, bool value);
  Future<void> writeString(String key, String value);
}

class FileSettingsStore implements SettingsStore {
  FileSettingsStore._(this._file, this._values);

  final File _file;
  final Map<String, Object?> _values;

  static Future<FileSettingsStore> create({
    String applicationId = 'holons-app',
    String applicationName = 'Holons App',
  }) async {
    final directory = Directory(
      _settingsDirectoryPath(
        applicationId: applicationId,
        applicationName: applicationName,
      ),
    );
    directory.createSync(recursive: true);

    final file = File(p.join(directory.path, 'settings.json'));
    final values = <String, Object?>{};
    if (file.existsSync()) {
      try {
        final decoded = jsonDecode(await file.readAsString());
        if (decoded is Map) {
          for (final entry in decoded.entries) {
            values[entry.key.toString()] = entry.value;
          }
        }
      } on Object {
        // Recreate the file on next write.
      }
    }

    return FileSettingsStore._(file, values);
  }

  @override
  bool readBool(String key, {bool defaultValue = false}) {
    final value = _values[key];
    return value is bool ? value : defaultValue;
  }

  @override
  String readString(String key, {String defaultValue = ''}) {
    final value = _values[key];
    return value is String ? value : defaultValue;
  }

  @override
  Future<void> writeBool(String key, bool value) async {
    _values[key] = value;
    await _flush();
  }

  @override
  Future<void> writeString(String key, String value) async {
    _values[key] = value;
    await _flush();
  }

  Future<void> _flush() async {
    await _file.writeAsString(
      '${const JsonEncoder.withIndent('  ').convert(_values)}\n',
    );
  }
}

class MemorySettingsStore implements SettingsStore {
  final Map<String, Object?> _values = <String, Object?>{};

  @override
  bool readBool(String key, {bool defaultValue = false}) {
    final value = _values[key];
    return value is bool ? value : defaultValue;
  }

  @override
  String readString(String key, {String defaultValue = ''}) {
    final value = _values[key];
    return value is String ? value : defaultValue;
  }

  @override
  Future<void> writeBool(String key, bool value) async {
    _values[key] = value;
  }

  @override
  Future<void> writeString(String key, String value) async {
    _values[key] = value;
  }
}

Future<void> applyLaunchEnvironmentOverrides(
  SettingsStore store, {
  Map<String, String>? environment,
  CoaxSettingsDefaults? defaults,
}) async {
  final env = environment ?? Platform.environment;
  final coaxDefaults = defaults ?? CoaxSettingsDefaults.standard();
  final listenUri = (env[_coaxServerListenUriEnv] ?? '').trim();
  final enabled = _parseBoolOverride(env[_coaxServerEnabledEnv]);

  if (listenUri.isEmpty && enabled == null) {
    return;
  }

  if (listenUri.isNotEmpty) {
    await store.writeString(
      _coaxSettingsKey,
      _snapshotFromListenUri(listenUri, defaults: coaxDefaults).encode(),
    );
  }

  if (enabled != null) {
    await store.writeBool(_coaxEnabledKey, enabled);
    return;
  }

  if (listenUri.isNotEmpty) {
    await store.writeBool(_coaxEnabledKey, true);
  }
}

String _settingsDirectoryPath({
  required String applicationId,
  required String applicationName,
}) {
  final home = Platform.environment['HOME']?.trim();

  if (Platform.isWindows) {
    final appData = Platform.environment['APPDATA']?.trim();
    if (appData != null && appData.isNotEmpty) {
      return p.join(appData, 'Organic Programming', applicationName);
    }
  }

  if (Platform.isMacOS && home != null && home.isNotEmpty) {
    return p.join(
      home,
      'Library',
      'Application Support',
      'Organic Programming',
      applicationName,
    );
  }

  if (Platform.isLinux) {
    final xdg = Platform.environment['XDG_CONFIG_HOME']?.trim();
    if (xdg != null && xdg.isNotEmpty) {
      return p.join(xdg, 'organic-programming', applicationId);
    }
    if (home != null && home.isNotEmpty) {
      return p.join(home, '.config', 'organic-programming', applicationId);
    }
  }

  return p.join(Directory.current.path, '.${applicationId.trim()}');
}

bool? _parseBoolOverride(String? value) {
  switch ((value ?? '').trim().toLowerCase()) {
    case '1':
    case 'true':
    case 'yes':
    case 'on':
      return true;
    case '0':
    case 'false':
    case 'no':
    case 'off':
      return false;
    default:
      return null;
  }
}

CoaxSettingsSnapshot _snapshotFromListenUri(
  String listenUri, {
  required CoaxSettingsDefaults defaults,
}) {
  final trimmed = listenUri.trim();
  if (trimmed.startsWith('unix://')) {
    return CoaxSettingsSnapshot(
      serverTransport: CoaxServerTransport.unix,
      serverHost: defaults.serverHost,
      serverPortText: defaults.serverPortText,
      serverUnixPath: trimmed.substring('unix://'.length),
    );
  }

  if (trimmed.startsWith('tcp://')) {
    final parsed = Uri.tryParse(trimmed);
    final host = parsed?.host.trim().isNotEmpty == true
        ? parsed!.host.trim()
        : defaults.serverHost;
    final port = parsed?.hasPort == true
        ? parsed!.port.toString()
        : defaults.serverPortText;
    return CoaxSettingsSnapshot(
      serverTransport: CoaxServerTransport.tcp,
      serverHost: host,
      serverPortText: port,
      serverUnixPath: defaults.serverUnixPath,
    );
  }

  return defaults.snapshot();
}
