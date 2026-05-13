// This is a generated file - do not edit.
//
// Generated from relay/v1/relay.proto.

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

import 'relay.pb.dart' as $0;

export 'relay.pb.dart';

@$pb.GrpcServiceName('relay.v1.RelayService')
class RelayServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  RelayServiceClient(super.channel, {super.options, super.interceptors});

  /// Tick: emit one log + increment one metric counter at the receiver.
  /// Used to test cross-holon observability relay: send a Tick to a leaf
  /// holon and verify the log propagates up the MemberEndpoints chain.
  /// Metrics are NOT relayed by the SDK - they are exposed locally and
  /// verified at each node directly.
  $grpc.ResponseFuture<$0.TickResponse> tick(
    $0.TickRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$tick, request, options: options);
  }

  // method descriptors

  static final _$tick = $grpc.ClientMethod<$0.TickRequest, $0.TickResponse>(
      '/relay.v1.RelayService/Tick',
      ($0.TickRequest value) => value.writeToBuffer(),
      $0.TickResponse.fromBuffer);
}

@$pb.GrpcServiceName('relay.v1.RelayService')
abstract class RelayServiceBase extends $grpc.Service {
  $core.String get $name => 'relay.v1.RelayService';

  RelayServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.TickRequest, $0.TickResponse>(
        'Tick',
        tick_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TickRequest.fromBuffer(value),
        ($0.TickResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.TickResponse> tick_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TickRequest> $request) async {
    return tick($call, await $request);
  }

  $async.Future<$0.TickResponse> tick(
      $grpc.ServiceCall call, $0.TickRequest request);
}
