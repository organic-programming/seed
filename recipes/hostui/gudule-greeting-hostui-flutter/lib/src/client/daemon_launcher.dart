import 'dart:async';
import 'dart:convert';
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
typedef BundledDaemonStarter = Future<BundledDaemonSession> Function(
  String binaryPath,
  String portFilePath,
);

class BundledDaemonSession {
  final Future<void> Function() stop;

  const BundledDaemonSession({required this.stop});
}

class DaemonLauncher {
  static const String _uuid = '1a409a1e-69e3-4846-9f9b-47b0a6f98f84';

  final HolonConnect _connect;
  final HolonDisconnect _disconnect;
  final TempDirectoryFactory _createTempDirectory;
  final CurrentDirectoryGetter _getCurrentDirectory;
  final CurrentDirectorySetter _setCurrentDirectory;
  final BundledDaemonStarter _startBundledDaemon;

  Directory? _root;
  BundledDaemonSession? _daemonSession;

  DaemonLauncher({
    HolonConnect? connect,
    HolonDisconnect? disconnect,
    TempDirectoryFactory? createTempDirectory,
    CurrentDirectoryGetter? getCurrentDirectory,
    CurrentDirectorySetter? setCurrentDirectory,
    BundledDaemonStarter? startBundledDaemon,
  })  : _connect = connect ?? holons.connect,
        _disconnect = disconnect ?? holons.disconnect,
        _createTempDirectory = createTempDirectory ??
            ((prefix) => Directory.systemTemp.createTemp(prefix)),
        _getCurrentDirectory =
            getCurrentDirectory ?? (() => Directory.current.path),
        _setCurrentDirectory =
            setCurrentDirectory ?? ((path) => Directory.current = path),
        _startBundledDaemon = startBundledDaemon ?? _startBundledDaemonDefault;

  String? get stagedRootPath => _root?.path;

  Future<ClientChannel> start(GreetingEndpoint endpoint) async {
    await stop(null);

    final configuredTarget = (endpoint.target ?? '').trim();
    if (configuredTarget.isNotEmpty) {
      return _connect(
        configuredTarget,
        const holons.ConnectOptions(transport: 'tcp'),
      );
    }

    final binaryPath = (endpoint.bundledBinaryPath ?? '').trim();
    final file = File(binaryPath);
    if (!file.existsSync()) {
      throw StateError('Daemon binary not found: $binaryPath');
    }

    final daemon =
        endpoint.daemon ?? GreetingDaemonIdentity.fromBinaryPath(binaryPath);

    final root = await _createTempDirectory('greeting-hostui-flutter-');
    final daemonRoot = await _stageHolonRoot(root, file.absolute.path, daemon);
    final portFilePath = _portFilePath(root.path, daemon.slug);
    final daemonSession = await _startBundledDaemon(
      file.absolute.path,
      portFilePath,
    );
    _root = root;
    _daemonSession = daemonSession;

    try {
      return await _withDiscoveryRoot(
        daemonRoot.path,
        () => _connect(
          daemon.slug,
          holons.ConnectOptions(
            transport: 'tcp',
            start: false,
            portFile: portFilePath,
          ),
        ),
      );
    } catch (_) {
      await daemonSession.stop();
      await _deleteRoot(root);
      _root = null;
      _daemonSession = null;
      rethrow;
    }
  }

  Future<void> stop([ClientChannel? channel]) async {
    final root = _root;
    _root = null;
    final daemonSession = _daemonSession;
    _daemonSession = null;

    try {
      if (channel != null) {
        await _disconnect(channel);
      }
    } finally {
      if (daemonSession != null) {
        await daemonSession.stop();
      }
      if (root != null) {
        await _deleteRoot(root);
      }
    }
  }

  Future<Directory> _stageHolonRoot(
    Directory root,
    String binaryPath,
    GreetingDaemonIdentity daemon,
  ) async {
    final holonDir = Directory(
      '${root.path}${Platform.pathSeparator}holons${Platform.pathSeparator}${daemon.slug}',
    );
    await holonDir.create(recursive: true);
    await File(
      '${holonDir.path}${Platform.pathSeparator}holon.yaml',
    ).writeAsString(_manifestFor(binaryPath, daemon));
    return root;
  }

