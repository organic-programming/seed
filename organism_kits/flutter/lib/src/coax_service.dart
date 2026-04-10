import 'dart:async';
import 'dart:convert';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/coax.pbgrpc.dart';

import 'coax_controller.dart';

abstract interface class HolonManager {
  Future<List<MemberInfo>> listMembers();
  Future<MemberInfo?> memberStatus(String slug);
  Future<MemberInfo> connectMember(String slug, {String transport = ''});
  Future<void> disconnectMember(String slug);
  Future<Object?> tellMember({
    required String slug,
    required String method,
    Object? payloadJson,
  });
}

class CoaxRpcService extends CoaxServiceBase {
  CoaxRpcService({
    HolonManager? holonManager,
    CoaxManager? coaxManager,
    @Deprecated('Use holonManager') OrganismController? organismController,
    @Deprecated('Use coaxManager') CoaxController? coaxController,
  }) : _holonManager = holonManager ?? organismController!,
       _coaxManager = coaxManager ?? coaxController!;

  final HolonManager _holonManager;
  final CoaxManager _coaxManager;

  @override
  Future<ListMembersResponse> listMembers(
    ServiceCall call,
    ListMembersRequest request,
  ) async {
    return ListMembersResponse(
      members: await _holonManager.listMembers(),
    );
  }

  @override
  Future<MemberStatusResponse> memberStatus(
    ServiceCall call,
    MemberStatusRequest request,
  ) async {
    final response = MemberStatusResponse();
    final member = await _holonManager.memberStatus(request.slug);
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
      member: await _holonManager.connectMember(
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
    await _holonManager.disconnectMember(request.slug);
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
      final responsePayload = await _holonManager.tellMember(
        slug: request.memberSlug,
        method: request.method,
        payloadJson: _decodePayloadJson(request.payload),
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
    unawaited(_coaxManager.turnOffAfterRpc());
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

@Deprecated('Use HolonManager')
typedef OrganismController = HolonManager;
