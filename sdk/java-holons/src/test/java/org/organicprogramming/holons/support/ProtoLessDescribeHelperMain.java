package org.organicprogramming.holons.support;

import holons.v1.Manifest;
import io.grpc.BindableService;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.holons.Serve;

import java.util.List;

public final class ProtoLessDescribeHelperMain {
    private ProtoLessDescribeHelperMain() {
    }

    public static void main(String[] args) throws Exception {
        Describe.useStaticResponse(holonMetaResponse());
        Serve.runWithOptions(
                "tcp://127.0.0.1:0",
                List.<BindableService>of(),
                new Serve.Options()
                        .withLogger(System.err::println)
                        .withOnListen(System.out::println));
    }

    private static holons.v1.Describe.DescribeResponse holonMetaResponse() {
        return holons.v1.Describe.DescribeResponse.newBuilder()
                .setManifest(Manifest.HolonManifest.newBuilder()
                        .setIdentity(Manifest.HolonManifest.Identity.newBuilder()
                                .setUuid("proto-less-static-0001")
                                .setGivenName("Static")
                                .setFamilyName("Only")
                                .setMotto("Served without runtime proto parsing.")
                                .setComposer("describe-test")
                                .setStatus("draft")
                                .setBorn("2026-03-23")
                                .build())
                        .setDescription("Static describe response used for proto-less runtime verification.")
                        .setLang("java")
                        .setKind("native")
                        .build())
                .addServices(holons.v1.Describe.ServiceDoc.newBuilder()
                        .setName("static.v1.Echo")
                        .setDescription("Static-only runtime metadata.")
                        .addMethods(holons.v1.Describe.MethodDoc.newBuilder()
                                .setName("Ping")
                                .setDescription("Replies with static metadata.")
                                .build())
                        .build())
                .build();
    }
}
