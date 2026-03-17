'use strict';

const path = require('node:path');
const protoLoader = require('@grpc/proto-loader');
const { serve } = require('@organic-programming/holons');

const publicApi = require('../api/public');
const grpcPb = require('../gen/node/greeting/v1/greeting_grpc_pb.js');

const ROOT = path.resolve(__dirname, '..');
const SHARED_PROTO = path.join(ROOT, '..', '..', '_protos', 'v1', 'greeting.proto');
const DOMAIN_PROTO_ROOT = path.join(ROOT, '..', '..', '_protos');

const reflectionPackageDefinition = protoLoader.loadSync(SHARED_PROTO, {
  includeDirs: [DOMAIN_PROTO_ROOT],
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

class GreetingService {
  listLanguages(call, callback) {
    callback(null, publicApi.listLanguages(call.request));
  }

  sayHello(call, callback) {
    callback(null, publicApi.sayHello(call.request));
  }
}

async function listenAndServe(listenUri) {
  return serve.runWithOptions(listenUri, (server) => {
    server.addService(grpcPb.GreetingServiceService, new GreetingService());
  }, {
    reflect: true,
    reflectionPackageDefinition,
    logger: console,
  });
}

module.exports = {
  GreetingService,
  listenAndServe,
};
