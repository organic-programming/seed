package org.organicprogramming.holons

import java.nio.file.Files
import java.nio.file.Path
import java.util.Comparator

/** Parsed holon identity. */
data class HolonIdentity(
    val uuid: String = "",
    val givenName: String = "",
    val familyName: String = "",
    val motto: String = "",
    val composer: String = "",
    val clade: String = "",
    val status: String = "",
    val born: String = "",
    val version: String = "",
    val lang: String = "",
    val parents: List<String> = emptyList(),
    val reproduction: String = "",
    val generatedBy: String = "",
    val protoStatus: String = "",
    val aliases: List<String> = emptyList(),
) {
    fun slug(): String {
        val given = givenName.trim()
        val family = familyName.trim().removeSuffix("?")
        if (given.isEmpty() && family.isEmpty()) {
            return ""
        }
        return "$given-$family".trim().lowercase().replace(" ", "-").trim('-')
    }
}

data class ResolvedSkill(
    val name: String = "",
    val description: String = "",
    val whenText: String = "",
    val steps: List<String> = emptyList(),
)

data class ResolvedSequenceParam(
    val name: String = "",
    val description: String = "",
    val required: Boolean = false,
    val defaultValue: String = "",
)

data class ResolvedSequence(
    val name: String = "",
    val description: String = "",
    val params: List<ResolvedSequenceParam> = emptyList(),
    val steps: List<String> = emptyList(),
)

data class ManifestIdentity(
    val identity: HolonIdentity,
    val sourcePath: Path,
)

data class ResolvedManifest(
    val identity: HolonIdentity = HolonIdentity(),
    val sourcePath: Path = Path.of(".").toAbsolutePath().normalize(),
    val description: String = "",
    val kind: String = "",
    val buildRunner: String = "",
    val buildMain: String = "",
    val artifactBinary: String = "",
    val artifactPrimary: String = "",
    val requiredFiles: List<String> = emptyList(),
    val memberPaths: List<String> = emptyList(),
    val skills: List<ResolvedSkill> = emptyList(),
    val sequences: List<ResolvedSequence> = emptyList(),
)

/** Parse holon.proto manifests into resolved holon metadata. */
object Identity {
    const val PROTO_MANIFEST_FILE_NAME = "holon.proto"

    private val manifestPattern = Regex("""option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{""")

    fun parseHolon(path: String): HolonIdentity = resolveProtoFile(Path.of(path)).identity

    fun parseManifest(path: Path): ResolvedManifest = resolveProtoFile(path)

    fun resolve(root: Path): ResolvedManifest = resolveProtoFile(resolveManifestPath(root))

    fun resolveManifest(root: Path): ManifestIdentity {
        val resolved = resolve(root)
        return ManifestIdentity(
            identity = resolved.identity,
            sourcePath = resolved.sourcePath,
        )
    }

    fun resolveProtoFile(path: Path): ResolvedManifest {
        val resolved = path.toAbsolutePath().normalize()
        require(Files.isRegularFile(resolved) && resolved.fileName?.toString() == PROTO_MANIFEST_FILE_NAME) {
            "$resolved is not a $PROTO_MANIFEST_FILE_NAME file"
        }
        return parseManifestFile(resolved)
    }

    fun findHolonProto(root: Path): Path? {
        val resolved = root.toAbsolutePath().normalize()
        if (Files.isRegularFile(resolved)) {
            return if (resolved.fileName?.toString() == PROTO_MANIFEST_FILE_NAME) resolved else null
        }
        if (!Files.isDirectory(resolved)) {
            return null
        }

        val direct = resolved.resolve(PROTO_MANIFEST_FILE_NAME)
        if (Files.isRegularFile(direct)) {
            return direct
        }

        val apiV1 = resolved.resolve("api").resolve("v1").resolve(PROTO_MANIFEST_FILE_NAME)
        if (Files.isRegularFile(apiV1)) {
            return apiV1
        }

        Files.walk(resolved).use { walk ->
            return walk
                .filter { Files.isRegularFile(it) && it.fileName?.toString() == PROTO_MANIFEST_FILE_NAME }
                .sorted(Comparator.comparing(Path::toString))
                .findFirst()
                .orElse(null)
        }
    }

