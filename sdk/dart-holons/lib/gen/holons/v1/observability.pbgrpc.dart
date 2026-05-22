// This is a generated file - do not edit.
//
// Generated from holons/v1/observability.proto.

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

import 'observability.pb.dart' as $0;

export 'observability.pb.dart';

/// HolonObservability emits OTLP-shaped records through the SDK-managed
/// observability service. Events are LogRecord values with event_name set.
///
/// Canonical event_name values for this wave:
///   instance.spawned
///   instance.ready
///   instance.exited
///   instance.crashed
///   session.started
///   session.ended
///   handler.panic
///   config.reloaded
@$pb.GrpcServiceName('holons.v1.HolonObservability')
class HolonObservabilityClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  HolonObservabilityClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseStream<$0.LogRecord> logs(
    $0.LogsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$logs, $async.Stream.fromIterable([request]),
        options: options);
  }

  $grpc.ResponseStream<$0.Metric> metrics(
    $0.MetricsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$metrics, $async.Stream.fromIterable([request]),
        options: options);
  }

  $grpc.ResponseStream<$0.LogRecord> events(
    $0.EventsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$events, $async.Stream.fromIterable([request]),
        options: options);
  }

  // method descriptors

  static final _$logs = $grpc.ClientMethod<$0.LogsRequest, $0.LogRecord>(
      '/holons.v1.HolonObservability/Logs',
      ($0.LogsRequest value) => value.writeToBuffer(),
      $0.LogRecord.fromBuffer);
  static final _$metrics = $grpc.ClientMethod<$0.MetricsRequest, $0.Metric>(
      '/holons.v1.HolonObservability/Metrics',
      ($0.MetricsRequest value) => value.writeToBuffer(),
      $0.Metric.fromBuffer);
  static final _$events = $grpc.ClientMethod<$0.EventsRequest, $0.LogRecord>(
      '/holons.v1.HolonObservability/Events',
      ($0.EventsRequest value) => value.writeToBuffer(),
      $0.LogRecord.fromBuffer);
}

@$pb.GrpcServiceName('holons.v1.HolonObservability')
abstract class HolonObservabilityServiceBase extends $grpc.Service {
  $core.String get $name => 'holons.v1.HolonObservability';

  HolonObservabilityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.LogsRequest, $0.LogRecord>(
        'Logs',
        logs_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.LogsRequest.fromBuffer(value),
        ($0.LogRecord value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MetricsRequest, $0.Metric>(
        'Metrics',
        metrics_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.MetricsRequest.fromBuffer(value),
        ($0.Metric value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.EventsRequest, $0.LogRecord>(
        'Events',
        events_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.EventsRequest.fromBuffer(value),
        ($0.LogRecord value) => value.writeToBuffer()));
  }

  $async.Stream<$0.LogRecord> logs_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.LogsRequest> $request) async* {
    yield* logs($call, await $request);
  }

  $async.Stream<$0.LogRecord> logs(
      $grpc.ServiceCall call, $0.LogsRequest request);

  $async.Stream<$0.Metric> metrics_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MetricsRequest> $request) async* {
    yield* metrics($call, await $request);
  }

  $async.Stream<$0.Metric> metrics(
      $grpc.ServiceCall call, $0.MetricsRequest request);

  $async.Stream<$0.LogRecord> events_Pre($grpc.ServiceCall $call,
      $async.Future<$0.EventsRequest> $request) async* {
    yield* events($call, await $request);
  }

  $async.Stream<$0.LogRecord> events(
      $grpc.ServiceCall call, $0.EventsRequest request);
}
