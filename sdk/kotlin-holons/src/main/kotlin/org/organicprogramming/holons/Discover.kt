package org.organicprogramming.holons

import holons.v1.Describe as HolonsDescribe
import io.grpc.CallOptions
import io.grpc.ConnectivityState
import io.grpc.ManagedChannel
import io.grpc.ManagedChannelBuilder
import io.grpc.stub.ClientCalls
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.io.BufferedReader
import java.io.Closeable
import java.io.IOException
import java.io.InputStream
import java.io.InputStreamReader
import java.io.OutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.net.URLDecoder
import java.nio.charset.StandardCharsets
import java.nio.file.FileVisitResult
import java.nio.file.FileVisitOption
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.Paths
import java.nio.file.SimpleFileVisitor
import java.nio.file.attribute.BasicFileAttributes
import java.time.Duration
import java.util.EnumSet
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

private const val DEFAULT_OPERATION_TIMEOUT_MS = 5_000L
private const val HOLON_PACKAGE_SCHEMA = "holon-package/v1"

private val discoveryJson = Json { ignoreUnknownKeys = true }
private val excludedDirNames = setOf(".git", ".op", "node_modules", "vendor", "build", "testdata")

internal var sourceDiscoverOffload: (scope: Int, expression: String?, root: String, specifiers: Int, limit: Int, timeout: Int) -> DiscoverResult =
    { scope, expression, root, specifiers, limit, timeout ->
        DiscoverResult(
            error = "source discovery requires local op offload (scope=$scope, expression=${expression ?: "null"}, root=$root, specifiers=0x${specifiers.toString(16)}, limit=$limit, timeout=$timeout)",
        )
    }

internal var packageDescribeProbeOverride: ((packageDir: Path, timeout: Int) -> HolonRef)? = null

private data class DiscoveryCandidate(
    val ref: HolonRef,
    val relativePath: String,
) {
    val pathDepth: Int = pathDepth(relativePath)
    val dedupeKey: String = candidateKey(ref, relativePath)
}

private data class DiscoveryLayer(
    val flag: Int,
    val name: String,
    val scan: (Path, String, Deadline?) -> List<DiscoveryCandidate>,
)

private class Deadline(timeoutMs: Int) {
    private val expiresAt = if (timeoutMs > 0) {
        System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMs.toLong())
    } else {
        Long.MAX_VALUE
    }

    fun check() {
        require(!expired()) { "discover timed out" }
    }

    fun remainingMillisOrDefault(defaultMs: Long = DEFAULT_OPERATION_TIMEOUT_MS): Long {
        if (expiresAt == Long.MAX_VALUE) {
            return defaultMs
        }
        val remaining = TimeUnit.NANOSECONDS.toMillis(expiresAt - System.nanoTime())
        return remaining.coerceAtLeast(1L)
    }

    private fun expired(): Boolean = expiresAt != Long.MAX_VALUE && System.nanoTime() > expiresAt
}