    fun resolveManifestPath(root: Path): Path {
        val resolved = root.toAbsolutePath().normalize()
        val searchRoots = mutableListOf(resolved)
        val parent = resolved.parent
        if (resolved.fileName?.toString() == "protos" && parent != null) {
            searchRoots.add(parent)
        } else if (parent != null && parent != resolved) {
            searchRoots.add(parent)
        }

        searchRoots.forEach { candidateRoot ->
            findHolonProto(candidateRoot)?.let { return it }
        }
        throw java.io.IOException("no $PROTO_MANIFEST_FILE_NAME found near $resolved")
    }

    private fun parseManifestFile(path: Path): ResolvedManifest {
        val text = Files.readString(path)
        val manifestBlock = extractManifestBlock(text)
            ?: throw IllegalArgumentException("$path: missing holons.v1.manifest option in holon.proto")

        val identityBlock = extractFirstBlock("identity", manifestBlock).orEmpty()
        val lineageBlock = extractFirstBlock("lineage", manifestBlock).orEmpty()
        val buildBlock = extractFirstBlock("build", manifestBlock).orEmpty()
        val artifactsBlock = extractFirstBlock("artifacts", manifestBlock).orEmpty()
        val requiresBlock = extractFirstBlock("requires", manifestBlock).orEmpty()

        return ResolvedManifest(
            identity = HolonIdentity(
                uuid = scalar("uuid", identityBlock),
                givenName = scalar("given_name", identityBlock),
                familyName = scalar("family_name", identityBlock),
                motto = scalar("motto", identityBlock),
                composer = scalar("composer", identityBlock),
                clade = scalar("clade", identityBlock),
                status = scalar("status", identityBlock),
                born = scalar("born", identityBlock),
                version = scalar("version", identityBlock),
                lang = scalar("lang", manifestBlock),
                parents = stringList("parents", lineageBlock),
                reproduction = scalar("reproduction", lineageBlock),
                generatedBy = scalar("generated_by", lineageBlock),
                protoStatus = scalar("proto_status", identityBlock),
                aliases = stringList("aliases", identityBlock),
            ),
            sourcePath = path,
            description = scalar("description", manifestBlock),
            kind = scalar("kind", manifestBlock),
            buildRunner = scalar("runner", buildBlock),
            buildMain = scalar("main", buildBlock),
            artifactBinary = scalar("binary", artifactsBlock),
            artifactPrimary = scalar("primary", artifactsBlock),
            requiredFiles = stringList("files", requiresBlock),
            memberPaths = memberPaths(buildBlock),
            skills = resolvedSkills(manifestBlock),
            sequences = resolvedSequences(manifestBlock),
        )
    }

    private fun memberPaths(buildBlock: String): List<String> =
        blockList("members", buildBlock)
            .map { scalar("path", it) }
            .filter { it.isNotBlank() }

    private fun resolvedSkills(manifestBlock: String): List<ResolvedSkill> =
        blockList("skills", manifestBlock).map { block ->
            ResolvedSkill(
                name = scalar("name", block),
                description = scalar("description", block),
                whenText = scalar("when", block),
                steps = stringList("steps", block),
            )
        }

    private fun resolvedSequences(manifestBlock: String): List<ResolvedSequence> =
        blockList("sequences", manifestBlock).map { block ->
            ResolvedSequence(
                name = scalar("name", block),
                description = scalar("description", block),
                params = blockList("params", block).map { paramBlock ->
                    ResolvedSequenceParam(
                        name = scalar("name", paramBlock),
                        description = scalar("description", paramBlock),
                        required = scalar("required", paramBlock).toBoolean(),
                        defaultValue = scalar("default", paramBlock),
                    )
                },
                steps = stringList("steps", block),
            )
        }

    private fun blockList(name: String, source: String): List<String> {
        if (source.isBlank()) {
            return emptyList()
        }

        val blocks = mutableListOf<String>()
        Regex("""\b${Regex.escape(name)}\s*:\s*\{""")
            .findAll(source)
            .forEach { match ->
                val braceIndex = source.indexOf('{', match.range.first)
                val endIndex = balancedBlockEnd(source, braceIndex)
                if (endIndex >= 0) {
                    blocks += source.substring(braceIndex + 1, endIndex)
                }
            }

        arrayContents(name, source).forEach { arrayBody ->
            blocks += extractInlineBlocks(arrayBody)
        }
        return blocks
    }

