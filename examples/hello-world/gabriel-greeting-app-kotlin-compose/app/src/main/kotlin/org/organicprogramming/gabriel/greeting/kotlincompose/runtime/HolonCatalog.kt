package org.organicprogramming.gabriel.greeting.kotlincompose.runtime

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity
import org.organicprogramming.holons.BUILT
import org.organicprogramming.holons.CACHED
import org.organicprogramming.holons.CWD
import org.organicprogramming.holons.Discover
import org.organicprogramming.holons.INSTALLED
import org.organicprogramming.holons.LOCAL
import org.organicprogramming.holons.NO_LIMIT
import org.organicprogramming.holons.NO_TIMEOUT
import org.organicprogramming.holons.SIBLINGS

interface HolonCatalog {
    suspend fun discover(): List<GabrielHolonIdentity>
}

class DesktopHolonCatalog : HolonCatalog {
    override suspend fun discover(): List<GabrielHolonIdentity> = withContext(Dispatchers.IO) {
        AppPaths.configureRuntimeEnvironment()
        val result = Discover(
            LOCAL,
            null,
            AppPaths.discoveryRoot(),
            SIBLINGS or CWD or BUILT or INSTALLED or CACHED,
            NO_LIMIT,
            NO_TIMEOUT,
        )

        if (!result.error.isNullOrBlank()) {
            throw IllegalStateException(result.error)
        }

        buildMap<String, GabrielHolonIdentity> {
            result.found.forEach { ref ->
                val holon = GabrielHolonIdentity.fromDiscovered(ref) ?: return@forEach
                putIfAbsent(holon.slug, holon)
            }
        }.values.sortedWith(
            compareBy<GabrielHolonIdentity> { it.sortRank }
                .thenBy { it.displayName.lowercase() },
        )
    }
}
