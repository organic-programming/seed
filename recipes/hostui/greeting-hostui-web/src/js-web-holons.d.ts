declare module "js-web-holons" {
  export type UnaryMetadata = Record<string, string>;

  export interface GrpcWebClient {
    readonly baseUrl: string;
    unary<Request, Response>(
      path: string,
      serialize: (request: Request) => Uint8Array,
      deserialize: (bytes: Uint8Array) => Response,
      request: Request,
      metadata?: UnaryMetadata,
      options?: { signal?: AbortSignal },
    ): Promise<Response>;
    close(): void;
  }

  export function connect(target: string): GrpcWebClient;
  export function disconnect(client: GrpcWebClient | null | undefined): void;
}