fun Discover(scope: Int, expression: String?, root: String?, specifiers: Int, limit: Int, timeout: Int): DiscoverResult {
    if (scope != LOCAL) {
        return DiscoverResult(error = "scope $scope not supported")
    }
    if (specifiers < 0 || specifiers and ALL.inv() != 0) {
        return DiscoverResult(error = "invalid specifiers 0x${specifiers.toString(16)}: valid range is 0x00-0x3F")
    }
    if (limit < 0) {
        return DiscoverResult(found = emptyList())
    }

    val effectiveSpecifiers = if (specifiers == 0) ALL else specifiers
    val trimmedExpression = expression?.trim()
    if (trimmedExpression != null && trimmedExpression.isEmpty()) {
        return DiscoverResult(found = emptyList())
    }

    val searchRoot = try {
        resolveDiscoverRoot(root)
    } catch (t: Throwable) {
        return DiscoverResult(error = t.message ?: "invalid root")
    }

    val deadline = Deadline(timeout)

    val direct = try {
        discoverPathExpression(trimmedExpression, searchRoot, effectiveSpecifiers, timeout, deadline)
    } catch (t: Throwable) {
        return DiscoverResult(error = t.message ?: "path discovery failed")
    }
    if (direct != null) {
        return if (limit > 0) direct.copy(found = direct.found.take(limit)) else direct
    }

    val layers = listOf(
        DiscoveryLayer(SIBLINGS, "siblings") { _, _, d ->
            discoverPackagesDirect(siblingsRoot(), "siblings", d)
        },
        DiscoveryLayer(CWD, "cwd") { resolvedRoot, _, d ->
            discoverPackagesRecursive(resolvedRoot, "cwd", d)
        },
        DiscoveryLayer(SOURCE, "source") { resolvedRoot, expr, d ->
            d?.check()
            val result = sourceDiscoverOffload(LOCAL, expr.ifEmpty { null }, resolvedRoot.toString(), SOURCE, NO_LIMIT, timeout)
            if (result.error != null) {
                throw IllegalStateException(result.error)
            }
            result.found.map { DiscoveryCandidate(it, relativePathFromRef(resolvedRoot, it.url)) }
        },
        DiscoveryLayer(BUILT, "built") { resolvedRoot, _, d ->
            discoverPackagesDirect(resolvedRoot.resolve(".op").resolve("build"), "built", d)
        },
        DiscoveryLayer(INSTALLED, "installed") { _, _, d ->
            discoverPackagesDirect(opBin(), "installed", d)
        },
        DiscoveryLayer(CACHED, "cached") { _, _, d ->
            discoverPackagesRecursive(cacheDir(), "cached", d)
        },
    )

    val found = mutableListOf<HolonRef>()
    val seen = linkedSetOf<String>()

    return try {
        for (layer in layers) {
            if (effectiveSpecifiers and layer.flag == 0) {
                continue
            }
            deadline.check()
            val candidates = layer.scan(searchRoot, trimmedExpression.orEmpty(), deadline)
            for (candidate in candidates) {
                if (!matchesExpression(candidate.ref, trimmedExpression)) {
                    continue
                }
                if (!seen.add(candidate.dedupeKey)) {
                    continue
                }
                found += candidate.ref
                if (limit > 0 && found.size >= limit) {
                    return DiscoverResult(found = found)
                }
            }
        }
        DiscoverResult(found = found)
    } catch (t: Throwable) {
        DiscoverResult(error = t.message ?: "discover failed")
    }
}

fun resolve(scope: Int, expression: String, root: String?, specifiers: Int, timeout: Int): ResolveResult {
    val result = Discover(scope, expression, root, specifiers, 1, timeout)
    if (result.error != null) {
        return ResolveResult(error = result.error)
    }
    val ref = result.found.firstOrNull()
        ?: return ResolveResult(error = "holon \"$expression\" not found")
    if (ref.error != null) {
        return ResolveResult(ref = ref, error = ref.error)
    }
    return ResolveResult(ref = ref)
}

private fun discoverPathExpression(
    expression: String?,
    searchRoot: Path,
    specifiers: Int,
    timeout: Int,
    deadline: Deadline,
): DiscoverResult? {
    val candidate = pathExpressionCandidate(expression, searchRoot) ?: return null
    deadline.check()

    if (!Files.exists(candidate)) {
        return DiscoverResult(found = emptyList())
    }

    if (Files.isDirectory(candidate)) {
        if (isPackageDirectory(candidate)) {
            return DiscoverResult(found = listOf(loadPackageRef(searchRoot, candidate, "path", timeout)))
        }
        if (specifiers and SOURCE != 0) {
            val result = sourceDiscoverOffload(LOCAL, expression, searchRoot.toString(), SOURCE, 1, timeout)
            return if (result.error != null) DiscoverResult(error = result.error) else DiscoverResult(found = result.found)
        }
        return DiscoverResult(found = emptyList())
    }

    return when (candidate.fileName?.toString()) {
        ".holon.json" -> {
            val dir = candidate.parent ?: return DiscoverResult(found = emptyList())
            DiscoverResult(found = listOf(loadPackageRef(searchRoot, dir, "path", timeout)))
        }
        Identity.PROTO_MANIFEST_FILE_NAME -> {
            if (specifiers and SOURCE == 0) {
                DiscoverResult(found = emptyList())
            } else {
                val result = sourceDiscoverOffload(LOCAL, expression, searchRoot.toString(), SOURCE, 1, timeout)
                if (result.error != null) DiscoverResult(error = result.error) else DiscoverResult(found = result.found)
            }
        }
        else -> DiscoverResult(found = listOf(probeBinaryRef(candidate, timeout)))
    }
}

