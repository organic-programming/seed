import 'package:grpc/grpc.dart';
import 'package:holons_app/holons_app.dart' show HolonTransportName;

import '../controller/greeting_controller.dart';
import '../gen/v1/holon.pbgrpc.dart';

class GreetingAppRpcService extends GreetingAppServiceBase {
  GreetingAppRpcService(this._controller);

  final GreetingController _controller;

  String _validatedLanguageCode(String value) {
    final code = value.trim();
    if (code.isEmpty) {
      throw GrpcError.invalidArgument('Language code is required');
    }
    final supported = _controller.availableLanguages.any(
      (language) => language.code == code,
    );
    if (!supported) {
      throw GrpcError.invalidArgument('Unsupported language "$code"');
    }
    return code;
  }

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
  Future<SelectTransportResponse> selectTransport(
    ServiceCall call,
    SelectTransportRequest request,
  ) async {
    final transport = HolonTransportName.parseCanonical(request.transport);
    if (transport == null) {
      throw GrpcError.invalidArgument(
        'Unsupported transport "${request.transport}". Expected one of: stdio, tcp, unix',
      );
    }
    if (!_controller.capabilities.holonTransportNames.contains(transport)) {
      throw GrpcError.invalidArgument(
        'Transport "${transport.rawValue}" is not available on this platform',
      );
    }

    await _controller.setTransport(transport.rawValue, reload: true);
    if (_controller.connectionError != null) {
      throw GrpcError.unavailable(_controller.connectionError!);
    }
    if (_controller.error != null) {
      throw GrpcError.unavailable(_controller.error!);
    }
    return SelectTransportResponse(transport: transport.rawValue);
  }

  @override
  Future<SelectLanguageResponse> selectLanguage(
    ServiceCall call,
    SelectLanguageRequest request,
  ) async {
    final code = _validatedLanguageCode(request.code);
    await _controller.setSelectedLanguage(code, greetNow: false);
    return SelectLanguageResponse(code: code);
  }

  @override
  Future<GreetResponse> greet(ServiceCall call, GreetRequest request) async {
    if (request.name.trim().isNotEmpty) {
      await _controller.setUserName(request.name, greetNow: false);
    }
    if (request.langCode.trim().isNotEmpty) {
      final code = _validatedLanguageCode(request.langCode);
      await _controller.setSelectedLanguage(code, greetNow: false);
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