  String _manifestFor(String binaryRef, GreetingDaemonIdentity daemon) {
    final escapedBinaryRef = _escapeYaml(binaryRef);
    return '''
schema: holon/v0
uuid: "$_uuid"
given_name: "gudule"
family_name: "${daemon.familyName}"
motto: "Greets users in 56 languages through the bundled daemon."
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

  String _escapeYaml(String value) {
    return value.replaceAll('\\', '\\\\').replaceAll('"', '\\"');
  }

  String _portFilePath(String rootPath, String daemonSlug) {
    return '$rootPath${Platform.pathSeparator}.op${Platform.pathSeparator}run${Platform.pathSeparator}$daemonSlug.port';
  }

  Future<T> _withDiscoveryRoot<T>(
    String path,
    Future<T> Function() action,
  ) async {
    final previousDirectory = _getCurrentDirectory();
    try {
      // dart-holons discovery is rooted at Directory.current.
      _setCurrentDirectory(path);
      return await action();
    } finally {
      _setCurrentDirectory(previousDirectory);
    }
  }

  Future<void> _deleteRoot(Directory root) async {
    if (root.existsSync()) {
      await root.delete(recursive: true);
    }
  }
}

Future<BundledDaemonSession> _startBundledDaemonDefault(
  String binaryPath,
  String portFilePath,
) async {
  final process = await Process.start(
    binaryPath,
    const <String>['serve', '--listen', 'tcp://127.0.0.1:0'],
  );
  final advertisedUri = Completer<String>();
  final recentLines = <String>[];

  void onLine(String line) {
    if (recentLines.length == 8) {
      recentLines.removeAt(0);
    }
    recentLines.add(line);
    final uri = _firstUri(line);
    if (uri.isNotEmpty && !advertisedUri.isCompleted) {
      advertisedUri.complete(uri);
    }
  }

  final stdoutSub = utf8.decoder
      .bind(process.stdout)
      .transform(const LineSplitter())
      .listen(onLine);
  final stderrSub = utf8.decoder
      .bind(process.stderr)
      .transform(const LineSplitter())
      .listen(onLine);

  process.exitCode.then((code) {
    if (!advertisedUri.isCompleted) {
      final details = recentLines.isEmpty ? '' : ': ${recentLines.join(' | ')}';
      advertisedUri.completeError(
        StateError(
          'Bundled daemon exited before advertising an address ($code)$details',
        ),
      );
    }
  });

  try {
    final uri = await advertisedUri.future.timeout(const Duration(seconds: 5));
    await _writePortFile(portFilePath, uri);
    return BundledDaemonSession(
      stop: () async {
        await stdoutSub.cancel();
        await stderrSub.cancel();
        await _stopProcess(process);
      },
    );
  } on Object {
    await stdoutSub.cancel();
    await stderrSub.cancel();
    await _stopProcess(process);
    rethrow;
  }
}

Future<void> _writePortFile(String portFilePath, String uri) async {
  final file = File(portFilePath);
  await file.parent.create(recursive: true);
  await file.writeAsString('${uri.trim()}\n');
}

Future<void> _stopProcess(Process process) async {
  process.kill(ProcessSignal.sigterm);
  await process.exitCode.timeout(
    const Duration(seconds: 2),
    onTimeout: () {
      process.kill(ProcessSignal.sigkill);
      return process.exitCode;
    },
  );
}

String _firstUri(String line) {
  for (final field in line.split(RegExp(r'\s+'))) {
    final trimmed = _trimUriField(field);
    if (trimmed.startsWith('tcp://') ||
        trimmed.startsWith('unix://') ||
        trimmed.startsWith('stdio://') ||
        trimmed.startsWith('ws://') ||
        trimmed.startsWith('wss://')) {
      return trimmed;
    }
  }
  return '';
}

String _trimUriField(String value) {
  var start = 0;
  var end = value.length;
  while (start < end && _isUriTrimChar(value.codeUnitAt(start))) {
    start += 1;
  }
  while (end > start && _isUriTrimChar(value.codeUnitAt(end - 1))) {
    end -= 1;
  }
  return value.substring(start, end);
}

bool _isUriTrimChar(int codeUnit) {
  return codeUnit == 34 ||
      codeUnit == 39 ||
      codeUnit == 40 ||
      codeUnit == 41 ||
      codeUnit == 44 ||
      codeUnit == 46 ||
      codeUnit == 91 ||
      codeUnit == 93 ||
      codeUnit == 123 ||
      codeUnit == 125;
}
