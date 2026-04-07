import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:grpc/src/client/connection.dart' show ClientConnection;
import 'package:grpc/src/client/http2_connection.dart'
    show Http2ClientConnection;

import 'discover.dart';
import 'discovery_probe.dart';
import 'discovery_types.dart';
import 'grpcclient.dart';
import 'transport.dart';

String Function() connectCurrentRootProvider = () => Directory.current.path;
Map<String, String> Function() connectEnvironmentProvider =
    () => Platform.environment;

class ConnectOptions {
  final Duration timeout;
  final String transport;
  final bool start;
  final String portFile;

  const ConnectOptions({
    this.timeout = const Duration(seconds: 5),
    this.transport = 'stdio',
    this.start = true,
    this.portFile = '',
  });
}

class _StartedHandle {
  final Process process;
  final List<String> cleanupPaths;

  const _StartedHandle(this.process, {this.cleanupPaths = const <String>[]});
}

final Expando<_StartedHandle> _started = Expando<_StartedHandle>('connect');
const ChannelOptions _stdioChannelOptions = ChannelOptions(
  credentials: ChannelCredentials.insecure(),
  idleTimeout: null,
);
const Duration _stdioStartupProbeWindow = Duration(milliseconds: 500);
const int _unixSocketPathMaxBytes = 103;

void resetConnectTestOverrides() {
  connectCurrentRootProvider = () => Directory.current.path;
  connectEnvironmentProvider = () => Platform.environment;
}

String defaultPortFilePathForTest(
  String slug, {
  String transport = 'tcp',
}) =>
    _defaultPortFilePath(slug, transport: transport);

String defaultUnixSocketURIForTest(String slug, String portFile) =>
    _defaultUnixSocketURI(slug, portFile);

String defaultWorkingDirectoryForTest(String path, String binaryPath) =>
    _defaultWorkingDirectory(path, binaryPath);

class _StdioClientChannel extends ClientChannel {
  final StdioTransportConnector _connector;

  _StdioClientChannel(this._connector)
      : super('localhost', port: 0, options: _stdioChannelOptions);

  @override
  ClientConnection createConnection() =>
      Http2ClientConnection.fromClientTransportConnector(_connector, options);
}

class _ReusableTarget {
  final ClientChannel channel;
  final String target;

  const _ReusableTarget({
    required this.channel,
    required this.target,
  });
}

Future<dynamic> connect(
  dynamic scopeOrTarget, [
  dynamic expressionOrOptions,
  dynamic root,
  dynamic specifiers,
  dynamic timeout,
]) async {
  if (scopeOrTarget is int) {
    return _connectUniform(
      scopeOrTarget,
      expressionOrOptions is String ? expressionOrOptions : '',
      root is String ? root : null,
      specifiers is int ? specifiers : ALL,
      timeout is int ? timeout : NO_TIMEOUT,
    );
  }

  if (scopeOrTarget is String) {
    final options =
        expressionOrOptions is ConnectOptions ? expressionOrOptions : null;
    final result = await _connectLegacy(scopeOrTarget, options);
    if (result.error != null && result.error!.isNotEmpty) {
      throw StateError(result.error!);
    }
    return result.channel;
  }

  throw ArgumentError(
      'connect expects either (scope, expression, ...) or (target, [options])');
}

void disconnect(dynamic result) {
  unawaited(disconnectAsync(result));
}

Future<void> disconnectAsync(dynamic result) => _disconnectAsync(result);

Future<ConnectResult> _connectUniform(
  int scope,
  String expression,
  String? root,
  int specifiers,
  int timeout,
) async {
  if (scope != LOCAL) {
    return ConnectResult(error: 'scope $scope not supported');
  }

  final target = expression.trim();
  if (target.isEmpty) {
    return const ConnectResult(error: 'expression is required');
  }

  final resolved = resolve(scope, target, root, specifiers, timeout);
  if (resolved.error != null) {
    return ConnectResult(origin: resolved.ref, error: resolved.error);
  }
  if (resolved.ref == null) {
    return ConnectResult(error: 'holon "$target" not found');
  }

  return _connectResolvedWithOptions(
    resolved.ref!,
    ConnectOptions(timeout: _timeoutDuration(timeout)),
  );
}

