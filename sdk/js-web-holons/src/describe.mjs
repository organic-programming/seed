import { connectDirect, disconnect } from "./connect.mjs";

export const HOLON_META_METHOD = "holons.v1.HolonMeta/Describe";

/**
 * Call HolonMeta/Describe on a remote holon over either WebSocket JSON-RPC
 * or HTTP POST, depending on the target URI.
 *
 * @param {string|{invoke: Function}} targetOrClient - explicit dial URI or an existing client
 * @param {Object} [request={}] - DescribeRequest payload
 * @param {Object} [options={}]
 * @param {Object} [options.connectOptions] - options forwarded to the direct URI dial helper
 * @param {Object} [options.invokeOptions] - options forwarded to client.invoke()
 * @param {boolean} [options.disconnect=true] - whether to close auto-created clients
 * @returns {Promise<Object>}
 */
export async function describe(targetOrClient, request = {}, options = {}) {
    const autoDisconnect = options.disconnect ?? true;
    const ownsClient = typeof targetOrClient === "string";
    const client = ownsClient
        ? connectDirect(targetOrClient, options.connectOptions || {})
        : targetOrClient;

    if (!client || typeof client.invoke !== "function") {
        throw new TypeError("describe() requires a connectable URI or client with invoke()");
    }

    try {
        return await client.invoke(HOLON_META_METHOD, request, options.invokeOptions || {});
    } finally {
        if (ownsClient && autoDisconnect) {
            disconnect(client);
        }
    }
}
