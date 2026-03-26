import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import protobuf from "protobufjs";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const PROTO_ROOT = path.resolve(HERE, "../../../../../../_protos");
const GOOGLE_PROTO_ROOT = "/opt/homebrew/include";
const PROTO_FILES = [
    path.join(PROTO_ROOT, "holons/v1/manifest.proto"),
    path.join(PROTO_ROOT, "holons/v1/describe.proto"),
    path.join(PROTO_ROOT, "holons/v1/coax.proto"),
];

export const root = new protobuf.Root();
const defaultResolvePath = protobuf.Root.prototype.resolvePath;

root.resolvePath = function resolvePath(origin, target) {
    const candidates = [];
    if (origin) {
        candidates.push(path.resolve(path.dirname(origin), target));
    }
    candidates.push(path.resolve(PROTO_ROOT, target));
    candidates.push(path.resolve(GOOGLE_PROTO_ROOT, target));

    for (const candidate of candidates) {
        if (fs.existsSync(candidate)) {
            return candidate;
        }
    }
    return defaultResolvePath.call(this, origin, target);
};

root.loadSync(PROTO_FILES, { keepCase: true });
root.resolveAll();
