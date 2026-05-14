// This is a generated file - do not edit.
//
// Generated from observability_cascade/v1/service.proto.

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

import 'service.pb.dart' as $0;

export 'service.pb.dart';

@$pb.GrpcServiceName('observability_cascade.v1.ObservabilityCascadeService')
class ObservabilityCascadeServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ObservabilityCascadeServiceClient(super.channel,
      {super.options, super.interceptors});

  /// Run the default 4-deep chain in this composite's own language.
  /// @example {}
  $grpc.ResponseFuture<$0.CascadeReport> runDefault(
    $0.RunRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$runDefault, request, options: options);
  }

  /// Run with long-lived Follow:true streams.
  /// @example {}
  $grpc.ResponseFuture<$0.CascadeReport> runLiveStream(
    $0.RunRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$runLiveStream, request, options: options);
  }

  /// Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
  /// @example {}
  $grpc.ResponseFuture<$0.MultiPatternReport> runMultiPattern(
    $0.RunRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$runMultiPattern, request, options: options);
  }

  // method descriptors

  static final _$runDefault =
      $grpc.ClientMethod<$0.RunRequest, $0.CascadeReport>(
          '/observability_cascade.v1.ObservabilityCascadeService/RunDefault',
          ($0.RunRequest value) => value.writeToBuffer(),
          $0.CascadeReport.fromBuffer);
  static final _$runLiveStream =
      $grpc.ClientMethod<$0.RunRequest, $0.CascadeReport>(
          '/observability_cascade.v1.ObservabilityCascadeService/RunLiveStream',
          ($0.RunRequest value) => value.writeToBuffer(),
          $0.CascadeReport.fromBuffer);
  static final _$runMultiPattern = $grpc.ClientMethod<$0.RunRequest,
          $0.MultiPatternReport>(
      '/observability_cascade.v1.ObservabilityCascadeService/RunMultiPattern',
      ($0.RunRequest value) => value.writeToBuffer(),
      $0.MultiPatternReport.fromBuffer);
}

@$pb.GrpcServiceName('observability_cascade.v1.ObservabilityCascadeService')
abstract class ObservabilityCascadeServiceBase extends $grpc.Service {
  $core.String get $name =>
      'observability_cascade.v1.ObservabilityCascadeService';

  ObservabilityCascadeServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.RunRequest, $0.CascadeReport>(
        'RunDefault',
        runDefault_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RunRequest.fromBuffer(value),
        ($0.CascadeReport value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RunRequest, $0.CascadeReport>(
        'RunLiveStream',
        runLiveStream_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RunRequest.fromBuffer(value),
        ($0.CascadeReport value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RunRequest, $0.MultiPatternReport>(
        'RunMultiPattern',
        runMultiPattern_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RunRequest.fromBuffer(value),
        ($0.MultiPatternReport value) => value.writeToBuffer()));
  }

  $async.Future<$0.CascadeReport> runDefault_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.RunRequest> $request) async {
    return runDefault($call, await $request);
  }

  $async.Future<$0.CascadeReport> runDefault(
      $grpc.ServiceCall call, $0.RunRequest request);

  $async.Future<$0.CascadeReport> runLiveStream_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.RunRequest> $request) async {
    return runLiveStream($call, await $request);
  }

  $async.Future<$0.CascadeReport> runLiveStream(
      $grpc.ServiceCall call, $0.RunRequest request);

  $async.Future<$0.MultiPatternReport> runMultiPattern_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.RunRequest> $request) async {
    return runMultiPattern($call, await $request);
  }

  $async.Future<$0.MultiPatternReport> runMultiPattern(
      $grpc.ServiceCall call, $0.RunRequest request);
}
