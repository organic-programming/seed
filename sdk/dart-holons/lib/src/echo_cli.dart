import 'dart:async';
import 'dart:convert';
import 'dart:io';

const String defaultEchoClientURI = 'stdio://';
const String defaultEchoClientSDK = 'dart-holons';
const String defaultEchoClientServerSDK = 'go-holons';
const String defaultEchoClientMessage = 'hello';
const int defaultEchoClientTimeoutMs = 5000;

const String defaultEchoServerSDK = 'dart-holons';
const String defaultEchoServerVersion = '0.1.0';

const String defaultGoCache = '/tmp/go-cache';
const String preferredGoBinary = '/Users/bpds/go/go1.25.1/bin/go';

const String echoClientUsage = 'usage: echo-client '
    '[--sdk name] [--server-sdk name] [--message hello] '
    '[--timeout-ms 5000] [--go go] '
    '[tcp://host:port|unix://path|stdio://|ws://host:port/grpc|wss://host:port/grpc]';

const String echoServerUsage =
    'usage: echo-server [--go go] [echo-server flags...]';

class EchoClientOptions {
  const EchoClientOptions({
    required this.uri,
    required this.sdk,
    required this.serverSDK,
    required this.message,
    required this.timeoutMs,
    required this.goBinary,
  });

  final String uri;
  final String sdk;
  final String serverSDK;
  final String message;
  final int timeoutMs;
  final String goBinary;
}

