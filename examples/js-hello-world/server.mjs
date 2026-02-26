import grpc from "@grpc/grpc-js";
import protoLoader from "@grpc/proto-loader";
import path from "node:path";
import { fileURLToPath } from "node:url";
import holons from "@organic-programming/holons";

const { serve: holonServe } = holons;

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const PROTO_PATH = path.join(__dirname, "api", "hello.proto");

const packageDef = protoLoader.loadSync(PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
});

const proto = grpc.loadPackageDefinition(packageDef).hello.v1;

/** Greet implementation — pure deterministic. */
export function greet(call, callback) {
    const name = call.request.name || "World";
    callback(null, { message: `Hello, ${name}!` });
}

/** Register all gRPC services on the server. */
export function register(server) {
    server.addService(proto.HelloService.service, { Greet: greet });
}

/** Start the gRPC server using the standard holons serve runner. */
export async function run(args = process.argv.slice(2)) {
    const listenURI = holonServe.parseFlags(args);
    return holonServe.runWithOptions(listenURI, register, {
        reflect: true,
        reflectionPackageDefinition: proto,
    });
}

// Run if invoked directly
const isMain = process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1]);
if (isMain) {
    run().catch((err) => {
        console.error(err);
        process.exit(1);
    });
}
