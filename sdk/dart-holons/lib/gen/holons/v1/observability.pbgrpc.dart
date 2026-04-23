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

/// HolonObservability is auto-registered by the SDK's serve runner
/// when OP_OBS is set. Provides structured logs, metrics snapshots,
/// and lifecycle events. See OBSERVABILITY.md.
@$pb.GrpcServiceName('holons.v1.HolonObservability')
class HolonObservabilityClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  HolonObservabilityClient(super.channel, {super.options, super.interceptors});

  /// Logs streams log entries. If follow=true, the stream stays open
  /// and emits new entries as they arrive. If follow=false, drains
  /// the current ring buffer and ends.
  $grpc.ResponseStream<$0.LogEntry> logs(
    $0.LogsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$logs, $async.Stream.fromIterable([request]),
        options: options);
  }

  /// Metrics returns a point-in-time snapshot of all current metrics.
  /// Unary, not streaming — scraping cadence is the caller's concern.
  $grpc.ResponseFuture<$0.MetricsSnapshot> metrics(
    $0.MetricsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$metrics, request, options: options);
  }

  /// Events streams lifecycle events. If follow=true, stays open.
  $grpc.ResponseStream<$0.EventInfo> events(
    $0.EventsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$events, $async.Stream.fromIterable([request]),
        options: options);
  }

  // method descriptors

  static final _$logs = $grpc.ClientMethod<$0.LogsRequest, $0.LogEntry>(
      '/holons.v1.HolonObservability/Logs',
      ($0.LogsRequest value) => value.writeToBuffer(),
      $0.LogEntry.fromBuffer);
  static final _$metrics =
      $grpc.ClientMethod<$0.MetricsRequest, $0.MetricsSnapshot>(
          '/holons.v1.HolonObservability/Metrics',
          ($0.MetricsRequest value) => value.writeToBuffer(),
          $0.MetricsSnapshot.fromBuffer);
  static final _$events = $grpc.ClientMethod<$0.EventsRequest, $0.EventInfo>(
      '/holons.v1.HolonObservability/Events',
      ($0.EventsRequest value) => value.writeToBuffer(),
      $0.EventInfo.fromBuffer);
}

@$pb.GrpcServiceName('holons.v1.HolonObservability')
abstract class HolonObservabilityServiceBase extends $grpc.Service {
  $core.String get $name => 'holons.v1.HolonObservability';

  HolonObservabilityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.LogsRequest, $0.LogEntry>(
        'Logs',
        logs_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.LogsRequest.fromBuffer(value),
        ($0.LogEntry value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MetricsRequest, $0.MetricsSnapshot>(
        'Metrics',
        metrics_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.MetricsRequest.fromBuffer(value),
        ($0.MetricsSnapshot value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.EventsRequest, $0.EventInfo>(
        'Events',
        events_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.EventsRequest.fromBuffer(value),
        ($0.EventInfo value) => value.writeToBuffer()));
  }

  $async.Stream<$0.LogEntry> logs_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.LogsRequest> $request) async* {
    yield* logs($call, await $request);
  }

  $async.Stream<$0.LogEntry> logs(
      $grpc.ServiceCall call, $0.LogsRequest request);

  $async.Future<$0.MetricsSnapshot> metrics_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MetricsRequest> $request) async {
    return metrics($call, await $request);
  }

  $async.Future<$0.MetricsSnapshot> metrics(
      $grpc.ServiceCall call, $0.MetricsRequest request);

  $async.Stream<$0.EventInfo> events_Pre($grpc.ServiceCall $call,
      $async.Future<$0.EventsRequest> $request) async* {
    yield* events($call, await $request);
  }

  $async.Stream<$0.EventInfo> events(
      $grpc.ServiceCall call, $0.EventsRequest request);
}
