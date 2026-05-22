import 'dart:async';
import 'dart:io' as io;

import 'package:fixnum/fixnum.dart';
import 'package:grpc/grpc.dart';
import 'package:grpc/service_api.dart' as grpc_api;

import 'gen/relay/v1/relay.pbgrpc.dart';
import 'observability.dart' as observability;

class RelayOptions {
  const RelayOptions({this.downstreamConn});

  final grpc_api.ClientChannel? downstreamConn;
}

Service relayService(RelayOptions options) => _RelayServer(options);

class _RelayServer extends RelayServiceBase {
  _RelayServer(this.options);

  final RelayOptions options;
  var _received = 0;

  @override
  Future<TickResponse> tick(ServiceCall call, TickRequest request) async {
    _received += 1;
    final obs = observability.current();
    final slug = _responderSlug(obs);
    final uid = obs.cfg.instanceUid;
    obs.logger('tick').info('tick received', fields: {
      'sender': request.sender,
      'note': request.note,
      'responder_slug': slug,
      'responder_uid': uid,
    });
    obs.counter(
      'cascade_ticks_total',
      help: 'Ticks received by this cascade node.',
      labels: {'responder_uid': uid},
    )?.inc();

    final hops = <HopReceipt>[];
    final downstream = options.downstreamConn;
    if (downstream != null) {
      final response = await RelayServiceClient(downstream).tick(request);
      hops.addAll(response.hops);
    }
    hops.add(HopReceipt(
      slug: slug,
      uid: uid,
      received: Int64(_received),
    ));
    return TickResponse(
      responderSlug: slug,
      responderInstanceUid: uid,
      hops: hops,
    );
  }
}

String _responderSlug(observability.Observability obs) {
  final configured = obs.cfg.slug.trim();
  if (configured.isNotEmpty) {
    return configured;
  }
  return io.Platform.resolvedExecutable.split(io.Platform.pathSeparator).last;
}
