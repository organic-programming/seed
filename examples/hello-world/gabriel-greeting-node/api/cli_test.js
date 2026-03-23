'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const cli = require('./cli');

function createBuffer() {
  let contents = '';
  return {
    write(chunk) {
      contents += String(chunk);
    },
    toString() {
      return contents;
    },
  };
}

test('runCLI prints version', async () => {
  const stdout = createBuffer();
  const stderr = createBuffer();

  const code = await cli.runCLI(['version'], stdout, stderr);

  assert.equal(code, 0);
  assert.equal(stdout.toString().trim(), cli.VERSION);
  assert.equal(stderr.toString(), '');
});

test('runCLI prints help', async () => {
  const stdout = createBuffer();
  const stderr = createBuffer();

  const code = await cli.runCLI(['help'], stdout, stderr);

  assert.equal(code, 0);
  assert.match(stdout.toString(), /usage: gabriel-greeting-node/);
  assert.match(stdout.toString(), /listLanguages/);
  assert.equal(stderr.toString(), '');
});

test('runCLI renders listLanguages as JSON', async () => {
  const stdout = createBuffer();
  const stderr = createBuffer();

  const code = await cli.runCLI(['listLanguages', '--format', 'json'], stdout, stderr);
  const payload = JSON.parse(stdout.toString());

  assert.equal(code, 0);
  assert.equal(payload.languages.length, 56);
  assert.equal(payload.languages[0].code, 'en');
  assert.equal(payload.languages[0].name, 'English');
  assert.equal(stderr.toString(), '');
});

test('runCLI renders sayHello as text', async () => {
  const stdout = createBuffer();
  const stderr = createBuffer();

  const code = await cli.runCLI(['sayHello', 'Bob', 'fr'], stdout, stderr);

  assert.equal(code, 0);
  assert.equal(stdout.toString().trim(), 'Bonjour Bob');
  assert.equal(stderr.toString(), '');
});

test('runCLI defaults sayHello to English JSON output', async () => {
  const stdout = createBuffer();
  const stderr = createBuffer();

  const code = await cli.runCLI(['sayHello', '--json'], stdout, stderr);
  const payload = JSON.parse(stdout.toString());

  assert.equal(code, 0);
  assert.equal(payload.greeting, 'Hello Mary');
  assert.equal(payload.language, 'English');
  assert.equal(payload.langCode, 'en');
  assert.equal(stderr.toString(), '');
});
