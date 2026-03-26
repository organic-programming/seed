// This is a generated file - do not edit.
//
// Generated from holons/v1/describe.proto.

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

import 'describe.pb.dart' as $0;

export 'describe.pb.dart';

/// HolonMeta is auto-registered by each SDK's serve runner.
/// It provides self-documentation for any running holon.
/// See HOLON_PROTO.md for the manifest schema.
@$pb.GrpcServiceName('holons.v1.HolonMeta')
class HolonMetaClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  HolonMetaClient(super.channel, {super.options, super.interceptors});

  /// Describe returns the holon's full manifest and its API catalog.
  $grpc.ResponseFuture<$0.DescribeResponse> describe(
    $0.DescribeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$describe, request, options: options);
  }

  // method descriptors

  static final _$describe =
      $grpc.ClientMethod<$0.DescribeRequest, $0.DescribeResponse>(
          '/holons.v1.HolonMeta/Describe',
          ($0.DescribeRequest value) => value.writeToBuffer(),
          $0.DescribeResponse.fromBuffer);
}

@$pb.GrpcServiceName('holons.v1.HolonMeta')
abstract class HolonMetaServiceBase extends $grpc.Service {
  $core.String get $name => 'holons.v1.HolonMeta';

  HolonMetaServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.DescribeRequest, $0.DescribeResponse>(
        'Describe',
        describe_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.DescribeRequest.fromBuffer(value),
        ($0.DescribeResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.DescribeResponse> describe_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DescribeRequest> $request) async {
    return describe($call, await $request);
  }

  $async.Future<$0.DescribeResponse> describe(
      $grpc.ServiceCall call, $0.DescribeRequest request);
}