Future<ConnectResult> _connectLegacy(
  String target,
  ConnectOptions? options,
) async {
  final normalized = _normalizeOptions(options);
  final trimmed = target.trim();
  if (trimmed.isEmpty) {
    return const ConnectResult(error: 'target is required');
  }

  try {
    if (_isLegacyDirectTarget(trimmed)) {
      final channel =
          await _dialReady(_normalizeDialTarget(trimmed), normalized.timeout);
      return ConnectResult(
        channel: channel,
        origin: HolonRef(url: trimmed),
      );
    }

    final root = connectCurrentRootProvider().trim();
    final resolved = resolve(
      LOCAL,
      trimmed,
      root.isEmpty ? null : root,
      ALL,
      _timeoutMillis(normalized.timeout),
    );
    if (resolved.error != null) {
      return ConnectResult(origin: resolved.ref, error: resolved.error);
    }
    if (resolved.ref == null) {
      return ConnectResult(error: 'holon "$trimmed" not found');
    }
    return _connectResolvedWithOptions(resolved.ref!, normalized);
  } on Object catch (error) {
    return ConnectResult(error: '$error');
  }
}

Future<ConnectResult> _connectResolvedWithOptions(
  HolonRef ref,
  ConnectOptions options,
) async {
  if (_isReachableTarget(ref.url)) {
    try {
      final channel =
          await _dialReady(_normalizeDialTarget(ref.url), options.timeout);
      return ConnectResult(channel: channel, origin: ref);
    } on Object catch (_) {
      // Fall through to local launch handling when the ref is local and launchable.
    }
  }

  final path = _pathFromFileUrl(ref.url);
  if (path == null) {
    return ConnectResult(origin: ref, error: 'target unreachable');
  }

  try {
    final binaryPath = _resolveBinaryPath(ref, path);
    final workingDirectory = _defaultWorkingDirectory(path, binaryPath);
    switch (options.transport) {
      case 'stdio':
        final started = await _startStdioHolon(
          binaryPath,
          workingDirectory: workingDirectory,
        );
        _started[started.$1] = _StartedHandle(started.$2);
        return ConnectResult(channel: started.$1, origin: ref);

      case 'tcp':
        final slug = _refSlug(ref, path);
        final portFile = options.portFile.isNotEmpty
            ? options.portFile
            : _defaultPortFilePath(slug, transport: 'tcp');
        final reusable = await _reusablePersistentTarget(
          slug: slug,
          transport: 'tcp',
          portFile: options.portFile,
          timeout: options.timeout,
        );
        if (reusable != null) {
          return ConnectResult(channel: reusable.channel, origin: ref);
        }
        if (!options.start) {
          return ConnectResult(origin: ref, error: 'target unreachable');
        }

        final started = await _startTcpHolon(
          binaryPath,
          options.timeout,
          workingDirectory: workingDirectory,
        );
        final channel =
            await _dialReady(_normalizeDialTarget(started.$1), options.timeout);
        await _writePortFile(portFile, started.$1);
        _started[channel] = _StartedHandle(
          started.$2,
          cleanupPaths: <String>[portFile],
        );
        return ConnectResult(
            channel: channel,
            origin: HolonRef(url: started.$1, info: ref.info));

      case 'unix':
        final slug = _refSlug(ref, path);
        final portFile = options.portFile.isNotEmpty
            ? options.portFile
            : _defaultPortFilePath(slug, transport: 'unix');
        final reusable = await _reusablePersistentTarget(
          slug: slug,
          transport: 'unix',
          portFile: options.portFile,
          timeout: options.timeout,
        );
        if (reusable != null) {
          return ConnectResult(channel: reusable.channel, origin: ref);
        }
        if (!options.start) {
          return ConnectResult(origin: ref, error: 'target unreachable');
        }

        final started = await _startUnixHolon(
          binaryPath,
          _refSlug(ref, path),
          portFile,
          options.timeout,
          workingDirectory: workingDirectory,
        );
        final channel = await _dialReady(started.$1, options.timeout);
        await _writePortFile(portFile, started.$1);
        _started[channel] = _StartedHandle(
          started.$2,
          cleanupPaths: <String>[
            portFile,
            started.$1.substring('unix://'.length),
          ],
        );
        return ConnectResult(
            channel: channel,
            origin: HolonRef(url: started.$1, info: ref.info));

      default:
        return ConnectResult(
          origin: ref,
          error: 'unsupported transport "${options.transport}"',
        );
    }
  } on Object catch (error) {
    return ConnectResult(origin: ref, error: '$error');
  }
}

