package org.organicprogramming.holons

import java.net.InetSocketAddress
import java.net.ServerSocket
import java.net.StandardProtocolFamily
import java.net.UnixDomainSocketAddress
import java.nio.channels.ServerSocketChannel
import java.nio.channels.SocketChannel
import java.nio.file.Files
import java.nio.file.Path

/** URI-based listener factory for gRPC servers. */
object Transport {
    const val DEFAULT_URI = "tcp://:9090"

    data class ParsedURI(
        val raw: String,
        val scheme: String,
        val host: String? = null,
        val port: Int? = null,
        val path: String? = null,
        val secure: Boolean = false,
    )

    sealed interface Listener {
        data class Tcp(val socket: ServerSocket) : Listener
        data class Unix(val channel: ServerSocketChannel, val path: String) : Listener
        data object Stdio : Listener
        data class WS(
            val host: String,
            val port: Int,
            val path: String,
            val secure: Boolean,
        ) : Listener
    }

    fun scheme(uri: String): String {
        val idx = uri.indexOf("://")
        return if (idx >= 0) uri.substring(0, idx) else uri
    }

    fun parseURI(uri: String): ParsedURI {
        val s = scheme(uri)
        return when (s) {
            "tcp" -> {
                require(uri.startsWith("tcp://")) { "invalid tcp URI: $uri" }
                val (host, port) = splitHostPort(uri.removePrefix("tcp://"), 9090)
                ParsedURI(raw = uri, scheme = "tcp", host = host, port = port)
            }
            "unix" -> {
                require(uri.startsWith("unix://")) { "invalid unix URI: $uri" }
                val path = uri.removePrefix("unix://")
                require(path.isNotEmpty()) { "invalid unix URI: $uri" }
                ParsedURI(raw = uri, scheme = "unix", path = path)
            }
            "stdio" -> ParsedURI(raw = "stdio://", scheme = "stdio")
            "ws", "wss" -> {
                val secure = s == "wss"
                val prefix = if (secure) "wss://" else "ws://"
                require(uri.startsWith(prefix)) { "invalid ws URI: $uri" }

                val trimmed = uri.removePrefix(prefix)
                val slash = trimmed.indexOf('/')
                val addr = if (slash >= 0) trimmed.substring(0, slash) else trimmed
                val path = if (slash >= 0) trimmed.substring(slash).ifEmpty { "/grpc" } else "/grpc"
                val (host, port) = splitHostPort(addr, if (secure) 443 else 80)
                ParsedURI(
                    raw = uri,
                    scheme = s,
                    host = host,
                    port = port,
                    path = path,
                    secure = secure,
                )
            }
            else -> throw IllegalArgumentException("unsupported transport URI: $uri")
        }
    }

    fun listen(uri: String): Listener {
        val parsed = parseURI(uri)
        return when (parsed.scheme) {
            "tcp" -> Listener.Tcp(listenTcp(parsed))
            "unix" -> Listener.Unix(
                channel = listenUnix(parsed),
                path = parsed.path ?: error("unix path missing"),
            )
            "stdio" -> Listener.Stdio
            "ws", "wss" -> Listener.WS(
                host = parsed.host ?: "0.0.0.0",
                port = parsed.port ?: if (parsed.secure) 443 else 80,
                path = parsed.path ?: "/grpc",
                secure = parsed.secure,
            )
            else -> throw IllegalArgumentException("unsupported transport URI: $uri")
        }
    }

    fun dialUnix(uri: String): SocketChannel {
        val parsed = parseURI(uri)
        require(parsed.scheme == "unix") { "dialUnix expects unix:// URI: $uri" }
        val path = parsed.path ?: error("unix path missing")
        return SocketChannel.open(UnixDomainSocketAddress.of(path))
    }

    private fun listenTcp(parsed: ParsedURI): ServerSocket {
        val host = parsed.host ?: "0.0.0.0"
        val port = parsed.port ?: 9090
        return ServerSocket().apply {
            reuseAddress = true
            bind(InetSocketAddress(host, port))
        }
    }

    private fun listenUnix(parsed: ParsedURI): ServerSocketChannel {
        val path = parsed.path ?: error("unix path missing")
        Files.deleteIfExists(Path.of(path))
        return ServerSocketChannel.open(StandardProtocolFamily.UNIX).apply {
            bind(UnixDomainSocketAddress.of(path))
        }
    }

    private fun splitHostPort(addr: String, defaultPort: Int): Pair<String, Int> {
        if (addr.isEmpty()) return "0.0.0.0" to defaultPort
        val lastColon = addr.lastIndexOf(':')
        if (lastColon >= 0) {
            val host = addr.substring(0, lastColon).ifEmpty { "0.0.0.0" }
            val portText = addr.substring(lastColon + 1)
            val port = if (portText.isEmpty()) defaultPort else portText.toInt()
            return host to port
        }
        return addr to defaultPort
    }

}
