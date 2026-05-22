import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import '../api/public.dart' as public_api;
import '../gen/describe_generated.dart';
import '../gen/dart/greeting/v1/greeting.pbgrpc.dart';
import 'greetings.dart';

const _transportUnknown = 'unknown';
const _greetingCounterHelp =
    'Greetings emitted, partitioned by language and transport.';

class GreetingService extends GreetingServiceBase {
  @override
  Future<ListLanguagesResponse> listLanguages(
    ServiceCall call,
    ListLanguagesRequest request,
  ) async {
    call;
    return public_api.listLanguages(request);
  }

  @override
  Future<SayHelloResponse> sayHello(
    ServiceCall call,
    SayHelloRequest request,
  ) async {
    call;
    final stopwatch = Stopwatch()..start();
    final response = public_api.sayHello(request);
    final name = _resolvedName(request, response);
    final transport = _currentTransport();
    final durationNs = stopwatch.elapsedMicroseconds * 1000;
    _emitGreeting(response, name, transport, durationNs);
    return response;
  }
}

String _resolvedName(SayHelloRequest request, SayHelloResponse response) {
  final name = request.name.trim();
  if (name.isNotEmpty) {
    return name;
  }
  return lookup(response.langCode).defaultName;
}

String _currentTransport() =>
    CurrentTransport.trim().isEmpty ? _transportUnknown : CurrentTransport;

void _emitGreeting(
  SayHelloResponse response,
  String name,
  String transport,
  int durationNs,
) {
  final message =
      'Greeted $name in ${response.language} (${response.langCode})';
  final obs = current();
  obs.logger('greeting').info(
    message,
    fields: {
      'lang_code': response.langCode,
      'language': response.language,
      'name': name,
      'greeting': response.greeting,
      'transport': transport,
      'duration_ns': durationNs,
    },
  );
  obs.counter(
    'greeting_emitted_total',
    help: _greetingCounterHelp,
    labels: {
      'lang_code': response.langCode,
      'language': response.language,
      'transport': transport,
    },
  )?.inc();
}

Future<void> listenAndServe(String listenUri, {bool reflect = false}) {
  useStaticResponse(staticDescribeResponse());
  return runWithOptions(
    listenUri,
    <Service>[GreetingService()],
    options: ServeOptions(reflect: reflect),
  );
}