class EchoServerOptions {
  const EchoServerOptions({
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

class ProcessInvocation {
  const ProcessInvocation({
    required this.command,
    required this.args,
    required this.workingDirectory,
    required this.environment,
  });

  final String command;
  final List<String> args;
  final String workingDirectory;
  final Map<String, String> environment;
}

EchoClientOptions parseEchoClientArgs(
  List<String> args, {
  Map<String, String>? environment,
}) {
  var uri = defaultEchoClientURI;
  var sdk = defaultEchoClientSDK;
  var serverSDK = defaultEchoClientServerSDK;
  var message = defaultEchoClientMessage;
  var timeoutMs = defaultEchoClientTimeoutMs;
  var goBinary = resolveGoBinary(environment: environment);
  var uriSet = false;

  for (var i = 0; i < args.length; i++) {
    final token = args[i];
    if (token == '--sdk') {
      sdk = _valueAt(args, ++i, '--sdk');
      continue;
    }
    if (token == '--server-sdk') {
      serverSDK = _valueAt(args, ++i, '--server-sdk');
      continue;
    }
    if (token == '--message') {
      message = _valueAt(args, ++i, '--message');
      continue;
    }
    if (token == '--go') {
      goBinary = _valueAt(args, ++i, '--go');
      continue;
    }
    if (token == '--timeout-ms') {
      final raw = _valueAt(args, ++i, '--timeout-ms');
      timeoutMs = int.tryParse(raw) ?? -1;
      if (timeoutMs <= 0) {
        throw FormatException('--timeout-ms must be a positive integer');
      }
      continue;
    }
    if (token == '--help' || token == '-h') {
      throw const FormatException(echoClientUsage);
    }
    if (token.startsWith('--')) {
      throw FormatException('unknown flag: $token');
    }
    if (uriSet) {
      throw FormatException('unexpected argument: $token');
    }
    uri = normalizeEchoURI(token);
    uriSet = true;
  }

  return EchoClientOptions(
    uri: normalizeEchoURI(uri),
    sdk: sdk,
    serverSDK: serverSDK,
    message: message,
    timeoutMs: timeoutMs,
    goBinary: goBinary,
  );
}

EchoServerOptions parseEchoServerArgs(
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
    passthrough.add(token);
  }

  return EchoServerOptions(
    passthroughArgs: List<String>.unmodifiable(passthrough),
    sdk: defaultEchoServerSDK,
    version: defaultEchoServerVersion,
    goBinary: goBinary,
  );
}

ProcessInvocation buildEchoClientInvocation(
  EchoClientOptions options, {
  String? sdkRootPath,
  Map<String, String>? baseEnvironment,
}) {
  final rootPath = sdkRootPath ?? _defaultSDKRootPath();
  final helperPath = '$rootPath/cmd/echo-client-go/main.go';
  final goHolonsPath = '$rootPath/../go-holons';
  final environment = _buildEnvironment(baseEnvironment);

  return ProcessInvocation(
    command: options.goBinary,
    args: <String>[
      'run',
      helperPath,
      '--sdk',
      options.sdk,
      '--server-sdk',
      options.serverSDK,
      '--message',
      options.message,
      '--timeout-ms',
      options.timeoutMs.toString(),
      '--go',
      options.goBinary,
      normalizeEchoURI(options.uri),
    ],
    workingDirectory: goHolonsPath,
    environment: environment,
  );
}

ProcessInvocation buildEchoServerInvocation(
  EchoServerOptions options, {
  String? sdkRootPath,
  Map<String, String>? baseEnvironment,
}) {
  final rootPath = sdkRootPath ?? _defaultSDKRootPath();
  final helperPath = '$rootPath/cmd/echo-server-go/main.go';
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

Future<String> runEchoClient(
  List<String> args, {
  Map<String, String>? environment,
  String? sdkRootPath,
}) async {
  final options = parseEchoClientArgs(args, environment: environment);
  final invocation = buildEchoClientInvocation(
    options,
    sdkRootPath: sdkRootPath,
    baseEnvironment: environment ?? Platform.environment,
  );

  final process = await Process.start(
    invocation.command,
    invocation.args,
    workingDirectory: invocation.workingDirectory,
    environment: invocation.environment,
  );

  final stdoutText = await process.stdout.transform(utf8.decoder).join();
  final stderrText = await process.stderr.transform(utf8.decoder).join();
  final code = await process.exitCode;

  if (code != 0) {
    final message = stderrText.trim().isEmpty
        ? 'echo helper exited with code $code'
        : stderrText.trim();
    throw ProcessException(invocation.command, invocation.args, message, code);
  }

  final payload = _lastNonEmptyLine(stdoutText);
  if (payload.isEmpty) {
    throw StateError('echo helper returned empty stdout');
  }

  jsonDecode(payload);
  return payload;
}

Future<int> runEchoServer(
  List<String> args, {
  Map<String, String>? environment,
  String? sdkRootPath,
}) async {
  final options = parseEchoServerArgs(args, environment: environment);
  final invocation = buildEchoServerInvocation(
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

  var forwardedSignal = false;
  StreamSubscription<ProcessSignal>? sigtermSubscription;
  StreamSubscription<ProcessSignal>? sigintSubscription;

  void forwardSignal(ProcessSignal signal) {
    forwardedSignal = true;
    if (!process.kill(signal)) {
      process.kill(ProcessSignal.sigterm);
    }
  }

  if (!Platform.isWindows) {
    sigtermSubscription = ProcessSignal.sigterm
        .watch()
        .listen((_) => forwardSignal(ProcessSignal.sigterm));
    sigintSubscription = ProcessSignal.sigint
        .watch()
        .listen((_) => forwardSignal(ProcessSignal.sigint));
  }

  final code = await process.exitCode;

  await sigtermSubscription?.cancel();
  await sigintSubscription?.cancel();

  if (forwardedSignal && code >= 128) {
    return 0;
  }
  return code;
}

String normalizeEchoURI(String uri) {
  return uri == 'stdio' ? 'stdio://' : uri;
}

String resolveGoBinary({Map<String, String>? environment}) {
  final env = environment ?? Platform.environment;
  final fromEnv = (env['GO_BIN'] ?? '').trim();
  if (fromEnv.isNotEmpty) {
    return fromEnv;
  }

  final preferred = File(preferredGoBinary);
  if (preferred.existsSync()) {
    return preferred.path;
  }
  return 'go';
}

String _valueAt(List<String> args, int index, String flag) {
  final value = index < args.length ? args[index] : null;
  if (value == null || value.isEmpty) {
    throw FormatException('missing value for $flag');
  }
  return value;
}

Map<String, String> _buildEnvironment(Map<String, String>? baseEnvironment) {
  final env = Map<String, String>.from(baseEnvironment ?? Platform.environment);
  if ((env['GOCACHE'] ?? '').trim().isEmpty) {
    env['GOCACHE'] = defaultGoCache;
  }
  return env;
}

bool _hasFlag(List<String> args, String name) {
  final withEquals = '$name=';
  return args.any((arg) => arg == name || arg.startsWith(withEquals));
}

String _lastNonEmptyLine(String text) {
  final lines = text
      .split(RegExp(r'\r?\n'))
      .map((line) => line.trim())
      .where((line) => line.isNotEmpty);
  if (lines.isEmpty) {
    return '';
  }
  return lines.last;
}

String _defaultSDKRootPath() {
  final pubspec = File('${Directory.current.path}/pubspec.yaml');
  if (pubspec.existsSync()) {
    return Directory.current.path;
  }
  return Directory.fromUri(Platform.script).parent.parent.path;
}