private fun discoverPackagesDirect(root: Path, origin: String, deadline: Deadline?): List<DiscoveryCandidate> =
    discoverPackages(root, origin, deadline, recursive = false)

private fun discoverPackagesRecursive(root: Path, origin: String, deadline: Deadline?): List<DiscoveryCandidate> =
    discoverPackages(root, origin, deadline, recursive = true)

private fun discoverPackages(
    root: Path,
    origin: String,
    deadline: Deadline?,
    recursive: Boolean,
): List<DiscoveryCandidate> {
    val resolvedRoot = root.toAbsolutePath().normalize()
    if (!Files.isDirectory(resolvedRoot)) {
        return emptyList()
    }

    val dirs = if (recursive) {
        recursivePackageDirs(resolvedRoot, deadline)
    } else {
        directPackageDirs(resolvedRoot)
    }

    val chosen = linkedMapOf<String, DiscoveryCandidate>()
    for (dir in dirs) {
        deadline?.check()
        val ref = loadPackageRef(resolvedRoot, dir, origin, deadline?.remainingMillisOrDefault()?.toInt() ?: NO_TIMEOUT)
        val candidate = DiscoveryCandidate(ref, relativePath(resolvedRoot, dir))
        val existing = chosen[candidate.dedupeKey]
        if (existing == null || candidate.pathDepth < existing.pathDepth) {
            chosen[candidate.dedupeKey] = candidate
        }
    }

    return chosen.values.sortedWith(compareBy<DiscoveryCandidate> { it.relativePath }.thenBy { it.ref.info?.uuid ?: it.ref.url })
}

private fun directPackageDirs(root: Path): List<Path> {
    if (!Files.isDirectory(root)) {
        return emptyList()
    }
    Files.list(root).use { stream ->
        return stream
            .filter { Files.isDirectory(it) && isPackageDirectory(it) }
            .sorted(compareBy(Path::toString))
            .toList()
    }
}

private fun recursivePackageDirs(root: Path, deadline: Deadline?): List<Path> {
    if (!Files.isDirectory(root)) {
        return emptyList()
    }

    val dirs = mutableListOf<Path>()
    Files.walkFileTree(
        root,
        EnumSet.noneOf(FileVisitOption::class.java),
        Int.MAX_VALUE,
        object : SimpleFileVisitor<Path>() {
            override fun preVisitDirectory(dir: Path, attrs: BasicFileAttributes): FileVisitResult {
                deadline?.check()
                if (dir == root) {
                    return FileVisitResult.CONTINUE
                }
                val name = dir.fileName?.toString().orEmpty()
                if (isPackageDirectory(dir)) {
                    dirs.add(dir)
                    return FileVisitResult.SKIP_SUBTREE
                }
                if (shouldSkipDirectory(name)) {
                    return FileVisitResult.SKIP_SUBTREE
                }
                return FileVisitResult.CONTINUE
            }
        },
    )
    return dirs.sortedBy(Path::toString)
}

