'use strict';

const { serve } = require('@organic-programming/holons');

const publicApi = require('../api/public');
const grpcPb = require('../gen/node/greeting/v1/greeting_grpc_pb.js');

class GreetingService {
  listLanguages(call, callback) {
    callback(null, publicApi.listLanguages(call.request));
  }

  sayHello(call, callback) {
    callback(null, publicApi.sayHello(call.request));
  }
}

async function listenAndServe(listenUri, reflect = false) {
  return serve.runWithOptions(listenUri, (server) => {
    server.addService(grpcPb.GreetingServiceService, new GreetingService());
  }, {
    reflect,
    logger: console,
  });
}

module.exports = {
  GreetingService,
  listenAndServe,
};
