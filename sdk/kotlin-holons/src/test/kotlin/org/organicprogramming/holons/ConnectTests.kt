package org.organicprogramming.holons

import io.grpc.ManagedChannel
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.coroutines.runBlocking
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ConnectTests {
    @Test
    fun unresolvableTarget() = withRuntimeFixture { fixture ->
        runBlocking {
        val result = connect(LOCAL, "missing", fixture.root.toString(), SOURCE, 2_000)
        assertNotNull(result.error)
        assertNull(result.channel)
        assertNull(result.origin)
        }
    }

    @Test
    fun returnsConnectResult() = withRuntimeFixture { fixture ->
        runBlocking {
            val connectFixture = createConnectFixture(fixture.root, "Connect", "Result")
            sourceDiscoverOffload = { _, expression, _, _, _, _ ->
                if (expression == connectFixture.slug) {
                    DiscoverResult(found = listOf(connectFixtureRef(connectFixture)))
                } else {
                    DiscoverResult()
                }
            }

            val result = connect(LOCAL, connectFixture.slug, fixture.root.toString(), SOURCE, 5_000)
            val channel = result.channel as? ManagedChannel
            assertNull(result.error)
            assertNotNull(channel)
            assertEquals("", result.uid)

            val pid = waitForPidFile(connectFixture.pidFile)
            val args = waitForArgsFile(connectFixture.argsFile)
            val response = invokePing(channel, "kotlin-connect")
            assertEquals("kotlin-connect", response["message"]?.jsonPrimitive?.content)
            assertEquals(listOf("serve", "--listen", "stdio://"), args)

            disconnect(result)
            waitForPidExit(pid)
        }
    }

    @Test
    fun populatesOrigin() = withRuntimeFixture { fixture ->
        runBlocking {
            val connectFixture = createConnectFixture(fixture.root, "Connect", "Origin")
            sourceDiscoverOffload = { _, expression, _, _, _, _ ->
                if (expression == connectFixture.slug) {
                    DiscoverResult(found = listOf(connectFixtureRef(connectFixture)))
                } else {
                    DiscoverResult()
                }
            }

            val result = connect(LOCAL, connectFixture.slug, fixture.root.toString(), SOURCE, 5_000)
            assertNull(result.error)
            assertNotNull(result.origin)
            assertEquals(connectFixture.slug, result.origin?.info?.slug)
            assertTrue(result.origin!!.url.startsWith("tcp://"))

            disconnect(result)
        }
    }

    @Test
    fun disconnectAcceptsConnectResult() = withRuntimeFixture { fixture ->
        runBlocking {
            val connectFixture = createConnectFixture(fixture.root, "Connect", "Disconnect")
            sourceDiscoverOffload = { _, expression, _, _, _, _ ->
                if (expression == connectFixture.slug) {
                    DiscoverResult(found = listOf(connectFixtureRef(connectFixture)))
                } else {
                    DiscoverResult()
                }
            }

            val result = connect(LOCAL, connectFixture.slug, fixture.root.toString(), SOURCE, 5_000)
            val pid = waitForPidFile(connectFixture.pidFile)
            assertNull(result.error)

            disconnect(result)
            waitForPidExit(pid)
        }
    }
}
