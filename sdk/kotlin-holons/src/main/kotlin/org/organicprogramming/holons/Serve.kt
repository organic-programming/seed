package org.organicprogramming.holons

import io.grpc.BindableService
import io.grpc.Server
import io.grpc.ServerServiceDefinition
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder
import io.grpc.protobuf.services.ProtoReflectionService
import java.nio.channels.Channels
import java.nio.channels.ServerSocketChannel
import java.nio.channels.SocketChannel
import java.net.InetSocketAddress
import java.net.Socket
import java.nio.file.Files
import java.nio.file.Path
import java.util.Collections
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.thread
import kotlin.io.path.exists

/** Standard gRPC server runner utilities. */
object Serve {
    data class Options(
        val describe: Boolean = true,
        val reflect: Boolean = false,
        val logger: (String) -> Unit = { message -> System.err.println(message) },
        val onListen: ((String) -> Unit)? = null,
        val shutdownGracePeriodSeconds: Long = 10,
    )

    data class ParsedFlags(
        val listenUri: String = Transport.DEFAULT_URI,
        val reflect: Boolean = false,
    )

    class RunningServer internal constructor(
        private val server: Server,
        val publicUri: String,
        private val logger: (String) -> Unit,
        private val auxiliaryStop: (() -> Unit)? = null,
    ) {
        private val stopped = AtomicBoolean(false)

        fun await() {
            server.awaitTermination()
        }

        fun stop(gracePeriodSeconds: Long = 10) {
            if (!stopped.compareAndSet(false, true)) {
                return
            }

            auxiliaryStop?.invoke()
            server.shutdown()
            if (!server.awaitTermination(gracePeriodSeconds, TimeUnit.SECONDS)) {
                logger("graceful stop timed out after ${gracePeriodSeconds}s; forcing hard stop")
                server.shutdownNow()
                server.awaitTermination(gracePeriodSeconds, TimeUnit.SECONDS)
            }
        }
    }

    private data class BoundServer(
        val server: Server,
        val publicUri: String,
    )

    fun parseFlags(args: Array<String>): String {
        return parseOptions(args).listenUri
    }

    fun parseOptions(args: Array<String>): ParsedFlags {
        var listenUri = Transport.DEFAULT_URI
        var reflect = false
        var i = 0
        while (i < args.size) {
            when {
                args[i] == "--listen" && i + 1 < args.size -> listenUri = args[i + 1]
                args[i] == "--port" && i + 1 < args.size -> listenUri = "tcp://:${args[i + 1]}"
                args[i] == "--reflect" -> reflect = true
            }
            i++
        }
        return ParsedFlags(listenUri, reflect)
    }

    fun run(listenUri: String, vararg services: BindableService) {
        runWithOptions(listenUri, services.asList())
    }

    fun runWithOptions(
        listenUri: String,
        services: Iterable<BindableService>,
        options: Options = Options(),
    ) {
        val running = startWithOptions(listenUri, services, options)
        val shutdownHook = Thread {
            options.logger("shutting down gRPC server")
            running.stop(options.shutdownGracePeriodSeconds)
        }

        Runtime.getRuntime().addShutdownHook(shutdownHook)
        try {
            running.await()
        } finally {
            runCatching { Runtime.getRuntime().removeShutdownHook(shutdownHook) }
        }
    }

