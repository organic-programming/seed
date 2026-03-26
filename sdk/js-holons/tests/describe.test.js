// HolonMeta Describe tests for js-holons.

'use strict';

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const grpc = require('@grpc/grpc-js');

const sdk = require('../src/index');
const SDK_ROOT = path.resolve(__dirname, '..');

const ECHO_PROTO = `syntax = "proto3";
package echo.v1;

// Echo echoes request payloads for documentation tests.
service Echo {
  // Ping echoes the inbound message.
  // @example {"message":"hello","sdk":"go-holons"}
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  // Message to echo back.
  // @required
  // @example "hello"
  string message = 1;

  // SDK marker included in the response.
  // @example "go-holons"
  string sdk = 2;
}

message PingResponse {
  // Echoed message.
  string message = 1;

  // SDK marker from the server.
  string sdk = 2;
}
`;

const HOLON_PROTO = `syntax = "proto3";

package holons.test.v1;

option (holons.v1.manifest) = {
  identity: {
    uuid: "echo-server-0000"
    given_name: "Echo"
    family_name: "Server"
    motto: "Reply precisely."
    composer: "describe-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "javascript"
};
`;

function canListenOnLoopback() {
    return new Promise((resolve) => {
        const probe = require('node:net').createServer();
        probe.once('error', (err) => {
            const code = err && err.code;
            resolve(code !== 'EPERM' && code !== 'EACCES');
        });
        probe.listen(0, '127.0.0.1', () => {
            probe.close(() => resolve(true));
        });
    });
}

function makeHolonDir(includeProto = true) {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'js-holons-describe-'));
    fs.writeFileSync(path.join(root, 'holon.proto'), HOLON_PROTO);
    if (includeProto) {
        const protoDir = path.join(root, 'protos', 'echo', 'v1');
        fs.mkdirSync(protoDir, { recursive: true });
        fs.writeFileSync(path.join(protoDir, 'echo.proto'), ECHO_PROTO);
    }
    return root;
}

function removeDir(root) {
    fs.rmSync(root, { recursive: true, force: true });
}

function findField(fields, name) {
    return fields.find((field) => field.name === name);
}

function writeStaticEchoServer(runtimeRoot, response) {
    const scriptPath = path.join(runtimeRoot, 'static-echo-server.js');
    fs.writeFileSync(scriptPath, [
        "'use strict';",
        '',
        `const sdk = require(${JSON.stringify(path.join(SDK_ROOT, 'src', 'index.js'))});`,
        `const echoServer = require(${JSON.stringify(path.join(SDK_ROOT, 'cmd', 'echo-server.js'))});`,
        '',
        `sdk.describe.useStaticResponse(${JSON.stringify(response)});`,
        '',
        'echoServer.run(process.argv)',
        '    .then((started) => {',
        "        if (started.listen !== 'stdio://' && started.listen !== 'stdio') {",
        "            process.stdout.write(`${started.publicURI}\\n`);",
        '        }',
        '    })',
        '    .catch((err) => {',
        "        process.stderr.write(`${err.stack || err.message}\\n`);",
        '        process.exit(1);',
        '    });',
        '',
    ].join('\n'));
    return scriptPath;
}

function waitForAdvertisedURI(child, timeoutMs = 5000) {
    return new Promise((resolve, reject) => {
        const stderrChunks = [];
        let settled = false;

        const finish = (err, uri) => {
            if (settled) {
                return;
            }
            settled = true;
            clearTimeout(timer);
            child.stdout?.off('data', onStdout);
            child.stderr?.off('data', onStderr);
            child.off('error', onError);
            child.off('exit', onExit);
            if (err) {
                reject(err);
                return;
            }
            resolve(uri);
        };

        const onStdout = (chunk) => {
            for (const line of String(chunk).split(/\r?\n/)) {
                const trimmed = line.trim();
                if (trimmed.includes('://')) {
                    finish(null, trimmed);
                    return;
                }
            }
        };

        const onStderr = (chunk) => {
            stderrChunks.push(Buffer.from(chunk));
        };

        const onError = (err) => {
            finish(err);
        };

        const onExit = (code, signal) => {
            const stderrText = Buffer.concat(stderrChunks).toString('utf8').trim();
            const details = stderrText ? `: ${stderrText}` : '';
            finish(new Error(`static echo server exited before advertising an address (${signal || code || 'unknown'})${details}`));
        };

        const timer = setTimeout(() => {
            finish(new Error('timed out waiting for static echo server startup'));
        }, timeoutMs);
        timer.unref?.();

        child.stdout?.on('data', onStdout);
        child.stderr?.on('data', onStderr);
        child.once('error', onError);
        child.once('exit', onExit);
    });
}

