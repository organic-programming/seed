'use strict';

const publicApi = require('./public');
const pb = require('../gen/node/relay/v1/relay_pb.js');
const { serve } = require('@organic-programming/holons');
const server = require('../_internal/server');

const VERSION = 'cascade-node-node {{ .Version }}';

async function main(args = process.argv.slice(2), stdout = process.stdout, stderr = process.stderr) {
  if (!args.length) {
    printUsage(stderr);
    return 1;
  }

  switch (canonicalCommand(args[0])) {
    case 'serve': {
      const serveArgs = args.slice(1);
      const serveOptions = serve.parseOptions(serveArgs);
      try {
        await server.listenAndServe(serveOptions.listenUri, serveOptions.reflect, parseMembers(serveArgs));
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
    case 'tick':
      return runTick(args.slice(1), stdout, stderr);
    default:
      stderr.write(`unknown command "${args[0]}"\n`);
      printUsage(stderr);
      return 1;
  }
}

function runTick(args, stdout, stderr) {
  const request = new pb.TickRequest();
  const positional = [];
  try {
    for (let index = 0; index < args.length; index += 1) {
      const arg = args[index];
      if (arg === '--sender') {
        index += 1;
        if (index >= args.length) throw new Error('--sender requires a value');
        request.setSender(args[index]);
      } else if (arg.startsWith('--sender=')) {
        request.setSender(arg.slice('--sender='.length));
      } else if (arg === '--note') {
        index += 1;
        if (index >= args.length) throw new Error('--note requires a value');
        request.setNote(args[index]);
      } else if (arg.startsWith('--note=')) {
        request.setNote(arg.slice('--note='.length));
      } else if (arg.startsWith('--')) {
        throw new Error(`unknown flag "${arg}"`);
      } else {
        positional.push(arg);
      }
    }
    if (!request.getSender() && positional[0]) request.setSender(positional[0]);
    if (!request.getNote() && positional[1]) request.setNote(positional[1]);
    const response = publicApi.tick(request);
    stdout.write(`${JSON.stringify(response.toObject(), null, 2)}\n`);
    return 0;
  } catch (error) {
    stderr.write(`tick: ${error.message}\n`);
    return 1;
  }
}

function parseMembers(args) {
  const members = [];
  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (arg === '--member') {
      index += 1;
      if (index >= args.length) throw new Error('--member requires <slug>=<address>');
      members.push(parseMember(args[index]));
    } else if (arg.startsWith('--member=')) {
      members.push(parseMember(arg.slice('--member='.length)));
    }
  }
  return members;
}

function parseMember(raw) {
  const idx = String(raw).indexOf('=');
  if (idx < 0) throw new Error('--member requires <slug>=<address>');
  const slug = raw.slice(0, idx).trim();
  const address = raw.slice(idx + 1).trim();
  if (!slug || !address) throw new Error('--member requires non-empty slug and address');
  return { slug, address };
}

function canonicalCommand(raw) {
  return String(raw).trim().toLowerCase().replace(/[-_\s]/g, '');
}

function printUsage(output) {
  output.write('usage: cascade-node-node <command> [args] [flags]\n');
  output.write('\n');
  output.write('commands:\n');
  output.write('  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server\n');
  output.write('  tick [sender] [note]                                Emit one local tick\n');
  output.write('  version                                             Print version and exit\n');
  output.write('  help                                                Print usage\n');
}

module.exports = {
  VERSION,
  main,
  runCLI: main,
  parseMembers,
  parseMember,
  canonicalCommand,
  printUsage,
};

