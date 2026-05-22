import 'dart:io';

import 'package:holons/src/composite.dart';
import 'package:test/test.dart';

void main() {
  test('member resolves embedded binary', () {
    final root = Directory.systemTemp.createTempSync('holons-dart-composite-');
    addTearDown(() => root.deleteSync(recursive: true));
    final binDir = Directory('${root.path}/composite.holon/bin/darwin_arm64')
      ..createSync(recursive: true);
    final self = File('${binDir.path}/composite')..writeAsStringSync('self');
    final memberDir = Directory('${binDir.path}/holons/node-a')
      ..createSync(recursive: true);
    File('${memberDir.path}/README.txt').writeAsStringSync('not executable');
    final member = File('${memberDir.path}/node-a-bin')
      ..writeAsStringSync('node');
    if (!Platform.isWindows) {
      Process.runSync('chmod', ['755', member.path]);
    }

    expect(memberFromExecutable(self.path, 'node-a'),
        equals(member.absolute.path));
  });

  test('member rejects empty id', () {
    expect(
      () => memberFromExecutable('/tmp/composite', ' '),
      throwsA(isA<ArgumentError>()),
    );
  });
}
