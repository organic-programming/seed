import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import '../api/public.dart' as public_api;
import '../gen/dart/greeting/v1/greeting.pb.dart';
import '../gen/dart/greeting/v1/greeting.pbgrpc.dart';

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
    return public_api.sayHello(request);
  }
}

Future<void> listenAndServe(String listenUri, {bool reflect = false}) {
  return runWithOptions(
    listenUri,
    <Service>[GreetingService()],
    options: ServeOptions(reflect: reflect),
  );
}
