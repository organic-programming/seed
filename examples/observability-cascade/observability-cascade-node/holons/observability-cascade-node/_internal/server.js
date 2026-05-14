'use strict';

const { describe, serve } = require('@organic-programming/holons');

const publicApi = require('../api/public');
const describeGenerated = require('../gen/describe_generated');
const grpcPb = require('../gen/node/relay/v1/relay_grpc_pb.js');

class RelayService {
  tick(call, callback) {
    callback(null, publicApi.tick(call.request));
  }
}

async function listenAndServe(listenUri, reflect = false, members = []) {
  describe.useStaticResponse(describeGenerated.staticDescribeResponse());
  return serve.runWithOptions(normalizeListenUri(listenUri), (server) => {
    server.addService(grpcPb.RelayServiceService, new RelayService());
  }, {
    reflect,
    logger: console,
    memberEndpoints: members,
    slug: 'observability-cascade-node-node',
  });
}

function normalizeListenUri(listenUri) {
  const match = String(listenUri || '').match(/^tcp:\/\/:(\d+)$/);
  if (match) return `tcp://0.0.0.0:${match[1]}`;
  return listenUri;
}

module.exports = {
  RelayService,
  listenAndServe,
  normalizeListenUri,
};
