package org.organicprogramming.holons

import java.util.concurrent.atomic.AtomicReference

object CurrentTransport {
    private val local = ThreadLocal<String>()
    private val active = AtomicReference("")

    fun get(): String = local.get().orEmpty().ifEmpty { active.get() }

    fun scoped(scheme: String): AutoCloseable {
        set(scheme)
        return AutoCloseable { clear() }
    }

    internal fun set(scheme: String) {
        val normalized = scheme.trim()
        local.set(normalized)
        active.set(normalized)
    }

    internal fun clear() {
        local.remove()
        active.set("")
    }
}
