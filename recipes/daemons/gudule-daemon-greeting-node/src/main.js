#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const { serve } = require('@organic-programming/holons');

const ROOT = findRecipeRoot(__dirname);
const SHARED_PROTO = path.resolve(ROOT, '../../protos/greeting/v1/greeting.proto');
const INCLUDE_DIRS = [path.resolve(ROOT, '../../protos')];

const GREETINGS = Object.freeze([
  { code: "en", name: "English", native: "English", template: "Hello, %s!" },
  { code: "fr", name: "French", native: "Français", template: "Bonjour, %s !" },
  { code: "es", name: "Spanish", native: "Español", template: "¡Hola, %s!" },
  { code: "de", name: "German", native: "Deutsch", template: "Hallo, %s!" },
  { code: "it", name: "Italian", native: "Italiano", template: "Ciao, %s!" },
  { code: "pt", name: "Portuguese", native: "Português", template: "Olá, %s!" },
  { code: "nl", name: "Dutch", native: "Nederlands", template: "Hallo, %s!" },
  { code: "ru", name: "Russian", native: "Русский", template: "Привет, %s!" },
  { code: "ja", name: "Japanese", native: "日本語", template: "こんにちは、%sさん！" },
  { code: "zh", name: "Chinese", native: "中文", template: "你好，%s！" },
  { code: "ko", name: "Korean", native: "한국어", template: "안녕하세요, %s!" },
  { code: "ar", name: "Arabic", native: "العربية", template: "مرحبا، %s!" },
  { code: "hi", name: "Hindi", native: "हिन्दी", template: "नमस्ते, %s!" },
  { code: "tr", name: "Turkish", native: "Türkçe", template: "Merhaba, %s!" },
  { code: "pl", name: "Polish", native: "Polski", template: "Cześć, %s!" },
  { code: "sv", name: "Swedish", native: "Svenska", template: "Hej, %s!" },
  { code: "no", name: "Norwegian", native: "Norsk", template: "Hei, %s!" },
  { code: "da", name: "Danish", native: "Dansk", template: "Hej, %s!" },
  { code: "fi", name: "Finnish", native: "Suomi", template: "Hei, %s!" },
  { code: "cs", name: "Czech", native: "Čeština", template: "Ahoj, %s!" },
  { code: "ro", name: "Romanian", native: "Română", template: "Bună, %s!" },
  { code: "hu", name: "Hungarian", native: "Magyar", template: "Szia, %s!" },
  { code: "el", name: "Greek", native: "Ελληνικά", template: "Γεια σου, %s!" },
  { code: "th", name: "Thai", native: "ไทย", template: "สวัสดี, %s!" },
  { code: "vi", name: "Vietnamese", native: "Tiếng Việt", template: "Xin chào, %s!" },
  { code: "id", name: "Indonesian", native: "Bahasa Indonesia", template: "Halo, %s!" },
  { code: "ms", name: "Malay", native: "Bahasa Melayu", template: "Hai, %s!" },
  { code: "sw", name: "Swahili", native: "Kiswahili", template: "Habari, %s!" },
  { code: "he", name: "Hebrew", native: "עברית", template: "שלום, %s!" },
  { code: "uk", name: "Ukrainian", native: "Українська", template: "Привіт, %s!" },
  { code: "bn", name: "Bengali", native: "বাংলা", template: "নমস্কার, %s!" },
  { code: "ta", name: "Tamil", native: "தமிழ்", template: "வணக்கம், %s!" },
  { code: "fa", name: "Persian", native: "فارسی", template: "سلام، %s!" },
  { code: "ur", name: "Urdu", native: "اردو", template: "السلام علیکم، %s!" },
  { code: "fil", name: "Filipino", native: "Filipino", template: "Kumusta, %s!" },
  { code: "ca", name: "Catalan", native: "Català", template: "Hola, %s!" },
  { code: "eu", name: "Basque", native: "Euskara", template: "Kaixo, %s!" },
  { code: "gl", name: "Galician", native: "Galego", template: "Ola, %s!" },
  { code: "is", name: "Icelandic", native: "Íslenska", template: "Halló, %s!" },
  { code: "et", name: "Estonian", native: "Eesti", template: "Tere, %s!" },
  { code: "lv", name: "Latvian", native: "Latviešu", template: "Sveiki, %s!" },
  { code: "lt", name: "Lithuanian", native: "Lietuvių", template: "Sveiki, %s!" },
  { code: "sk", name: "Slovak", native: "Slovenčina", template: "Ahoj, %s!" },
  { code: "sl", name: "Slovenian", native: "Slovenščina", template: "Živjo, %s!" },
  { code: "hr", name: "Croatian", native: "Hrvatski", template: "Bok, %s!" },
  { code: "sr", name: "Serbian", native: "Српски", template: "Здраво, %s!" },
  { code: "bg", name: "Bulgarian", native: "Български", template: "Здравей, %s!" },
  { code: "ka", name: "Georgian", native: "ქართული", template: "გამარჯობა, %s!" },
  { code: "hy", name: "Armenian", native: "Հայերեն", template: "Բարև, %s!" },
  { code: "am", name: "Amharic", native: "አማርኛ", template: "ሰላም, %s!" },
  { code: "mn", name: "Mongolian", native: "Монгол", template: "Сайн уу, %s!" },
  { code: "ne", name: "Nepali", native: "नेपाली", template: "नमस्कार, %s!" },
  { code: "kk", name: "Kazakh", native: "Қазақша", template: "Сәлем, %s!" },
  { code: "uz", name: "Uzbek", native: "Oʻzbekcha", template: "Salom, %s!" },
  { code: "yo", name: "Yoruba", native: "Yorùbá", template: "Báwo, %s!" },
  { code: "zu", name: "Zulu", native: "isiZulu", template: "Sawubona, %s!" }
]);

