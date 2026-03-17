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

type GreetingRuntimeGlobals = {
  __GUDULE_DAEMON__?: string;
  __GUDULE_DAEMON_SLUG__?: string;
  __GUDULE_ASSEMBLY_FAMILY__?: string;
};

type WebHostUIContext = {
  assemblyFamily: string;
  daemonLabel: string;
  transport: string;
  target: string;
};

const LIST_LANGUAGES_PATH = "/greeting.v1.GreetingService/ListLanguages";
const SAY_HELLO_PATH = "/greeting.v1.GreetingService/SayHello";
const DEFAULT_WEB_FAMILY = "Greeting-Hostui-Web";
const DEFAULT_WEB_TRANSPORT = "tcp";

function resolveDaemonTarget(): string {
  const env = (globalThis as GreetingRuntimeGlobals).__GUDULE_DAEMON__;
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

function displayVariant(variant: string): string {
  const overrides = new Map([
    ["cpp", "CPP"],
    ["js", "JS"],
    ["qt", "Qt"],
  ]);

  return variant
    .split("-")
    .filter((token) => token.length > 0)
    .map((token) => overrides.get(token) ?? `${token[0].toUpperCase()}${token.slice(1)}`)
    .join("-");
}

function familyFromVariant(variant: string): string {
  return `Greeting-${displayVariant(variant)}-Web`;
}

function parseDaemonVariant(value: string | undefined): string | null {
  const trimmed = String(value ?? "").trim();
  if (!trimmed) {
    return null;
  }

  const match = trimmed.match(/gudule-(?:greeting-daemon|daemon-greeting)-([a-z0-9-]+)/i);
  return match?.[1]?.toLowerCase() ?? null;
}

function displayConnectionTarget(value: string): string {
  const trimmed = value.trim();
  for (const prefix of ["tcp://", "http://", "https://", "ws://", "wss://"]) {
    if (trimmed.startsWith(prefix)) {
      return trimmed.slice(prefix.length);
    }
  }
  return trimmed;
}

function resolveRuntimeContext(): WebHostUIContext {
  const globals = globalThis as GreetingRuntimeGlobals;
  const target = resolveDaemonTarget();
  const variant =
    parseDaemonVariant(globals.__GUDULE_DAEMON_SLUG__)
    ?? parseDaemonVariant(globals.__GUDULE_DAEMON__)
    ?? ("location" in globalThis ? parseDaemonVariant(globalThis.location.pathname) : null);
  const explicitFamily = globals.__GUDULE_ASSEMBLY_FAMILY__?.trim();

  return {
    assemblyFamily: explicitFamily || (variant ? familyFromVariant(variant) : DEFAULT_WEB_FAMILY),
    daemonLabel: variant ? `gudule-daemon-greeting-${variant}` : target,
    transport: DEFAULT_WEB_TRANSPORT,
    target,
  };
}

const hostUIContext = resolveRuntimeContext();
const client = connect(hostUIContext.target);
let connectedLogged = false;

console.error(
  `[HostUI] assembly=${hostUIContext.assemblyFamily} daemon=${hostUIContext.daemonLabel} transport=${hostUIContext.transport}`,
);

function logConnected(): void {
  if (connectedLogged) {
    return;
  }
  connectedLogged = true;
  console.error(
    `[HostUI] connected to ${hostUIContext.daemonLabel} on ${displayConnectionTarget(hostUIContext.target)}`,
  );
}

if ("addEventListener" in globalThis && typeof globalThis.addEventListener === "function") {
  globalThis.addEventListener("beforeunload", () => {
    disconnect(client);
  });
}

export function resolveWebAssemblyFamily(): string {
  return hostUIContext.assemblyFamily;
}

export async function listLanguages(): Promise<GreetingLanguage[]> {
  const response = await client.unary(
    LIST_LANGUAGES_PATH,
    (request) => toBinary(ListLanguagesRequestSchema, request),
    (bytes) => fromBinary(ListLanguagesResponseSchema, bytes),
    create(ListLanguagesRequestSchema),
  );
  logConnected();
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
  logConnected();
  return response.greeting;
}
