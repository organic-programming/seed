import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import { connect, disconnect } from "js-web-holons";

import {
  ListLanguagesRequestSchema,
  ListLanguagesResponseSchema,
  SayHelloRequestSchema,
  SayHelloResponseSchema,
} from "./gen/proto/greeting_pb";

export type GreetingLanguage = {
  code: string;
  name: string;
  native: string;
};

const LIST_LANGUAGES_PATH = "/greeting.v1.GreetingService/ListLanguages";
const SAY_HELLO_PATH = "/greeting.v1.GreetingService/SayHello";

function resolveDaemonTarget(): string {
  const env = (globalThis as { __GUDULE_DAEMON__?: string }).__GUDULE_DAEMON__;
  if (env && env.trim().length > 0) {
    return env.trim();
  }

  if ("location" in globalThis) {
    const { host, protocol } = globalThis.location;
    if (protocol !== "file:" && host.trim().length > 0) {
      return host;
    }
  }

  return "127.0.0.1:9091";
}

const client = connect(resolveDaemonTarget());

if ("addEventListener" in globalThis && typeof globalThis.addEventListener === "function") {
  globalThis.addEventListener("beforeunload", () => {
    disconnect(client);
  });
}

export async function listLanguages(): Promise<GreetingLanguage[]> {
  const response = await client.unary(
    LIST_LANGUAGES_PATH,
    (request) => toBinary(ListLanguagesRequestSchema, request),
    (bytes) => fromBinary(ListLanguagesResponseSchema, bytes),
    create(ListLanguagesRequestSchema),
  );
  return response.languages.map((language) => ({
    code: language.code,
    name: language.name,
    native: language.native,
  }));
}

export async function sayHello(name: string, langCode: string): Promise<string> {
  const response = await client.unary(
    SAY_HELLO_PATH,
    (request) => toBinary(SayHelloRequestSchema, request),
    (bytes) => fromBinary(SayHelloResponseSchema, bytes),
    create(SayHelloRequestSchema, {
      name,
      langCode,
    }),
  );
  return response.greeting;
}
