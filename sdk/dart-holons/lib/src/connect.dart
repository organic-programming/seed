import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:grpc/src/client/connection.dart' show ClientConnection;
import 'package:grpc/src/client/http2_connection.dart'
    show Http2ClientConnection;

import 'discover.dart';
import 'grpcclient.dart';
import 'transport.dart';

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
  final bool ephemeral;

  const _StartedHandle(this.process, this.ephemeral);
}

final Expando<_StartedHandle> _started = Expando<_StartedHandle>('connect');
const ChannelOptions _stdioChannelOptions = ChannelOptions(
  credentials: ChannelCredentials.insecure(),
  idleTimeout: null,
);
const Duration _stdioStartupProbeWindow = Duration(milliseconds: 500);

class _StdioClientChannel extends ClientChannel {
  final StdioTransportConnector _connector;

  _StdioClientChannel(this._connector)
      : super('localhost', port: 0, options: _stdioChannelOptions);

  @override
  ClientConnection createConnection() =>
      Http2ClientConnection.fromClientTransportConnector(_connector, options);
}

Future<ClientChannel> connect(String target, [ConnectOptions? opts]) async {
  final trimmed = target.trim();
  if (trimmed.isEmpty) {
    throw ArgumentError('target is required');
  }

  final options = _normalizeOptions(opts);

  if (_isDirectTarget(trimmed)) {
    return _dialReady(_normalizeDialTarget(trimmed), options.timeout);
  }

  final entry = await findBySlug(trimmed);
  if (entry == null) {
    throw StateError('holon "$trimmed" not found');
  }

  final portFile = options.portFile.isNotEmpty
      ? options.portFile
      : _defaultPortFilePath(entry.slug);

  final reusable = await _usablePortFile(portFile, options.timeout);
  if (reusable != null) {
    return reusable;
  }
  if (!options.start) {
    throw StateError('holon "$trimmed" is not running');
  }

  final binaryPath = _resolveBinaryPath(entry);
  switch (options.transport) {
    case 'stdio':
      final started = await _startStdioHolon(binaryPath);
      _started[started.$1] = _StartedHandle(started.$2, true);
      return started.$1;

    case 'tcp':
      final started = await _startTcpHolon(binaryPath, options.timeout);
      final channel = await _dialReady(
        _normalizeDialTarget(started.$1),
        options.timeout,
      );

      try {
        await _writePortFile(portFile, started.$1);
      } catch (_) {
        await channel.shutdown();
        _stopProcess(started.$2);
        rethrow;
      }

      _started[channel] = _StartedHandle(started.$2, false);
      return channel;

    case 'unix':
      final started = await _startUnixHolon(
        binaryPath,
        entry.slug,
        portFile,
        options.timeout,
      );
      final channel = await _dialReady(started.$1, options.timeout);

      try {
        await _writePortFile(portFile, started.$1);
      } catch (_) {
        await channel.shutdown();
        await _stopProcess(started.$2);
        rethrow;
      }

      _started[channel] = _StartedHandle(started.$2, false);
      return channel;

    default:
      throw UnsupportedError('unsupported transport "${options.transport}"');
  }
}

