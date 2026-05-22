import 'dart:io';

import 'package:holons/holons.dart' show SettingsStore;

import 'coax_configuration.dart';

const _coaxEnabledKey = 'coax.server.enabled';
const _coaxSettingsKey = 'coax.server.settings';
const _coaxServerEnabledEnv = 'OP_COAX_SERVER_ENABLED';
const _coaxServerListenUriEnv = 'OP_COAX_SERVER_LISTEN_URI';

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
