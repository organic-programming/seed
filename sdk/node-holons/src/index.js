// holons — Organic Programming SDK for JavaScript
//
// Transport, serve, and identity utilities for building holons in JS/TS.

const transport = require('./transport');
const serve = require('./serve');
const describe = require('./describe');
const identity = require('./identity');
const discover = require('./discover');
const observability = require('./observability');
const composite = require('./composite');
const connectApi = require('./connect');
const discoveryTypes = require('./discovery_types');
const grpcclient = require('./grpcclient');
const holonrpcServer = require('./holonrpc_server');
const holonrpcClient = require('./holonrpc_client');
const relay = require('./relay');

const holonrpc = {
    ...holonrpcServer,
    ...holonrpcClient,
};

module.exports = {
    transport,
    serve,
    describe,
    composite,
    observability,
    identity,
    grpcclient,
    relay,
    holonrpc,
    ...discoveryTypes,
    Discover: discover.Discover,
    resolve: discover.resolve,
    connect: connectApi.connect,
    disconnect: connectApi.disconnect,
    discover: {
        ...discoveryTypes,
        Discover: discover.Discover,
        resolve: discover.resolve,
    },
};
