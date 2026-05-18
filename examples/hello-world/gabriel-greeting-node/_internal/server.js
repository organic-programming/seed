'use strict';

const { describe, observability, serve } = require('@organic-programming/holons');

const publicApi = require('../api/public');
const describeGenerated = require('../gen/describe_generated');
const grpcPb = require('../gen/node/greeting/v1/greeting_grpc_pb.js');
const { lookup } = require('./greetings');

class GreetingService {
  listLanguages(call, callback) {
    callback(null, publicApi.listLanguages(call.request));
  }

  sayHello(call, callback) {
    const start = process.hrtime.bigint();
    const response = publicApi.sayHello(call.request);
    const name = (call.request.getName() || '').trim() || lookup(response.getLangCode()).defaultName;
    // Node Serve does not yet expose a handler-visible current transport.
    const transport = 'unknown';
    const durationNs = Number(process.hrtime.bigint() - start);
    const message = `Greeted ${name} in ${response.getLanguage()} (${response.getLangCode()})`;
    const obs = observability.current();
    obs.logger('greeting').info(message, {
      lang_code: response.getLangCode(),
      language: response.getLanguage(),
      name,
      greeting: response.getGreeting(),
      transport,
      duration_ns: durationNs,
    });
    const counter = obs.counter(
      'greeting_emitted_total',
      'Greetings emitted, partitioned by language and transport.',
      {
        lang_code: response.getLangCode(),
        language: response.getLanguage(),
        transport,
      },
    );
    if (counter) counter.inc();
    callback(null, response);
  }
}

async function listenAndServe(listenUri, reflect = false) {
  describe.useStaticResponse(describeGenerated.staticDescribeResponse());
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