    fun startWithOptions(
        listenUri: String,
        services: Iterable<BindableService>,
        options: Options = Options(),
    ): RunningServer {
        val parsed = Transport.parseURI(listenUri.ifBlank { Transport.DEFAULT_URI })
        val resolvedServices = services.toMutableList()
        val extraDefinitions = mutableListOf<ServerServiceDefinition>()
        val describeEnabled =
            try {
                maybeAddDescribe(extraDefinitions, options)
            } catch (error: Exception) {
                val message = error.message ?: error::class.simpleName ?: "unknown error"
                options.logger("HolonMeta registration failed: $message")
                throw error
            }
        val reflectionEnabled = maybeAddReflection(extraDefinitions, options)

        return when (parsed.scheme) {
            "tcp" -> {
                val host = parsed.host ?: "0.0.0.0"
                val port = parsed.port ?: 9090
                val bound = bindTcpServer(host, port, resolvedServices, extraDefinitions)
                announce(bound.publicUri, describeEnabled, reflectionEnabled, options)
                RunningServer(bound.server, bound.publicUri, options.logger)
            }
            "stdio" -> {
                val bound = bindTcpServer("127.0.0.1", 0, resolvedServices, extraDefinitions)
                lateinit var running: RunningServer
                val bridge = StdioServerBridge("127.0.0.1", parseTarget(bound.publicUri).second) {
                    running.stop(options.shutdownGracePeriodSeconds)
                }
                running = RunningServer(
                    bound.server,
                    "stdio://",
                    options.logger,
                    auxiliaryStop = { bridge.close() },
                )
                bridge.start()
                announce("stdio://", describeEnabled, reflectionEnabled, options)
                running
            }
            "unix" -> {
                val bound = bindTcpServer("127.0.0.1", 0, resolvedServices, extraDefinitions)
                val (host, port) = parseTarget(bound.publicUri)
                val path = parsed.path ?: error("unix path missing")
                val publicUri = "unix://$path"
                val bridge = UnixServerBridge(path, host, port)
                bridge.start()
                announce(publicUri, describeEnabled, reflectionEnabled, options)
                RunningServer(
                    bound.server,
                    publicUri,
                    options.logger,
                    auxiliaryStop = { bridge.close() },
                )
            }
            else -> throw IllegalArgumentException(
                "Serve.run(...) currently supports tcp://, unix:// and stdio:// only: $listenUri",
            )
        }
    }

    private fun bindTcpServer(
        host: String,
        port: Int,
        services: Iterable<BindableService>,
        definitions: Iterable<ServerServiceDefinition>,
    ): BoundServer {
        val builder = NettyServerBuilder.forAddress(InetSocketAddress(host, port))
        services.forEach { service ->
            builder.addService(service)
        }
        definitions.forEach { definition ->
            builder.addService(definition)
        }
        val server = builder.build().start()
        val publicUri = "tcp://${advertisedHost(host)}:${server.port}"
        return BoundServer(server, publicUri)
    }

    private fun announce(publicUri: String, describeEnabled: Boolean, reflectionEnabled: Boolean, options: Options) {
        val mode =
            (if (describeEnabled) "Describe ON" else "Describe OFF") +
                ", " +
                (if (reflectionEnabled) "reflection ON" else "reflection OFF")
        options.onListen?.invoke(publicUri)
        options.logger("gRPC server listening on $publicUri ($mode)")
    }

    private fun maybeAddDescribe(definitions: MutableList<ServerServiceDefinition>, options: Options): Boolean {
        if (!options.describe) {
            return false
        }
        definitions += Describe.serviceDefinition()
        return true
    }

    private fun maybeAddReflection(definitions: MutableList<ServerServiceDefinition>, options: Options): Boolean {
        if (!options.reflect) {
            return false
        }
        definitions += ProtoReflectionService.newInstance().bindService()
        return true
    }

    private fun advertisedHost(host: String): String =
        when (host) {
            "", "0.0.0.0" -> "127.0.0.1"
            "::" -> "::1"
            else -> host
        }

    private fun parseTarget(uri: String): Pair<String, Int> {
        require(uri.startsWith("tcp://")) { "unexpected uri: $uri" }
        val target = uri.removePrefix("tcp://")
        val idx = target.lastIndexOf(':')
        return target.substring(0, idx) to target.substring(idx + 1).toInt()
    }

