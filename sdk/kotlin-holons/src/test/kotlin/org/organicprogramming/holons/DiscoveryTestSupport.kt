package org.organicprogramming.holons

import io.grpc.CallOptions
import io.grpc.ManagedChannel
import io.grpc.MethodDescriptor
import io.grpc.stub.ClientCalls
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import java.io.ByteArrayInputStream
import java.io.IOException
import java.io.InputStream
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.time.Duration
import java.util.concurrent.TimeUnit
import kotlin.test.assertTrue

private val sdkRoot: Path = Path.of("").toAbsolutePath().normalize()
private val json = Json
private val pingMethod: MethodDescriptor<String, String> = MethodDescriptor.newBuilder<String, String>()
    .setType(MethodDescriptor.MethodType.UNARY)
    .setFullMethodName(MethodDescriptor.generateFullMethodName("echo.v1.Echo", "Ping"))
    .setRequestMarshaller(stringMarshaller())
    .setResponseMarshaller(stringMarshaller())
    .build()

internal data class RuntimeFixture(
    val root: Path,
    val opHome: Path,
    val opBin: Path,
)

internal data class PackageSeed(
    val slug: String,
    val uuid: String,
    val givenName: String,
    val familyName: String,
    val runner: String = "go-module",
    val entrypoint: String,
    val kind: String = "native",
    val aliases: List<String> = emptyList(),
    val architectures: List<String> = emptyList(),
    val hasDist: Boolean = false,
    val hasSource: Boolean = false,
    val transport: String = "",
)

internal data class ConnectFixture(
    val slug: String,
    val sourceDir: Path,
    val pidFile: Path,
    val argsFile: Path,
    val info: HolonInfo,
)

internal fun <T> withRuntimeFixture(block: (RuntimeFixture) -> T): T {
    val root = Files.createTempDirectory("kotlin-holons-runtime-")
    val opHome = root.resolve("runtime")
    val opBin = opHome.resolve("bin")

    val previousUserDir = System.getProperty("user.dir")
    val previousOpPath = System.getProperty("OPPATH")
    val previousOpBin = System.getProperty("OPBIN")
    val previousSiblingsRoot = System.getProperty("holons.siblings.root")
    val previousSourceOffload = sourceDiscoverOffload
    val previousProbeOverride = packageDescribeProbeOverride

    System.setProperty("user.dir", root.toString())
    System.setProperty("OPPATH", opHome.toString())
    System.setProperty("OPBIN", opBin.toString())
    System.clearProperty("holons.siblings.root")

    sourceDiscoverOffload = { _, _, _, _, _, _ -> DiscoverResult() }
    packageDescribeProbeOverride = null

    return try {
        block(RuntimeFixture(root, opHome, opBin))
    } finally {
        restoreProperty("user.dir", previousUserDir)
        restoreProperty("OPPATH", previousOpPath)
        restoreProperty("OPBIN", previousOpBin)
        restoreProperty("holons.siblings.root", previousSiblingsRoot)
        sourceDiscoverOffload = previousSourceOffload
        packageDescribeProbeOverride = previousProbeOverride
        root.toFile().deleteRecursively()
    }
}

internal fun restoreProperty(name: String, value: String?) {
    if (value == null) {
        System.clearProperty(name)
    } else {
        System.setProperty(name, value)
    }
}

internal fun sortedSlugs(result: DiscoverResult): List<String> =
    result.found.mapNotNull { it.info?.slug }.sorted()

internal fun writePackageHolon(dir: Path, seed: PackageSeed) {
    Files.createDirectories(dir)
    val aliases = if (seed.aliases.isEmpty()) "[]" else seed.aliases.joinToString(prefix = "[", postfix = "]") { "\"$it\"" }
    val architectures = if (seed.architectures.isEmpty()) "[]" else seed.architectures.joinToString(prefix = "[", postfix = "]") { "\"$it\"" }
    Files.writeString(
        dir.resolve(".holon.json"),
        """
        {
          "schema": "holon-package/v1",
          "slug": "${seed.slug}",
          "uuid": "${seed.uuid}",
          "identity": {
            "given_name": "${seed.givenName}",
            "family_name": "${seed.familyName}",
            "aliases": $aliases
          },
          "lang": "kotlin",
          "runner": "${seed.runner}",
          "status": "draft",
          "kind": "${seed.kind}",
          "transport": "${seed.transport}",
          "entrypoint": "${seed.entrypoint}",
          "architectures": $architectures,
          "has_dist": ${seed.hasDist},
          "has_source": ${seed.hasSource}
        }
        """.trimIndent() + "\n",
    )
}

