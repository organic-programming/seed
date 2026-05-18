'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const grpc = require('@grpc/grpc-js');
const { observability } = require('@organic-programming/holons');

const pb = require('../gen/node/greeting/v1/greeting_pb.js');
const grpcPb = require('../gen/node/greeting/v1/greeting_grpc_pb.js');
const { GreetingService } = require('./server');

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

test('RPC SayHello emits observability signals', async (t) => {
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

  const runtime = await startServer();
  t.after(() => runtime.client.close());
  t.after(() => runtime.server.forceShutdown());

  const request = new pb.SayHelloRequest();
  request.setName(' Bob ');
  request.setLangCode('en');

  const response = await callUnary(runtime.client, 'sayHello', request);

  assert.equal(response.getGreeting(), 'Hello Bob');
  const snapshot = obs.registry.snapshot();
  const counter = snapshot.counters.find((sample) => sample.name === 'greeting_emitted_total');
  assert.ok(counter);
  assert.deepEqual(counter.labels, {
    lang_code: 'en',
    language: 'English',
    transport: 'unknown',
  });
  assert.equal(counter.value, 1);

  const entry = obs.logRing.drain().find((logEntry) => logEntry.message === 'Greeted Bob in English (en)');
  assert.ok(entry);
  assert.deepEqual(Object.keys(entry.fields).sort(), [
    'duration_ns',
    'greeting',
    'lang_code',
    'language',
    'name',
    'transport',
  ]);
  assert.equal(entry.fields.lang_code, 'en');
  assert.equal(entry.fields.language, 'English');
  assert.equal(entry.fields.name, 'Bob');
  assert.equal(entry.fields.greeting, 'Hello Bob');
  assert.equal(entry.fields.transport, 'unknown');
  assert.ok(Number.parseInt(entry.fields.duration_ns, 10) >= 0);
});
