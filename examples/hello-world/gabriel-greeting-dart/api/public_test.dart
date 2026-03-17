import 'package:test/test.dart';

import '../gen/dart/greeting/v1/greeting.pb.dart';
import 'public.dart' as public_api;

void main() {
  test('listLanguages includes English', () {
    final response = public_api.listLanguages(ListLanguagesRequest());
    final english = response.languages.firstWhere((language) => language.code == 'en');

    expect(english.name, equals('English'));
    expect(english.native, equals('English'));
  });

  test('sayHello uses requested language', () {
    final response = public_api.sayHello(
      SayHelloRequest()
        ..name = 'Alice'
        ..langCode = 'fr',
    );

    expect(response.greeting, equals('Bonjour Alice'));
    expect(response.language, equals('French'));
    expect(response.langCode, equals('fr'));
  });

  test('sayHello uses localized default name', () {
    final response = public_api.sayHello(
      SayHelloRequest()..langCode = 'ja',
    );

    expect(response.greeting, equals('こんにちは、マリアさん'));
    expect(response.language, equals('Japanese'));
    expect(response.langCode, equals('ja'));
  });

  test('sayHello falls back to English', () {
    final response = public_api.sayHello(
      SayHelloRequest()..langCode = 'unknown',
    );

    expect(response.greeting, equals('Hello Mary'));
    expect(response.language, equals('English'));
    expect(response.langCode, equals('en'));
  });
}
