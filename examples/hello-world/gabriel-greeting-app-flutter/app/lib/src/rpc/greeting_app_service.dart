import 'package:grpc/grpc.dart';
import 'package:holons_app/holons_app.dart' show HolonRpcSelectionAdapter;

import '../controller/greeting_controller.dart';
import '../gen/v1/holon.pbgrpc.dart';
import '../model/app_model.dart';

class GreetingAppRpcService extends GreetingAppServiceBase {
  GreetingAppRpcService(this._controller)
    : _selection = HolonRpcSelectionAdapter<GabrielHolonIdentity>(_controller);

  final GreetingController _controller;
  final HolonRpcSelectionAdapter<GabrielHolonIdentity> _selection;

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
    final identity = await _selection.selectHolon(request.slug);
    return SelectHolonResponse(
      slug: identity.slug,
      displayName: identity.displayName,
    );
  }

  @override
  Future<SelectTransportResponse> selectTransport(
    ServiceCall call,
    SelectTransportRequest request,
  ) async {
    final transport = await _selection.selectTransport(request.transport);
    return SelectTransportResponse(transport: transport);
  }

  @override
  Future<SelectLanguageResponse> selectLanguage(
    ServiceCall call,
    SelectLanguageRequest request,
  ) async {
    final code = _validatedLanguageCode(request.code);
    await _controller.setSelectedLanguage(code);
    _selection.throwIfRuntimeError();
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
    _selection.throwIfRuntimeError();
    return GreetResponse(greeting: _controller.greeting);
  }
}