private fun loadPackageRef(root: Path, dir: Path, origin: String, timeout: Int): HolonRef {
    val jsonError = runCatching { loadPackageInfo(dir) }.fold(
        onSuccess = { info ->
            return HolonRef(url = fileURL(dir), info = info)
        },
        onFailure = { it },
    )

    val probed = runCatching {
        packageDescribeProbeOverride?.invoke(dir, timeout) ?: probePackageRef(dir, timeout)
    }.getOrElse { t ->
        return HolonRef(url = fileURL(dir), error = t.message ?: jsonError.message ?: "package probe failed")
    }

    return if (probed.url.isBlank()) {
        probed.copy(url = fileURL(dir))
    } else {
        probed
    }
}

private fun loadPackageInfo(dir: Path): HolonInfo {
    val manifestPath = dir.resolve(".holon.json")
    val document = discoveryJson.parseToJsonElement(Files.readString(manifestPath)).jsonObject

    val schema = document.string("schema")
    require(schema.isBlank() || schema == HOLON_PACKAGE_SCHEMA) {
        "unsupported .holon.json schema \"$schema\""
    }

    val identity = document.objectValue("identity")
    val givenName = identity.string("given_name")
    val familyName = identity.string("family_name")
    return HolonInfo(
        slug = document.string("slug").ifBlank { slugify(givenName, familyName) },
        uuid = document.string("uuid"),
        identity = IdentityInfo(
            givenName = givenName,
            familyName = familyName,
            motto = identity.string("motto"),
            aliases = identity.stringList("aliases"),
        ),
        lang = document.string("lang"),
        runner = document.string("runner"),
        status = document.string("status"),
        kind = document.string("kind"),
        transport = document.string("transport"),
        entrypoint = document.string("entrypoint"),
        architectures = document.stringList("architectures"),
        hasDist = document.boolean("has_dist"),
        hasSource = document.boolean("has_source"),
    )
}

private fun probePackageRef(dir: Path, timeout: Int): HolonRef {
    val binaryPath = packageBinaryPath(dir)
    val info = probeBinaryInfo(binaryPath, timeout)
    return HolonRef(url = fileURL(dir), info = info.copy(hasSource = false))
}

private fun probeBinaryRef(binaryPath: Path, timeout: Int): HolonRef =
    runCatching { HolonRef(url = fileURL(binaryPath), info = probeBinaryInfo(binaryPath, timeout)) }
        .getOrElse { HolonRef(url = fileURL(binaryPath), error = it.message ?: "binary probe failed") }

private fun probeBinaryInfo(binaryPath: Path, timeout: Int): HolonInfo {
    require(Files.isRegularFile(binaryPath)) { "$binaryPath is not a regular file" }

    val duration = operationTimeout(timeout)
    val process = ProcessBuilder(binaryPath.toString(), "serve", "--listen", "stdio://").start()
    val bridge = DiscoverStdioBridge(process)
    val channel = try {
        ManagedChannelBuilder.forAddress("127.0.0.1", bridge.port()).usePlaintext().build().also {
            waitForReady(it, duration)
        }
    } catch (t: Throwable) {
        bridge.close()
        stopProcess(process)
        throw t
    }

    return try {
        val response = ClientCalls.blockingUnaryCall(
            channel,
            Describe.describeMethod(),
            CallOptions.DEFAULT.withDeadlineAfter(duration.toMillis(), TimeUnit.MILLISECONDS),
            HolonsDescribe.DescribeRequest.getDefaultInstance(),
        )
        holonInfoFromDescribeResponse(response)
    } finally {
        runCatching { channel.shutdownNow() }
        runCatching { channel.awaitTermination(2, TimeUnit.SECONDS) }
        bridge.close()
        stopProcess(process)
    }
}