Future<void> _disconnectAsync(dynamic value) async {
  final channel = value is ConnectResult ? value.channel : value;
  if (channel == null) {
    return;
  }

  final handle = _started[channel];
  _started[channel] = null;

  try {
    if (channel is ClientChannel) {
      await channel.shutdown();
    } else if (channel is ClientTransportConnectorChannel) {
      await channel.shutdown();
    } else {
      try {
        await channel.shutdown();
      } on Object {
        // Ignore unknown channel types.
      }
    }
  } finally {
    if (handle != null) {
      await _stopProcess(handle.process);
      await _cleanupOwnedArtifacts(handle.cleanupPaths);
    }
  }
}

ConnectOptions _normalizeOptions(ConnectOptions? opts) {
  final options = opts ?? const ConnectOptions();
  final transportName = options.transport.trim().isEmpty
      ? 'stdio'
      : options.transport.trim().toLowerCase();
  return ConnectOptions(
    timeout: options.timeout <= Duration.zero
        ? const Duration(seconds: 5)
        : options.timeout,
    transport: transportName,
    start: options.start,
    portFile: options.portFile.trim(),
  );
}

Future<ClientChannel> _dialReady(String target, Duration timeout) async {
  if (target.startsWith('unix://')) {
    final socketPath = target.substring('unix://'.length);
    await _waitForUnixReady(socketPath, timeout);
    return ClientChannel(
      InternetAddress(socketPath, type: InternetAddressType.unix),
      port: 0,
      options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
    );
  }

  final parsed = _parseHostPort(target);
  final channel = ClientChannel(
    parsed.$1,
    port: parsed.$2,
    options: const ChannelOptions(credentials: ChannelCredentials.insecure()),
  );

  try {
    await _waitForTcpReady(parsed.$1, parsed.$2, timeout);
    return channel;
  } catch (_) {
    await channel.shutdown();
    rethrow;
  }
}

Future<void> _waitForTcpReady(String host, int port, Duration timeout) async {
  final deadline = DateTime.now().add(timeout);
  final attemptTimeout = _probeAttemptTimeout(timeout);
  while (DateTime.now().isBefore(deadline)) {
    try {
      final socket = await Socket.connect(
        host,
        port,
        timeout: attemptTimeout,
      );
      socket.destroy();
      return;
    } on SocketException {
      await Future<void>.delayed(const Duration(milliseconds: 50));
    }
  }
  throw StateError('timed out waiting for gRPC readiness');
}

