import 'dart:io' as io;

import 'package:holons/holons.dart';

import '../_internal/server.dart' as rpc;

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
        final parsed = parseOptions(args.sublist(1));
        final members = parseMemberRefs(args.sublist(1));
        await rpc.listenAndServe(
          parsed.listenUri,
          reflect: parsed.reflect,
          members: members,
        );
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

List<MemberRef> parseMemberRefs(List<String> args) {
  final members = <MemberRef>[];
  for (var i = 0; i < args.length; i++) {
    final arg = args[i];
    if (arg == '--member') {
      if (i + 1 >= args.length) {
        throw ArgumentError('--member requires <slug>=<address>');
      }
      members.add(parseMemberRef(args[i + 1]));
      i += 1;
    } else if (arg.startsWith('--member=')) {
      members.add(parseMemberRef(arg.substring('--member='.length)));
    }
  }
  return members;
}

MemberRef parseMemberRef(String raw) {
  final index = raw.indexOf('=');
  if (index < 0) {
    throw ArgumentError('--member requires <slug>=<address>');
  }
  final slug = raw.substring(0, index).trim();
  final address = raw.substring(index + 1).trim();
  if (slug.isEmpty || address.isEmpty) {
    throw ArgumentError('--member requires non-empty slug and address');
  }
  return MemberRef(slug: slug, address: address);
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
    '  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server',
  );
  out.writeln(
    '  version                                             Print version and exit',
  );
  out.writeln(
    '  help                                                Print this help',
  );
}