    private fun extractInlineBlocks(source: String): List<String> {
        val blocks = mutableListOf<String>()
        var insideString = false
        var escaped = false

        var index = 0
        while (index < source.length) {
            val ch = source[index]
            if (insideString) {
                if (escaped) {
                    escaped = false
                } else if (ch == '\\') {
                    escaped = true
                } else if (ch == '"') {
                    insideString = false
                }
                index += 1
                continue
            }

            when (ch) {
                '"' -> insideString = true
                '{' -> {
                    val endIndex = balancedBlockEnd(source, index)
                    if (endIndex >= 0) {
                        blocks += source.substring(index + 1, endIndex)
                        index = endIndex
                    }
                }
            }
            index += 1
        }

        return blocks
    }

    private fun extractManifestBlock(source: String): String? {
        val match = manifestPattern.find(source) ?: return null
        val braceIndex = source.indexOf('{', match.range.first)
        return if (braceIndex >= 0) balancedBlockContents(source, braceIndex) else null
    }

    private fun extractFirstBlock(name: String, source: String): String? {
        val match = Regex("""\b${Regex.escape(name)}\s*:\s*\{""").find(source) ?: return null
        val braceIndex = source.indexOf('{', match.range.first)
        return if (braceIndex >= 0) balancedBlockContents(source, braceIndex) else null
    }

    private fun scalar(name: String, source: String): String {
        Regex("""\b${Regex.escape(name)}\s*:\s*"((?:[^"\\]|\\.)*)"""")
            .find(source)
            ?.groupValues
            ?.getOrNull(1)
            ?.let(::unescapeProtoString)
            ?.let { return it }

        return Regex("""\b${Regex.escape(name)}\s*:\s*([^\s,\]\}]+)""")
            .find(source)
            ?.groupValues
            ?.getOrNull(1)
            .orEmpty()
    }

    private fun stringList(name: String, source: String): List<String> =
        arrayContents(name, source).flatMap { arrayBody ->
            Regex(""""((?:[^"\\]|\\.)*)"|([^\s,\]]+)""")
                .findAll(arrayBody)
                .mapNotNull { match ->
                    match.groups[1]?.value?.let(::unescapeProtoString)
                        ?: match.groups[2]?.value
                }
                .toList()
        }

    private fun arrayContents(name: String, source: String): List<String> {
        if (source.isBlank()) {
            return emptyList()
        }

        val arrays = mutableListOf<String>()
        Regex("""\b${Regex.escape(name)}\s*:\s*\[""")
            .findAll(source)
            .forEach { match ->
                val openIndex = source.indexOf('[', match.range.first)
                val endIndex = balancedRangeEnd(source, openIndex, '[', ']')
                if (endIndex >= 0) {
                    arrays += source.substring(openIndex + 1, endIndex)
                }
            }
        return arrays
    }

    private fun balancedBlockContents(source: String, openingBrace: Int): String? {
        val endIndex = balancedBlockEnd(source, openingBrace)
        return if (endIndex >= 0) source.substring(openingBrace + 1, endIndex) else null
    }

    private fun balancedBlockEnd(source: String, openingBrace: Int): Int =
        balancedRangeEnd(source, openingBrace, '{', '}')

    private fun balancedRangeEnd(source: String, openingIndex: Int, open: Char, close: Char): Int {
        var depth = 0
        var insideString = false
        var escaped = false

        var index = openingIndex
        while (index < source.length) {
            val ch = source[index]
            if (insideString) {
                if (escaped) {
                    escaped = false
                } else if (ch == '\\') {
                    escaped = true
                } else if (ch == '"') {
                    insideString = false
                }
                index += 1
                continue
            }

            when (ch) {
                '"' -> insideString = true
                open -> depth += 1
                close -> {
                    depth -= 1
                    if (depth == 0) {
                        return index
                    }
                }
            }
            index += 1
        }

        return -1
    }

    private fun unescapeProtoString(value: String): String =
        value.replace("\\\"", "\"").replace("\\\\", "\\")
}
