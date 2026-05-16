import 'dart:convert';
import 'dart:io';

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

    final file = File(_joinPath(directory.path, 'settings.json'));
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

String _settingsDirectoryPath({
  required String applicationId,
  required String applicationName,
}) {
  final normalizedApplicationId =
      applicationId.trim().isEmpty ? 'holons-app' : applicationId.trim();
  final normalizedApplicationName =
      applicationName.trim().isEmpty ? 'Holons App' : applicationName.trim();
  final home = Platform.environment['HOME']?.trim();

  if (Platform.isWindows) {
    final appData = Platform.environment['APPDATA']?.trim();
    if (appData != null && appData.isNotEmpty) {
      return _joinPath(
        appData,
        'Organic Programming',
        normalizedApplicationName,
      );
    }
  }

  if (Platform.isMacOS && home != null && home.isNotEmpty) {
    return _joinPath(
      home,
      'Library',
      'Application Support',
      'Organic Programming',
      normalizedApplicationName,
    );
  }

  if (Platform.isLinux) {
    final xdg = Platform.environment['XDG_CONFIG_HOME']?.trim();
    if (xdg != null && xdg.isNotEmpty) {
      return _joinPath(xdg, 'organic-programming', normalizedApplicationId);
    }
    if (home != null && home.isNotEmpty) {
      return _joinPath(
        home,
        '.config',
        'organic-programming',
        normalizedApplicationId,
      );
    }
  }

  return _joinPath(Directory.current.path, '.$normalizedApplicationId');
}

String _joinPath(
  String first,
  String second, [
  String? third,
  String? fourth,
  String? fifth,
]) {
  final segments = <String>[first, second];
  if (third != null) {
    segments.add(third);
  }
  if (fourth != null) {
    segments.add(fourth);
  }
  if (fifth != null) {
    segments.add(fifth);
  }
  return segments
      .where((segment) => segment.trim().isNotEmpty)
      .join(Platform.pathSeparator);
}
