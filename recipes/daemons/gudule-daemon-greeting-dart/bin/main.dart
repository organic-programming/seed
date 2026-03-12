import 'dart:io';

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import '../gen/dart/greeting/v1/greeting.pb.dart';
import '../gen/dart/greeting/v1/greeting.pbgrpc.dart';
import '../lib/src/greetings.dart';
import '../lib/src/recipe_paths.dart';

class GreetingService extends GreetingServiceBase {
  @override
  Future<ListLanguagesResponse> listLanguages(
    ServiceCall call,
    ListLanguagesRequest request,
  ) async {
    return ListLanguagesResponse()
      ..languages.addAll(
        greetings.map(
          (entry) => Language()
            ..code = entry.code
            ..name = entry.name
            ..native = entry.native,
        ),
      );
  }

  @override
  Future<SayHelloResponse> sayHello(
    ServiceCall call,
    SayHelloRequest request,
  ) async {
    final entry = lookupGreeting(request.langCode);
    final name = request.name.trim().isEmpty ? 'World' : request.name.trim();
    return SayHelloResponse()
      ..greeting = entry.template.replaceFirst('%s', name)
      ..language = entry.name
      ..langCode = entry.code;
  }
}

Never usage() {
  stderr.writeln('usage: gudule-daemon-greeting-dart <serve|version> [flags]');
  exit(1);
}

Future<void> main(List<String> args) async {
  if (args.isEmpty) {
    usage();
  }

  switch (args.first) {
    case 'serve':
      final recipeRoot = locateRecipeRoot();
      final listenUri = parseFlags(args.skip(1).toList());
      await runWithOptions(
        listenUri,
        <Service>[GreetingService()],
        options: ServeOptions(
          protoDir: recipeRoot == null ? null : '$recipeRoot/protos',
          holonYamlPath: recipeRoot == null ? null : '$recipeRoot/holon.yaml',
        ),
      );
      return;
    case 'version':
      stdout.writeln('gudule-daemon-greeting-dart v0.4.2');
      return;
    default:
      usage();
  }
}
