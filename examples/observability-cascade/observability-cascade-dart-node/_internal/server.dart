import 'dart:io' as io;

import 'package:grpc/grpc.dart';
import 'package:holons/holons.dart';

import '../gen/dart/relay/v1/relay.pbgrpc.dart';
import '../gen/describe_generated.dart';

class RelayServer extends RelayServiceBase {
  @override
  Future<TickResponse> tick(ServiceCall call, TickRequest request) async {
    call;
    final obs = current();
    final slug = _responderSlug(obs);
    final uid = obs.cfg.instanceUid;
    obs.logger('tick').info('tick received', {
      'sender': request.sender,
      'note': request.note,
      'responder_slug': slug,
      'responder_uid': uid,
    });
    obs
        .counter(
          'cascade_ticks_total',
          help: 'Ticks received by this cascade node.',
          labels: {'responder_uid': uid},
        )
        ?.inc();
    return TickResponse(responderSlug: slug, responderInstanceUid: uid);
  }
}

Future<void> listenAndServe(
  String listenUri, {
  bool reflect = false,
  List<MemberRef> members = const [],
}) {
  useStaticResponse(staticDescribeResponse());
  return runWithOptions(listenUri, <Service>[
    RelayServer(),
  ], options: ServeOptions(reflect: reflect, memberEndpoints: members));
}

String _responderSlug(Observability obs) {
  final configured = obs.cfg.slug.trim();
  if (configured.isNotEmpty) {
    return configured;
  }
  return io.Platform.resolvedExecutable.split(io.Platform.pathSeparator).last;
}
