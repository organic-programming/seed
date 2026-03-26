// TypeScript declarations for @organic-programming/holons

import * as net from 'net';
import * as grpc from '@grpc/grpc-js';
import { EventEmitter } from 'events';
import { ChildProcess } from 'child_process';

// --- Transport ---

export namespace transport {
    const DEFAULT_URI: string;

    function listen(uri: string, options?: ListenOptions): net.Server | StdioListener | WSListener;
    function scheme(uri: string): string;
    function parseURI(uri: string): ParsedURI;

    interface ParsedTCPURI {
        scheme: 'tcp';
        uri: string;
        host: string;
        port: string;
    }

    interface ParsedUnixURI {
        scheme: 'unix';
        uri: string;
        path: string;
    }

    interface ParsedStdioURI {
        scheme: 'stdio';
        uri: 'stdio://';
    }

    interface ParsedWSURI {
        scheme: 'ws' | 'wss';
        uri: string;
        secure: boolean;
        host: string;
        port: string;
        path: string;
    }

    type ParsedURI = ParsedTCPURI | ParsedUnixURI | ParsedStdioURI | ParsedWSURI;

    interface ListenOptions {
        tcp?: {
            connectionListener?: (socket: net.Socket) => void;
        };
        unix?: {
            connectionListener?: (socket: net.Socket) => void;
        };
        ws?: {
            tls?: {
                key?: string | Buffer;
                cert?: string | Buffer;
                keyFile?: string;
                certFile?: string;
            };
        };
    }

    class StdioListener extends EventEmitter {
        accept(): { readable: NodeJS.ReadStream; writable: NodeJS.WriteStream; close(): void };
        close(): void;
        readonly address: string;
    }

    class WSListener extends EventEmitter {
        constructor(uri: string, options?: ListenOptions['ws']);
        start(): Promise<void>;
        ready(): Promise<void>;
        accept(): Promise<Duplex>;
        close(): void;
        readonly address: string;
        host: string;
        port: number;
        path: string;
        scheme: 'ws' | 'wss';
    }
}

// --- Serve ---

export namespace serve {
    type RegisterFunc = (server: grpc.Server) => void;

    interface RunOptions {
        reflect?: boolean;
        reflectionPackageDefinition?: grpc.GrpcObject;
        ws?: {
            tls?: {
                key?: string | Buffer;
                cert?: string | Buffer;
                keyFile?: string;
                certFile?: string;
            };
        };
        logger?: {
            error: (...args: any[]) => void;
            warn?: (...args: any[]) => void;
        };
    }

    interface HolonServer extends grpc.Server {
        stopHolon?: () => Promise<void>;
    }

    interface ParsedFlags {
        listenUri: string;
        reflect: boolean;
    }

    function parseFlags(args: string[]): string;
    function parseOptions(args: string[]): ParsedFlags;
    function run(listenUri: string, registerFn: RegisterFunc): Promise<HolonServer>;
    function runWithOptions(
        listenUri: string,
        registerFn: RegisterFunc,
        reflectOrOptions?: boolean | RunOptions,
    ): Promise<HolonServer>;
    const DEFAULT_URI: string;
}

export namespace describe {
    const HOLON_META_SERVICE_NAME: string;
    const NO_INCODE_DESCRIPTION_MESSAGE: string;
    const holons: {
        HOLON_META_SERVICE_DEF: grpc.ServiceDefinition;
        FieldLabel: Record<string, number>;
        DescribeRequest: any;
        DescribeResponse: any;
    };

    function buildResponse(protoDir: string, manifestPath?: string): {
        manifest: {
            identity: identity.HolonIdentity;
            kind: string;
            lang: string;
            build: discover.HolonBuild;
            artifacts: discover.HolonArtifacts;
        };
        services: Array<Record<string, any>>;
    };

    function useStaticResponse(response: Record<string, any> | null): void;
    function staticResponse(): Record<string, any> | null;
    function register(server: grpc.Server): void;
}

// --- Identity ---

export namespace identity {
    const PROTO_MANIFEST_FILE_NAME: string;

    interface HolonIdentity {
        uuid: string;
        given_name: string;
        family_name: string;
        motto: string;
        composer: string;
        clade: string;
        status: string;
        born: string;
        lang: string;
        parents: string[];
        reproduction: string;
        generated_by: string;
        proto_status: string;
        aliases: string[];
    }

    function parseHolon(filePath: string): HolonIdentity;
    function parseManifest(filePath: string): {
        identity: HolonIdentity;
        kind: string;
        build_runner: string;
        build_main: string;
        artifact_binary: string;
        artifact_primary: string;
        source_path?: string;
    };
    function findHolonProto(root: string): string | null;
    function resolveManifestPath(root: string): string;
    function resolve(root: string): {
        identity: HolonIdentity;
        kind: string;
        build_runner: string;
        build_main: string;
        artifact_binary: string;
        artifact_primary: string;
        source_path: string;
    };
    function resolveProtoFile(filePath: string): {
        identity: HolonIdentity;
        kind: string;
        build_runner: string;
        build_main: string;
        artifact_binary: string;
        artifact_primary: string;
        source_path: string;
    };
    function slugForIdentity(identity: HolonIdentity): string;
}

// --- Discover ---

export namespace discover {
    interface HolonBuild {
        runner: string;
        main: string;
    }