    private class StdioServerBridge(
        host: String,
        port: Int,
        private val onDisconnect: () -> Unit,
    ) : AutoCloseable {
        private val socket = Socket(host, port)
        private val closed = AtomicBoolean(false)
        private val completions = AtomicInteger(2)

        fun start() {
            thread(isDaemon = true, name = "holons-serve-stdio-up") {
                pump(System.`in`, socket.getOutputStream(), shutdownOutput = true)
                markComplete()
            }
            thread(isDaemon = true, name = "holons-serve-stdio-down") {
                pump(socket.getInputStream(), System.out, shutdownOutput = false)
                markComplete()
            }
        }

        override fun close() {
            if (!closed.compareAndSet(false, true)) {
                return
            }
            runCatching { socket.close() }
        }

        private fun pump(
            input: java.io.InputStream,
            output: java.io.OutputStream,
            shutdownOutput: Boolean,
        ) {
            val buffer = ByteArray(16 * 1024)
            try {
                while (true) {
                    val read = input.read(buffer)
                    if (read <= 0) {
                        if (shutdownOutput) {
                            runCatching { socket.shutdownOutput() }
                        }
                        return
                    }
                    output.write(buffer, 0, read)
                    output.flush()
                }
            } catch (_: Exception) {
                // Closed during shutdown or EOF propagation.
            }
        }

        private fun markComplete() {
            if (completions.decrementAndGet() == 0) {
                onDisconnect()
            }
        }
    }

    private class UnixServerBridge(
        path: String,
        private val host: String,
        private val port: Int,
    ) : AutoCloseable {
        private val listener: ServerSocketChannel
        private val socketPath: String
        private val closed = AtomicBoolean(false)
        private val connections = Collections.synchronizedSet(mutableSetOf<AutoCloseable>())
        private var acceptThread: Thread? = null

        init {
            val transportListener = Transport.listen("unix://$path")
            require(transportListener is Transport.Listener.Unix) { "expected unix listener for $path" }
            listener = transportListener.channel
            socketPath = transportListener.path
        }

        fun start() {
            acceptThread = thread(
                start = true,
                isDaemon = true,
                name = "holons-serve-unix-accept",
            ) {
                acceptLoop()
            }
        }

        override fun close() {
            if (!closed.compareAndSet(false, true)) {
                return
            }

            runCatching { listener.close() }
            synchronized(connections) {
                connections.toList().forEach { connection ->
                    runCatching { connection.close() }
                }
                connections.clear()
            }
            runCatching { Files.deleteIfExists(Path.of(socketPath)) }
            runCatching { acceptThread?.join(200) }
        }

        private fun acceptLoop() {
            while (!closed.get()) {
                var client: SocketChannel? = null
                try {
                    client = listener.accept()
                    if (client == null) {
                        continue
                    }
                    if (closed.get()) {
                        client.close()
                        return
                    }

                    val accepted = client
                    thread(
                        start = true,
                        isDaemon = true,
                        name = "holons-serve-unix-client",
                    ) {
                        handleClient(accepted)
                    }
                } catch (_: Exception) {
                    runCatching { client?.close() }
                    if (closed.get()) {
                        return
                    }
                }
            }
        }

        private fun handleClient(client: SocketChannel) {
            var upstream: Socket? = null
            try {
                upstream = Socket(host, port)
                track(client)
                track(upstream)

                val clientInput = Channels.newInputStream(client)
                val clientOutput = Channels.newOutputStream(client)
                val upstreamInput = upstream.getInputStream()
                val upstreamOutput = upstream.getOutputStream()

                val up = thread(start = true, isDaemon = true, name = "holons-serve-unix-up") {
                    pump(clientInput, upstreamOutput)
                }
                val down = thread(start = true, isDaemon = true, name = "holons-serve-unix-down") {
                    pump(upstreamInput, clientOutput)
                }
                up.join()
                down.join()
            } catch (_: Exception) {
                // Closed during shutdown or client disconnect.
            } finally {
                untrack(client)
                runCatching { client.close() }
                if (upstream != null) {
                    untrack(upstream)
                    runCatching { upstream.close() }
                }
            }
        }

        private fun track(connection: AutoCloseable) {
            if (closed.get()) {
                runCatching { connection.close() }
                return
            }
            connections.add(connection)
        }

        private fun untrack(connection: AutoCloseable) {
            connections.remove(connection)
        }

        private fun pump(
            input: java.io.InputStream,
            output: java.io.OutputStream,
        ) {
            val buffer = ByteArray(16 * 1024)
            try {
                while (true) {
                    val read = input.read(buffer)
                    if (read <= 0) {
                        return
                    }
                    output.write(buffer, 0, read)
                    output.flush()
                }
            } catch (_: Exception) {
                // Closed during shutdown or EOF propagation.
            }
        }
    }
}
