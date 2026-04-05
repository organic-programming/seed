import 'package:grpc/grpc.dart';

import '../controller/greeting_controller.dart';
import '../gen/v1/holon.pbgrpc.dart';

class GreetingAppRpcService extends GreetingAppServiceBase {
  GreetingAppRpcService(this._controller);

  final GreetingController _controller;

  @override
  Future<SelectHolonResponse> selectHolon(
    ServiceCall call,
    SelectHolonRequest request,
  ) async {
    try {
      await _controller.selectHolonBySlug(request.slug);
      final identity = _controller.selectedHolon;
      if (identity == null) {
        throw GrpcError.notFound("Holon '${request.slug}' not found");
      }
      return SelectHolonResponse(
        slug: identity.slug,
        displayName: identity.displayName,
      );
    } on GrpcError {
      rethrow;
    } on Object catch (error) {
      throw GrpcError.notFound('$error');
    }
  }

  @override
  Future<SelectLanguageResponse> selectLanguage(
    ServiceCall call,
    SelectLanguageRequest request,
  ) async {
    await _controller.setSelectedLanguage(request.code, greetNow: false);
    return SelectLanguageResponse(code: request.code);
  }

  @override
  Future<GreetResponse> greet(ServiceCall call, GreetRequest request) async {
    if (request.name.trim().isNotEmpty) {
      await _controller.setUserName(request.name, greetNow: false);
    }
    if (request.langCode.trim().isNotEmpty) {
      await _controller.setSelectedLanguage(request.langCode, greetNow: false);
    }
    if (_controller.selectedLanguageCode.trim().isEmpty) {
      throw GrpcError.invalidArgument('No language selected');
    }
    await _controller.greet(
      name: request.name.trim().isEmpty ? null : request.name,
      langCode: request.langCode.trim().isEmpty ? null : request.langCode,
    );
    if (_controller.error != null) {
      throw GrpcError.unavailable(_controller.error!);
    }
    return GreetResponse(greeting: _controller.greeting);
  }
}
