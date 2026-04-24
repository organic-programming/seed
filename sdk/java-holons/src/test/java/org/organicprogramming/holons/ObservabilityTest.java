package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;

import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.*;

final class ObservabilityTest {
    @Test
    void parseOpObsDropsV2Tokens() {
        Set<Observability.Family> all = Set.of(
                Observability.Family.LOGS,
                Observability.Family.METRICS,
                Observability.Family.EVENTS,
                Observability.Family.PROM);
        assertEquals(all, Observability.parseOpObs("all,otel"));
        assertEquals(all, Observability.parseOpObs("all,sessions"));
    }

    @Test
    void checkEnvRejectsV2TokensAndOpSessions() {
        assertThrows(Observability.InvalidTokenException.class,
                () -> Observability.checkEnv(Map.of("OP_OBS", "logs,otel")));
        assertThrows(Observability.InvalidTokenException.class,
                () -> Observability.checkEnv(Map.of("OP_OBS", "logs,sessions")));
        Observability.InvalidTokenException err = assertThrows(Observability.InvalidTokenException.class,
                () -> Observability.checkEnv(Map.of("OP_SESSIONS", "metrics")));
        assertEquals("OP_SESSIONS", err.variable);
        assertDoesNotThrow(() -> Observability.checkEnv(Map.of("OP_OBS", "logs,metrics,events,prom,all")));
    }
}
