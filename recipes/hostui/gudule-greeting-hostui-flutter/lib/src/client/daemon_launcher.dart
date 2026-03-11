import 'dart:async';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons;

import 'greeting_target.dart';

typedef HolonConnect = Future<ClientChannel> Function(
  String target, [
  holons.ConnectOptions? opts,
]);
typedef HolonDisconnect = Future<void> Function(ClientChannel channel);
typedef TempDirectoryFactory = Future<Directory> Function(String prefix);
typedef CurrentDirectoryGetter = String Function();
typedef CurrentDirectorySetter = void Function(String path);
typedef PidKiller = Future<void> Function(int pid);

class DaemonLauncher {
  static const String _slug = GreetingTargetResolver.daemonSlug;
  static const String _uuid = '1a409a1e-69e3-4846-9f9b-47b0a6f98f84';

  final HolonConnect _connect;
  final HolonDisconnect _disconnect;
  final TempDirectoryFactory _createTempDirectory;
  final CurrentDirectoryGetter _getCurrentDirectory;
  final CurrentDirectorySetter _setCurrentDirectory;
  final PidKiller _killPid;

  Directory? _root;
  File? _pidFile;

  DaemonLauncher({
    HolonConnect? connect,
    HolonDisconnect? disconnect,
    TempDirectoryFactory? createTempDirectory,
    CurrentDirectoryGetter? getCurrentDirectory,
    CurrentDirectorySetter? setCurrentDirectory,
    PidKiller? killPid,
  })  : _connect = connect ?? holons.connect,
        _disconnect = disconnect ?? holons.disconnect,
        _createTempDirectory = createTempDirectory ??
            ((prefix) => Directory.systemTemp.createTemp(prefix)),
        _getCurrentDirectory =
            getCurrentDirectory ?? (() => Directory.current.path),
        _setCurrentDirectory =
            setCurrentDirectory ?? ((path) => Directory.current = path),
        _killPid = killPid ?? _killPidDefault;

  String? get stagedRootPath => _root?.path;

  Future<ClientChannel> start({
    String? target,
    String? bundledBinaryPath,
  }) async {
    await stop(null);

    final configuredTarget = (target ?? '').trim();
    if (configuredTarget.isNotEmpty) {
      return _connect(
        configuredTarget,
        const holons.ConnectOptions(transport: 'tcp'),
      );
    }

    final binaryPath = (bundledBinaryPath ?? '').trim();
    final file = File(binaryPath);
    if (!file.existsSync()) {
      throw StateError('Daemon binary not found: $binaryPath');
    }

    final root = await _createTempDirectory('greeting-hostui-flutter-');
    _root = root;
    final holonDir = Directory(
      '${root.path}${Platform.pathSeparator}holons${Platform.pathSeparator}$_slug',
    );
    await holonDir.create(recursive: true);

    final binaryRef = await _stageBinary(holonDir, file.absolute.path);
    await File(
      '${holonDir.path}${Platform.pathSeparator}holon.yaml',
    ).writeAsString(_manifestFor(binaryRef));

    final previousDirectory = _getCurrentDirectory();
    try {
      _setCurrentDirectory(root.path);
      return await _connect(
        _slug,
        const holons.ConnectOptions(transport: 'tcp'),
      );
    } catch (_) {
      await _stopRecordedProcess();
      await _deleteRoot(root);
      _root = null;
      _pidFile = null;
      rethrow;
    } finally {
      _setCurrentDirectory(previousDirectory);
    }
  }

  Future<void> stop([ClientChannel? channel]) async {
    final root = _root;
    _root = null;
    final pidFile = _pidFile;
    _pidFile = null;

    try {
      if (channel != null) {
        await _disconnect(channel);
      }
    } finally {
      if (pidFile != null) {
        await _stopRecordedProcess(pidFile);
      }
      if (root != null) {
        await _deleteRoot(root);
      }
    }
  }

  Future<String> _stageBinary(Directory holonDir, String binaryPath) async {
    if (Platform.isWindows) {
      return binaryPath;
    }

    final binDir = Directory(
      '${holonDir.path}${Platform.pathSeparator}.op${Platform.pathSeparator}build${Platform.pathSeparator}bin',
    );
    await binDir.create(recursive: true);

    final pidFile = File(
      '${holonDir.path}${Platform.pathSeparator}daemon.pid',
    );
    _pidFile = pidFile;

    final wrapper = File(
      '${binDir.path}${Platform.pathSeparator}gudule-daemon-greeting-go-wrapper',
    );
    await wrapper.writeAsString(_wrapperScript(binaryPath, pidFile.path));
    final result = await Process.run('chmod', <String>['755', wrapper.path]);
    if (result.exitCode != 0) {
      throw StateError('Failed to mark wrapper executable: ${result.stderr}');
    }
    return wrapper.uri.pathSegments.last;
  }

  String _manifestFor(String binaryRef) {
    final escapedBinaryRef = _escapeYaml(binaryRef);
    return '''
schema: holon/v0
uuid: "$_uuid"
given_name: "gudule"
family_name: "Greeting-Daemon-Go"
motto: "Greets users in 56 languages through the extracted Go daemon."
composer: "Codex"
clade: "deterministic/pure"
status: "draft"
born: "2026-03-11"
generated_by: "manual"
kind: native
build:
  runner: go-module
artifacts:
  binary: "$escapedBinaryRef"
''';
  }

  String _wrapperScript(String binaryPath, String pidFilePath) {
    return '''
#!/bin/sh
printf '%s\\n' "\$\$" > ${_shellQuote(pidFilePath)}
exec ${_shellQuote(binaryPath)} "\$@"
''';
  }

  String _escapeYaml(String value) {
    return value.replaceAll('\\', '\\\\').replaceAll('"', '\\"');
  }

  String _shellQuote(String value) {
    return "'${value.replaceAll("'", "'\"'\"'")}'";
  }

  Future<void> _stopRecordedProcess([File? pidFile]) async {
    final file = pidFile ?? _pidFile;
    if (file == null || !file.existsSync()) {
      return;
    }

    final rawPid = (await file.readAsString()).trim();
    final pid = int.tryParse(rawPid);
    if (pid == null) {
      return;
    }
    await _killPid(pid);
  }

  Future<void> _deleteRoot(Directory root) async {
    if (root.existsSync()) {
      await root.delete(recursive: true);
    }
  }
}

Future<void> _killPidDefault(int pid) async {
  if (Platform.isWindows) {
    return;
  }

  Process.killPid(pid, ProcessSignal.sigterm);
  final deadline = DateTime.now().add(const Duration(seconds: 2));
  while (DateTime.now().isBefore(deadline)) {
    if (!await _pidExists(pid)) {
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }

  Process.killPid(pid, ProcessSignal.sigkill);
}

Future<bool> _pidExists(int pid) async {
  final result = await Process.run('/bin/kill', <String>['-0', '$pid']);
  return result.exitCode == 0;
}
