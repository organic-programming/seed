import 'dart:io' as io;

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import '../gen/describe_generated.dart';

const String version = 'observability-cascade-dart-node 0.1.0';

Future<int> main(
  List<String> args, {
  StringSink? stdoutSink,
  StringSink? stderrSink,
}) async {
  final stdout = stdoutSink ?? io.stdout;
  final stderr = stderrSink ?? io.stderr;

  if (args.isEmpty) {
    printUsage(stderr);
    return 1;
  }

  switch (canonicalCommand(args.first)) {
    case 'serve':
      try {
        useStaticResponse(staticDescribeResponse());
        final childFlags = parseChildFlags(args.sublist(1));
        final parsed = parseOptions(childFlags.remaining);
        final transportName = parseTransport(childFlags.remaining);
        fromEnv(const Config(), io.Platform.environment);
        SpawnedMember? downstream;
        try {
          if (childFlags.children.isNotEmpty) {
            final child = childFlags.children.first;
            downstream = await spawnMember(
              slug: child.slug,
              binaryPath: child.binary,
              transport: transportName,
              downstreamChain: childFlags.children.skip(1).toList(),
            );
          }
          await runWithOptions(
            parsed.listenUri,
            <Service>[
              relayService(RelayOptions(downstreamConn: downstream?.conn)),
            ],
            options: ServeOptions(reflect: parsed.reflect),
          );
        } finally {
          await downstream?.stop();
        }
      } catch (error) {
        stderr.writeln('serve: $error');
        return 1;
      }
      return 0;
    case 'version':
      stdout.writeln(version);
      return 0;
    case 'help':
      printUsage(stdout);
      return 0;
    default:
      stderr.writeln('unknown command "${args.first}"');
      printUsage(stderr);
      return 1;
  }
}

String parseTransport(List<String> args) {
  for (var i = 0; i < args.length; i++) {
    final arg = args[i];
    if (arg == '--transport' && i + 1 < args.length) {
      return args[i + 1];
    }
    if (arg.startsWith('--transport=')) {
      return arg.substring('--transport='.length);
    }
  }
  return 'stdio';
}

String canonicalCommand(String raw) {
  final normalized = raw.trim().toLowerCase();
  return normalized.replaceAll('-', '').replaceAll('_', '').replaceAll(' ', '');
}

void printUsage(StringSink out) {
  out.writeln(
    'usage: observability-cascade-dart-node <command> [args] [flags]',
  );
  out.writeln('');
  out.writeln('commands:');
  out.writeln(
    '  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server',
  );
  out.writeln(
    '  version                                             Print version and exit',
  );
  out.writeln(
    '  help                                                Print this help',
  );
}
