package org.organicprogramming.holons

import holons.v1.Describe as HolonsDescribe
import holons.v1.Manifest as HolonsManifest

object StaticDescribeServerMain {
    @JvmStatic
    fun main(args: Array<String>) {
        Describe.useStaticResponse(staticDescribeResponse())
        Serve.runWithOptions(
            listenUri = args.firstOrNull().orEmpty().ifBlank { "tcp://127.0.0.1:0" },
            services = emptyList(),
            options = Serve.Options(
                onListen = { uri ->
                    println(uri)
                    System.out.flush()
                },
            ),
        )
    }

    private fun staticDescribeResponse(): HolonsDescribe.DescribeResponse =
        HolonsDescribe.DescribeResponse.newBuilder()
            .setManifest(
                HolonsManifest.HolonManifest.newBuilder()
                    .setIdentity(
                        HolonsManifest.HolonManifest.Identity.newBuilder()
                            .setUuid("static-describe-uuid")
                            .setGivenName("Static")
                            .setFamilyName("Only")
                            .setMotto("Registered at startup.")
                            .build(),
                    )
                    .setLang("kotlin")
                    .setKind("native")
                    .build(),
            )
            .addServices(
                HolonsDescribe.ServiceDoc.newBuilder()
                    .setName("echo.v1.Echo")
                    .addMethods(
                        HolonsDescribe.MethodDoc.newBuilder()
                            .setName("Ping")
                            .setInputType("echo.v1.PingRequest")
                            .setOutputType("echo.v1.PingResponse")
                            .build(),
                    )
                    .build(),
            )
            .build()
}