internal fun writeStaticDescribePackage(dir: Path) {
    val binaryDir = dir.resolve("bin").resolve(currentPlatformDirName())
    Files.createDirectories(binaryDir)
    val wrapper = binaryDir.resolve("static-wrapper")
    Files.writeString(
        wrapper,
        """
        #!/usr/bin/env bash
        set -euo pipefail
        if [[ "${'$'}{1:-}" == "serve" ]]; then
          shift
        fi
        if [[ "${'$'}{1:-}" == "--listen" ]]; then
          shift
        fi
        exec ${shellQuote(Path.of(System.getProperty("java.home"), "bin", "java").toString())} -cp ${shellQuote(System.getProperty("java.class.path"))} org.organicprogramming.holons.StaticDescribeServerMain "${'$'}{1:-stdio://}"
        """.trimIndent() + "\n",
    )
    assertTrue(wrapper.toFile().setExecutable(true), "static wrapper should be executable")
}

internal fun createConnectFixture(root: Path, givenName: String, familyName: String): ConnectFixture {
    val slug = "$givenName-$familyName".lowercase()
    val sourceDir = root.resolve("holons").resolve(slug)
    val binaryDir = sourceDir.resolve(".op").resolve("build").resolve("bin")
    Files.createDirectories(binaryDir)

    val wrapper = binaryDir.resolve("echo-wrapper")
    val pidFile = root.resolve("$slug.pid")
    val argsFile = root.resolve("$slug.args")

    Files.writeString(
        wrapper,
        """
        #!/usr/bin/env bash
        set -euo pipefail
        printf '%s\n' "${'$'}${'$'}" > ${shellQuote(pidFile.toString())}
        : > ${shellQuote(argsFile.toString())}
        for arg in "${'$'}@"; do
          printf '%s\n' "${'$'}arg" >> ${shellQuote(argsFile.toString())}
        done
        exec ${shellQuote(sdkRoot.resolve("bin").resolve("echo-server").toString())} "${'$'}@"
        """.trimIndent() + "\n",
    )
    assertTrue(wrapper.toFile().setExecutable(true), "echo wrapper should be executable")

    val info = HolonInfo(
        slug = slug,
        uuid = "$slug-uuid",
        identity = IdentityInfo(givenName = givenName, familyName = familyName),
        lang = "kotlin",
        runner = "shell",
        status = "draft",
        kind = "service",
        transport = "",
        entrypoint = "echo-wrapper",
        architectures = emptyList(),
        hasDist = true,
        hasSource = true,
    )

    return ConnectFixture(slug = slug, sourceDir = sourceDir, pidFile = pidFile, argsFile = argsFile, info = info)
}

internal fun connectFixtureRef(fixture: ConnectFixture): HolonRef =
    HolonRef(url = fileURL(fixture.sourceDir), info = fixture.info)

internal fun waitForPidFile(path: Path): Long {
    val deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos()
    while (System.nanoTime() < deadline) {
        try {
            val pid = Files.readString(path).trim().toLong()
            if (pid > 0) {
                return pid
            }
        } catch (_: Exception) {
            // Still starting.
        }
        Thread.sleep(25)
    }
    error("timed out waiting for pid file $path")
}

internal fun waitForArgsFile(path: Path): List<String> {
    val deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos()
    while (System.nanoTime() < deadline) {
        try {
            val lines = Files.readAllLines(path, StandardCharsets.UTF_8).filter { it.isNotBlank() }
            if (lines.isNotEmpty()) {
                return lines
            }
        } catch (_: IOException) {
            // Still starting.
        }
        Thread.sleep(25)
    }
    error("timed out waiting for args file $path")
}

internal fun waitForPidExit(pid: Long) {
    val deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos()
    while (System.nanoTime() < deadline) {
        val alive = ProcessHandle.of(pid).map(ProcessHandle::isAlive).orElse(false)
        if (!alive) {
            return
        }
        Thread.sleep(25)
    }
    error("timed out waiting for pid $pid to exit")
}

internal fun invokePing(channel: ManagedChannel, message: String): JsonObject =
    json.parseToJsonElement(
        ClientCalls.blockingUnaryCall(
            channel,
            pingMethod,
            CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
            """{"message":"$message"}""",
        ),
    ).jsonObject

private fun stringMarshaller() = object : MethodDescriptor.Marshaller<String> {
    override fun stream(value: String): InputStream =
        ByteArrayInputStream(value.toByteArray(StandardCharsets.UTF_8))

    override fun parse(stream: InputStream): String =
        stream.readAllBytes().toString(StandardCharsets.UTF_8)
}

private fun shellQuote(value: String): String = "'" + value.replace("'", "'\"'\"'") + "'"
