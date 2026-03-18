'use strict';

const publicApi = require('./public');
const pb = require('../gen/node/greeting/v1/greeting_pb.js');
const { serve } = require('@organic-programming/holons');
const server = require('../_internal/server');

const VERSION = 'gabriel-greeting-node {{ .Version }}';

async function main(args = process.argv.slice(2), stdout = process.stdout, stderr = process.stderr) {
  if (!args.length) {
    printUsage(stderr);
    return 1;
  }

  switch (canonicalCommand(args[0])) {
    case 'serve': {
      const listenUri = serve.parseFlags(args.slice(1));
      try {
        await server.listenAndServe(listenUri);
      } catch (error) {
        stderr.write(`serve: ${error.message}\n`);
        return 1;
      }
      return 0;
    }
    case 'version':
      stdout.write(`${VERSION}\n`);
      return 0;
    case 'help':
      printUsage(stdout);
      return 0;
    case 'listlanguages':
      return runListLanguages(args.slice(1), stdout, stderr);
    case 'sayhello':
      return runSayHello(args.slice(1), stdout, stderr);
    default:
      stderr.write(`unknown command "${args[0]}"\n`);
      printUsage(stderr);
      return 1;
  }
}

function runListLanguages(args, stdout, stderr) {
  let options;
  let positional;
  try {
    ({ options, positional } = parseCommandOptions(args));
  } catch (error) {
    stderr.write(`listLanguages: ${error.message}\n`);
    return 1;
  }

  if (positional.length) {
    stderr.write('listLanguages: accepts no positional arguments\n');
    return 1;
  }

  try {
    writeResponse(stdout, publicApi.listLanguages(new pb.ListLanguagesRequest()), options.format);
    return 0;
  } catch (error) {
    stderr.write(`listLanguages: ${error.message}\n`);
    return 1;
  }
}

function runSayHello(args, stdout, stderr) {
  let options;
  let positional;
  try {
    ({ options, positional } = parseCommandOptions(args));
  } catch (error) {
    stderr.write(`sayHello: ${error.message}\n`);
    return 1;
  }

  if (positional.length > 2) {
    stderr.write('sayHello: accepts at most <name> [lang_code]\n');
    return 1;
  }

  const request = new pb.SayHelloRequest();
  request.setLangCode('en');
  if (positional[0]) {
    request.setName(positional[0]);
  }
  if (positional.length >= 2) {
    if (options.lang) {
      stderr.write('sayHello: use either a positional lang_code or --lang, not both\n');
      return 1;
    }
    request.setLangCode(positional[1]);
  }
  if (options.lang) {
    request.setLangCode(options.lang);
  }

  try {
    writeResponse(stdout, publicApi.sayHello(request), options.format);
    return 0;
  } catch (error) {
    stderr.write(`sayHello: ${error.message}\n`);
    return 1;
  }
}

function parseCommandOptions(args) {
  const options = { format: 'text', lang: '' };
  const positional = [];

  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (arg === '--json') {
      options.format = 'json';
    } else if (arg === '--format') {
      index += 1;
      if (index >= args.length) {
        throw new Error('--format requires a value');
      }
      options.format = parseOutputFormat(args[index]);
    } else if (arg.startsWith('--format=')) {
      options.format = parseOutputFormat(arg.slice('--format='.length));
    } else if (arg === '--lang') {
      index += 1;
      if (index >= args.length) {
        throw new Error('--lang requires a value');
      }
      options.lang = args[index].trim();
    } else if (arg.startsWith('--lang=')) {
      options.lang = arg.slice('--lang='.length).trim();
    } else if (arg.startsWith('--')) {
      throw new Error(`unknown flag "${arg}"`);
    } else {
      positional.push(arg);
    }
  }

  return { options, positional };
}

function parseOutputFormat(raw) {
  const normalized = String(raw).trim().toLowerCase();
  if (normalized === '' || normalized === 'text' || normalized === 'txt') {
    return 'text';
  }
  if (normalized === 'json') {
    return 'json';
  }
  throw new Error(`unsupported format "${raw}"`);
}

function writeResponse(stdout, message, format) {
  if (format === 'json') {
    stdout.write(`${JSON.stringify(toJson(message), null, 2)}\n`);
    return;
  }
  if (format === 'text') {
    writeText(stdout, message);
    return;
  }
  throw new Error(`unsupported format "${format}"`);
}

function writeText(stdout, message) {
  if (message instanceof pb.SayHelloResponse) {
    stdout.write(`${message.getGreeting()}\n`);
    return;
  }
  if (message instanceof pb.ListLanguagesResponse) {
    for (const language of message.getLanguagesList()) {
      stdout.write(`${language.getCode()}\t${language.getName()}\t${language.getNative()}\n`);
    }
    return;
  }
  throw new Error(`unsupported text output for ${message.constructor.name}`);
}

function toJson(message) {
  if (message instanceof pb.SayHelloResponse) {
    return {
      greeting: message.getGreeting(),
      language: message.getLanguage(),
      langCode: message.getLangCode(),
    };
  }
  if (message instanceof pb.ListLanguagesResponse) {
    return {
      languages: message.getLanguagesList().map((language) => ({
        code: language.getCode(),
        name: language.getName(),
        native: language.getNative(),
      })),
    };
  }
  throw new Error(`unsupported JSON output for ${message.constructor.name}`);
}

function canonicalCommand(raw) {
  return String(raw).trim().toLowerCase().replace(/[-_\s]/g, '');
}

function printUsage(output) {
  output.write('usage: gabriel-greeting-node <command> [args] [flags]\n');
  output.write('\n');
  output.write('commands:\n');
  output.write('  serve [--listen <uri>]                    Start the gRPC server\n');
  output.write('  version                                  Print version and exit\n');
  output.write('  help                                     Print usage\n');
  output.write('  listLanguages [--format text|json]       List supported languages\n');
  output.write('  sayHello [name] [lang_code] [--format text|json] [--lang <code>]\n');
  output.write('\n');
  output.write('examples:\n');
  output.write('  gabriel-greeting-node serve --listen stdio\n');
  output.write('  gabriel-greeting-node listLanguages --format json\n');
  output.write('  gabriel-greeting-node sayHello Alice fr\n');
  output.write('  gabriel-greeting-node sayHello Alice --lang fr --format json\n');
}

module.exports = {
  VERSION,
  main,
  runCLI: main,
  parseCommandOptions,
  parseOutputFormat,
  writeResponse,
  canonicalCommand,
  printUsage,
};
