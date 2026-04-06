import 'dart:async';

import 'package:grpc/grpc.dart';
import 'package:holons/gen/holons/v1/coax.pbgrpc.dart';
import 'package:holons/gen/holons/v1/manifest.pb.dart';

import '../controller/coax_controller.dart';
import '../controller/greeting_controller.dart';
import '../model/app_model.dart';

class CoaxRpcService extends CoaxServiceBase {
  CoaxRpcService({
    required GreetingController greetingController,
    required CoaxController coaxController,
  }) : _greetingController = greetingController,
       _coaxController = coaxController;

  final GreetingController _greetingController;
  final CoaxController _coaxController;

  @override
  Future<ListMembersResponse> listMembers(
    ServiceCall call,
    ListMembersRequest request,
  ) async {
    return ListMembersResponse(
      members: _greetingController.availableHolons
          .map(_memberForIdentity)
          .toList(growable: false),
    );
  }

  @override
  Future<MemberStatusResponse> memberStatus(
    ServiceCall call,
    MemberStatusRequest request,
  ) async {
    final response = MemberStatusResponse();
    final identity = _greetingController.availableHolons.firstWhere(
      (item) => item.slug == request.slug,
      orElse: () => const GabrielHolonIdentity(
        slug: '',
        familyName: '',
        binaryName: '',
        buildRunner: '',
        displayName: '',
        sortRank: 0,
        holonUuid: '',
        born: '',
        sourceKind: '',
        discoveryPath: '',
        hasSource: false,
      ),
    );
    if (identity.slug.isNotEmpty) {
      response.member = _memberForIdentity(identity);
    }
    return response;
  }

  @override
  Future<ConnectMemberResponse> connectMember(
    ServiceCall call,
    ConnectMemberRequest request,
  ) async {
    final identity = _greetingController.availableHolons.firstWhere(
      (item) => item.slug == request.slug,
      orElse: () =>
          throw GrpcError.notFound("Member '${request.slug}' not found"),
    );
    if (request.transport.trim().isNotEmpty) {
      await _greetingController.setTransport(request.transport, reload: false);
    }
    await _greetingController.selectHolonBySlug(identity.slug, reload: false);
    await _greetingController.loadLanguages(greetAfterLoad: false);
    if (!_greetingController.isRunning || _greetingController.error != null) {
      return ConnectMemberResponse(
        member: _memberForIdentity(
          identity,
          overrideState: MemberState.MEMBER_STATE_ERROR,
        ),
      );
    }
    return ConnectMemberResponse(
      member: _memberForIdentity(
        identity,
        overrideState: _greetingController.isRunning
            ? MemberState.MEMBER_STATE_CONNECTED
            : MemberState.MEMBER_STATE_ERROR,
      ),
    );
  }

  @override
  Future<DisconnectMemberResponse> disconnectMember(
    ServiceCall call,
    DisconnectMemberRequest request,
  ) async {
    await _greetingController.stop();
    return DisconnectMemberResponse();
  }

  @override
  Future<TellResponse> tell(ServiceCall call, TellRequest request) async {
    throw GrpcError.unimplemented('Tell is not yet implemented');
  }

  @override
  Future<TurnOffCoaxResponse> turnOffCoax(
    ServiceCall call,
    TurnOffCoaxRequest request,
  ) async {
    unawaited(_coaxController.disableAfterRpc());
    return TurnOffCoaxResponse();
  }

  MemberInfo _memberForIdentity(
    GabrielHolonIdentity identity, {
    MemberState? overrideState,
  }) {
    return MemberInfo(
      slug: identity.slug,
      identity: HolonManifest_Identity(
        familyName: identity.familyName,
        givenName: identity.displayName,
      ),
      state: overrideState ?? _memberStateFor(identity),
      isOrganism: false,
    );
  }

  MemberState _memberStateFor(GabrielHolonIdentity identity) {
    if (_greetingController.selectedHolon?.slug == identity.slug &&
        _greetingController.isRunning) {
      return MemberState.MEMBER_STATE_CONNECTED;
    }
    return MemberState.MEMBER_STATE_AVAILABLE;
  }
}
