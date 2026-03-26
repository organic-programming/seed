/**
 * js-web-holons — Browser-only Holon-RPC client SDK.
 *
 * Supported dial transports:
 *   - WebSocket JSON-RPC: ws://, wss://
 *   - HTTP+SSE JSON-RPC: http://, https://
 *
 * The SDK is dial-only and does not expose serve/listen helpers.
 *
 * @module js-web-holons
 */

export { discoverFromManifest, findBySlug } from "./discover.mjs";
export { HolonClient, HolonError } from "./client.mjs";
export { HolonHTTPClient, connect, disconnect } from "./connect.mjs";
export { HOLON_META_METHOD, describe } from "./describe.mjs";
