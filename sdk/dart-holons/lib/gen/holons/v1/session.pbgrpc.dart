// This is a generated file - do not edit.
//
// Generated from holons/v1/session.proto.

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

import 'session.pb.dart' as $0;

export 'session.pb.dart';

/// HolonSession is auto-registered by the SDK's serve runner
/// when OP_SESSIONS is enabled. Provides session introspection.
/// See SESSIONS.md for the full specification.
@$pb.GrpcServiceName('holons.v1.HolonSession')
class HolonSessionClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  HolonSessionClient(super.channel, {super.options, super.interceptors});

  /// Sessions returns active and optionally past sessions.
  $grpc.ResponseFuture<$0.SessionsResponse> sessions(
    $0.SessionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sessions, request, options: options);
  }

  /// WatchSessions streams session lifecycle events as they happen.
  /// Used by `op sessions --watch` and by tooling that needs live
  /// visibility without polling.
  $grpc.ResponseStream<$0.SessionEvent> watchSessions(
    $0.WatchSessionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$watchSessions, $async.Stream.fromIterable([request]),
        options: options);
  }

  // method descriptors

  static final _$sessions =
      $grpc.ClientMethod<$0.SessionsRequest, $0.SessionsResponse>(
          '/holons.v1.HolonSession/Sessions',
          ($0.SessionsRequest value) => value.writeToBuffer(),
          $0.SessionsResponse.fromBuffer);
  static final _$watchSessions =
      $grpc.ClientMethod<$0.WatchSessionsRequest, $0.SessionEvent>(
          '/holons.v1.HolonSession/WatchSessions',
          ($0.WatchSessionsRequest value) => value.writeToBuffer(),
          $0.SessionEvent.fromBuffer);
}

@$pb.GrpcServiceName('holons.v1.HolonSession')
abstract class HolonSessionServiceBase extends $grpc.Service {
  $core.String get $name => 'holons.v1.HolonSession';

  HolonSessionServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.SessionsRequest, $0.SessionsResponse>(
        'Sessions',
        sessions_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.SessionsRequest.fromBuffer(value),
        ($0.SessionsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.WatchSessionsRequest, $0.SessionEvent>(
        'WatchSessions',
        watchSessions_Pre,
        false,
        true,
        ($core.List<$core.int> value) =>
            $0.WatchSessionsRequest.fromBuffer(value),
        ($0.SessionEvent value) => value.writeToBuffer()));
  }

  $async.Future<$0.SessionsResponse> sessions_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SessionsRequest> $request) async {
    return sessions($call, await $request);
  }

  $async.Future<$0.SessionsResponse> sessions(
      $grpc.ServiceCall call, $0.SessionsRequest request);

  $async.Stream<$0.SessionEvent> watchSessions_Pre($grpc.ServiceCall $call,
      $async.Future<$0.WatchSessionsRequest> $request) async* {
    yield* watchSessions($call, await $request);
  }

  $async.Stream<$0.SessionEvent> watchSessions(
      $grpc.ServiceCall call, $0.WatchSessionsRequest request);
}
