import 'dart:async';
import 'dart:convert';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/coax.pbgrpc.dart';

import 'coax_controller.dart';

abstract interface class OrganismController {
  Future<List<MemberInfo>> listMembers();
  Future<MemberInfo?> memberStatus(String slug);
  Future<MemberInfo> connectMember(String slug, {String transport = ''});
  Future<void> disconnectMember(String slug);
  Future<Object?> tellMember({
    required String memberSlug,
    required String method,
    Object? payload,
  });
}

class CoaxRpcService extends CoaxServiceBase {
  CoaxRpcService({
    required OrganismController organismController,
    required CoaxController coaxController,
  }) : _organismController = organismController,
       _coaxController = coaxController;

  final OrganismController _organismController;
  final CoaxController _coaxController;

  @override
  Future<ListMembersResponse> listMembers(
    ServiceCall call,
    ListMembersRequest request,
  ) async {
    return ListMembersResponse(
      members: await _organismController.listMembers(),
    );
  }

  @override
  Future<MemberStatusResponse> memberStatus(
    ServiceCall call,
    MemberStatusRequest request,
  ) async {
    final response = MemberStatusResponse();
    final member = await _organismController.memberStatus(request.slug);
    if (member != null) {
      response.member = member;
    }
    return response;
  }

  @override
  Future<ConnectMemberResponse> connectMember(
    ServiceCall call,
    ConnectMemberRequest request,
  ) async {
    return ConnectMemberResponse(
      member: await _organismController.connectMember(
        request.slug,
        transport: request.transport,
      ),
    );
  }

  @override
  Future<DisconnectMemberResponse> disconnectMember(
    ServiceCall call,
    DisconnectMemberRequest request,
  ) async {
    await _organismController.disconnectMember(request.slug);
    return DisconnectMemberResponse();
  }

  @override
  Future<TellResponse> tell(ServiceCall call, TellRequest request) async {
    if (request.memberSlug.trim().isEmpty) {
      throw GrpcError.invalidArgument('member_slug is required');
    }
    if (request.method.trim().isEmpty) {
      throw GrpcError.invalidArgument('method is required');
    }

    try {
      final responsePayload = await _organismController.tellMember(
        memberSlug: request.memberSlug,
        method: request.method,
        payload: _decodePayloadJson(request.payload),
      );
      final encodedPayload = responsePayload == null
          ? const <String, Object?>{}
          : responsePayload;
      return TellResponse(
        payload: utf8.encode(jsonEncode(encodedPayload)),
      );
    } on GrpcError {
      rethrow;
    } on ArgumentError catch (error) {
      throw GrpcError.invalidArgument(error.message ?? error.toString());
    } on StateError catch (error) {
      throw GrpcError.unavailable(error.message.toString());
    } on Object catch (error) {
      throw GrpcError.unavailable(error.toString());
    }
  }

  @override
  Future<TurnOffCoaxResponse> turnOffCoax(
    ServiceCall call,
    TurnOffCoaxRequest request,
  ) async {
    unawaited(_coaxController.disableAfterRpc());
    return TurnOffCoaxResponse();
  }

  Object? _decodePayloadJson(List<int> payload) {
    if (payload.isEmpty) {
      return const <String, Object?>{};
    }

    try {
      return jsonDecode(utf8.decode(payload));
    } on FormatException catch (error) {
      throw GrpcError.invalidArgument('payload must be valid JSON: $error');
    }
  }
}
