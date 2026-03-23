import 'dart:convert';

import 'package:test/test.dart';

import 'cli.dart' as cli;

void main() {
  test('runCli prints version', () async {
    final stdout = StringBuffer();
    final stderr = StringBuffer();

    final code = await cli.main(<String>['version'], stdoutSink: stdout, stderrSink: stderr);

    expect(code, equals(0));
    expect(stdout.toString().trim(), equals(cli.version));
    expect(stderr.toString(), isEmpty);
  });

  test('runCli prints help', () async {
    final stdout = StringBuffer();
    final stderr = StringBuffer();

    final code = await cli.main(<String>['help'], stdoutSink: stdout, stderrSink: stderr);

    expect(code, equals(0));
    expect(stdout.toString(), contains('usage: gabriel-greeting-dart'));
    expect(stdout.toString(), contains('listLanguages'));
    expect(stderr.toString(), isEmpty);
  });

  test('runCli renders listLanguages as JSON', () async {
    final stdout = StringBuffer();
    final stderr = StringBuffer();

    final code = await cli.main(
      <String>['listLanguages', '--format', 'json'],
      stdoutSink: stdout,
      stderrSink: stderr,
    );

    final payload = jsonDecode(stdout.toString()) as Map<String, dynamic>;
    final languages = payload['languages'] as List<dynamic>;

    expect(code, equals(0));
    expect(languages, hasLength(56));
    expect(languages.first['code'], equals('en'));
    expect(languages.first['name'], equals('English'));
    expect(stderr.toString(), isEmpty);
  });

  test('runCli renders sayHello as text', () async {
    final stdout = StringBuffer();
    final stderr = StringBuffer();

    final code = await cli.main(
      <String>['sayHello', 'Bob', 'fr'],
      stdoutSink: stdout,
      stderrSink: stderr,
    );

    expect(code, equals(0));
    expect(stdout.toString().trim(), equals('Bonjour Bob'));
    expect(stderr.toString(), isEmpty);
  });

  test('runCli defaults sayHello to English JSON output', () async {
    final stdout = StringBuffer();
    final stderr = StringBuffer();

    final code = await cli.main(
      <String>['sayHello', '--json'],
      stdoutSink: stdout,
      stderrSink: stderr,
    );

    final payload = jsonDecode(stdout.toString()) as Map<String, dynamic>;
    expect(code, equals(0));
    expect(payload['greeting'], equals('Hello Mary'));
    expect(payload['language'], equals('English'));
    expect(payload['langCode'], equals('en'));
    expect(stderr.toString(), isEmpty);
  });
}
