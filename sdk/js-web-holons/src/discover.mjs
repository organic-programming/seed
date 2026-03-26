import { parseManifestText } from "./manifest.mjs";

function slugFor(identity) {
    const given = String(identity.given_name || "").trim();
    const family = String(identity.family_name || "").trim().replace(/\?$/, "");
    if (!given && !family) {
        return "";
    }
    return `${given}-${family}`
        .trim()
        .toLowerCase()
        .replace(/ /g, "-")
        .replace(/^-+|-+$/g, "");
}

function toEntry(document, sourceURL) {
    const url = new URL(sourceURL);
    const relativePath = url.pathname.replace(/^\/+/, "") || ".";
    return {
        slug: slugFor(document.identity),
        uuid: document.identity.uuid || "",
        dir: sourceURL,
        relative_path: relativePath,
        origin: "remote",
        identity: document.identity,
        manifest: {
            kind: document.kind || "",
            build: document.build || { runner: "", main: "" },
            artifacts: document.artifacts || { binary: "", primary: "" },
        },
    };
}

export async function discoverFromManifest(url, options = {}) {
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (typeof fetchImpl !== "function") {
        throw new Error("fetch implementation required");
    }

    const response = await fetchImpl(url, {
        headers: {
            Accept: "text/plain, application/octet-stream, application/json",
        },
    });
    if (!response.ok) {
        throw new Error(`failed to fetch manifest: ${response.status} ${response.statusText}`);
    }

    const contentType = String(response.headers?.get?.("content-type") || "").toLowerCase();
    const text = await response.text();
    if (contentType.includes("application/json")) {
        const parsed = JSON.parse(text);
        const docs = Array.isArray(parsed) ? parsed : [parsed];
        return docs.map((doc) => toEntry({
            identity: {
                uuid: String(doc.uuid || ""),
                given_name: String(doc.given_name || ""),
                family_name: String(doc.family_name || ""),
                motto: String(doc.motto || ""),
                composer: String(doc.composer || ""),
                clade: String(doc.clade || ""),
                status: String(doc.status || ""),
                born: String(doc.born || ""),
                lang: String(doc.lang || ""),
                parents: Array.isArray(doc.parents) ? doc.parents.map(String) : [],
                reproduction: String(doc.reproduction || ""),
                generated_by: String(doc.generated_by || ""),
                proto_status: String(doc.proto_status || ""),
                aliases: Array.isArray(doc.aliases) ? doc.aliases.map(String) : [],
            },
            manifest: {
                kind: String(doc.kind || ""),
                build: {
                    runner: String(doc.build?.runner || ""),
                    main: String(doc.build?.main || ""),
                },
                artifacts: {
                    binary: String(doc.artifacts?.binary || ""),
                    primary: String(doc.artifacts?.primary || ""),
                },
            },
        }, url));
    }

    return [toEntry(parseManifestText(text, url), url)];
}

export function findBySlug(entries, slug) {
    const needle = String(slug || "").trim();
    if (!needle) {
        return null;
    }

    let match = null;
    for (const entry of entries || []) {
        if (entry?.slug !== needle) {
            continue;
        }
        if (match && match.uuid !== entry.uuid) {
            throw new Error(`ambiguous holon "${needle}"`);
        }
        match = entry;
    }
    return match;
}
