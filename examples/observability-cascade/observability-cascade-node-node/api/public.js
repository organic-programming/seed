'use strict';

const { observability } = require('@organic-programming/holons');
const pb = require('../gen/node/relay/v1/relay_pb.js');

function tick(request) {
  const obs = observability.current();
  const slug = responderSlug(obs);
  const uid = obs.cfg.instanceUid || '';
  obs.logger('tick').info('tick received', {
    sender: request.getSender(),
    note: request.getNote(),
    responder_slug: slug,
    responder_uid: uid,
  });
  const counter = obs.counter(
    'cascade_ticks_total',
    'Ticks received by this cascade node.',
    { responder_uid: uid },
  );
  if (counter) counter.inc();
  const response = new pb.TickResponse();
  response.setResponderSlug(slug);
  response.setResponderInstanceUid(uid);
  return response;
}

function responderSlug(obs) {
  const configured = String(obs.cfg.slug || '').trim();
  if (configured) return configured;
  return 'observability-cascade-node-node';
}

module.exports = {
  tick,
  responderSlug,
};

