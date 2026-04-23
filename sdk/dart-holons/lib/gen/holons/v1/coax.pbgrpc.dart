// This is a generated file - do not edit.
//
// Generated from holons/v1/coax.proto.

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

import 'coax.pb.dart' as $0;

export 'coax.pb.dart';

/// CoaxService is the COAX (coaccessibility) interaction surface for any
/// organism — a composite holon that assembles member holons.
///
/// It enables programmatic discovery, connection, and interaction with
/// the organism's members: the same operations a human performs through
/// the UI, driven through the same shared state. Agent-initiated actions
/// (selecting a member, invoking a method) must be reflected in the
/// organism's interface in real time — COAX does not bypass the UI,
/// it drives through it.
///
/// This service is recursive: a member that is itself an organism
/// exposes its own CoaxService at its own level.
///
/// See CONSTITUTION.md Article 1 for the COAX principle.
/// See apps_kits/DESIGN.md for how App Kits implement this surface.
@$pb.GrpcServiceName('holons.v1.CoaxService')
class CoaxServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  CoaxServiceClient(super.channel, {super.options, super.interceptors});

  /// List the organism's available member holons.
  /// Equivalent to browsing the holon picker in the UI.
  /// The organism controls which members are listed — internal holons
  /// may be intentionally omitted to keep the exposure surface minimal.
  /// Organism Kits provide built-in exposure strategies (all, filtered,
  /// or none) — the organism picks one, no custom filtering needed.
  /// @example {}
  $grpc.ResponseFuture<$0.ListMembersResponse> listMembers(
    $0.ListMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMembers, request, options: options);
  }

  /// Query the runtime status of a specific member.
  /// @example {"slug":"gabriel-greeting-go"}
  $grpc.ResponseFuture<$0.MemberStatusResponse> memberStatus(
    $0.MemberStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$memberStatus, request, options: options);
  }

  /// Connect a member holon (start it if needed).
  /// The organism resolves the member, launches its process if necessary,
  /// and establishes a gRPC channel — identical to a user selecting a
  /// holon in a picker.
  /// @example {"slug":"gabriel-greeting-go","transport":"tcp"}
  $grpc.ResponseFuture<$0.ConnectMemberResponse> connectMember(
    $0.ConnectMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$connectMember, request, options: options);
  }

  /// Disconnect a member holon.
  /// @example {"slug":"gabriel-greeting-go"}
  $grpc.ResponseFuture<$0.DisconnectMemberResponse> disconnectMember(
    $0.DisconnectMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$disconnectMember, request, options: options);
  }

  /// Forward a command to a member holon by slug.
  /// The organism resolves, connects if needed, and relays the call.
  ///
  /// This is the generalized equivalent of AppleScript's
  ///   tell application "X" to do Y
  /// but universal (proto-based), typed, and platform-independent.
  ///
  /// Tell operates at the organism level: it is the single entry point
  /// for an external caller to interact with any member without needing
  /// to discover and connect to each one separately.
  /// @example {"member_slug":"gabriel-greeting-go","method":"greeting.v1.GreetingService/SayHello","payload":"eyJuYW1lIjoiQm9iIiwibGFuZ19jb2RlIjoiZnIifQ=="}
  $grpc.ResponseFuture<$0.TellResponse> tell(
    $0.TellRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$tell, request, options: options);
  }

  /// Shut down the COAX server gracefully.
  /// The response is sent before the server stops accepting new calls.
  ///
  /// TurnOn does not exist — it is impossible by design: one cannot
  /// call an RPC on a server that is not running. The COAX server is
  /// started by the organism itself (UI toggle, launch argument, or
  /// startup configuration).
  /// @example {}
  $grpc.ResponseFuture<$0.TurnOffCoaxResponse> turnOffCoax(
    $0.TurnOffCoaxRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$turnOffCoax, request, options: options);
  }

  // method descriptors

  static final _$listMembers =
      $grpc.ClientMethod<$0.ListMembersRequest, $0.ListMembersResponse>(
          '/holons.v1.CoaxService/ListMembers',
          ($0.ListMembersRequest value) => value.writeToBuffer(),
          $0.ListMembersResponse.fromBuffer);
  static final _$memberStatus =
      $grpc.ClientMethod<$0.MemberStatusRequest, $0.MemberStatusResponse>(
          '/holons.v1.CoaxService/MemberStatus',
          ($0.MemberStatusRequest value) => value.writeToBuffer(),
          $0.MemberStatusResponse.fromBuffer);
  static final _$connectMember =
      $grpc.ClientMethod<$0.ConnectMemberRequest, $0.ConnectMemberResponse>(
          '/holons.v1.CoaxService/ConnectMember',
          ($0.ConnectMemberRequest value) => value.writeToBuffer(),
          $0.ConnectMemberResponse.fromBuffer);
  static final _$disconnectMember = $grpc.ClientMethod<
          $0.DisconnectMemberRequest, $0.DisconnectMemberResponse>(
      '/holons.v1.CoaxService/DisconnectMember',
      ($0.DisconnectMemberRequest value) => value.writeToBuffer(),
      $0.DisconnectMemberResponse.fromBuffer);
  static final _$tell = $grpc.ClientMethod<$0.TellRequest, $0.TellResponse>(
      '/holons.v1.CoaxService/Tell',
      ($0.TellRequest value) => value.writeToBuffer(),
      $0.TellResponse.fromBuffer);
  static final _$turnOffCoax =
      $grpc.ClientMethod<$0.TurnOffCoaxRequest, $0.TurnOffCoaxResponse>(
          '/holons.v1.CoaxService/TurnOffCoax',
          ($0.TurnOffCoaxRequest value) => value.writeToBuffer(),
          $0.TurnOffCoaxResponse.fromBuffer);
}