private fun holonInfoFromDescribeResponse(response: HolonsDescribe.DescribeResponse): HolonInfo {
    val manifest = requireNotNull(response.manifest) { "Describe returned no manifest" }
    val identity = requireNotNull(manifest.identity) { "Describe returned no manifest identity" }
    return HolonInfo(
        slug = slugify(identity.givenName, identity.familyName),
        uuid = identity.uuid,
        identity = IdentityInfo(
            givenName = identity.givenName,
            familyName = identity.familyName,
            motto = identity.motto,
            aliases = identity.aliasesList.toList(),
        ),
        lang = manifest.lang,
        runner = if (manifest.hasBuild()) manifest.build.runner else "",
        status = identity.status,
        kind = manifest.kind,
        transport = manifest.transport,
        entrypoint = if (manifest.hasArtifacts()) manifest.artifacts.binary else "",
        architectures = manifest.platformsList.toList(),
        hasDist = false,
        hasSource = false,
    )
}

private fun matchesExpression(ref: HolonRef, expression: String?): Boolean {
    if (expression == null) {
        return true
    }
    if (expression.isBlank()) {
        return false
    }

    val info = ref.info
    if (info != null) {
        if (info.slug == expression) {
            return true
        }
        if (info.uuid.startsWith(expression)) {
            return true
        }
        if (info.identity.aliases.any { it == expression }) {
            return true
        }
    }

    val path = runCatching { pathFromFileURL(ref.url) }.getOrNull()
    if (path != null) {
        val baseName = path.fileName?.toString().orEmpty()
        val strippedBase = baseName.removeSuffix(".holon")
        if (expression == baseName || expression == strippedBase) {
            return true
        }
    }

    return false
}

private fun candidateKey(ref: HolonRef, relativePath: String): String =
    ref.info?.uuid?.trim().orEmpty().ifEmpty {
        ref.url.ifBlank { relativePath }
    }

private fun pathExpressionCandidate(expression: String?, searchRoot: Path): Path? {
    val trimmed = expression?.trim().orEmpty()
    if (trimmed.isEmpty()) {
        return null
    }
    if (trimmed.startsWith("file://", ignoreCase = true)) {
        return pathFromFileURL(trimmed)
    }
    val isPathLike = Paths.get(trimmed).isAbsolute ||
        trimmed.startsWith(".") ||
        trimmed.contains('/') ||
        trimmed.contains('\\') ||
        trimmed.endsWith(".holon")
    if (!isPathLike) {
        return null
    }
    return if (Paths.get(trimmed).isAbsolute) Path.of(trimmed) else searchRoot.resolve(trimmed).normalize()
}

private fun resolveDiscoverRoot(root: String?): Path {
    if (root == null) {
        return Path.of(System.getProperty("user.dir", ".")).toAbsolutePath().normalize()
    }
    val trimmed = root.trim()
    require(trimmed.isNotEmpty()) { "root cannot be empty" }
    val resolved = Path.of(trimmed).toAbsolutePath().normalize()
    require(Files.exists(resolved)) { "root \"$trimmed\" does not exist" }
    require(Files.isDirectory(resolved)) { "root \"$trimmed\" is not a directory" }
    return resolved
}

private fun siblingsRoot(): Path {
    val configured = getenvOrProperty("holons.siblings.root", "HOLONS_SIBLINGS_ROOT")
    if (configured.isNotBlank()) {
        return Path.of(configured).toAbsolutePath().normalize()
    }
    return Path.of("__missing_siblings_root__")
}

private fun opPath(): Path {
    val configured = getenvOrProperty("OPPATH")
    if (configured.isNotBlank()) {
        return Path.of(configured).toAbsolutePath().normalize()
    }
    return Path.of(System.getProperty("user.home", "."), ".op").toAbsolutePath().normalize()
}

private fun opBin(): Path {
    val configured = getenvOrProperty("OPBIN")
    if (configured.isNotBlank()) {
        return Path.of(configured).toAbsolutePath().normalize()
    }
    return opPath().resolve("bin")
}

private fun cacheDir(): Path = opPath().resolve("cache")

private fun isPackageDirectory(dir: Path): Boolean =
    Files.isDirectory(dir) && dir.fileName?.toString().orEmpty().endsWith(".holon")

private fun shouldSkipDirectory(name: String): Boolean {
    if (name.endsWith(".holon")) {
        return false
    }
    if (name in excludedDirNames) {
        return true
    }
    return name.startsWith(".")
}

