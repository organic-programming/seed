import 'package:grpc/grpc.dart';

import '../../gen/greeting/v1/greeting.pbgrpc.dart';
import 'daemon_launcher.dart';

/// gRPC client for the GreetingService.
class GreetingClient {
  ClientChannel? _channel;
  GreetingServiceClient? _stub;
  final DaemonLauncher _launcher = DaemonLauncher();

  bool get isConnected => _stub != null;

  /// Starts the daemon through the Holons SDK and binds the gRPC stub.
  Future<void> connectDaemon() async {
    await close();
    final channel = await _launcher.start();
    _channel = channel;
    _stub = GreetingServiceClient(channel);
  }

  /// Fetches all available greeting languages.
  Future<ListLanguagesResponse> listLanguages() async {
    return _stub!.listLanguages(ListLanguagesRequest());
  }

  /// Greets the user in the specified language.
  Future<SayHelloResponse> sayHello(String name, String langCode) async {
    return _stub!.sayHello(
      SayHelloRequest(name: name, langCode: langCode),
    );
  }

  /// Shuts down the gRPC channel and daemon process.
  Future<void> close() async {
    final channel = _channel;
    _stub = null;
    _channel = null;
    await _launcher.stop(channel);
  }
}
