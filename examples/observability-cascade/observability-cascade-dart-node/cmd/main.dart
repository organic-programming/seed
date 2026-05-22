import 'dart:io';

import '../api/cli.dart' as cli;

Future<void> main(List<String> args) async {
  final code = await cli.main(args);
  if (code != 0) {
    exit(code);
  }
}
