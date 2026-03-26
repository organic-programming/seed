import 'dart:io';

import 'echo_cli.dart';

const String defaultHolonRPCServerSDK = 'dart-holons';
const String defaultHolonRPCServerVersion = '0.1.0';

const String holonRPCServerUsage = 'usage: holon-rpc-server '
    '[ws://host:port/rpc] [--sdk name] [--version version] [--once] [--go go]';

class HolonRPCServerOptions {
  const HolonRPCServerOptions({
    required this.passthroughArgs,
    required this.sdk,
    required this.version,
    required this.goBinary,
  });

  final List<String> passthroughArgs;
  final String sdk;
  final String version;
  final String goBinary;
}

HolonRPCServerOptions parseHolonRPCServerArgs(
  List<String> args, {
  Map<String, String>? environment,
}) {
  var goBinary = resolveGoBinary(environment: environment);
  final passthrough = <String>[];

  for (var i = 0; i < args.length; i++) {
    final token = args[i];
    if (token == '--go') {
      goBinary = _valueAt(args, ++i, '--go');
      continue;
    }
    if (token == '--help' || token == '-h') {
      throw const FormatException(holonRPCServerUsage);
    }
    passthrough.add(token);
  }

  return HolonRPCServerOptions(
    passthroughArgs: List<String>.unmodifiable(passthrough),
    sdk: defaultHolonRPCServerSDK,
    version: defaultHolonRPCServerVersion,
    goBinary: goBinary,
  );
}

ProcessInvocation buildHolonRPCServerInvocation(
  HolonRPCServerOptions options, {
  String? sdkRootPath,
  Map<String, String>? baseEnvironment,
}) {
  final rootPath = sdkRootPath ?? _defaultSDKRootPath();
  final helperPath = '$rootPath/cmd/holon-rpc-server-go/main.go';
  final goHolonsPath = '$rootPath/../go-holons';
  final commandArgs = <String>[
    'run',
    helperPath,
    ...options.passthroughArgs,
  ];

  if (!_hasFlag(options.passthroughArgs, '--sdk')) {
    commandArgs
      ..add('--sdk')
      ..add(options.sdk);
  }
  if (!_hasFlag(options.passthroughArgs, '--version')) {
    commandArgs
      ..add('--version')
      ..add(options.version);
  }

  return ProcessInvocation(
    command: options.goBinary,
    args: commandArgs,
    workingDirectory: goHolonsPath,
    environment: _buildEnvironment(baseEnvironment),
  );
}

Future<int> runHolonRPCServer(
  List<String> args, {
  Map<String, String>? environment,
  String? sdkRootPath,
}) async {
  final options = parseHolonRPCServerArgs(args, environment: environment);
  final invocation = buildHolonRPCServerInvocation(
    options,
    sdkRootPath: sdkRootPath,
    baseEnvironment: environment ?? Platform.environment,
  );

  final process = await Process.start(
    invocation.command,
    invocation.args,
    workingDirectory: invocation.workingDirectory,
    environment: invocation.environment,
    mode: ProcessStartMode.inheritStdio,
  );

  return process.exitCode;
}

String _valueAt(List<String> args, int index, String flag) {
  final value = index < args.length ? args[index] : null;
  if (value == null || value.isEmpty) {
    throw FormatException('missing value for $flag');
  }
  return value;
}

bool _hasFlag(List<String> args, String name) {
  final withEquals = '$name=';
  return args.any((arg) => arg == name || arg.startsWith(withEquals));
}

Map<String, String> _buildEnvironment(Map<String, String>? baseEnvironment) {
  final env = Map<String, String>.from(baseEnvironment ?? Platform.environment);
  if ((env['GOCACHE'] ?? '').trim().isEmpty) {
    env['GOCACHE'] = defaultGoCache;
  }
  return env;
}

String _defaultSDKRootPath() {
  final pubspec = File('${Directory.current.path}/pubspec.yaml');
  if (pubspec.existsSync()) {
    return Directory.current.path;
  }
  return Directory.fromUri(Platform.script).parent.parent.path;
}