Future<void> _waitForUnixReady(String path, Duration timeout) async {
  final deadline = DateTime.now().add(timeout);
  final attemptTimeout = _probeAttemptTimeout(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (await _probeUnixReady(path, attemptTimeout)) {
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('timed out waiting for unix gRPC readiness');
}

Future<bool> _probeUnixReady(String path, Duration timeout) async {
  try {
    final socket = await Socket.connect(
      InternetAddress(path, type: InternetAddressType.unix),
      0,
      timeout: timeout,
    );
    socket.destroy();
    return true;
  } on Object {
    return false;
  }
}

Duration _probeAttemptTimeout(Duration timeout) {
  if (timeout <= Duration.zero) {
    return const Duration(seconds: 1);
  }
  final milliseconds = timeout.inMilliseconds ~/ 4;
  if (milliseconds <= 200) {
    return const Duration(milliseconds: 200);
  }
  if (milliseconds >= 1000) {
    return const Duration(seconds: 1);
  }
  return Duration(milliseconds: milliseconds);
}

Future<_ReusableTarget?> _reusablePersistentTarget({
  required String slug,
  required String transport,
  required String portFile,
  required Duration timeout,
}) async {
  final overridePath = portFile.trim();
  if (overridePath.isNotEmpty) {
    return _usablePortFile(overridePath, transport, timeout);
  }

  final primary = _defaultPortFilePath(slug, transport: transport);
  final reusable = await _usablePortFile(primary, transport, timeout);
  if (reusable != null) {
    return reusable;
  }

  final legacy = _legacyPortFilePath(slug);
  final migrated = await _usablePortFile(legacy, transport, timeout);
  if (migrated == null) {
    return null;
  }

  await _writePortFile(primary, migrated.target);
  return migrated;
}

Future<_ReusableTarget?> _usablePortFile(
  String portFile,
  String expectedTransport,
  Duration timeout,
) async {
  try {
    final raw = (await File(portFile).readAsString()).trim();
    if (raw.isEmpty) {
      await File(portFile).delete();
      return null;
    }
    if (!_targetMatchesTransport(raw, expectedTransport)) {
      await File(portFile).delete();
      return null;
    }
    final channel = await _dialReady(
      _normalizeDialTarget(raw),
      timeout < const Duration(seconds: 1)
          ? timeout
          : const Duration(seconds: 1),
    );
    return _ReusableTarget(channel: channel, target: raw);
  } on Object {
    final file = File(portFile);
    if (file.existsSync()) {
      await file.delete();
    }
    return null;
  }
}

Future<(ClientChannel, Process)> _startStdioHolon(
  String binaryPath, {
  required String workingDirectory,
}) async {
  final process = await Process.start(
    binaryPath,
    const <String>['serve', '--listen', 'stdio://'],
    workingDirectory: workingDirectory,
  );
  final recentLines = <String>[];
  utf8.decoder
      .bind(process.stderr)
      .transform(const LineSplitter())
      .listen((line) {
    if (recentLines.length == 8) {
      recentLines.removeAt(0);
    }
    recentLines.add(line);
  });

  final sentinel = Object();
  final startup = await Future.any<Object?>(<Future<Object?>>[
    process.exitCode
        .then<Object?>((code) => (code, _recentLineDetails(recentLines))),
    Future<Object?>.delayed(_stdioStartupProbeWindow, () => sentinel),
  ]);
  if (startup != sentinel) {
    final failure = startup as (int, String);
    throw StateError(
      'holon exited before accepting stdio RPCs (${failure.$1})${failure.$2}',
    );
  }

  final connector = StdioTransportConnector.fromProcess(process);
  final channel = _StdioClientChannel(connector);
  return (channel, process);
}

Future<(String, Process)> _startTcpHolon(
  String binaryPath,
  Duration timeout, {
  required String workingDirectory,
}) async {
  final process = await Process.start(
    binaryPath,
    const <String>['serve', '--listen', 'tcp://127.0.0.1:0'],
    workingDirectory: workingDirectory,
  );

  final completer = Completer<String>();
  String? fallbackUri;
  Timer? fallbackTimer;
  void handleLine(String line) {
    final uri = _firstUri(line);
    if (uri.isEmpty || completer.isCompleted) {
      return;
    }
    if (_prefersStartupUriLine(line)) {
      fallbackTimer?.cancel();
      completer.complete(uri);
      return;
    }
    fallbackUri ??= uri;
    fallbackTimer ??= Timer(const Duration(milliseconds: 200), () {
      if (!completer.isCompleted && fallbackUri != null) {
        completer.complete(fallbackUri!);
      }
    });
  }

  utf8.decoder
      .bind(process.stdout)
      .transform(const LineSplitter())
      .listen(handleLine);
  utf8.decoder
      .bind(process.stderr)
      .transform(const LineSplitter())
      .listen(handleLine);

  process.exitCode.then((code) {
    if (!completer.isCompleted) {
      fallbackTimer?.cancel();
      completer.completeError(
        StateError('holon exited before advertising an address ($code)'),
      );
    }
  });

  try {
    final uri = await completer.future.timeout(timeout);
    fallbackTimer?.cancel();
    return (uri, process);
  } on Object {
    fallbackTimer?.cancel();
    await _stopProcess(process);
    rethrow;
  }
}

Future<(String, Process)> _startUnixHolon(
  String binaryPath,
  String slug,
  String portFile,
  Duration timeout, {
  required String workingDirectory,
}) async {
  final uri = _defaultUnixSocketURI(slug, portFile);
  final socketPath = uri.substring('unix://'.length);
  final socketFile = File(socketPath);
  if (socketFile.existsSync()) {
    try {
      socketFile.deleteSync();
    } catch (_) {}
  }

  final process = await Process.start(
    binaryPath,
    <String>['serve', '--listen', uri],
    workingDirectory: workingDirectory,
  );

  final recentLines = <String>[];
  utf8.decoder
      .bind(process.stderr)
      .transform(const LineSplitter())
      .listen((line) {
    if (recentLines.length == 8) {
      recentLines.removeAt(0);
    }
    recentLines.add(line);
  });

  var exited = false;
  var exitCode = 0;
  final attemptTimeout = _probeAttemptTimeout(timeout);
  unawaited(process.exitCode.then((code) {
    exited = true;
    exitCode = code;
  }));

  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (socketFile.existsSync() &&
        await _probeUnixReady(socketPath, attemptTimeout)) {
      return (uri, process);
    }
    if (exited) {
      final details = _recentLineDetails(recentLines);
      throw StateError(
        'holon exited before binding unix socket ($exitCode)$details',
      );
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }

  try {
    await _stopProcess(process);
  } catch (_) {}
  throw StateError(
    'timed out waiting for unix holon startup${_recentLineDetails(recentLines)}',
  );
}

String _resolveBinaryPath(HolonRef ref, String path) {
  final info = ref.info;
  if (File(path).existsSync()) {
    return path;
  }

  if (Directory(path).existsSync() && _basename(path).endsWith('.holon')) {
    final direct = findPackageBinary(path);
    if (direct != null) {
      return direct;
    }

    final entrypoint = (info?.entrypoint ?? '').trim();
    if (entrypoint.isNotEmpty) {
      final candidate = _joinPath(
        _joinPath(path, 'bin/${currentArchDirectory()}'),
        _basename(entrypoint),
      );
      if (File(candidate).existsSync()) {
        return candidate;
      }
    }
    throw StateError(
        'built binary not found for holon "${_refSlug(ref, path)}"');
  }

  final entrypoint = (info?.entrypoint ?? '').trim();
  if (entrypoint.isNotEmpty) {
    if (_isAbsolutePath(entrypoint) && File(entrypoint).existsSync()) {
      return entrypoint;
    }

    final buildBin = _joinPath(
      _joinPath(path, '.op/build/bin'),
      _basename(entrypoint),
    );
    if (File(buildBin).existsSync()) {
      return buildBin;
    }

    final fromPath = _searchPath(_basename(entrypoint));
    if (fromPath != null) {
      return fromPath;
    }
  }

  final slugCandidate =
      _joinPath(_joinPath(path, '.op/build/bin'), _refSlug(ref, path));
  if (File(slugCandidate).existsSync()) {
    return slugCandidate;
  }

  throw StateError('built binary not found for holon "${_refSlug(ref, path)}"');
}

String _defaultPortFilePath(
  String slug, {
  required String transport,
}) =>
    '${_connectRunDirectory()}${Platform.pathSeparator}$slug.${_transportPortFileSuffix(transport)}.port';

String _legacyPortFilePath(String slug) =>
    '${_connectRunDirectory()}${Platform.pathSeparator}$slug.port';

String _connectRunDirectory() {
  if (_isPublishedBundleMode()) {
    return _normalizeAbsolutePath(
      _joinPath(_joinPath(Directory.systemTemp.path, 'holons'), 'run'),
    );
  }
  return _joinPath(_connectOpPath(), 'run');
}

bool _isPublishedBundleMode() {
  final publishedRoot = discoverPublishedHolonsRootProvider()?.trim() ?? '';
  if (publishedRoot.isEmpty) {
    return false;
  }
  return Directory(_normalizeAbsolutePath(publishedRoot)).existsSync();
}

String _connectOpPath() {
  final env = connectEnvironmentProvider();
  final configured = (env['OPPATH'] ?? '').trim();
  if (configured.isNotEmpty) {
    return _normalizeAbsolutePath(configured);
  }

  for (final key in <String>['HOME', 'USERPROFILE']) {
    final home = (env[key] ?? '').trim();
    if (home.isNotEmpty) {
      return _normalizeAbsolutePath(_joinPath(home, '.op'));
    }
  }

  return _normalizeAbsolutePath(
    _joinPath(Directory.systemTemp.path, '.op'),
  );
}

String _defaultWorkingDirectory(String path, String binaryPath) {
  final packageDir = Directory(path);
  final candidate = packageDir.existsSync()
      ? packageDir.absolute.path
      : Directory(binaryPath).parent.absolute.path;
  return _writableWorkingDirectory(candidate);
}

String _writableWorkingDirectory(String path) {
  final normalized = _normalizeAbsolutePath(path);
  if (_isWritableDirectory(normalized)) {
    return normalized;
  }
  return _normalizeAbsolutePath(Directory.systemTemp.path);
}

bool _isWritableDirectory(String path) {
  final directory = Directory(path);
  if (!directory.existsSync()) {
    return false;
  }

  final probe = File(
    _joinPath(
      directory.path,
      '.holons-write-probe-${pid}-${DateTime.now().microsecondsSinceEpoch}',
    ),
  );
  try {
    probe.writeAsStringSync('probe');
    probe.deleteSync();
    return true;
  } on FileSystemException {
    return false;
  }
}

String _transportPortFileSuffix(String transport) {
  switch (transport.trim().toLowerCase()) {
    case 'unix':
      return 'unix';
    case 'tcp':
    case 'auto':
    default:
      return 'tcp';
  }
}

String _defaultUnixSocketURI(String slug, String portFile) {
  final hash = _fnv1a64(utf8.encode(portFile)) & 0xFFFFFFFF;
  final socketPath = _socketPathInTempHierarchy(
    'h${hash.toRadixString(16).padLeft(8, '0')}.s',
  );
  return 'unix://${_normalizeAbsolutePath(socketPath)}';
}

String _socketPathInTempHierarchy(String socketName) {
  final tempDir = _normalizeAbsolutePath(Directory.systemTemp.path);
  for (final candidateDir in _socketDirectoryCandidates(tempDir)) {
    if (!_isWritableDirectory(candidateDir)) {
      continue;
    }
    final candidatePath = _normalizeAbsolutePath(
      _joinPath(candidateDir, socketName),
    );
    if (_fitsUnixSocketPath(candidatePath)) {
      return candidatePath;
    }
  }
  return _normalizeAbsolutePath(_joinPath(tempDir, socketName));
}

Iterable<String> _socketDirectoryCandidates(String startPath) sync* {
  var current = _normalizeAbsolutePath(startPath);
  final seen = <String>{};
  while (seen.add(current)) {
    yield current;
    final parent = _normalizeAbsolutePath(Directory(current).parent.path);
    if (parent == current) {
      break;
    }
    current = parent;
  }
}

bool _fitsUnixSocketPath(String path) =>
    utf8.encode(path).length <= _unixSocketPathMaxBytes;

bool _targetMatchesTransport(String target, String expectedTransport) {
  final normalized = expectedTransport.trim().toLowerCase();
  final trimmed = target.trim().toLowerCase();
  switch (normalized) {
    case 'unix':
      return trimmed.startsWith('unix://');
    case 'tcp':
      return !trimmed.startsWith('unix://');
    default:
      return true;
  }
}

Future<void> _writePortFile(String portFile, String uri) async {
  final file = File(portFile);
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

Future<void> _cleanupOwnedArtifacts(List<String> paths) async {
  for (final path in paths) {
    final trimmed = path.trim();
    if (trimmed.isEmpty) {
      continue;
    }
    final file = File(trimmed);
    if (!file.existsSync()) {
      continue;
    }
    try {
      await file.delete();
    } on FileSystemException {
      // Ignore cleanup failures after process shutdown.
    }
  }
}

String _recentLineDetails(List<String> recentLines) {
  if (recentLines.isEmpty) {
    return '';
  }
  return ': ${recentLines.join(' | ')}';
}

int _fnv1a64(List<int> bytes) {
  var hash = 0xcbf29ce484222325;
  const prime = 0x100000001b3;
  for (final byte in bytes) {
    hash ^= byte & 0xff;
    hash = (hash * prime) & 0xFFFFFFFFFFFFFFFF;
  }
  return hash;
}

bool _isLegacyDirectTarget(String target) =>
    target.startsWith('tcp://') ||
    target.startsWith('unix://') ||
    (target.contains(':') && !target.contains(Platform.pathSeparator));

bool _isReachableTarget(String target) =>
    target.startsWith('tcp://') || target.startsWith('unix://');

String _normalizeDialTarget(String target) {
  if (!target.contains('://')) {
    return target;
  }

  final parsed = parseUri(target);
  if (parsed.scheme == 'tcp') {
    final host = (parsed.host == null ||
            parsed.host!.isEmpty ||
            parsed.host == '0.0.0.0')
        ? '127.0.0.1'
        : parsed.host!;
    return '$host:${parsed.port}';
  }

  return target;
}

(String, int) _parseHostPort(String target) {
  final index = target.lastIndexOf(':');
  if (index <= 0 || index >= target.length - 1) {
    throw ArgumentError('invalid host:port target: $target');
  }
  return (target.substring(0, index), int.parse(target.substring(index + 1)));
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

bool _prefersStartupUriLine(String line) {
  final normalized = line.trim().toLowerCase();
  if (normalized.isEmpty) {
    return false;
  }
  if (normalized.contains('grpc bridge listening on')) {
    return true;
  }
  if (normalized.contains('backend stdout') ||
      normalized.contains('backend stderr') ||
      normalized.contains(' backend ')) {
    return false;
  }
  return normalized.contains('listening on') ||
      normalized.contains('serving on') ||
      normalized.contains('public uri');
}

String? _pathFromFileUrl(String raw) {
  if (!raw.startsWith('file://')) {
    return null;
  }
  return Uri.parse(raw).toFilePath();
}

String _refSlug(HolonRef ref, String path) {
  final slug = ref.info?.slug.trim() ?? '';
  if (slug.isNotEmpty) {
    return slug;
  }
  return _basename(path).replaceFirst(RegExp(r'\.holon$'), '');
}

String? _searchPath(String binaryName) {
  final path = Platform.environment['PATH'];
  if (path == null || path.trim().isEmpty) {
    return null;
  }

  final separator = Platform.isWindows ? ';' : ':';
  for (final dir in path.split(separator)) {
    final candidate = '$dir${Platform.pathSeparator}$binaryName';
    if (File(candidate).existsSync()) {
      return candidate;
    }
  }
  return null;
}

Duration _timeoutDuration(int timeout) {
  if (timeout <= 0) {
    return const Duration(seconds: 5);
  }
  return Duration(milliseconds: timeout);
}

int _timeoutMillis(Duration timeout) => timeout.inMilliseconds;

bool _isAbsolutePath(String path) {
  if (path.startsWith('/')) {
    return true;
  }
  return RegExp(r'^[A-Za-z]:[\\/]').hasMatch(path);
}

String _joinPath(String left, String right) {
  final separator = Platform.pathSeparator;
  final normalizedLeft =
      left.endsWith(separator) ? left.substring(0, left.length - 1) : left;
  final normalizedRight =
      right.startsWith(separator) ? right.substring(1) : right;
  return '$normalizedLeft$separator$normalizedRight';
}

String _basename(String path) {
  final normalized = path.replaceAll('\\', '/');
  final trimmed = normalized.endsWith('/') && normalized.length > 1
      ? normalized.substring(0, normalized.length - 1)
      : normalized;
  final index = trimmed.lastIndexOf('/');
  return index >= 0 ? trimmed.substring(index + 1) : trimmed;
}

String _normalizeAbsolutePath(String path) {
  final candidate = path.trim().isEmpty ? '.' : path;
  return Directory(candidate).absolute.path;
}
