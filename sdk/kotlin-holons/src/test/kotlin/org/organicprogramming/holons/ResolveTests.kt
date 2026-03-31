package org.organicprogramming.holons

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ResolveTests {
    @Test
    fun knownSlug() = withRuntimeFixture { fixture ->
        writePackageHolon(fixture.root.resolve("alpha.holon"), PackageSeed("alpha", "uuid-alpha", "Alpha", "One", entrypoint = "alpha"))

        val result = resolve(LOCAL, "alpha", fixture.root.toString(), CWD, NO_TIMEOUT)
        assertNull(result.error)
        assertEquals("alpha", result.ref?.info?.slug)
    }

    @Test
    fun missingTarget() = withRuntimeFixture { fixture ->
        val result = resolve(LOCAL, "missing", fixture.root.toString(), ALL, NO_TIMEOUT)
        assertNotNull(result.error)
        assertTrue(result.error!!.contains("not found"))
    }

    @Test
    fun invalidSpecifiers() = withRuntimeFixture { fixture ->
        val result = resolve(LOCAL, "alpha", fixture.root.toString(), 0xFF, NO_TIMEOUT)
        assertNotNull(result.error)
        assertTrue(result.error!!.contains("invalid specifiers"))
    }
}
