package org.organicprogramming.holons

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class DiscoverTests {
    @Test
    fun discoverAllLayers() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("cwd-alpha.holon"), PackageSeed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha", entrypoint = "cwd-alpha"))
        writePackageHolon(fixture.root.resolve(".op/build/built-beta.holon"), PackageSeed("built-beta", "uuid-built-beta", "Built", "Beta", entrypoint = "built-beta"))
        writePackageHolon(fixture.opBin.resolve("installed-gamma.holon"), PackageSeed("installed-gamma", "uuid-installed-gamma", "Installed", "Gamma", entrypoint = "installed-gamma"))
        writePackageHolon(fixture.opHome.resolve("cache/deps/cached-delta.holon"), PackageSeed("cached-delta", "uuid-cached-delta", "Cached", "Delta", entrypoint = "cached-delta"))

        val result = Discover(LOCAL, null, fixture.root.toString(), ALL, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("built-beta", "cached-delta", "cwd-alpha", "installed-gamma"), sortedSlugs(result))
    }

    @Test
    fun filterBySpecifiers() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("cwd-alpha.holon"), PackageSeed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha", entrypoint = "cwd-alpha"))
        writePackageHolon(fixture.root.resolve(".op/build/built-beta.holon"), PackageSeed("built-beta", "uuid-built-beta", "Built", "Beta", entrypoint = "built-beta"))
        writePackageHolon(fixture.opBin.resolve("installed-gamma.holon"), PackageSeed("installed-gamma", "uuid-installed-gamma", "Installed", "Gamma", entrypoint = "installed-gamma"))

        val result = Discover(LOCAL, null, fixture.root.toString(), BUILT or INSTALLED, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("built-beta", "installed-gamma"), sortedSlugs(result))
    }

    @Test
    fun matchBySlug() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))
        writePackageHolon(fixture.root.resolve("beta.holon"), PackageSeed("beta", "uuid-beta", "Beta", "Two", entrypoint = "beta"))

        val result = Discover(LOCAL, "beta", fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("beta"), sortedSlugs(result))
    }

    @Test
    fun matchByAlias() = withRuntimeFixture { fixture ->
        writePackageHolon(
            fixture.root.resolve("alpha.holon"),
            PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha", aliases = listOf("first")),
        )

        val result = Discover(LOCAL, "first", fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("alpha"), sortedSlugs(result))
    }

    @Test
    fun matchByUuidPrefix() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "12345678-aaaa", "Alpha", "One", entrypoint = "alpha"))

        val result = Discover(LOCAL, "12345678", fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("alpha"), sortedSlugs(result))
    }

    @Test
    fun matchByPath() = withRuntimeFixture { fixture ->
        val target = fixture.root.resolve("beta.holon")
        writePackageHolon(target, PackageSeed("beta", "uuid-beta", "Beta", "Two", entrypoint = "beta"))

        val result = Discover(LOCAL, target.toString(), fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(1, result.found.size)
        assertEquals(fileURL(target), result.found.single().url)
    }

    @Test
    fun limitOne() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))
        writePackageHolon(fixture.root.resolve("beta.holon"), PackageSeed("beta", "uuid-beta", "Beta", "Two", entrypoint = "beta"))

        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, 1, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(1, result.found.size)
    }

    @Test
    fun limitZeroMeansUnlimited() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))
        writePackageHolon(fixture.root.resolve("beta.holon"), PackageSeed("beta", "uuid-beta", "Beta", "Two", entrypoint = "beta"))

        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, 0, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(2, result.found.size)
    }

    @Test
    fun negativeLimitReturnsEmpty() = withRuntimeFixture { fixture ->
        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, -1, NO_TIMEOUT)
        assertNull(result.error)
        assertTrue(result.found.isEmpty())
    }

    @Test
    fun invalidSpecifiers() = withRuntimeFixture { fixture ->
        val result = Discover(LOCAL, null, fixture.root.toString(), 0xFF, NO_LIMIT, NO_TIMEOUT)
        assertNotNull(result.error)
        assertTrue(result.error!!.contains("invalid specifiers"))
    }

    @Test
    fun specifiersZeroTreatedAsAll() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("cwd-alpha.holon"), PackageSeed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha", entrypoint = "cwd-alpha"))
        writePackageHolon(fixture.root.resolve(".op/build/built-beta.holon"), PackageSeed("built-beta", "uuid-built-beta", "Built", "Beta", entrypoint = "built-beta"))
        writePackageHolon(fixture.opBin.resolve("installed-gamma.holon"), PackageSeed("installed-gamma", "uuid-installed-gamma", "Installed", "Gamma", entrypoint = "installed-gamma"))
        writePackageHolon(fixture.opHome.resolve("cache/deps/cached-delta.holon"), PackageSeed("cached-delta", "uuid-cached-delta", "Cached", "Delta", entrypoint = "cached-delta"))

        val allResult = Discover(LOCAL, null, fixture.root.toString(), ALL, NO_LIMIT, NO_TIMEOUT)
        val zeroResult = Discover(LOCAL, null, fixture.root.toString(), 0, NO_LIMIT, NO_TIMEOUT)

        assertNull(allResult.error)
        assertNull(zeroResult.error)
        assertEquals(sortedSlugs(allResult), sortedSlugs(zeroResult))
    }

    @Test
    fun nullExpressionReturnsAll() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))
        writePackageHolon(fixture.root.resolve("beta.holon"), PackageSeed("beta", "uuid-beta", "Beta", "Two", entrypoint = "beta"))

        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(2, result.found.size)
    }

    @Test
    fun missingExpressionReturnsEmpty() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))

        val result = Discover(LOCAL, "", fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertTrue(result.found.isEmpty())
    }

    @Test
    fun excludedDirsSkipped() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("kept/alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))
        listOf(
            ".git/hidden.holon",
            ".op/hidden.holon",
            "node_modules/hidden.holon",
            "vendor/hidden.holon",
            "build/hidden.holon",
            "testdata/hidden.holon",
            ".cache/hidden.holon",
        ).forEachIndexed { index, relative ->
            writePackageHolon(
                fixture.root.resolve(relative),
                PackageSeed("ignored-$index", "uuid-ignored-$index", "Ignored", "Holon", entrypoint = "ignored-$index"),
            )
        }

        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("alpha"), sortedSlugs(result))
    }

    @Test
    fun deduplicateByUuid() = withRuntimeFixture { fixture ->
        val cwdPath = fixture.root.resolve("alpha.holon")
        val builtPath = fixture.root.resolve(".op/build/alpha-built.holon")
        writePackageHolon(cwdPath, PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))
        writePackageHolon(builtPath, PackageSeed("alpha-built", "uuid-alpha", "Alpha", "One", entrypoint = "alpha-built"))

        val result = Discover(LOCAL, null, fixture.root.toString(), ALL, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(1, result.found.size)
        assertEquals(fileURL(cwdPath), result.found.single().url)
    }

    @Test
    fun holonJsonFastPath() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))

        var probeCalls = 0
        packageDescribeProbeOverride = { dir, _ ->
            probeCalls += 1
            HolonRef(url = fileURL(dir), error = "probe should not run")
        }

        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(0, probeCalls)
        assertEquals(listOf("alpha"), sortedSlugs(result))
    }

    @Test
    fun describeFallbackWhenHolonJsonMissing() = withRuntimeFixture { fixture ->
        val pkg = fixture.root.resolve("static-only.holon")
        java.nio.file.Files.createDirectories(pkg)
        var probeCalls = 0
        packageDescribeProbeOverride = { dir, _ ->
            probeCalls += 1
            HolonRef(
                url = fileURL(dir),
                info = HolonInfo(
                    slug = "static-only",
                    uuid = "static-describe-uuid",
                    identity = IdentityInfo("Static", "Only", "Registered at startup."),
                    lang = "kotlin",
                    runner = "shell",
                    status = "draft",
                    kind = "native",
                    transport = "",
                    entrypoint = "static-wrapper",
                    architectures = emptyList(),
                    hasDist = true,
                    hasSource = false,
                ),
            )
        }

        val result = Discover(LOCAL, null, fixture.root.toString(), CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(1, probeCalls)
        assertEquals(listOf("static-only"), sortedSlugs(result))
    }

    @Test
    fun siblingsLayer() = withRuntimeFixture { fixture ->
        val siblingsRoot = fixture.root.resolve("bundle/Holons")
        System.setProperty("holons.siblings.root", siblingsRoot.toString())
        writePackageHolon(siblingsRoot.resolve("bundle.holon"), PackageSeed("bundle", "uuid-bundle", "Bundle", "Holon", entrypoint = "bundle"))

        val result = Discover(LOCAL, null, fixture.root.toString(), SIBLINGS, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("bundle"), sortedSlugs(result))
    }

    @Test
    fun sourceLayerOffloadsToLocalOp() = withRuntimeFixture { fixture ->
        val sourceDir = fixture.root.resolve("proto-holon")
        var calls = 0
        sourceDiscoverOffload = { scope, expression, root, specifiers, limit, timeout ->
            calls += 1
            assertEquals(LOCAL, scope)
            assertNull(expression)
            assertEquals(fixture.root.toString(), root)
            assertEquals(SOURCE, specifiers)
            assertEquals(NO_LIMIT, limit)
            assertEquals(3210, timeout)
            DiscoverResult(
                found = listOf(
                    HolonRef(
                        url = fileURL(sourceDir),
                        info = HolonInfo(
                            slug = "proto-holon",
                            uuid = "uuid-proto",
                            identity = IdentityInfo("Proto", "Holon"),
                            lang = "kotlin",
                            runner = "shell",
                            status = "draft",
                            kind = "service",
                            transport = "",
                            entrypoint = "proto-holon",
                            architectures = emptyList(),
                            hasDist = false,
                            hasSource = true,
                        ),
                    ),
                ),
            )
        }

        val result = Discover(LOCAL, null, fixture.root.toString(), SOURCE, NO_LIMIT, 3210)
        assertNull(result.error)
        assertEquals(1, calls)
        assertEquals(listOf("proto-holon"), sortedSlugs(result))
    }

    @Test
    fun builtLayer() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve(".op/build/built.holon"), PackageSeed("built", "uuid-built", "Built", "Holon", entrypoint = "built"))

        val result = Discover(LOCAL, null, fixture.root.toString(), BUILT, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("built"), sortedSlugs(result))
    }

    @Test
    fun installedLayer() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.opBin.resolve("installed.holon"), PackageSeed("installed", "uuid-installed", "Installed", "Holon", entrypoint = "installed"))

        val result = Discover(LOCAL, null, fixture.root.toString(), INSTALLED, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("installed"), sortedSlugs(result))
    }

    @Test
    fun cachedLayer() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.opHome.resolve("cache/deep/cached.holon"), PackageSeed("cached", "uuid-cached", "Cached", "Holon", entrypoint = "cached"))

        val result = Discover(LOCAL, null, fixture.root.toString(), CACHED, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("cached"), sortedSlugs(result))
    }

    @Test
    fun nilRootDefaultsToCwd() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))

        val result = Discover(LOCAL, null, null, CWD, NO_LIMIT, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals(listOf("alpha"), sortedSlugs(result))
    }

    @Test
    fun emptyRootReturnsError() = withRuntimeFixture {
        val result = Discover(LOCAL, null, "", ALL, NO_LIMIT, NO_TIMEOUT)
        assertNotNull(result.error)
        assertTrue(result.error!!.contains("root"))
    }

    @Test
    fun unsupportedScopeReturnsError() = withRuntimeFixture { fixture ->
        val proxy = Discover(PROXY, null, fixture.root.toString(), ALL, NO_LIMIT, NO_TIMEOUT)
        val delegated = Discover(DELEGATED, null, fixture.root.toString(), ALL, NO_LIMIT, NO_TIMEOUT)
        assertNotNull(proxy.error)
        assertNotNull(delegated.error)
    }
}