Future<void> disconnect(ClientChannel channel) async {
  final handle = _started[channel];
  _started[channel] = null;
  await channel.shutdown();
  if (handle != null && handle.ephemeral) {
    await _stopProcess(handle.process);
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
  while (DateTime.now().isBefore(deadline)) {
    try {
      final socket = await Socket.connect(
        host,
        port,
        timeout: const Duration(milliseconds: 200),
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
  while (DateTime.now().isBefore(deadline)) {
    if (await _probeUnixReady(path)) {
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 50));
  }
  throw StateError('timed out waiting for unix gRPC readiness');
}

Future<bool> _probeUnixReady(String path) async {
  try {
    final socket = await Socket.connect(
      InternetAddress(path, type: InternetAddressType.unix),
      0,
      timeout: const Duration(milliseconds: 200),
    );
    socket.destroy();
    return true;
  } on Object {
    return false;
  }
}

Future<ClientChannel?> _usablePortFile(
    String portFile, Duration timeout) async {
  try {
    final raw = (await File(portFile).readAsString()).trim();
    if (raw.isEmpty) {
      await File(portFile).delete();
      return null;
    }
    return _dialReady(
      _normalizeDialTarget(raw),
      timeout < const Duration(seconds: 1)
          ? timeout
          : const Duration(seconds: 1),
    );
  } on Object {
    final file = File(portFile);
    if (file.existsSync()) {
      await file.delete();
    }
    return null;
  }
}

Future<(ClientChannel, Process)> _startStdioHolon(String binaryPath) async {
  final process = await Process.start(
    binaryPath,
    const <String>['serve', '--listen', 'stdio://'],
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
    String binaryPath, Duration timeout) async {
  final process = await Process.start(
    binaryPath,
    const <String>['serve', '--listen', 'tcp://127.0.0.1:0'],
  );

  final completer = Completer<String>();
  final stdoutSub = utf8.decoder
      .bind(process.stdout)
      .transform(const LineSplitter())
      .listen((line) {
    final uri = _firstUri(line);
    if (uri.isNotEmpty && !completer.isCompleted) {
      completer.complete(uri);
    }
  });
  final stderrSub = utf8.decoder
      .bind(process.stderr)
      .transform(const LineSplitter())
      .listen((line) {
    final uri = _firstUri(line);
    if (uri.isNotEmpty && !completer.isCompleted) {
      completer.complete(uri);
    }
  });

  process.exitCode.then((code) {
    if (!completer.isCompleted) {
      completer.completeError(
        StateError('holon exited before advertising an address ($code)'),
      );
    }
  });

  try {
    final uri = await completer.future.timeout(timeout);
    return (uri, process);
  } on Object {
    await _stopProcess(process);
    rethrow;
  } finally {
    await stdoutSub.cancel();
    await stderrSub.cancel();
  }
}

Future<(String, Process)> _startUnixHolon(
  String binaryPath,
  String slug,
  String portFile,
  Duration timeout,
) async {
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
  );

  final recentLines = <String>[];
  final stderrSub = utf8.decoder
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
  unawaited(process.exitCode.then((code) {
    exited = true;
    exitCode = code;
  }));

  try {
    final deadline = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(deadline)) {
      if (socketFile.existsSync() && await _probeUnixReady(socketPath)) {
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
  } finally {
    await stderrSub.cancel();
  }
}

String _resolveBinaryPath(HolonEntry entry) {
  final manifest = entry.manifest;
  if (manifest == null) {
    throw StateError('holon "${entry.slug}" has no manifest');
  }

  final binary = manifest.artifacts.binary.trim();
  if (binary.isEmpty) {
    throw StateError('holon "${entry.slug}" has no artifacts.binary');
  }

  if (binary.startsWith('/')) {
    if (File(binary).existsSync()) {
      return binary;
    }
  }

  final candidate =
      '${entry.dir}${Platform.pathSeparator}.op${Platform.pathSeparator}build${Platform.pathSeparator}bin${Platform.pathSeparator}${binary.split(Platform.pathSeparator).last}'
          .replaceAll('${Platform.pathSeparator}${Platform.pathSeparator}',
              Platform.pathSeparator);
  if (File(candidate).existsSync()) {
    return candidate;
  }

  final resolved = Platform.environment['PATH']
      ?.split(Platform.isWindows ? ';' : ':')
      .map((dir) =>
          '$dir${Platform.pathSeparator}${binary.split(Platform.pathSeparator).last}')
      .firstWhere((file) => File(file).existsSync(), orElse: () => '');
  if (resolved != null && resolved.isNotEmpty) {
    return resolved;
  }

  throw StateError('built binary not found for holon "${entry.slug}"');
}

String _defaultPortFilePath(String slug) =>
    '${Directory.current.path}${Platform.pathSeparator}.op${Platform.pathSeparator}run${Platform.pathSeparator}$slug.port';

String _defaultUnixSocketURI(String slug, String portFile) {
  final label = _socketLabel(slug);
  final hash = _fnv1a64(utf8.encode(portFile)) & 0xFFFFFFFFFFFF;
  return 'unix:///tmp/holons-$label-${hash.toRadixString(16).padLeft(12, '0')}.sock';
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

String _recentLineDetails(List<String> recentLines) {
  if (recentLines.isEmpty) {
    return '';
  }
  return ': ${recentLines.join(' | ')}';
}

String _socketLabel(String slug) {
  final buffer = StringBuffer();
  var lastWasDash = false;

  for (final rune in slug.trim().toLowerCase().runes) {
    final char = String.fromCharCode(rune);
    final isAsciiLetter = rune >= 97 && rune <= 122;
    final isDigit = rune >= 48 && rune <= 57;
    if (isAsciiLetter || isDigit) {
      buffer.write(char);
      lastWasDash = false;
    } else if ((char == '-' || char == '_') &&
        buffer.isNotEmpty &&
        !lastWasDash) {
      buffer.write('-');
      lastWasDash = true;
    }

    if (buffer.length >= 24) {
      break;
    }
  }

  final label = buffer.toString().replaceAll(RegExp(r'^-+|-+$'), '');
  return label.isEmpty ? 'socket' : label;
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

bool _isDirectTarget(String target) =>
    target.contains('://') || target.contains(':');

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
  return codeUnit == 34 || // "
      codeUnit == 39 || // '
      codeUnit == 40 || // (
      codeUnit == 41 || // )
      codeUnit == 44 || // ,
      codeUnit == 46 || // .
      codeUnit == 91 || // [
      codeUnit == 93 || // ]
      codeUnit == 123 || // {
      codeUnit == 125; // }
}
