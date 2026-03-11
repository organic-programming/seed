// This is a generated file - do not edit.
//
// Generated from greeting/v1/greeting.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'greeting.pb.dart' as $0;

export 'greeting.pb.dart';

@$pb.GrpcServiceName('greeting.v1.GreetingService')
class GreetingServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  GreetingServiceClient(super.channel, {super.options, super.interceptors});

  /// Returns all available greeting languages.
  $grpc.ResponseFuture<$0.ListLanguagesResponse> listLanguages(
    $0.ListLanguagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listLanguages, request, options: options);
  }

  /// Greets the user in the chosen language.
  $grpc.ResponseFuture<$0.SayHelloResponse> sayHello(
    $0.SayHelloRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sayHello, request, options: options);
  }

  // method descriptors

  static final _$listLanguages =
      $grpc.ClientMethod<$0.ListLanguagesRequest, $0.ListLanguagesResponse>(
          '/greeting.v1.GreetingService/ListLanguages',
          ($0.ListLanguagesRequest value) => value.writeToBuffer(),
          $0.ListLanguagesResponse.fromBuffer);
  static final _$sayHello =
      $grpc.ClientMethod<$0.SayHelloRequest, $0.SayHelloResponse>(
          '/greeting.v1.GreetingService/SayHello',
          ($0.SayHelloRequest value) => value.writeToBuffer(),
          $0.SayHelloResponse.fromBuffer);
}

@$pb.GrpcServiceName('greeting.v1.GreetingService')
abstract class GreetingServiceBase extends $grpc.Service {
  $core.String get $name => 'greeting.v1.GreetingService';

  GreetingServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.ListLanguagesRequest, $0.ListLanguagesResponse>(
            'ListLanguages',
            listLanguages_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListLanguagesRequest.fromBuffer(value),
            ($0.ListLanguagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SayHelloRequest, $0.SayHelloResponse>(
        'SayHello',
        sayHello_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SayHelloRequest.fromBuffer(value),
        ($0.SayHelloResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ListLanguagesResponse> listLanguages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListLanguagesRequest> $request) async {
    return listLanguages($call, await $request);
  }

  $async.Future<$0.ListLanguagesResponse> listLanguages(
      $grpc.ServiceCall call, $0.ListLanguagesRequest request);

  $async.Future<$0.SayHelloResponse> sayHello_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SayHelloRequest> $request) async {
    return sayHello($call, await $request);
  }

  $async.Future<$0.SayHelloResponse> sayHello(
      $grpc.ServiceCall call, $0.SayHelloRequest request);
}
