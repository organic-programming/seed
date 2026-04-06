import 'dart:async';
import 'dart:convert';
import 'dart:io';

import '../gen/holons/v1/describe.pb.dart';
import '../gen/holons/v1/describe.pbgrpc.dart';
import 'discovery_types.dart';
import 'grpcclient.dart';

const Duration defaultDiscoveryProbeTimeout = Duration(seconds: 5);
const Duration _stdioStartupProbeWindow = Duration(milliseconds: 500);

Future<HolonInfo> describeBinary(
  String binaryPath, {
  Duration timeout = defaultDiscoveryProbeTimeout,
}) async {
  final effectiveTimeout =
      timeout <= Duration.zero ? defaultDiscoveryProbeTimeout : timeout;
  final started = await dialStdio(binaryPath);
  final channel = started.$1;
  final process = started.$2;

  try {
    final sentinel = Object();
    final startup = await Future.any<Object?>(<Future<Object?>>[
      process.exitCode.then<Object?>((code) => code),
      Future<Object?>.delayed(_stdioStartupProbeWindow, () => sentinel),
    ]);
    if (startup != sentinel) {
      throw StateError('holon exited before accepting stdio RPCs ($startup)');
    }

    final response = await HolonMetaClient(channel)
        .describe(DescribeRequest())
        .timeout(effectiveTimeout);
    return holonInfoFromDescribeResponse(response);
  } finally {
    await channel.shutdown();
    await _stopProcess(process);
  }
}

Future<HolonInfo> describePackage(
  String packageDir, {
  Duration timeout = defaultDiscoveryProbeTimeout,
}) async {
  final binaryPath = findPackageBinary(packageDir);
  if (binaryPath == null) {
    throw FileSystemException('package binary not found', packageDir);
  }
  return describeBinary(binaryPath, timeout: timeout);
}

String? findPackageBinary(String packageDir) {
  final archDir =
      '$packageDir${Platform.pathSeparator}bin${Platform.pathSeparator}${currentArchDirectory()}';
  final directory = Directory(archDir);
  if (!directory.existsSync()) {
    return null;
  }

  final manifestEntrypoint = _packageEntrypoint(packageDir);
  if (manifestEntrypoint.isNotEmpty) {
    final preferred =
        '$archDir${Platform.pathSeparator}${_basename(manifestEntrypoint)}';
    if (File(preferred).existsSync()) {
      return preferred;
    }
  }

  final candidates = directory
      .listSync(followLinks: false)
      .whereType<File>()
      .map((file) => file.path)
      .toList()
    ..sort();

  for (final candidate in candidates) {
    if (_looksLikeRunnableBinary(candidate)) {
      return candidate;
    }
  }
  return candidates.isEmpty ? null : candidates.first;
}

String _packageEntrypoint(String packageDir) {
  final manifestFile = File('$packageDir${Platform.pathSeparator}.holon.json');
  if (!manifestFile.existsSync()) {
    return '';
  }

  try {
    final decoded = jsonDecode(manifestFile.readAsStringSync());
    if (decoded is! Map) {
      return '';
    }
    final entrypoint = decoded['entrypoint'];
    return entrypoint is String ? entrypoint.trim() : '';
  } on Object {
    return '';
  }
}

bool _looksLikeRunnableBinary(String path) {
  final name = _basename(path).toLowerCase();
  if (name.endsWith('.json') ||
      name.endsWith('.pb') ||
      name.endsWith('.proto') ||
      name.endsWith('.txt') ||
      name.endsWith('.md')) {
    return false;
  }
  if (name.startsWith('describe_generated.')) {
    return false;
  }
  return true;
}

String _basename(String path) {
  final normalized = path.replaceAll('\\', Platform.pathSeparator);
  final index = normalized.lastIndexOf(Platform.pathSeparator);
  if (index < 0) {
    return normalized;
  }
  return normalized.substring(index + 1);
}

