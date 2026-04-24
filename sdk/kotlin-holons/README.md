# kotlin-holons

Kotlin runtime SDK for holons.

## serve

```kotlin
package org.organicprogramming.example

import gen.DescribeGenerated
import org.organicprogramming.holons.Describe
import org.organicprogramming.holons.Serve

fun main() {
    Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    Serve.runWithOptions(
        "tcp://127.0.0.1:9090",
        listOf(GreetingServer()),
        Serve.Options(reflect = false),
    )
}
```

## transport

Pass the listener URI directly to `Serve.runWithOptions(...)` or via `Serve.parseOptions(args)`, for example `tcp://127.0.0.1:9090`, `unix:///tmp/gabriel.sock`, or `stdio://`.

For outbound JSON-RPC transports, use `HolonRPCClient().connect("ws://127.0.0.1:8080/rpc")`, `HolonRPCClient().connect("wss://example.com/rpc")`, or `HolonRPCHTTPClient("http://127.0.0.1:8080/api/v1/rpc")`.

## identity / describe

Wire the generated Incode Description with one line:

```kotlin
Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
```

At build time, `op build` generates `gen/describe_generated.kt`; at runtime, `Serve` fails fast with `no Incode Description registered — run op build` if that static response is missing.

## discover

```kotlin
val entry = Discover.findBySlug("gabriel-greeting-kotlin")
```

## connect

```kotlin
val channel = kotlinx.coroutines.runBlocking { Connect.connect("gabriel-greeting-kotlin") }
```

## Build and test

Kotlin SDK tests require a JDK 21 toolchain.

```sh
./gradlew test
```
