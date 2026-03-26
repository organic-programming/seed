package org.organicprogramming.holons

import java.io.File
import java.nio.ByteBuffer
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class HolonsTest {
    @Test fun schemeExtraction() {
        assertEquals("tcp", Transport.scheme("tcp://:9090"))
        assertEquals("unix", Transport.scheme("unix:///tmp/x.sock"))
        assertEquals("stdio", Transport.scheme("stdio://"))
        assertEquals("ws", Transport.scheme("ws://127.0.0.1:8080/grpc"))
        assertEquals("wss", Transport.scheme("wss://example.com:443/grpc"))
    }

    @Test fun defaultUri() {
        assertEquals("tcp://:9090", Transport.DEFAULT_URI)
    }

    @Test fun tcpListen() {
        val lis = Transport.listen("tcp://127.0.0.1:0")
        val tcp = lis as Transport.Listener.Tcp
        assertTrue(tcp.socket.localPort > 0)
        tcp.socket.close()
    }

    @Test fun parseUriWss() {
        val parsed = Transport.parseURI("wss://example.com:8443")
        assertEquals("wss", parsed.scheme)
        assertEquals("example.com", parsed.host)
        assertEquals(8443, parsed.port)
        assertEquals("/grpc", parsed.path)
        assertTrue(parsed.secure)
    }

    @Test fun stdioListenVariant() {
        assertEquals(Transport.Listener.Stdio, Transport.listen("stdio://"))
    }

    @Test fun unixListenAndDialRoundTrip() {
        val path = File.createTempFile("holons-kotlin", ".sock").absolutePath
        File(path).delete()
        val uri = "unix://$path"

        val lis = Transport.listen(uri) as Transport.Listener.Unix
        val serverError = arrayOfNulls<Throwable>(1)

        val server = Thread {
            try {
                lis.channel.accept().use { accepted ->
                    val input = ByteBuffer.allocate(4)
                    while (input.hasRemaining()) {
                        accepted.read(input)
                    }
                    input.flip()
                    while (input.hasRemaining()) {
                        accepted.write(input)
                    }
                }
            } catch (t: Throwable) {
                serverError[0] = t
            }
        }
        server.start()

        Transport.dialUnix(uri).use { client ->
            client.write(ByteBuffer.wrap("ping".toByteArray(StandardCharsets.UTF_8)))
            val out = ByteBuffer.allocate(4)
            while (out.hasRemaining()) {
                client.read(out)
            }
            assertEquals("ping", String(out.array(), StandardCharsets.UTF_8))
        }

        server.join(3000)
        lis.channel.close()
        serverError[0]?.let { throw AssertionError("unix server failed", it) }
    }

    @Test fun wsListenVariant() {
        val ws = Transport.listen("ws://127.0.0.1:8080/holon") as Transport.Listener.WS
        assertEquals("127.0.0.1", ws.host)
        assertEquals(8080, ws.port)
        assertEquals("/holon", ws.path)
        assertTrue(!ws.secure)
    }

    @Test fun unsupportedUri() {
        assertFailsWith<IllegalArgumentException> { Transport.listen("ftp://host") }
    }

    @Test fun parseFlagsListen() {
        assertEquals("tcp://:8080", Serve.parseFlags(arrayOf("--listen", "tcp://:8080")))
    }

    @Test fun parseFlagsPort() {
        assertEquals("tcp://:3000", Serve.parseFlags(arrayOf("--port", "3000")))
    }

    @Test fun parseFlagsDefault() {
        assertEquals(Transport.DEFAULT_URI, Serve.parseFlags(emptyArray()))
    }

    @Test fun parseHolon() {
        val root = Files.createTempDirectory("holons-kotlin-identity-")
        val manifest = root.resolve("holon.proto")
        try {
            manifest.toFile().writeText(
                """
                syntax = "proto3";
                package test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    uuid: "abc-123"
                    given_name: "test"
                    family_name: "Test"
                    version: "1.2.3"
                  }
                  lang: "kotlin"
                };
                """.trimIndent()
            )

            val id = Identity.parseHolon(manifest.toString())
            assertEquals("abc-123", id.uuid)
            assertEquals("test", id.givenName)
            assertEquals("kotlin", id.lang)
            assertEquals("1.2.3", id.version)

            val resolved = Identity.parseManifest(manifest)
            assertEquals(manifest.toAbsolutePath().normalize(), resolved.sourcePath)
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test fun parseInvalidMapping() {
        val root = Files.createTempDirectory("holons-kotlin-invalid-identity-")
        val manifest = root.resolve("holon.proto")
        try {
            manifest.toFile().writeText("syntax = \"proto3\";\npackage test.v1;\n")
            assertFailsWith<IllegalArgumentException> { Identity.parseHolon(manifest.toString()) }
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test fun echoClientScriptUsesGoHelperAndDefaultGocache() {
        val run = runEchoClientScript(
            args = listOf("--message", "cert", "tcp://127.0.0.1:19090"),
            gocache = null,
        )

        assertEquals(0, run.exitCode)
        assertTrue(run.stdout.contains("\"status\":\"pass\""))
        assertEquals(
            File("..", "go-holons").canonicalPath,
            File(run.workingDirectory).canonicalPath,
        )
        assertEquals("/tmp/go-cache", run.gocache)

        assertEquals("run", run.arguments[0])
        assertTrue(run.arguments[1].endsWith("/kotlin-holons/cmd/echo-client-go/main.go"))
        assertEquals("--sdk", run.arguments[2])
        assertEquals("kotlin-holons", run.arguments[3])
        assertEquals("--server-sdk", run.arguments[4])
        assertEquals("go-holons", run.arguments[5])
        assertEquals("--message", run.arguments[6])
        assertEquals("cert", run.arguments[7])
        assertEquals("tcp://127.0.0.1:19090", run.arguments[8])
    }

    @Test fun echoClientScriptPreservesProvidedGocache() {
        val run = runEchoClientScript(
            args = listOf("stdio://"),
            gocache = "/tmp/kotlin-holons-custom-cache",
        )

        assertEquals(0, run.exitCode)
        assertEquals("/tmp/kotlin-holons-custom-cache", run.gocache)
    }

    @Test fun echoClientScriptForwardsWebSocketTarget() {
        val run = runEchoClientScript(
            args = listOf("--message", "cert", "ws://127.0.0.1:28080/grpc"),
            gocache = null,
        )

        assertEquals(0, run.exitCode)
        assertTrue(run.arguments.contains("ws://127.0.0.1:28080/grpc"))
    }

    @Test fun echoServerScriptUsesGoHelperAndDefaultGocache() {
        val run = runEchoServerScript(
            args = listOf("--listen", "tcp://127.0.0.1:0"),
            gocache = null,
        )

        assertEquals(0, run.exitCode)
        assertEquals(
            File("..", "go-holons").canonicalPath,
            File(run.workingDirectory).canonicalPath,
        )
        assertEquals("/tmp/go-cache", run.gocache)

        assertEquals("run", run.arguments[0])
        assertTrue(run.arguments[1].endsWith("/kotlin-holons/cmd/echo-server-go/main.go"))
        assertEquals("--sdk", run.arguments[2])
        assertEquals("kotlin-holons", run.arguments[3])
        assertEquals("--listen", run.arguments[4])
        assertEquals("tcp://127.0.0.1:0", run.arguments[5])
    }

    @Test fun echoServerScriptPreservesServeModeArguments() {
        val run = runEchoServerScript(
            args = listOf("serve", "--listen", "stdio://", "--version", "0.2.0"),
            gocache = "/tmp/kotlin-holons-custom-cache",
        )

        assertEquals(0, run.exitCode)
        assertEquals("/tmp/kotlin-holons-custom-cache", run.gocache)
        assertEquals("run", run.arguments[0])
        assertTrue(run.arguments[1].endsWith("/kotlin-holons/cmd/echo-server-go/main.go"))
        assertEquals("serve", run.arguments[2])
        assertEquals("--sdk", run.arguments[3])
        assertEquals("kotlin-holons", run.arguments[4])
        assertEquals("--listen", run.arguments[5])
        assertEquals("stdio://", run.arguments[6])
        assertEquals("--version", run.arguments[7])
        assertEquals("0.2.0", run.arguments[8])
    }

    @Test fun holonRpcServerScriptInjectsDefaultSdkAndDefaultGocache() {
        val run = runHolonRpcServerScript(
            args = listOf("ws://127.0.0.1:8080/rpc", "--once"),
            gocache = null,
        )

        assertEquals(0, run.exitCode)
        assertEquals(
            File("..", "go-holons").canonicalPath,
            File(run.workingDirectory).canonicalPath,
        )
        assertEquals("/tmp/go-cache", run.gocache)

        assertEquals("run", run.arguments[0])
        assertTrue(run.arguments[1].endsWith("/kotlin-holons/cmd/holon-rpc-server-go/main.go"))
        assertEquals("--sdk", run.arguments[2])
        assertEquals("kotlin-holons", run.arguments[3])
        assertEquals("ws://127.0.0.1:8080/rpc", run.arguments[4])
        assertEquals("--once", run.arguments[5])
    }

    @Test fun holonRpcServerScriptPreservesExplicitSdk() {
        val run = runHolonRpcServerScript(
            args = listOf("--sdk", "override-sdk", "ws://127.0.0.1:9090/rpc"),
            gocache = "/tmp/kotlin-holons-custom-cache",
        )

        assertEquals(0, run.exitCode)
        assertEquals("/tmp/kotlin-holons-custom-cache", run.gocache)
        assertEquals("run", run.arguments[0])
        assertTrue(run.arguments[1].endsWith("/kotlin-holons/cmd/holon-rpc-server-go/main.go"))
        assertEquals("--sdk", run.arguments[2])
        assertEquals("override-sdk", run.arguments[3])
    }

    private fun runEchoClientScript(
        args: List<String>,
        gocache: String?,
    ): ScriptRun {
        return runScript("bin/echo-client", args, gocache)
    }

    private fun runEchoServerScript(
        args: List<String>,
        gocache: String?,
    ): ScriptRun {
        return runScript("bin/echo-server", args, gocache)
    }

    private fun runHolonRpcServerScript(
        args: List<String>,
        gocache: String?,
    ): ScriptRun {
        return runScript("bin/holon-rpc-server", args, gocache)
    }

    private fun runScript(
        scriptPath: String,
        args: List<String>,
        gocache: String?,
    ): ScriptRun {
        val script = File(scriptPath)
        assertTrue(script.exists(), "missing ${script.path}")
        assertTrue(script.canExecute(), "not executable: ${script.path}")

        val tmpDir = Files.createTempDirectory("kotlin-echo-client-test").toFile()
        val argsFile = File(tmpDir, "args.txt")
        val pwdFile = File(tmpDir, "pwd.txt")
        val gocacheFile = File(tmpDir, "gocache.txt")
        val fakeGo = File(tmpDir, "go")

        fakeGo.writeText(
            """
            #!/usr/bin/env bash
            set -euo pipefail
            printf '%s\n' "${'$'}PWD" > "${pwdFile.absolutePath}"
            printf '%s\n' "${'$'}{GOCACHE:-}" > "${gocacheFile.absolutePath}"
            : > "${argsFile.absolutePath}"
            for arg in "${'$'}@"; do
              printf '%s\n' "${'$'}arg" >> "${argsFile.absolutePath}"
            done
            printf '%s\n' '{"status":"pass","sdk":"kotlin-holons","server_sdk":"go-holons"}'
            """.trimIndent(),
        )
        fakeGo.setExecutable(true)

        try {
            val process = ProcessBuilder(listOf(script.absolutePath) + args)
                .redirectErrorStream(true)
                .directory(File(System.getProperty("user.dir")))
                .apply {
                    environment()["GO_BIN"] = fakeGo.absolutePath
                    if (gocache == null) {
                        environment().remove("GOCACHE")
                    } else {
                        environment()["GOCACHE"] = gocache
                    }
                }
                .start()

            val stdout = process.inputStream.bufferedReader(StandardCharsets.UTF_8).readText().trim()
            val exitCode = process.waitFor()

            return ScriptRun(
                exitCode = exitCode,
                stdout = stdout,
                arguments = if (argsFile.exists()) argsFile.readLines() else emptyList(),
                workingDirectory = if (pwdFile.exists()) pwdFile.readText().trim() else "",
                gocache = if (gocacheFile.exists()) gocacheFile.readText().trim() else "",
            )
        } finally {
            tmpDir.deleteRecursively()
        }
    }

    private data class ScriptRun(
        val exitCode: Int,
        val stdout: String,
        val arguments: List<String>,
        val workingDirectory: String,
        val gocache: String,
    )
}
