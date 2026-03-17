'use strict';

const pb = require('../gen/node/greeting/v1/greeting_pb.js');
const { GREETINGS, lookup } = require('../_internal/greetings');

function listLanguages(_request = new pb.ListLanguagesRequest()) {
  const response = new pb.ListLanguagesResponse();
  for (const greeting of GREETINGS) {
    const language = new pb.Language();
    language.setCode(greeting.langCode);
    language.setName(greeting.langEnglish);
    language.setNative(greeting.langNative);
    response.addLanguages(language);
  }
  return response;
}

function sayHello(request) {
  const greeting = lookup(request.getLangCode());
  const name = (request.getName() || '').trim() || greeting.defaultName;
  const response = new pb.SayHelloResponse();
  response.setGreeting(greeting.template.replace('%s', name));
  response.setLanguage(greeting.langEnglish);
  response.setLangCode(greeting.langCode);
  return response;
}

module.exports = {
  listLanguages,
  sayHello,
};
