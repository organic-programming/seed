package org.organicprogramming.gabriel.greeting.kotlincompose.settings

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SettingsStoreTest {
    @Test
    fun launchEnvironmentOverridesConfigureCoaxState() {
        val store = MemorySettingsStore()

        applyLaunchEnvironmentOverrides(
            store,
            mapOf(
                "OP_COAX_SERVER_ENABLED" to "1",
                "OP_COAX_SERVER_LISTEN_URI" to "tcp://127.0.0.1:60042",
            ),
        )

        assertTrue(store.readBool(coaxEnabledKey()))
        val snapshot = org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxSettingsSnapshot.decode(
            store.readString(coaxSettingsKey()),
        )
        assertEquals("127.0.0.1", snapshot.serverHost)
        assertEquals("60042", snapshot.serverPortText)
    }
}