const GREETING_INDEX = new Map(GREETINGS.map((entry) => [entry.code, entry]));
const greetingDefinition = loadGreetingDefinition();

function findRecipeRoot(startDir) {
  let current = startDir;

  while (true) {
    if (
      fs.existsSync(path.join(current, 'holon.yaml')) &&
      fs.existsSync(path.join(current, 'package.json'))
    ) {
      return current;
    }

    const parent = path.dirname(current);
    if (parent === current) {
      throw new Error(`could not locate recipe root from ${startDir}`);
    }
    current = parent;
  }
}

function loadGreetingDefinition() {
  const packageDefinition = protoLoader.loadSync(SHARED_PROTO, {
    keepCase: false,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
    includeDirs: INCLUDE_DIRS,
  });
  return grpc.loadPackageDefinition(packageDefinition).greeting.v1;
}

function lookup(langCode) {
  return GREETING_INDEX.get((langCode || '').trim()) || GREETING_INDEX.get('en');
}

function listLanguages(_call, callback) {
  callback(null, {
    languages: GREETINGS.map((entry) => ({
      code: entry.code,
      name: entry.name,
      native: entry.native,
    })),
  });
}

function sayHello(call, callback) {
  const request = call.request || {};
  const entry = lookup(request.langCode);
  const name = (request.name || '').trim() || 'World';

  callback(null, {
    greeting: entry.template.replace('%s', name),
    language: entry.name,
    langCode: entry.code,
  });
}

function usage() {
  console.error('usage: gudule-daemon-greeting-node <serve|version> [flags]');
  process.exit(1);
}

async function main(argv) {
  const args = argv.slice(2);
  if (args.length === 0) {
    usage();
  }

  const command = args[0];
  if (command === 'serve') {
    const listenUri = serve.parseFlags(args.slice(1));
    await serve.runWithOptions(
      listenUri,
      (server) => {
        server.addService(greetingDefinition.GreetingService.service, {
          ListLanguages: listLanguages,
          SayHello: sayHello,
        });
      },
      { reflect: false, logger: console }
    );
    return;
  }

  if (command === 'version') {
    console.log('gudule-daemon-greeting-node v0.4.2');
    return;
  }

  usage();
}

main(process.argv).catch((error) => {
  console.error(error);
  process.exit(1);
});
