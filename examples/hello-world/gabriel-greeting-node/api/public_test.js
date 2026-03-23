'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const pb = require('../gen/node/greeting/v1/greeting_pb.js');
const publicApi = require('./public');

test('listLanguages includes English', () => {
  const response = publicApi.listLanguages(new pb.ListLanguagesRequest());
  const english = response.getLanguagesList().find((language) => language.getCode() === 'en');

  assert.ok(english);
  assert.equal(english.getName(), 'English');
  assert.equal(english.getNative(), 'English');
});

test('sayHello uses requested language', () => {
  const request = new pb.SayHelloRequest();
  request.setName('Bob');
  request.setLangCode('fr');

  const response = publicApi.sayHello(request);

  assert.equal(response.getGreeting(), 'Bonjour Bob');
  assert.equal(response.getLanguage(), 'French');
  assert.equal(response.getLangCode(), 'fr');
});

test('sayHello uses localized default name', () => {
  const request = new pb.SayHelloRequest();
  request.setLangCode('ja');

  const response = publicApi.sayHello(request);

  assert.equal(response.getGreeting(), 'こんにちは、マリアさん');
  assert.equal(response.getLanguage(), 'Japanese');
  assert.equal(response.getLangCode(), 'ja');
});

test('sayHello falls back to English', () => {
  const request = new pb.SayHelloRequest();
  request.setLangCode('unknown');

  const response = publicApi.sayHello(request);

  assert.equal(response.getGreeting(), 'Hello Mary');
  assert.equal(response.getLanguage(), 'English');
  assert.equal(response.getLangCode(), 'en');
});
