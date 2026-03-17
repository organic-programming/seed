import '../gen/dart/greeting/v1/greeting.pb.dart';
import '../_internal/greetings.dart';

ListLanguagesResponse listLanguages(ListLanguagesRequest request) {
  request;
  return ListLanguagesResponse()
    ..languages.addAll(
      greetings.map(
        (entry) => Language()
          ..code = entry.langCode
          ..name = entry.langEnglish
          ..native = entry.langNative,
      ),
    );
}

SayHelloResponse sayHello(SayHelloRequest request) {
  final greeting = lookup(request.langCode);
  final name = request.name.trim().isEmpty ? greeting.defaultName : request.name.trim();
  return SayHelloResponse()
    ..greeting = greeting.template.replaceFirst('%s', name)
    ..language = greeting.langEnglish
    ..langCode = greeting.langCode;
}
