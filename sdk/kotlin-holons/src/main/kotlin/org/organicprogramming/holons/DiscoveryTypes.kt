package org.organicprogramming.holons

const val LOCAL = 0
const val PROXY = 1
const val DELEGATED = 2

const val SIBLINGS = 0x01
const val CWD = 0x02
const val SOURCE = 0x04
const val BUILT = 0x08
const val INSTALLED = 0x10
const val CACHED = 0x20
const val ALL = 0x3F

const val NO_LIMIT = 0
const val NO_TIMEOUT = 0

data class IdentityInfo(
    val givenName: String,
    val familyName: String,
    val motto: String = "",
    val aliases: List<String> = emptyList(),
)

data class HolonInfo(
    val slug: String,
    val uuid: String,
    val identity: IdentityInfo,
    val lang: String,
    val runner: String,
    val status: String,
    val kind: String,
    val transport: String,
    val entrypoint: String,
    val architectures: List<String>,
    val hasDist: Boolean,
    val hasSource: Boolean,
)

data class HolonRef(
    val url: String,
    val info: HolonInfo? = null,
    val error: String? = null,
)

data class DiscoverResult(
    val found: List<HolonRef> = emptyList(),
    val error: String? = null,
)

data class ResolveResult(
    val ref: HolonRef? = null,
    val error: String? = null,
)

data class ConnectResult(
    val channel: Any? = null,
    val uid: String = "",
    val origin: HolonRef? = null,
    val error: String? = null,
)
