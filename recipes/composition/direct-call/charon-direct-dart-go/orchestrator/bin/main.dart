import 'dart:io';

Future<void> main() async {
  final script = Platform.environment['CHARON_RUN_SCRIPT'] ??
      '${File(Platform.resolvedExecutable).parent.path}/scripts/run.sh';
  final process = await Process.start(
    '/bin/sh',
    [script],
    mode: ProcessStartMode.inheritStdio,
  );
  final code = await process.exitCode;
  exit(code);
}
