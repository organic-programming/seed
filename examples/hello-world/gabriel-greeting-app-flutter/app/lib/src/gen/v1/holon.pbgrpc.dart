// This is a generated file - do not edit.
//
// Generated from v1/holon.proto.

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

import 'holon.pb.dart' as $0;

export 'holon.pb.dart';

/// GreetingAppService is the COAX-facing domain surface for the Flutter app.
/// These methods drive the same state transitions a human performs through the UI.
@$pb.GrpcServiceName('greeting.v1.GreetingAppService')
class GreetingAppServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  GreetingAppServiceClient(super.channel, {super.options, super.interceptors});

  /// Select which greeting holon to use (by slug).
  $grpc.ResponseFuture<$0.SelectHolonResponse> selectHolon(
    $0.SelectHolonRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$selectHolon, request, options: options);
  }

  /// Select which transport the greeting holon connection should use.
  $grpc.ResponseFuture<$0.SelectTransportResponse> selectTransport(
    $0.SelectTransportRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$selectTransport, request, options: options);
  }

  /// Select a language for the greeting.
  $grpc.ResponseFuture<$0.SelectLanguageResponse> selectLanguage(
    $0.SelectLanguageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$selectLanguage, request, options: options);
  }

  /// Greet using the current selection.
  $grpc.ResponseFuture<$0.GreetResponse> greet(
    $0.GreetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$greet, request, options: options);
  }

  // method descriptors

  static final _$selectHolon =
      $grpc.ClientMethod<$0.SelectHolonRequest, $0.SelectHolonResponse>(
          '/greeting.v1.GreetingAppService/SelectHolon',
          ($0.SelectHolonRequest value) => value.writeToBuffer(),
          $0.SelectHolonResponse.fromBuffer);
  static final _$selectTransport =
      $grpc.ClientMethod<$0.SelectTransportRequest, $0.SelectTransportResponse>(
          '/greeting.v1.GreetingAppService/SelectTransport',
          ($0.SelectTransportRequest value) => value.writeToBuffer(),
          $0.SelectTransportResponse.fromBuffer);
  static final _$selectLanguage =
      $grpc.ClientMethod<$0.SelectLanguageRequest, $0.SelectLanguageResponse>(
          '/greeting.v1.GreetingAppService/SelectLanguage',
          ($0.SelectLanguageRequest value) => value.writeToBuffer(),
          $0.SelectLanguageResponse.fromBuffer);
  static final _$greet = $grpc.ClientMethod<$0.GreetRequest, $0.GreetResponse>(
      '/greeting.v1.GreetingAppService/Greet',
      ($0.GreetRequest value) => value.writeToBuffer(),
      $0.GreetResponse.fromBuffer);
}

@$pb.GrpcServiceName('greeting.v1.GreetingAppService')
abstract class GreetingAppServiceBase extends $grpc.Service {
  $core.String get $name => 'greeting.v1.GreetingAppService';

  GreetingAppServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.SelectHolonRequest, $0.SelectHolonResponse>(
            'SelectHolon',
            selectHolon_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SelectHolonRequest.fromBuffer(value),
            ($0.SelectHolonResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SelectTransportRequest,
            $0.SelectTransportResponse>(
        'SelectTransport',
        selectTransport_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SelectTransportRequest.fromBuffer(value),
        ($0.SelectTransportResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SelectLanguageRequest,
            $0.SelectLanguageResponse>(
        'SelectLanguage',
        selectLanguage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SelectLanguageRequest.fromBuffer(value),
        ($0.SelectLanguageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GreetRequest, $0.GreetResponse>(
        'Greet',
        greet_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GreetRequest.fromBuffer(value),
        ($0.GreetResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.SelectHolonResponse> selectHolon_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SelectHolonRequest> $request) async {
    return selectHolon($call, await $request);
  }

  $async.Future<$0.SelectHolonResponse> selectHolon(
      $grpc.ServiceCall call, $0.SelectHolonRequest request);

  $async.Future<$0.SelectTransportResponse> selectTransport_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SelectTransportRequest> $request) async {
    return selectTransport($call, await $request);
  }

  $async.Future<$0.SelectTransportResponse> selectTransport(
      $grpc.ServiceCall call, $0.SelectTransportRequest request);

  $async.Future<$0.SelectLanguageResponse> selectLanguage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SelectLanguageRequest> $request) async {
    return selectLanguage($call, await $request);
  }

  $async.Future<$0.SelectLanguageResponse> selectLanguage(
      $grpc.ServiceCall call, $0.SelectLanguageRequest request);

  $async.Future<$0.GreetResponse> greet_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.GreetRequest> $request) async {
    return greet($call, await $request);
  }

  $async.Future<$0.GreetResponse> greet(
      $grpc.ServiceCall call, $0.GreetRequest request);
}
