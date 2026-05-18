import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart' as holons_obs;
import 'package:holons_app/holons_app.dart' show HolonRpcSelectionAdapter;

import '../controller/greeting_controller.dart';
import '../gen/v1/holon.pbgrpc.dart';
import '../model/app_model.dart';

// Dart serve does not yet expose a handler-visible current transport.
const _transportUnknown = 'unknown';
const _greetingCounterHelp =
    'Greetings emitted, partitioned by language and transport.';

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
    call;
    final stopwatch = Stopwatch()..start();
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
    final response = GreetResponse(greeting: _controller.greeting);
    final langCode = _controller.selectedLanguageCode.trim();
    final language = _languageName(langCode);
    final name = _resolvedName(request.name);
    final transport = _currentTransport();
    final durationNs = stopwatch.elapsedMicroseconds * 1000;
    _emitGreeting(
      response: response,
      langCode: langCode,
      language: language,
      name: name,
      transport: transport,
      durationNs: durationNs,
    );
    return response;
  }

  String _resolvedName(String requestedName) {
    final name = requestedName.trim();
    if (name.isNotEmpty) {
      return name;
    }
    return _controller.userName.trim();
  }

  String _languageName(String langCode) {
    for (final language in _controller.availableLanguages) {
      if (language.code == langCode) {
        return language.name;
      }
    }
    return langCode;
  }

  String _currentTransport() => _transportUnknown;

  void _emitGreeting({
    required GreetResponse response,
    required String langCode,
    required String language,
    required String name,
    required String transport,
    required int durationNs,
  }) {
    final message = 'Greeted $name in $language ($langCode)';
    final obs = holons_obs.current();
    final fields = <String, Object?>{
      'lang_code': langCode,
      'language': language,
      'name': name,
      'greeting': response.greeting,
      'transport': transport,
      'duration_ns': durationNs,
    };
    obs.logger('greeting').info(message, fields: fields);
    obs
        .counter(
          'greeting_emitted_total',
          help: _greetingCounterHelp,
          labels: {
            'lang_code': langCode,
            'language': language,
            'transport': transport,
          },
        )
        ?.inc();
  }
}