async function terminateChild(child, timeoutMs = 1000) {
    if (!child || child.exitCode !== null) {
        return;
    }

    child.kill('SIGTERM');
    await new Promise((resolve) => {
        const onExit = () => {
            clearTimeout(timer);
            resolve();
        };

        const timer = setTimeout(() => {
            child.off('exit', onExit);
            if (child.exitCode === null) {
                child.kill('SIGKILL');
                child.once('exit', () => resolve());
                return;
            }
            resolve();
        }, timeoutMs);
        timer.unref?.();

        child.once('exit', onExit);
    });
}

describe('describe', () => {
    it('buildResponse() extracts docs from echo proto', () => {
        const root = makeHolonDir(true);
        try {
            const response = sdk.describe.buildResponse(path.join(root, 'protos'));
            const identity = response.manifest.identity;

            assert.equal(identity.given_name, 'Echo');
            assert.equal(identity.family_name, 'Server');
            assert.equal(identity.motto, 'Reply precisely.');
            assert.equal(response.services.length, 1);
            assert.equal(response.services[0].name, 'echo.v1.Echo');
            assert.equal(response.services[0].description, 'Echo echoes request payloads for documentation tests.');
            assert.equal(response.services[0].methods.length, 1);
            assert.equal(response.services[0].methods[0].name, 'Ping');
            assert.equal(response.services[0].methods[0].description, 'Ping echoes the inbound message.');
            assert.equal(response.services[0].methods[0].example_input, '{"message":"hello","sdk":"go-holons"}');

            const messageField = findField(response.services[0].methods[0].input_fields, 'message');
            assert.ok(messageField);
            assert.equal(messageField.type, 'string');
            assert.equal(messageField.number, 1);
            assert.equal(messageField.description, 'Message to echo back.');
            assert.equal(messageField.label, sdk.describe.holons.FieldLabel.FIELD_LABEL_OPTIONAL);
            assert.equal(messageField.required, true);
            assert.equal(messageField.example, '"hello"');
        } finally {
            removeDir(root);
        }
    });

    it('serve.runWithOptions() serves the registered static response with no adjacent proto files', async (t) => {
        if (!await canListenOnLoopback()) {
            t.skip('socket bind not permitted in this environment');
            return;
        }

        const buildRoot = makeHolonDir(true);
        const runtimeRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'js-holons-static-runtime-'));
        const response = sdk.describe.buildResponse(path.join(buildRoot, 'protos'));
        const scriptPath = writeStaticEchoServer(runtimeRoot, response);
        const protoFiles = fs.readdirSync(runtimeRoot).filter((entry) => entry.endsWith('.proto'));
        assert.deepEqual(protoFiles, []);

        const HolonMetaClient = grpc.makeGenericClientConstructor(
            sdk.describe.holons.HOLON_META_SERVICE_DEF,
            'HolonMeta',
            {},
        );

        let child = null;
        let client = null;

        try {
            child = spawn(process.execPath, [scriptPath, '--listen', 'tcp://127.0.0.1:0'], {
                cwd: runtimeRoot,
                stdio: ['ignore', 'pipe', 'pipe'],
            });

            const publicURI = await waitForAdvertisedURI(child);
            const parsed = sdk.transport.parseURI(publicURI);
            client = new HolonMetaClient(`${parsed.host}:${parsed.port}`, grpc.credentials.createInsecure());
            const response = await new Promise((resolve, reject) => {
                client.Describe({}, (err, out) => {
                    if (err) {
                        reject(err);
                        return;
                    }
                    resolve(out);
                });
            });

            assert.equal(response.manifest.identity.given_name, 'Echo');
            assert.equal(response.manifest.identity.family_name, 'Server');
            assert.equal(response.manifest.identity.motto, 'Reply precisely.');
            assert.deepEqual(response.services.map((service) => service.name), ['echo.v1.Echo']);
        } finally {
            client?.close();
            await terminateChild(child);
            removeDir(buildRoot);
            removeDir(runtimeRoot);
        }
    });
});
