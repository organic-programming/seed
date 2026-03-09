import 'dart:convert';
import 'dart:io';

import 'package:holons/holons.dart' as holons;

/// Pure deterministic HelloService.
String greet(String name) {
  final n = name.isEmpty ? 'World' : name;
  return 'Hello, $n!';
}

void serve(List<String> args) {
  final listenUri = holons.parseFlags(args);
  stderr.writeln('dart-hello-world listening on $listenUri');
  stdout.writeln(jsonEncode(<String, String>{'message': greet('')}));
}

void main(List<String> args) {
  if (args.isNotEmpty && args.first == 'serve') {
    serve(args.skip(1).toList());
    return;
  }

  final name = args.isNotEmpty ? args.first : '';
  print(greet(name));
}
