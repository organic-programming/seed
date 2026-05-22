'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const grpc = require('@grpc/grpc-js');
const { observability } = require('@organic-programming/holons');

const pb = require('../gen/node/greeting/v1/greeting_pb.js');
const grpcPb = require('../gen/node/greeting/v1/greeting_grpc_pb.js');
const { GreetingService, listenAndServe } = require('./server');

function startServer() {
  const server = new grpc.Server();
  server.addService(grpcPb.GreetingServiceService, new GreetingService());

  return new Promise((resolve, reject) => {
    server.bindAsync('127.0.0.1:0', grpc.ServerCredentials.createInsecure(), (error, port) => {
      if (error) {
        reject(error);
        return;
      }
      server.start();
      resolve({
        server,
        client: new grpcPb.GreetingServiceClient(`127.0.0.1:${port}`, grpc.credentials.createInsecure()),
      });
    });
  });
}

function callUnary(client, method, request) {
  return new Promise((resolve, reject) => {
    client[method](request, (error, response) => {
      if (error) {
        reject(error);
        return;
      }
      resolve(response);
    });
  });
}

test('RPC ListLanguages returns all languages', async (t) => {
  const runtime = await startServer();
  t.after(() => runtime.client.close());
  t.after(() => runtime.server.forceShutdown());

  const response = await callUnary(runtime.client, 'listLanguages', new pb.ListLanguagesRequest());

  assert.equal(response.getLanguagesList().length, 56);
});

test('RPC ListLanguages populates required fields', async (t) => {
  const runtime = await startServer();
  t.after(() => runtime.client.close());
  t.after(() => runtime.server.forceShutdown());

  const response = await callUnary(runtime.client, 'listLanguages', new pb.ListLanguagesRequest());

  for (const language of response.getLanguagesList()) {
    assert.ok(language.getCode());
    assert.ok(language.getName());
    assert.ok(language.getNative());
  }
});

test('RPC SayHello uses requested language', async (t) => {
  const runtime = await startServer();
  t.after(() => runtime.client.close());
  t.after(() => runtime.server.forceShutdown());

  const request = new pb.SayHelloRequest();
  request.setName('Bob');
  request.setLangCode('fr');

  const response = await callUnary(runtime.client, 'sayHello', request);

  assert.equal(response.getGreeting(), 'Bonjour Bob');
  assert.equal(response.getLanguage(), 'French');
  assert.equal(response.getLangCode(), 'fr');
});

test('RPC SayHello uses localized default name', async (t) => {
  const runtime = await startServer();
  t.after(() => runtime.client.close());
  t.after(() => runtime.server.forceShutdown());

  const request = new pb.SayHelloRequest();
  request.setLangCode('fr');

  const response = await callUnary(runtime.client, 'sayHello', request);

  assert.equal(response.getGreeting(), 'Bonjour Marie');
  assert.equal(response.getLangCode(), 'fr');
});

test('RPC SayHello falls back to English', async (t) => {
  const runtime = await startServer();
  t.after(() => runtime.client.close());
  t.after(() => runtime.server.forceShutdown());

  const request = new pb.SayHelloRequest();
  request.setName('Bob');
  request.setLangCode('xx');

  const response = await callUnary(runtime.client, 'sayHello', request);

  assert.equal(response.getGreeting(), 'Hello Bob');
  assert.equal(response.getLangCode(), 'en');
});

test('SayHello emits observability signals with stdio transport', async (t) => {
  const oldOpObs = process.env.OP_OBS;
  process.env.OP_OBS = 'logs,metrics';
  observability.reset();
  const obs = observability.configure({ slug: 'gabriel-greeting-node' });
  t.after(() => {
    observability.reset();
    if (oldOpObs === undefined) {
      delete process.env.OP_OBS;
    } else {
      process.env.OP_OBS = oldOpObs;
    }
  });

  const server = await listenAndServe('stdio://');
  t.after(async () => {
    if (server.__holonsRuntime) await server.stopHolon();
  });

  const request = new pb.SayHelloRequest();
  request.setName(' Bob ');
  request.setLangCode('en');

  const response = await new Promise((resolve, reject) => {
    new GreetingService().sayHello({ request }, (error, out) => {
      if (error) {
        reject(error);
        return;
      }
      resolve(out);
    });
  });

  assert.equal(response.getGreeting(), 'Hello Bob');
  const snapshot = obs.registry.snapshot();
  const counter = snapshot.counters.find((sample) => sample.name === 'greeting_emitted_total');
  assert.ok(counter);
  assert.deepEqual(counter.labels, {
    lang_code: 'en',
    language: 'English',
    transport: 'stdio',
  });
  assert.equal(counter.value, 1);

  const entry = obs.logRing.drain().find((logEntry) => observability.bodyString(logEntry) === 'Greeted Bob in English (en)');
  assert.ok(entry);
  const attrs = ['duration_ns', 'greeting', 'lang_code', 'language', 'name', 'transport']
    .map((key) => entry.attributes.find((attr) => attr.key === key));
  assert.equal(attrs.every(Boolean), true);
  assert.deepEqual(attrs.map((attr) => attr.key).sort(), [
    'duration_ns',
    'greeting',
    'lang_code',
    'language',
    'name',
    'transport',
  ]);
  assert.deepEqual(attr(entry, 'holons.slug').value, { string_value: 'gabriel-greeting-node' });
  assert.deepEqual(attr(entry, 'service.name').value, { string_value: 'gabriel-greeting-node' });
  assert.ok(attr(entry, 'holons.session_id').value.string_value);
  assert.deepEqual(attr(entry, 'lang_code').value, { string_value: 'en' });
  assert.deepEqual(attr(entry, 'language').value, { string_value: 'English' });
  assert.deepEqual(attr(entry, 'name').value, { string_value: 'Bob' });
  assert.deepEqual(attr(entry, 'greeting').value, { string_value: 'Hello Bob' });
  assert.deepEqual(attr(entry, 'transport').value, { string_value: 'stdio' });
  assert.equal(Object.prototype.hasOwnProperty.call(attr(entry, 'duration_ns').value, 'int_value'), true);
  assert.ok(Number(attr(entry, 'duration_ns').value.int_value) >= 0);
});

function attr(record, key) {
  const found = (record.attributes || []).find((item) => item.key === key);
  assert.ok(found, `missing attribute ${key}`);
  return found;
}
