import fs from "node:fs";
import path from "node:path";

export function parseManifestText(text, sourceURL) {
    const manifestBlock = extractManifestBlock(text);
    if (!manifestBlock) {
        throw new Error(`${sourceURL}: missing holons.v1.manifest option in holon.proto`);
    }

    const identityBlock = extractBlock("identity", manifestBlock) || "";
    const lineageBlock = extractBlock("lineage", manifestBlock) || "";
    const buildBlock = extractBlock("build", manifestBlock) || "";
    const artifactsBlock = extractBlock("artifacts", manifestBlock) || "";

    return {
        identity: {
            uuid: scalar("uuid", identityBlock),
            given_name: scalar("given_name", identityBlock),
            family_name: scalar("family_name", identityBlock),
            motto: scalar("motto", identityBlock),
            composer: scalar("composer", identityBlock),
            clade: scalar("clade", identityBlock),
            status: scalar("status", identityBlock),
            born: scalar("born", identityBlock),
            lang: scalar("lang", manifestBlock),
            parents: stringList("parents", lineageBlock),
            reproduction: scalar("reproduction", lineageBlock),
            generated_by: scalar("generated_by", lineageBlock),
            proto_status: scalar("proto_status", identityBlock),
            aliases: stringList("aliases", identityBlock),
        },
        kind: scalar("kind", manifestBlock),
        build: {
            runner: scalar("runner", buildBlock),
            main: scalar("main", buildBlock),
        },
        artifacts: {
            binary: scalar("binary", artifactsBlock),
            primary: scalar("primary", artifactsBlock),
        },
    };
}

export function parseManifestFile(filePath) {
    return parseManifestText(fs.readFileSync(filePath, "utf8"), filePath);
}

export function resolveManifestPath(root) {
    const resolved = path.resolve(String(root || ""));
    const searchRoots = [resolved];
    const parent = path.dirname(resolved);
    if (path.basename(resolved) === "protos") {
        searchRoots.push(parent);
    } else if (!searchRoots.includes(parent)) {
        searchRoots.push(parent);
    }

    for (const candidateRoot of searchRoots) {
        const candidate = findHolonProto(candidateRoot);
        if (candidate) {
            return candidate;
        }
    }

    throw new Error(`no holon.proto found near ${resolved}`);
}

export function findHolonProto(root) {
    const resolved = path.resolve(String(root || ""));
    if (fs.existsSync(resolved) && fs.statSync(resolved).isFile()) {
        return path.basename(resolved) === "holon.proto" ? resolved : null;
    }
    if (!fs.existsSync(resolved) || !fs.statSync(resolved).isDirectory()) {
        return null;
    }

    const direct = path.join(resolved, "holon.proto");
    if (fs.existsSync(direct) && fs.statSync(direct).isFile()) {
        return path.resolve(direct);
    }

    const apiV1 = path.join(resolved, "api", "v1", "holon.proto");
    if (fs.existsSync(apiV1) && fs.statSync(apiV1).isFile()) {
        return path.resolve(apiV1);
    }

    const candidates = [];
    walk(resolved, candidates);
    candidates.sort();
    return candidates[0] || null;
}

function walk(currentDir, candidates) {
    const entries = fs.readdirSync(currentDir, { withFileTypes: true });
    entries.sort((left, right) => left.name.localeCompare(right.name));
    for (const entry of entries) {
        const candidate = path.join(currentDir, entry.name);
        if (entry.isDirectory()) {
            walk(candidate, candidates);
            continue;
        }
        if (entry.isFile() && entry.name === "holon.proto") {
            candidates.push(path.resolve(candidate));
        }
    }
}

function extractManifestBlock(source) {
    const match = /option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{/m.exec(source);
    if (!match) {
        return null;
    }
    const braceIndex = source.indexOf("{", match.index);
    if (braceIndex < 0) {
        return null;
    }
    return balancedBlockContents(source, braceIndex);
}

function extractBlock(name, source) {
    const match = new RegExp(`\\b${escapeRegExp(name)}\\s*:\\s*\\{`, "m").exec(source);
    if (!match) {
        return null;
    }
    const braceIndex = source.indexOf("{", match.index);
    if (braceIndex < 0) {
        return null;
    }
    return balancedBlockContents(source, braceIndex);
}

function scalar(name, source) {
    const quoted = new RegExp(`\\b${escapeRegExp(name)}\\s*:\\s*"((?:[^"\\\\]|\\\\.)*)"`, "m").exec(source);
    if (quoted) {
        return unescapeProtoString(quoted[1]);
    }

    const bare = new RegExp(`\\b${escapeRegExp(name)}\\s*:\\s*([^\\s,\\]\\}]+)`, "m").exec(source);
    return bare ? bare[1] : "";
}

function stringList(name, source) {
    const match = new RegExp(`\\b${escapeRegExp(name)}\\s*:\\s*\\[(.*?)\\]`, "ms").exec(source);
    if (!match) {
        return [];
    }

    const values = [];
    const tokenPattern = /"((?:[^"\\]|\\.)*)"|([^\s,\]]+)/g;
    let token = tokenPattern.exec(match[1]);
    while (token) {
        if (token[1] !== undefined) {
            values.push(unescapeProtoString(token[1]));
        } else if (token[2] !== undefined) {
            values.push(token[2]);
        }
        token = tokenPattern.exec(match[1]);
    }
    return values;
}

function balancedBlockContents(source, openingBrace) {
    let depth = 0;
    let insideString = false;
    let escaped = false;
    const contentStart = openingBrace + 1;

    for (let index = openingBrace; index < source.length; index += 1) {
        const char = source[index];
        if (insideString) {
            if (escaped) {
                escaped = false;
            } else if (char === "\\") {
                escaped = true;
            } else if (char === "\"") {
                insideString = false;
            }
            continue;
        }

        if (char === "\"") {
            insideString = true;
        } else if (char === "{") {
            depth += 1;
        } else if (char === "}") {
            depth -= 1;
            if (depth === 0) {
                return source.slice(contentStart, index);
            }
        }
    }

    return null;
}

function unescapeProtoString(value) {
    return value.replace(/\\"/g, "\"").replace(/\\\\/g, "\\");
}

function escapeRegExp(value) {
    return String(value).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
