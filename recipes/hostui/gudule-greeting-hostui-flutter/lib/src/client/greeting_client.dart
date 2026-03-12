import 'package:grpc/grpc.dart';

import '../../gen/greeting/v1/greeting.pbgrpc.dart';
import 'daemon_launcher.dart';
import 'greeting_target.dart';

class GreetingClient {
  ClientChannel? _channel;
  GreetingServiceClient? _stub;
  final DaemonLauncher _launcher;

  GreetingClient({DaemonLauncher? launcher})
      : _launcher = launcher ?? DaemonLauncher();

  bool get isConnected => _stub != null;

  Future<void> connect(GreetingEndpoint endpoint) async {
    await close();
    final channel = await _launcher.start(endpoint);
    _channel = channel;
    _stub = GreetingServiceClient(channel);
  }

  Future<ListLanguagesResponse> listLanguages() async {
    return _stub!.listLanguages(ListLanguagesRequest());
  }

  Future<SayHelloResponse> sayHello(String name, String langCode) async {
    return _stub!.sayHello(
      SayHelloRequest(name: name, langCode: langCode),
    );
  }

  Future<void> close() async {
    final channel = _channel;
    _stub = null;
    _channel = null;
    await _launcher.stop(channel);
  }
}
