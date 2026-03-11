import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

/// Connects to either an explicit daemon target or a staged packaged daemon.
class DaemonLauncher {
  static const String _packagedHolonSlug = 'greeting-daemon';
  static const String _packagedHolonUuid =
      '6492d55a-55b8-4ecb-a406-2a2a401f7c01';
  static const String _packagedBinaryName = 'gudule-greeting-daemon';
  static const String _devBinaryName = 'gudule-daemon-greeting-go';

  Directory? _stageRoot;

  Future<ClientChannel> start() async {
    await stop();

    final targetOverride = _targetOverride();
    if (targetOverride != null) {
      return holons.connect(targetOverride);
    }

    final daemonPath = _resolveDaemonPath();
    final stageRoot = await _stageHolonRoot(daemonPath);
    _stageRoot = stageRoot;

    final previousDirectory = Directory.current;
    try {
      Directory.current = stageRoot.path;
      return holons.connect(_packagedHolonSlug);
    } finally {
      Directory.current = previousDirectory;
    }
  }

  Future<void> stop([ClientChannel? channel]) async {
    try {
      if (channel != null) {
        await holons.disconnect(channel);
      }
    } finally {
      final stageRoot = _stageRoot;
      _stageRoot = null;
      if (stageRoot != null && stageRoot.existsSync()) {
        await stageRoot.delete(recursive: true);
      }
    }
  }

  String? _targetOverride() {
    final value = Platform.environment['GUDULE_DAEMON_TARGET']?.trim() ?? '';
    return value.isEmpty ? null : value;
  }

  String _resolveDaemonPath() {
    for (final candidate in _daemonCandidates()) {
      final file = File(candidate);
      if (file.existsSync()) {
        return file.absolute.path;
      }
    }
    throw StateError('Daemon binary not found: $_packagedBinaryName');
  }

  List<String> _daemonCandidates() {
    final exeDir = File(Platform.resolvedExecutable).parent;
    final currentDir = Directory.current;
    final packagedName = _platformBinaryName(_packagedBinaryName);
    final devName = _platformBinaryName(_devBinaryName);

    return <String>[
      if (Platform.isMacOS)
        '${exeDir.parent.path}${Platform.pathSeparator}Resources${Platform.pathSeparator}$packagedName',
      '${exeDir.path}${Platform.pathSeparator}$packagedName',
      '${currentDir.path}${Platform.pathSeparator}$packagedName',
      '${currentDir.path}${Platform.pathSeparator}..${Platform.pathSeparator}..${Platform.pathSeparator}daemons${Platform.pathSeparator}greeting-daemon-go${Platform.pathSeparator}.op${Platform.pathSeparator}build${Platform.pathSeparator}bin${Platform.pathSeparator}$devName',
      '${currentDir.path}${Platform.pathSeparator}..${Platform.pathSeparator}..${Platform.pathSeparator}daemons${Platform.pathSeparator}greeting-daemon-go${Platform.pathSeparator}$devName',
    ];
  }

  Future<Directory> _stageHolonRoot(String binaryPath) async {
    final root = await Directory.systemTemp.createTemp(
      'greeting-daemon-stage-',
    );
    final holonDir = Directory(
      '${root.path}${Platform.pathSeparator}holons${Platform.pathSeparator}$_packagedHolonSlug',
    );
    await holonDir.create(recursive: true);
    await File(
      '${holonDir.path}${Platform.pathSeparator}holon.yaml',
    ).writeAsString(_manifestFor(binaryPath));
    return root;
  }

  String _manifestFor(String binaryPath) {
    final escapedPath = binaryPath.replaceAll('\\', '\\\\').replaceAll('"', '\\"');
    return '''
schema: holon/v0
uuid: "$_packagedHolonUuid"
given_name: "greeting"
family_name: "daemon"
motto: "Packaged greeting daemon fallback."
composer: "Codex"
clade: deterministic/pure
status: draft
born: "2026-03-11"
generated_by: codex
kind: native
build:
  runner: recipe
artifacts:
  binary: "$escapedPath"
''';
  }

  String _platformBinaryName(String base) {
    if (Platform.isWindows) {
      return '$base.exe';
    }
    return base;
  }
}
