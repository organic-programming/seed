'use strict';

const { composite, describe, observability, relay, serve } = require('@organic-programming/holons');

const describeGenerated = require('../gen/describe_generated');

const VERSION = 'observability-cascade-node-node {{ .Version }}';

async function main(args = process.argv.slice(2), stdout = process.stdout, stderr = process.stderr) {
  if (!args.length) {
    printUsage(stderr);
    return 1;
  }

  switch (canonicalCommand(args[0])) {
    case 'serve':
      return serveNode(args.slice(1), stderr);
    case 'version':
      stdout.write(`${VERSION}\n`);
      return 0;
    case 'help':
      printUsage(stdout);
      return 0;
    default:
      stderr.write(`unknown command "${args[0]}"\n`);
      printUsage(stderr);
      return 1;
  }
}

async function serveNode(args, stderr) {
  const { children, remaining } = serve.parseChildFlags(args);
  const serveOptions = serve.parseOptions(remaining);
  const transportName = parseTransport(remaining);
  describe.useStaticResponse(describeGenerated.staticDescribeResponse());
  observability.fromEnv({ slug: 'observability-cascade-node-node' });

  let downstream = null;
  try {
    if (children.length > 0) {
      downstream = await composite.SpawnMember({
        slug: children[0].slug,
        binaryPath: children[0].binary,
        transport: transportName,
        downstreamChain: children.slice(1),
      });
    }
    const server = await serve.runWithOptions(normalizeListenUri(serveOptions.listenUri), (grpcServer) => {
      relay.registerServer(grpcServer, { downstreamConn: downstream });
    }, {
      reflect: serveOptions.reflect,
      slug: 'observability-cascade-node-node',
    });
    const stopServer = server.stopHolon ? server.stopHolon.bind(server) : async () => {};
    server.stopHolon = async () => {
      if (downstream) {
        await downstream.stop().catch(() => {});
        downstream = null;
      }
      await stopServer();
    };
    return 0;
  } catch (error) {
    if (downstream) await downstream.stop().catch(() => {});
    stderr.write(`serve: ${error.message}\n`);
    return 1;
  }
}

function parseTransport(args) {
  for (let index = 0; index < args.length; index += 1) {
    if (args[index] === '--transport' && index + 1 < args.length) return args[index + 1];
    if (args[index].startsWith('--transport=')) return args[index].slice('--transport='.length);
  }
  return 'stdio';
}

function normalizeListenUri(listenUri) {
  const match = String(listenUri || '').match(/^tcp:\/\/:(\d+)$/);
  if (match) return `tcp://0.0.0.0:${match[1]}`;
  return listenUri;
}

function canonicalCommand(raw) {
  return String(raw).trim().toLowerCase().replace(/[-_\s]/g, '');
}

function printUsage(output) {
  output.write('usage: observability-cascade-node-node <command> [args] [flags]\n');
  output.write('\n');
  output.write('commands:\n');
  output.write('  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server\n');
  output.write('  version                                                           Print version and exit\n');
  output.write('  help                                                              Print usage\n');
}

module.exports = {
  VERSION,
  main,
  runCLI: main,
  parseTransport,
  canonicalCommand,
  printUsage,
};