HolonInfo holonInfoFromDescribeResponse(DescribeResponse response) {
  if (!response.hasManifest()) {
    throw StateError('Describe returned no manifest');
  }

  final manifest = response.manifest;
  if (!manifest.hasIdentity()) {
    throw StateError('Describe returned no manifest identity');
  }

  final identity = manifest.identity;
  final slug = _slugFromNames(identity.givenName, identity.familyName);

  return HolonInfo(
    slug: slug,
    uuid: identity.uuid,
    identity: IdentityInfo(
      givenName: identity.givenName,
      familyName: identity.familyName,
      motto: identity.motto,
      aliases: List<String>.from(identity.aliases),
    ),
    lang: manifest.lang,
    runner: manifest.hasBuild() ? manifest.build.runner : '',
    status: identity.status,
    kind: manifest.kind,
    transport: manifest.transport,
    entrypoint: manifest.hasArtifacts() ? manifest.artifacts.binary : '',
    architectures: List<String>.from(manifest.platforms),
    hasDist: false,
    hasSource: false,
  );
}

Future<int> runDiscoveryProbeCli(List<String> args) async {
  if (args.isEmpty) {
    stderr.writeln(
        'usage: discovery_probe <describe-package|describe-binary> <path> [timeout-ms]');
    return 64;
  }

  final command = args.first.trim();
  if (args.length < 2 || args[1].trim().isEmpty) {
    stderr.writeln('$command requires a path');
    return 64;
  }

  final path = args[1].trim();
  final timeout = args.length >= 3
      ? _durationFromArg(args[2])
      : defaultDiscoveryProbeTimeout;

  try {
    final info = switch (command) {
      'describe-package' => await describePackage(path, timeout: timeout),
      'describe-binary' => await describeBinary(path, timeout: timeout),
      _ => throw ArgumentError('unknown command "$command"'),
    };
    stdout.writeln(jsonEncode(info.toJson()));
    return 0;
  } on Object catch (error) {
    stderr.writeln(error);
    return 1;
  }
}

String currentArchDirectory() {
  final osName = switch (Platform.operatingSystem) {
    'macos' => 'darwin',
    'linux' => 'linux',
    'windows' => 'windows',
    final other => other,
  };
  return '${osName}_${_currentArchName()}';
}

Future<void> _stopProcess(Process process) async {
  process.kill(ProcessSignal.sigterm);
  try {
    await process.exitCode.timeout(const Duration(seconds: 2));
  } on TimeoutException {
    process.kill(ProcessSignal.sigkill);
    await process.exitCode;
  }
}

Duration _durationFromArg(String arg) {
  final millis = int.tryParse(arg.trim());
  if (millis == null || millis <= 0) {
    return defaultDiscoveryProbeTimeout;
  }
  return Duration(milliseconds: millis);
}

String _currentArchName() {
  if (Platform.isWindows) {
    final raw = (Platform.environment['PROCESSOR_ARCHITECTURE'] ?? '')
        .trim()
        .toLowerCase();
    if (raw == 'amd64' || raw == 'x86_64') {
      return 'amd64';
    }
    if (raw == 'arm64' || raw == 'aarch64') {
      return 'arm64';
    }
    return raw.isEmpty ? 'unknown' : raw;
  }

  final result = Process.runSync('uname', const <String>['-m']);
  final raw =
      result.exitCode == 0 ? result.stdout.toString().trim().toLowerCase() : '';
  if (raw == 'x86_64' || raw == 'amd64') {
    return 'amd64';
  }
  if (raw == 'arm64' || raw == 'aarch64') {
    return 'arm64';
  }
  return raw.isEmpty ? 'unknown' : raw;
}

String _slugFromNames(String givenName, String familyName) {
  final family = familyName.trim().replaceFirst(RegExp(r'\?$'), '');
  final joined = '${givenName.trim()}-$family'
      .trim()
      .toLowerCase()
      .replaceAll(' ', '-')
      .replaceAll(RegExp(r'^-+|-+$'), '');
  return joined;
}
