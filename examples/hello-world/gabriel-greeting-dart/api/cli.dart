import 'dart:convert';
import 'dart:io' as io;

import 'package:holons/holons.dart';
import 'package:protobuf/protobuf.dart';

import '../gen/dart/greeting/v1/greeting.pb.dart';
import '../_internal/server.dart' as rpc;
import 'public.dart' as public_api;

const String version = 'gabriel-greeting-dart v0.1.0';

Future<int> main(List<String> args, {StringSink? stdoutSink, StringSink? stderrSink}) async {
  final stdout = stdoutSink ?? io.stdout;
  final stderr = stderrSink ?? io.stderr;

  if (args.isEmpty) {
    printUsage(stderr);
    return 1;
  }

  switch (canonicalCommand(args.first)) {
    case 'serve':
      final listenUri = parseFlags(args.sublist(1));
      try {
        await rpc.listenAndServe(listenUri);
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
    case 'listlanguages':
      return runListLanguages(args.sublist(1), stdout: stdout, stderr: stderr);
    case 'sayhello':
      return runSayHello(args.sublist(1), stdout: stdout, stderr: stderr);
    default:
      stderr.writeln('unknown command "${args.first}"');
      printUsage(stderr);
      return 1;
  }
}

int runListLanguages(
  List<String> args, {
  required StringSink stdout,
  required StringSink stderr,
}) {
  try {
    final parsed = parseCommandOptions(args);
    if (parsed.positional.isNotEmpty) {
      stderr.writeln('listLanguages: accepts no positional arguments');
      return 1;
    }
    final response = public_api.listLanguages(ListLanguagesRequest());
    writeResponse(stdout, response, parsed.format);
    return 0;
  } catch (error) {
    stderr.writeln('listLanguages: $error');
    return 1;
  }
}

int runSayHello(
  List<String> args, {
  required StringSink stdout,
  required StringSink stderr,
}) {
  try {
    final parsed = parseCommandOptions(args);
    if (parsed.positional.length > 2) {
      stderr.writeln('sayHello: accepts at most <name> [lang_code]');
      return 1;
    }

    final request = SayHelloRequest()..langCode = 'en';
    if (parsed.positional.isNotEmpty) {
      request.name = parsed.positional[0];
    }
    if (parsed.positional.length >= 2) {
      if (parsed.lang.isNotEmpty) {
        stderr.writeln('sayHello: use either a positional lang_code or --lang, not both');
        return 1;
      }
      request.langCode = parsed.positional[1];
    }
    if (parsed.lang.isNotEmpty) {
      request.langCode = parsed.lang;
    }

    final response = public_api.sayHello(request);
    writeResponse(stdout, response, parsed.format);
    return 0;
  } catch (error) {
    stderr.writeln('sayHello: $error');
    return 1;
  }
}

CommandOptions parseCommandOptions(List<String> args) {
  final options = CommandOptions();
  var index = 0;

  while (index < args.length) {
    final arg = args[index];
    if (arg == '--json') {
      options.format = 'json';
    } else if (arg == '--format') {
      index += 1;
      if (index >= args.length) {
        throw ArgumentError('--format requires a value');
      }
      options.format = parseOutputFormat(args[index]);
    } else if (arg.startsWith('--format=')) {
      options.format = parseOutputFormat(arg.substring('--format='.length));
    } else if (arg == '--lang') {
      index += 1;
      if (index >= args.length) {
        throw ArgumentError('--lang requires a value');
      }
      options.lang = args[index].trim();
    } else if (arg.startsWith('--lang=')) {
      options.lang = arg.substring('--lang='.length).trim();
    } else if (arg.startsWith('--')) {
      throw ArgumentError('unknown flag "$arg"');
    } else {
      options.positional.add(arg);
    }
    index += 1;
  }

  return options;
}

String parseOutputFormat(String raw) {
  final normalized = raw.trim().toLowerCase();
  if (normalized.isEmpty || normalized == 'text' || normalized == 'txt') {
    return 'text';
  }
  if (normalized == 'json') {
    return 'json';
  }
  throw ArgumentError('unsupported format "$raw"');
}

void writeResponse(StringSink stdout, GeneratedMessage message, String format) {
  switch (format) {
    case 'json':
      stdout.writeln(jsonEncode(message.toProto3Json()));
      return;
    case 'text':
      writeText(stdout, message);
      return;
    default:
      throw ArgumentError('unsupported format "$format"');
  }
}

void writeText(StringSink stdout, GeneratedMessage message) {
  if (message is SayHelloResponse) {
    stdout.writeln(message.greeting);
    return;
  }
  if (message is ListLanguagesResponse) {
    for (final language in message.languages) {
      stdout.writeln('${language.code}\t${language.name}\t${language.native}');
    }
    return;
  }
  throw ArgumentError('unsupported text output for ${message.runtimeType}');
}

String canonicalCommand(String raw) {
  return raw.trim().toLowerCase().replaceAll(RegExp(r'[-_\s]'), '');
}

void printUsage(StringSink output) {
  output.writeln('usage: gabriel-greeting-dart <command> [args] [flags]');
  output.writeln('');
  output.writeln('commands:');
  output.writeln('  serve [--listen <uri>]                    Start the gRPC server');
  output.writeln('  version                                  Print version and exit');
  output.writeln('  help                                     Print usage');
  output.writeln('  listLanguages [--format text|json]       List supported languages');
  output.writeln('  sayHello [name] [lang_code] [--format text|json] [--lang <code>]');
  output.writeln('');
  output.writeln('examples:');
  output.writeln('  gabriel-greeting-dart serve --listen stdio');
  output.writeln('  gabriel-greeting-dart listLanguages --format json');
  output.writeln('  gabriel-greeting-dart sayHello Alice fr');
  output.writeln('  gabriel-greeting-dart sayHello Alice --lang fr --format json');
}

class CommandOptions {
  String format = 'text';
  String lang = '';
  final List<String> positional = <String>[];
}
