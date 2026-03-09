package org.organicprogramming.hello

import io.grpc.CallOptions
import io.grpc.ManagedChannel
import io.grpc.MethodDescriptor
import io.grpc.stub.ClientCalls
import kotlinx.coroutines.runBlocking
import org.organicprogramming.holons.Connect
import java.io.ByteArrayInputStream
import java.io.IOException
import java.io.InputStream
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.TimeUnit

private val pingMethod: MethodDescriptor<String, String> =
    MethodDescriptor.newBuilder<String, String>()
        .setType(MethodDescriptor.MethodType.UNARY)
        .setFullMethodName(MethodDescriptor.generateFullMethodName("echo.v1.Echo", "Ping"))
        .setRequestMarshaller(StringMarshaller)
        .setResponseMarshaller(StringMarshaller)
        .build()

fun main() = runBlocking {
    val tempRoot = Files.createTempDirectory("kotlin-holons-connect-")
    val previousDir = System.getProperty("user.dir", ".")

    try {
        writeEchoHolon(tempRoot, resolveEchoServer(previousDir))
        System.setProperty("user.dir", tempRoot.toString())

        val channel = Connect.connect("echo-server")
        try {
            val response = ClientCalls.blockingUnaryCall(
                channel,
                pingMethod,
                CallOptions.DEFAULT.withDeadlineAfter(5, TimeUnit.SECONDS),
                """{"message":"hello-from-kotlin"}""",
            )
            println(response)
        } finally {
            Connect.disconnect(channel)
        }
    } finally {
        System.setProperty("user.dir", previousDir)
        tempRoot.toFile().deleteRecursively()
    }
}

private fun resolveEchoServer(projectDir: String): String {
    val path = Path.of(projectDir)
        .resolve("../../sdk/kotlin-holons/bin/echo-server")
        .toAbsolutePath()
        .normalize()
    check(Files.isRegularFile(path)) { "echo-server not found at $path" }
    return path.toString()
}

private fun writeEchoHolon(root: Path, binaryPath: String) {
    val holonDir = root.resolve("holons/echo-server")
    Files.createDirectories(holonDir)
    Files.writeString(
        holonDir.resolve("holon.yaml"),
        """
        uuid: "echo-server-connect-example"
        given_name: Echo
        family_name: Server
        motto: Reply precisely.
        composer: "connect-example"
        kind: service
        build:
          runner: kotlin
          main: bin/echo-server
        artifacts:
          binary: "$binaryPath"
        """.trimIndent() + System.lineSeparator(),
    )
}

private object StringMarshaller : MethodDescriptor.Marshaller<String> {
    override fun stream(value: String): InputStream =
        ByteArrayInputStream(value.toByteArray(StandardCharsets.UTF_8))

    override fun parse(stream: InputStream): String =
        try {
            String(stream.readAllBytes(), StandardCharsets.UTF_8)
        } catch (error: IOException) {
            throw RuntimeException(error)
        }
}
