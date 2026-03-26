import 'dart:io';

import 'package:holons/src/holonrpc_server_cli.dart';

Future<void> main(List<String> args) async {
  try {
    exitCode = await runHolonRPCServer(args);
  } on FormatException catch (error) {
    stderr.writeln(error.message);
    if (error.message != holonRPCServerUsage) {
      stderr.writeln(holonRPCServerUsage);
    }
    exitCode = 2;
  } on ProcessException catch (error) {
    final message = error.message.trim();
    stderr.writeln(message.isEmpty ? error.toString() : message);
    exitCode = error.errorCode == 0 ? 1 : error.errorCode;
  } on Object catch (error) {
    stderr.writeln(error.toString());
    exitCode = 1;
  }
}
