package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.organicprogramming.holons.DiscoveryTestSupport.PackageSeed;
import org.organicprogramming.holons.DiscoveryTestSupport.RuntimeFixture;
import org.organicprogramming.holons.DiscoveryTypes.ResolveResult;

import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.organicprogramming.holons.DiscoveryTestSupport.runtimeEnv;
import static org.organicprogramming.holons.DiscoveryTestSupport.runtimeFixture;
import static org.organicprogramming.holons.DiscoveryTestSupport.writePackageHolon;

class ResolveTests {

    @Test
    void knownSlug(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            ResolveResult result = Discover.resolve(Discover.LOCAL, "alpha", root.toString(), Discover.CWD, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertNotNull(result.ref);
            assertNotNull(result.ref.info);
            assertEquals("alpha", result.ref.info.slug);
        }
    }

    @Test
    void missingTarget(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        try (var env = runtimeEnv(fixture)) {
            ResolveResult result = Discover.resolve(Discover.LOCAL, "missing", root.toString(), Discover.ALL, Discover.NO_TIMEOUT);
            assertNotEquals("", result.error);
        }
    }

    @Test
    void invalidSpecifiers(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        try (var env = runtimeEnv(fixture)) {
            ResolveResult result = Discover.resolve(Discover.LOCAL, "alpha", root.toString(), 0xFF, Discover.NO_TIMEOUT);
            assertNotEquals("", result.error);
        }
    }
}
