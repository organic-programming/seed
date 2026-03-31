package org.organicprogramming.holons;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.organicprogramming.holons.DiscoveryTestSupport.PackageSeed;
import org.organicprogramming.holons.DiscoveryTestSupport.RuntimeFixture;
import org.organicprogramming.holons.DiscoveryTypes.DiscoverResult;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.organicprogramming.holons.DiscoveryTestSupport.holonInfo;
import static org.organicprogramming.holons.DiscoveryTestSupport.holonRef;
import static org.organicprogramming.holons.DiscoveryTestSupport.runtimeEnv;
import static org.organicprogramming.holons.DiscoveryTestSupport.runtimeFixture;
import static org.organicprogramming.holons.DiscoveryTestSupport.slugs;
import static org.organicprogramming.holons.DiscoveryTestSupport.writePackageHolon;

class DiscoverTests {

    @AfterEach
    void resetHooks() {
        Discover.setPackageProbeForTests(null);
        Discover.setSourceBridgeForTests(null);
        Discover.setBundleRootResolverForTests(null);
    }

    @Test
    void discoverAllLayers(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        Path bundleRoot = root.resolve("TestApp.app").resolve("Contents").resolve("Resources").resolve("Holons");

        writePackageHolon(bundleRoot.resolve("bundle.holon"),
                new PackageSeed("uuid-bundle", "Bundle", "Holon").slug("bundle"));
        writePackageHolon(root.resolve("cwd-alpha.holon"),
                new PackageSeed("uuid-cwd-alpha", "Cwd", "Alpha").slug("cwd-alpha"));
        writePackageHolon(root.resolve(".op").resolve("build").resolve("built-beta.holon"),
                new PackageSeed("uuid-built-beta", "Built", "Beta").slug("built-beta"));
        writePackageHolon(fixture.opBin().resolve("installed-gamma.holon"),
                new PackageSeed("uuid-installed-gamma", "Installed", "Gamma").slug("installed-gamma"));
        writePackageHolon(fixture.opHome().resolve("cache").resolve("deps").resolve("cached-delta.holon"),
                new PackageSeed("uuid-cached-delta", "Cached", "Delta").slug("cached-delta"));

        Discover.setBundleRootResolverForTests(() -> bundleRoot);
        Discover.setSourceBridgeForTests((expression, searchRoot, limit, timeout) -> {
            DiscoverResult bridged = new DiscoverResult();
            bridged.found.add(holonRef(root.resolve("source-epsilon"), "source-epsilon", "uuid-source-epsilon", "Source", "Epsilon"));
            return bridged;
        });

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("built-beta", "bundle", "cached-delta", "cwd-alpha", "installed-gamma", "source-epsilon"), slugs(result));
        }
    }

    @Test
    void filterBySpecifiers(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("cwd-alpha.holon"),
                new PackageSeed("uuid-cwd-alpha", "Cwd", "Alpha").slug("cwd-alpha"));
        writePackageHolon(root.resolve(".op").resolve("build").resolve("built-beta.holon"),
                new PackageSeed("uuid-built-beta", "Built", "Beta").slug("built-beta"));
        writePackageHolon(fixture.opBin().resolve("installed-gamma.holon"),
                new PackageSeed("uuid-installed-gamma", "Installed", "Gamma").slug("installed-gamma"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(
                    Discover.LOCAL,
                    null,
                    root.toString(),
                    Discover.BUILT | Discover.INSTALLED,
                    Discover.NO_LIMIT,
                    Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("built-beta", "installed-gamma"), slugs(result));
        }
    }

    @Test
    void matchBySlug(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));
        writePackageHolon(root.resolve("beta.holon"), new PackageSeed("uuid-beta", "Beta", "Two").slug("beta"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, "beta", root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("beta"), slugs(result));
        }
    }

    @Test
    void matchByAlias(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"),
                new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha").aliases(List.of("first")));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, "first", root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("alpha"), slugs(result));
        }
    }

    @Test
    void matchByUuidPrefix(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"),
                new PackageSeed("12345678-aaaa-bbbb-cccc-dddddddddddd", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, "12345678", root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("alpha"), slugs(result));
        }
    }

    @Test
    void matchByPath(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        Path alpha = root.resolve("alpha.holon");
        writePackageHolon(alpha, new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, alpha.toString(), root.toString(), Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("alpha"), slugs(result));
            assertEquals(alpha.toAbsolutePath().normalize().toUri().toString(), result.found.get(0).url);
        }
    }

    @Test
    void limitOne(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));
        writePackageHolon(root.resolve("beta.holon"), new PackageSeed("uuid-beta", "Beta", "Two").slug("beta"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, 1, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(1, result.found.size());
        }
    }

    @Test
    void limitZeroMeansUnlimited(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));
        writePackageHolon(root.resolve("beta.holon"), new PackageSeed("uuid-beta", "Beta", "Two").slug("beta"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, 0, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(2, result.found.size());
        }
    }

    @Test
    void negativeLimitReturnsEmpty(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, -1, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertTrue(result.found.isEmpty());
        }
    }

    @Test
    void invalidSpecifiers(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), 0xFF, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertTrue(result.error.contains("invalid specifiers"));
        }
    }

    @Test
    void specifiersZeroTreatedAsAll(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("cwd-alpha.holon"),
                new PackageSeed("uuid-cwd-alpha", "Cwd", "Alpha").slug("cwd-alpha"));
        writePackageHolon(root.resolve(".op").resolve("build").resolve("built-beta.holon"),
                new PackageSeed("uuid-built-beta", "Built", "Beta").slug("built-beta"));
        writePackageHolon(fixture.opBin().resolve("installed-gamma.holon"),
                new PackageSeed("uuid-installed-gamma", "Installed", "Gamma").slug("installed-gamma"));
        writePackageHolon(fixture.opHome().resolve("cache").resolve("cached-delta.holon"),
                new PackageSeed("uuid-cached-delta", "Cached", "Delta").slug("cached-delta"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult all = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            DiscoverResult zero = Discover.Discover(Discover.LOCAL, null, root.toString(), 0, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", all.error);
            assertEquals("", zero.error);
            assertEquals(slugs(all), slugs(zero));
        }
    }

    @Test
    void nullExpressionReturnsAll(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));
        writePackageHolon(root.resolve("beta.holon"), new PackageSeed("uuid-beta", "Beta", "Two").slug("beta"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(2, result.found.size());
        }
    }

    @Test
    void missingExpressionReturnsEmpty(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, "missing", root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertTrue(result.found.isEmpty());
        }
    }

    @Test
    void excludedDirsSkipped(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("kept.holon"), new PackageSeed("uuid-kept", "Kept", "Holon").slug("kept"));
        for (String skipped : List.of(".git", ".op", "node_modules", "vendor", "build", "testdata", ".cache")) {
            writePackageHolon(root.resolve(skipped).resolve("hidden.holon"),
                    new PackageSeed("uuid-" + skipped, "Ignored", "Holon").slug("ignored-" + skipped.replace('.', '_')));
        }

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("kept"), slugs(result));
        }
    }

    @Test
    void deduplicateByUuid(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        Path cwd = root.resolve("alpha.holon");
        writePackageHolon(cwd, new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));
        writePackageHolon(root.resolve(".op").resolve("build").resolve("alpha-built.holon"),
                new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha-built"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(1, result.found.size());
            assertEquals(cwd.toAbsolutePath().normalize().toUri().toString(), result.found.get(0).url);
        }
    }

    @Test
    void holonJsonFastPath(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        AtomicInteger probeCalls = new AtomicInteger();
        Discover.setPackageProbeForTests(packageDir -> {
            probeCalls.incrementAndGet();
            return holonInfo("alpha", "uuid-alpha", "Alpha", "One");
        });
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("alpha"), slugs(result));
            assertEquals(0, probeCalls.get());
        }
    }

    @Test
    void describeFallbackWhenHolonJsonMissing(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        Path alpha = root.resolve("alpha.holon");
        Files.createDirectories(alpha);
        AtomicInteger probeCalls = new AtomicInteger();
        Discover.setPackageProbeForTests(packageDir -> {
            probeCalls.incrementAndGet();
            assertEquals(alpha.toAbsolutePath().normalize(), packageDir);
            return holonInfo("alpha", "uuid-alpha", "Alpha", "One");
        });

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("alpha"), slugs(result));
            assertEquals(1, probeCalls.get());
        }
    }

    @Test
    void siblingsLayer(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        Path bundleRoot = root.resolve("App.app").resolve("Contents").resolve("Resources").resolve("Holons");
        writePackageHolon(bundleRoot.resolve("bundle.holon"),
                new PackageSeed("uuid-bundle", "Bundle", "Holon").slug("bundle"));
        Discover.setBundleRootResolverForTests(() -> bundleRoot);

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.SIBLINGS, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("bundle"), slugs(result));
        }
    }

    @Test
    void sourceLayerOffloadsToLocalOp(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        AtomicInteger calls = new AtomicInteger();
        AtomicReference<String> capturedExpression = new AtomicReference<>("unset");
        AtomicReference<Path> capturedRoot = new AtomicReference<>();
        Discover.setSourceBridgeForTests((expression, searchRoot, limit, timeout) -> {
            calls.incrementAndGet();
            capturedExpression.set(expression);
            capturedRoot.set(searchRoot);
            DiscoverResult bridged = new DiscoverResult();
            bridged.found.add(holonRef(root.resolve("proto-holon"), "proto-holon", "uuid-proto", "Proto", "Holon"));
            return bridged;
        });

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.SOURCE, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("proto-holon"), slugs(result));
            assertEquals(1, calls.get());
            assertEquals(null, capturedExpression.get());
            assertEquals(root.toAbsolutePath().normalize(), capturedRoot.get());
        }
    }

    @Test
    void builtLayer(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve(".op").resolve("build").resolve("built.holon"),
                new PackageSeed("uuid-built", "Built", "Holon").slug("built"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.BUILT, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("built"), slugs(result));
        }
    }

    @Test
    void installedLayer(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(fixture.opBin().resolve("installed.holon"),
                new PackageSeed("uuid-installed", "Installed", "Holon").slug("installed"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.INSTALLED, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("installed"), slugs(result));
        }
    }

    @Test
    void cachedLayer(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(fixture.opHome().resolve("cache").resolve("deep").resolve("cached.holon"),
                new PackageSeed("uuid-cached", "Cached", "Holon").slug("cached"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, root.toString(), Discover.CACHED, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("cached"), slugs(result));
        }
    }

    @Test
    void nilRootDefaultsToCwd(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        writePackageHolon(root.resolve("alpha.holon"), new PackageSeed("uuid-alpha", "Alpha", "One").slug("alpha"));

        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, null, Discover.CWD, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertEquals("", result.error);
            assertEquals(List.of("alpha"), slugs(result));
        }
    }

    @Test
    void emptyRootReturnsError(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        try (var env = runtimeEnv(fixture)) {
            DiscoverResult result = Discover.Discover(Discover.LOCAL, null, "", Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT);
            assertTrue(result.error.contains("root cannot be empty"));
        }
    }

    @Test
    void unsupportedScopeReturnsError(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        try (var env = runtimeEnv(fixture)) {
            assertNotEquals("", Discover.Discover(Discover.PROXY, null, root.toString(), Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT).error);
            assertNotEquals("", Discover.Discover(Discover.DELEGATED, null, root.toString(), Discover.ALL, Discover.NO_LIMIT, Discover.NO_TIMEOUT).error);
        }
    }
}