private fun relativePath(root: Path, dir: Path): String {
    val rel = runCatching { root.relativize(dir).toString() }.getOrElse { dir.toString() }
    return rel.replace('\\', '/').ifEmpty { "." }
}

private fun relativePathFromRef(root: Path, url: String): String {
    val path = runCatching { pathFromFileURL(url) }.getOrNull() ?: return url
    return relativePath(root, path)
}

private fun pathDepth(path: String): Int {
    val trimmed = path.trim().trim('/').removePrefix("./")
    if (trimmed.isEmpty() || trimmed == ".") {
        return 0
    }
    return trimmed.split('/').size
}

internal fun fileURL(path: Path): String = "file://${path.toAbsolutePath().normalize().toString().replace('\\', '/')}"

internal fun pathFromFileURL(raw: String): Path {
    require(raw.startsWith("file://", ignoreCase = true)) { "holon URL \"$raw\" is not a local file target" }
    val withoutScheme = raw.removePrefix("file://")
    val decoded = URLDecoder.decode(withoutScheme, StandardCharsets.UTF_8)
    val normalized = if (decoded.startsWith("/") || decoded.startsWith("//")) decoded else "/$decoded"
    return Path.of(normalized).toAbsolutePath().normalize()
}

internal fun packageBinaryPath(dir: Path): Path {
    val archDir = dir.resolve("bin").resolve(currentPlatformDirName())
    require(Files.isDirectory(archDir)) { "package binary directory missing at $archDir" }
    Files.list(archDir).use { stream ->
        val candidate = stream
            .filter { Files.isRegularFile(it) }
            .sorted(compareBy(Path::toString))
            .findFirst()
            .orElse(null)
        require(candidate != null) { "package binary not found under $archDir" }
        return candidate
    }
}

internal fun currentPlatformDirName(): String = "${normalizedOs()}_${normalizedArch()}"

private fun normalizedOs(): String {
    val value = System.getProperty("os.name", "").lowercase()
    return when {
        value.contains("mac") || value.contains("darwin") -> "darwin"
        value.contains("win") -> "windows"
        value.contains("linux") -> "linux"
        else -> value.replace(Regex("[^a-z0-9]+"), "")
    }
}

private fun normalizedArch(): String {
    val value = System.getProperty("os.arch", "").lowercase()
    return when (value) {
        "x86_64", "amd64" -> "amd64"
        "aarch64", "arm64" -> "arm64"
        else -> value.replace(Regex("[^a-z0-9]+"), "")
    }
}

internal fun operationTimeout(timeout: Int): Duration =
    if (timeout <= 0) Duration.ofMillis(DEFAULT_OPERATION_TIMEOUT_MS) else Duration.ofMillis(timeout.toLong())

internal fun waitForReady(channel: ManagedChannel, timeout: Duration) {
    val deadline = System.nanoTime() + timeout.toNanos()
    var state = channel.getState(true)
    while (state != ConnectivityState.READY) {
        require(state != ConnectivityState.SHUTDOWN) { "gRPC channel shut down before becoming ready" }

        val remaining = deadline - System.nanoTime()
        require(remaining > 0) { "timed out waiting for gRPC readiness" }

        val latch = CountDownLatch(1)
        channel.notifyWhenStateChanged(state) { latch.countDown() }
        if (!latch.await(remaining, TimeUnit.NANOSECONDS)) {
            error("timed out waiting for gRPC readiness")
        }
        state = channel.getState(false)
    }
}

internal fun normalizeDialTarget(target: String): String {
    if (!target.contains("://")) {
        return target
    }
    val parsed = Transport.parseURI(target)
    return when (parsed.scheme) {
        "tcp" -> {
            val host = if (parsed.host.isNullOrBlank() || parsed.host == "0.0.0.0") "127.0.0.1" else parsed.host
            "$host:${parsed.port}"
        }
        "unix" -> "unix://${parsed.path}"
        else -> target
    }
}