@$pb.GrpcServiceName('holons.v1.CoaxService')
abstract class CoaxServiceBase extends $grpc.Service {
  $core.String get $name => 'holons.v1.CoaxService';

  CoaxServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.ListMembersRequest, $0.ListMembersResponse>(
            'ListMembers',
            listMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListMembersRequest.fromBuffer(value),
            ($0.ListMembersResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.MemberStatusRequest, $0.MemberStatusResponse>(
            'MemberStatus',
            memberStatus_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.MemberStatusRequest.fromBuffer(value),
            ($0.MemberStatusResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ConnectMemberRequest, $0.ConnectMemberResponse>(
            'ConnectMember',
            connectMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ConnectMemberRequest.fromBuffer(value),
            ($0.ConnectMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DisconnectMemberRequest,
            $0.DisconnectMemberResponse>(
        'DisconnectMember',
        disconnectMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DisconnectMemberRequest.fromBuffer(value),
        ($0.DisconnectMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TellRequest, $0.TellResponse>(
        'Tell',
        tell_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.TellRequest.fromBuffer(value),
        ($0.TellResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.TurnOffCoaxRequest, $0.TurnOffCoaxResponse>(
            'TurnOffCoax',
            turnOffCoax_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.TurnOffCoaxRequest.fromBuffer(value),
            ($0.TurnOffCoaxResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ListMembersResponse> listMembers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListMembersRequest> $request) async {
    return listMembers($call, await $request);
  }

  $async.Future<$0.ListMembersResponse> listMembers(
      $grpc.ServiceCall call, $0.ListMembersRequest request);

  $async.Future<$0.MemberStatusResponse> memberStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.MemberStatusRequest> $request) async {
    return memberStatus($call, await $request);
  }

  $async.Future<$0.MemberStatusResponse> memberStatus(
      $grpc.ServiceCall call, $0.MemberStatusRequest request);

  $async.Future<$0.ConnectMemberResponse> connectMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ConnectMemberRequest> $request) async {
    return connectMember($call, await $request);
  }

  $async.Future<$0.ConnectMemberResponse> connectMember(
      $grpc.ServiceCall call, $0.ConnectMemberRequest request);

  $async.Future<$0.DisconnectMemberResponse> disconnectMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DisconnectMemberRequest> $request) async {
    return disconnectMember($call, await $request);
  }

  $async.Future<$0.DisconnectMemberResponse> disconnectMember(
      $grpc.ServiceCall call, $0.DisconnectMemberRequest request);

  $async.Future<$0.TellResponse> tell_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.TellRequest> $request) async {
    return tell($call, await $request);
  }

  $async.Future<$0.TellResponse> tell(
      $grpc.ServiceCall call, $0.TellRequest request);

  $async.Future<$0.TurnOffCoaxResponse> turnOffCoax_Pre($grpc.ServiceCall $call,
      $async.Future<$0.TurnOffCoaxRequest> $request) async {
    return turnOffCoax($call, await $request);
  }

  $async.Future<$0.TurnOffCoaxResponse> turnOffCoax(
      $grpc.ServiceCall call, $0.TurnOffCoaxRequest request);
}
