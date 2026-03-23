import 'package:test/test.dart';
import '../bin/hello.dart';

void main() {
  test('greet with name', () {
    expect(greet('Bob'), equals('Hello, Bob!'));
  });

  test('greet default', () {
    expect(greet(''), equals('Hello, World!'));
  });
}