internal fun stopProcess(process: Process?) {
    if (process == null || !process.isAlive) {
        return
    }
    process.destroy()
    if (!process.waitFor(2, TimeUnit.SECONDS)) {
        process.destroyForcibly()
        process.waitFor(2, TimeUnit.SECONDS)
    }
}

private fun getenvOrProperty(vararg names: String): String {
    for (name in names) {
        val prop = System.getProperty(name)
        if (!prop.isNullOrBlank()) {
            return prop.trim()
        }
        val env = System.getenv(name)
        if (!env.isNullOrBlank()) {
            return env.trim()
        }
    }
    return ""
}

private fun slugify(givenName: String, familyName: String): String =
    listOf(givenName.trim(), familyName.trim().removeSuffix("?"))
        .filter { it.isNotEmpty() }
        .joinToString("-")
        .lowercase()
        .replace(" ", "-")
        .trim('-')

private fun JsonObject.string(key: String): String =
    (this[key] as? JsonPrimitive)?.content?.trim().orEmpty()

private fun JsonObject.boolean(key: String): Boolean =
    (this[key] as? JsonPrimitive)?.booleanOrNull ?: false

private fun JsonObject.stringList(key: String): List<String> =
    (this[key] as? JsonArray)
        ?.mapNotNull { (it as? JsonPrimitive)?.content?.trim() }
        ?.filter { it.isNotEmpty() }
        ?: emptyList()

private fun JsonObject.objectValue(key: String): JsonObject =
    (this[key] as? JsonObject) ?: emptyMap<String, JsonElement>().let(::JsonObject)

private class DiscoverStdioBridge(private val process: Process) : Closeable {
    private val listener = ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"))
    private val acceptThread = Thread(::acceptLoop, "holons-discover-stdio-bridge").apply {
        isDaemon = true
        start()
    }

    @Volatile
    private var socket: Socket? = null

    @Volatile
    private var closed = false

    fun port(): Int = listener.localPort

    override fun close() {
        closed = true
        runCatching { listener.close() }
        socket?.let { runCatching { it.close() } }
        socket = null
        runCatching { process.outputStream.close() }
        runCatching { process.inputStream.close() }
        runCatching { process.errorStream.close() }
        runCatching { acceptThread.join(200) }
    }

    private fun acceptLoop() {
        try {
            val accepted = listener.accept()
            if (closed) {
                accepted.close()
                return
            }
            socket = accepted

            val upstream = pump(
                accepted.getInputStream(),
                process.outputStream,
                closeOutput = true,
                name = "holons-discover-stdio-up",
            )
            val downstream = pump(
                process.inputStream,
                accepted.getOutputStream(),
                closeOutput = true,
                name = "holons-discover-stdio-down",
            )

            drain(process.errorStream, "holons-discover-stdio-stderr")
            upstream.join()
            downstream.join()
        } catch (_: IOException) {
            // Closed during shutdown.
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
        } finally {
            socket?.let { runCatching { it.close() } }
            socket = null
        }
    }

    private fun pump(input: InputStream, output: OutputStream, closeOutput: Boolean, name: String): Thread =
        Thread({
            val buffer = ByteArray(16 * 1024)
            try {
                while (true) {
                    val read = input.read(buffer)
                    if (read < 0) {
                        break
                    }
                    output.write(buffer, 0, read)
                    output.flush()
                }
            } catch (_: IOException) {
                // Closed during shutdown.
            } finally {
                if (closeOutput) {
                    runCatching { output.close() }
                }
            }
        }, name).apply {
            isDaemon = true
            start()
        }

    private fun drain(stream: InputStream, name: String) {
        Thread({
            val reader = BufferedReader(InputStreamReader(stream, StandardCharsets.UTF_8))
            while (runCatching { reader.readLine() }.getOrNull() != null) {
                // Drain stderr so subprocesses do not block.
            }
        }, name).apply {
            isDaemon = true
            start()
        }
    }
}