    interface HolonArtifacts {
        binary: string;
        primary: string;
    }

    interface HolonManifest {
        identity: identity.HolonIdentity;
        kind: string;
        build: HolonBuild;
        artifacts: HolonArtifacts;
    }

    interface HolonEntry {
        slug: string;
        uuid: string;
        dir: string;
        relative_path: string;
        origin: string;
        identity: identity.HolonIdentity;
        manifest: HolonManifest | null;
    }

    function discover(root: string): Promise<HolonEntry[]>;
    function discoverLocal(): Promise<HolonEntry[]>;
    function discoverAll(): Promise<HolonEntry[]>;
    function findBySlug(slug: string): Promise<HolonEntry | null>;
    function findByUUID(prefix: string): Promise<HolonEntry | null>;
}

// --- Connect ---

export namespace connect {
    interface ConnectOptions {
        timeout?: number;
        transport?: 'tcp' | 'stdio';
        start?: boolean;
        port_file?: string;
    }

    function connect(target: string, opts?: ConnectOptions): Promise<grpc.Client>;
    function disconnect(client: grpc.Client): Promise<void>;
}

// --- gRPC Client ---

export namespace grpcclient {
    type ClientCtor<TClient> = new (
        address: string,
        credentials: grpc.ChannelCredentials,
        options?: grpc.ChannelOptions,
    ) => TClient;

    interface DialOptions {
        credentials?: grpc.ChannelCredentials;
        channelOptions?: grpc.ChannelOptions;
    }

    interface DialURIOptions extends DialOptions {
        command?: string;
        args?: string[];
        env?: NodeJS.ProcessEnv;
        ws?: Record<string, unknown>;
    }

    function dial<TClient>(addressOrURI: string, ClientCtor: ClientCtor<TClient>, options?: DialOptions): TClient;
    function dialWebSocket<TClient>(
        uri: string,
        ClientCtor: ClientCtor<TClient>,
        options?: DialURIOptions,
    ): Promise<{ client: TClient; close: () => Promise<void> }>;

    function dialStdio<TClient>(
        binaryPath: string,
        ClientCtor: ClientCtor<TClient>,
        options?: DialURIOptions,
    ): Promise<{ client: TClient; process: ChildProcess; close: () => Promise<void> }>;

    function dialURI<TClient>(
        uri: string,
        ClientCtor: ClientCtor<TClient>,
        options?: DialURIOptions,
    ): Promise<{ client: TClient; close: () => Promise<void> }>;
}

// --- Holon-RPC Client + Server ---

export namespace holonrpc {
    class HolonRPCError extends Error {
        constructor(code: number, message: string, data?: unknown);
        code: number;
        data?: unknown;
    }

    interface HolonRPCConnection {
        id: string;
        protocol: 'holon-rpc';
    }

    interface HolonRPCServerOptions {
        tls?: {
            key?: string | Buffer;
            cert?: string | Buffer;
            keyFile?: string;
            certFile?: string;
        };
    }

    interface HolonRPCClientOptions {
        connectTimeout?: number;
        invokeTimeout?: number;
        ws?: Record<string, unknown>;
        http?: {
            fetch?: typeof fetch;
            headers?: Record<string, string>;
        };
    }

    interface HolonRPCConnectOptions {
        timeout?: number;
        ws?: Record<string, unknown>;
        http?: {
            fetch?: typeof fetch;
            headers?: Record<string, string>;
        };
    }

    interface HolonRPCInvokeOptions {
        timeout?: number;
    }

    interface HolonRPCSSEEvent {
        event: string;
        id: string;
        result?: Record<string, unknown>;
        error?: {
            code: number;
            message: string;
            data?: unknown;
        };
    }

    class HolonRPCClient extends EventEmitter {
        constructor(options?: HolonRPCClientOptions);
        register(method: string, handler: (params: Record<string, unknown>) => unknown): void;
        unregister(method: string): void;
        connected(): boolean;
        connect(url: string, options?: HolonRPCConnectOptions): Promise<void>;
        connectWithReconnect(url: string, options?: HolonRPCConnectOptions): Promise<void>;
        close(): Promise<void>;
        invoke(
            method: string,
            params?: Record<string, unknown>,
            options?: HolonRPCInvokeOptions,
        ): Promise<Record<string, unknown>>;
        stream(
            method: string,
            params?: Record<string, unknown>,
            options?: HolonRPCInvokeOptions,
        ): Promise<HolonRPCSSEEvent[]>;
        streamQuery(
            method: string,
            params?: Record<string, unknown>,
            options?: HolonRPCInvokeOptions,
        ): Promise<HolonRPCSSEEvent[]>;
    }

    class HolonRPCServer extends EventEmitter {
        constructor(uri?: string, options?: HolonRPCServerOptions);
        uri: string;
        address: string;
        register(method: string, handler: (params: Record<string, unknown>, client: HolonRPCConnection) => unknown): void;
        unregister(method: string): void;
        listClients(): HolonRPCConnection[];
        start(): Promise<void>;
        close(): Promise<void>;
        invoke(
            client: HolonRPCConnection | string,
            method: string,
            params?: Record<string, unknown>,
            options?: { timeout?: number },
        ): Promise<Record<string, unknown>>;
    }

    const DEFAULT_URI: string;
}
