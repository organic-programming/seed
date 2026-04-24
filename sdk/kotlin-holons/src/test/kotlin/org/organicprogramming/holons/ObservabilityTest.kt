package org.organicprogramming.holons

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class ObservabilityTest {
    @Test
    fun parseOpObsDropsV2Tokens() {
        val all = setOf(
            Observability.Family.LOGS,
            Observability.Family.METRICS,
            Observability.Family.EVENTS,
            Observability.Family.PROM,
        )
        assertEquals(all, Observability.parseOpObs("all,otel"))
        assertEquals(all, Observability.parseOpObs("all,sessions"))
    }

    @Test
    fun checkEnvRejectsV2TokensAndOpSessions() {
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.checkEnv(mapOf("OP_OBS" to "logs,otel"))
        }
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.checkEnv(mapOf("OP_OBS" to "logs,sessions"))
        }
        val err = assertFailsWith<Observability.InvalidTokenException> {
            Observability.checkEnv(mapOf("OP_SESSIONS" to "metrics"))
        }
        assertEquals("OP_SESSIONS", err.variable)
    }
}
