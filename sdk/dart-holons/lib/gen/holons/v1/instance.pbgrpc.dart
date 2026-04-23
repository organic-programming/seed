// This is a generated file - do not edit.
//
// Generated from holons/v1/instance.proto.

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

import 'instance.pb.dart' as $0;

export 'instance.pb.dart';

/// HolonInstance is auto-registered by the parent supervisor
/// (typically `op`). It is NOT registered by individual holons —
/// listing instances is a supervisor concern. See INSTANCES.md.
@$pb.GrpcServiceName('holons.v1.HolonInstance')
class HolonInstanceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  HolonInstanceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.ListInstancesResponse> list(
    $0.ListInstancesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$list, request, options: options);
  }

  $grpc.ResponseFuture<$0.InstanceInfo> get(
    $0.GetInstanceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$get, request, options: options);
  }

  // method descriptors

  static final _$list =
      $grpc.ClientMethod<$0.ListInstancesRequest, $0.ListInstancesResponse>(
          '/holons.v1.HolonInstance/List',
          ($0.ListInstancesRequest value) => value.writeToBuffer(),
          $0.ListInstancesResponse.fromBuffer);
  static final _$get =
      $grpc.ClientMethod<$0.GetInstanceRequest, $0.InstanceInfo>(
          '/holons.v1.HolonInstance/Get',
          ($0.GetInstanceRequest value) => value.writeToBuffer(),
          $0.InstanceInfo.fromBuffer);
}

@$pb.GrpcServiceName('holons.v1.HolonInstance')
abstract class HolonInstanceServiceBase extends $grpc.Service {
  $core.String get $name => 'holons.v1.HolonInstance';

  HolonInstanceServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.ListInstancesRequest, $0.ListInstancesResponse>(
            'List',
            list_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListInstancesRequest.fromBuffer(value),
            ($0.ListInstancesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetInstanceRequest, $0.InstanceInfo>(
        'Get',
        get_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetInstanceRequest.fromBuffer(value),
        ($0.InstanceInfo value) => value.writeToBuffer()));
  }

  $async.Future<$0.ListInstancesResponse> list_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListInstancesRequest> $request) async {
    return list($call, await $request);
  }

  $async.Future<$0.ListInstancesResponse> list(
      $grpc.ServiceCall call, $0.ListInstancesRequest request);

  $async.Future<$0.InstanceInfo> get_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetInstanceRequest> $request) async {
    return get($call, await $request);
  }

  $async.Future<$0.InstanceInfo> get(
      $grpc.ServiceCall call, $0.GetInstanceRequest request);
}
