import 'package:grpc/grpc.dart';
import 'package:protobuf/protobuf.dart';

typedef GeneratedMessageFactory<T extends GeneratedMessage> = T Function();

abstract interface class JsonUnaryMethodDescriptor {
  String get path;

  Future<Object?> invoke(
    ClientChannel channel, {
    Object? payload,
    CallOptions? callOptions,
  });
}

class UnaryJsonMethodDescriptor<
  Request extends GeneratedMessage,
  Response extends GeneratedMessage
> implements JsonUnaryMethodDescriptor {
  const UnaryJsonMethodDescriptor({
    required this.path,
    required this.createRequest,
    required this.createResponse,
    this.defaultCallOptions,
  });

  final String path;
  final GeneratedMessageFactory<Request> createRequest;
  final GeneratedMessageFactory<Response> createResponse;
  final CallOptions? defaultCallOptions;

  @override
  Future<Object?> invoke(
    ClientChannel channel, {
    Object? payload,
    CallOptions? callOptions,
  }) async {
    final request = createRequest();
    request.mergeFromProto3Json(payload ?? const <String, Object?>{});

    final method = ClientMethod<Request, Response>(
      path,
      (value) => value.writeToBuffer(),
      (bytes) {
        final response = createResponse();
        response.mergeFromBuffer(bytes);
        return response;
      },
    );

    final call = channel.createCall(
      method,
      Stream<Request>.value(request),
      callOptions ?? defaultCallOptions ?? CallOptions(),
    );
    final response = await call.response.single;
    return response.toProto3Json();
  }
}

class UnaryJsonMethodRegistry {
  UnaryJsonMethodRegistry(Iterable<JsonUnaryMethodDescriptor> methods)
    : _methodsByPath = _index(methods);

  final Map<String, JsonUnaryMethodDescriptor> _methodsByPath;

  Future<Object?> invoke(
    ClientChannel channel,
    String method, {
    Object? payload,
    CallOptions? callOptions,
  }) {
    return resolve(method).invoke(
      channel,
      payload: payload,
      callOptions: callOptions,
    );
  }

  JsonUnaryMethodDescriptor resolve(String method) {
    final canonical = canonicalGrpcMethodPath(method);
    final descriptor = _methodsByPath[canonical];
    if (descriptor != null) {
      return descriptor;
    }

    final available = _methodsByPath.keys.toList()..sort();
    throw ArgumentError.value(
      method,
      'method',
      'Unknown unary gRPC method. Available: ${available.join(', ')}',
    );
  }

  static Map<String, JsonUnaryMethodDescriptor> _index(
    Iterable<JsonUnaryMethodDescriptor> methods,
  ) {
    final indexed = <String, JsonUnaryMethodDescriptor>{};
    for (final method in methods) {
      indexed[canonicalGrpcMethodPath(method.path)] = method;
    }
    return indexed;
  }
}

String canonicalGrpcMethodPath(String method) {
  final trimmed = method.trim();
  if (trimmed.isEmpty) {
    throw ArgumentError.value(method, 'method', 'Method path must not be empty');
  }
  return trimmed.startsWith('/') ? trimmed : '/$trimmed';
}

Future<Object?> invokeUnaryJson(
  ClientChannel channel,
  String method, {
  Object? payload,
  required UnaryJsonMethodRegistry registry,
  CallOptions? callOptions,
}) {
  return registry.invoke(
    channel,
    method,
    payload: payload,
    callOptions: callOptions,
  );
}
