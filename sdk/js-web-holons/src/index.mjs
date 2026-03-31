/**
 * js-web-holons — Browser-only Holon-RPC client SDK.
 *
 * Phase 1 discovery exposes the shared cross-SDK API surface, but browser
 * discovery layers are currently validation-only and return empty results.
 *
 * @module js-web-holons
 */

export * from "./discovery_types.mjs";
export { Discover, resolve } from "./discover.mjs";
export { HolonClient, HolonError } from "./client.mjs";
export { HolonHTTPClient, connect, disconnect } from "./connect.mjs";
export { HOLON_META_METHOD, describe } from "./describe.mjs";
