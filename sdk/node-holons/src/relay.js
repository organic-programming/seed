'use strict';

const path = require('node:path');

const observability = require('./observability');
const relayPb = require('./gen/relay/v1/relay_pb');
const relayGrpc = require('./gen/relay/v1/relay_grpc_pb');

class RelayService {
    constructor(options = {}) {
        this.downstream = options.downstream || options.downstreamConn || null;
        this.received = 0;
    }

    async tick(call, callback) {
        try {
            const req = call.request || new relayPb.TickRequest();
            this.received += 1;
            const count = this.received;
            const obs = observability.current();
            const slug = responderSlug(obs);
            const uid = obs.cfg.instanceUid || '';
            obs.logger('tick').info('tick received', {
                sender: req.getSender(),
                note: req.getNote(),
                responder_slug: slug,
                responder_uid: uid,
            });
            const counter = obs.counter(
                'cascade_ticks_total',
                'Ticks received by this cascade node.',
                { responder_uid: uid },
            );
            if (counter) counter.inc();

            const out = new relayPb.TickResponse();
            out.setResponderSlug(slug);
            out.setResponderInstanceUid(uid);
            const downstream = relayClientFor(this.downstream);
            if (downstream) {
                const resp = await unary(downstream.tick.bind(downstream), req, 5000);
                for (const hop of resp.getHopsList()) out.addHops(hop);
            }
            const hop = new relayPb.HopReceipt();
            hop.setSlug(slug);
            hop.setUid(uid);
            hop.setReceived(count);
            out.addHops(hop);
            callback(null, out);
        } catch (err) {
            callback(err);
        }
    }
}

function registerServer(server, options = {}) {
    server.addService(relayGrpc.RelayServiceService, new RelayService(options));
}

function relayClientFor(raw) {
    if (!raw) return null;
    if (raw.relayClient) return raw.relayClient;
    if (typeof raw.tick === 'function') return raw;
    const target = raw.target || raw.clientTarget || raw.address || raw;
    if (!target || typeof target !== 'string') return null;
    return new relayGrpc.RelayServiceClient(target, require('@grpc/grpc-js').credentials.createInsecure());
}

function responderSlug(obs) {
    const configured = String(obs && obs.cfg ? obs.cfg.slug || '' : '').trim();
    if (configured) return configured;
    return path.basename(process.argv[1] || '').replace(/\.exe$/, '');
}

function unary(method, request, timeoutMs) {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('timeout')), timeoutMs);
        method(request, (err, out) => {
            clearTimeout(timer);
            if (err) reject(err);
            else resolve(out || {});
        });
    });
}

module.exports = {
    RelayService,
    registerServer,
    RegisterServer: registerServer,
    relayPb,
    relayGrpc,
};
