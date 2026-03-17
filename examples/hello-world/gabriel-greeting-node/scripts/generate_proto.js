'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

const ROOT = path.resolve(__dirname, '..');
const TEMP_OUT = path.join(ROOT, 'gen', 'node', '_tmp');
const FINAL_OUT = path.join(ROOT, 'gen', 'node', 'greeting', 'v1');
const SHARED_PROTO = path.join(ROOT, '..', '..', '_protos', 'v1', 'greeting.proto');
const DOMAIN_PROTO_ROOT = path.join(ROOT, '..', '..', '_protos');
const PLATFORM_PROTO_ROOT = path.join(ROOT, '..', '..', '..', '_protos');

function main() {
  fs.rmSync(TEMP_OUT, { force: true, recursive: true });
  fs.mkdirSync(TEMP_OUT, { recursive: true });
  fs.mkdirSync(FINAL_OUT, { recursive: true });

  runGrpcTools([
    `--proto_path=${DOMAIN_PROTO_ROOT}`,
    `--js_out=import_style=commonjs,binary:${TEMP_OUT}`,
    `--grpc_out=grpc_js:${TEMP_OUT}`,
    SHARED_PROTO,
  ]);

  moveGeneratedFile(path.join(TEMP_OUT, 'v1', 'greeting_pb.js'), path.join(FINAL_OUT, 'greeting_pb.js'));
  moveGeneratedFile(path.join(TEMP_OUT, 'v1', 'greeting_grpc_pb.js'), path.join(FINAL_OUT, 'greeting_grpc_pb.js'));
  fs.rmSync(TEMP_OUT, { force: true, recursive: true });

  const descriptorFile = path.join(ROOT, 'gen', 'node', 'holon_descriptor.bin');
  try {
    execFileSync(
      'protoc',
      [
        '--proto_path=api',
        `--proto_path=${DOMAIN_PROTO_ROOT}`,
        `--proto_path=${PLATFORM_PROTO_ROOT}`,
        `--descriptor_set_out=${descriptorFile}`,
        'v1/holon.proto',
      ],
      { cwd: ROOT, stdio: 'inherit' },
    );
  } finally {
    fs.rmSync(descriptorFile, { force: true });
  }
}

function runGrpcTools(args) {
  const bin = path.join(ROOT, 'node_modules', '.bin', 'grpc_tools_node_protoc');
  execFileSync(bin, args, { cwd: ROOT, stdio: 'inherit' });
}

function moveGeneratedFile(source, target) {
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.rmSync(target, { force: true });
  fs.renameSync(source, target);
}

main();
