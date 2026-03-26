package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class IdentityTest {

    @Test
    void resolveProtoFileExposesResolvedManifest(@TempDir Path tmp) throws IOException {
        Path manifest = writeHolon(tmp.resolve("v1").resolve(Identity.PROTO_MANIFEST_FILE_NAME));

        Identity.ResolvedManifest resolved = Identity.resolveProtoFile(manifest);

        assertEquals(manifest.toAbsolutePath().normalize(), resolved.sourcePath());
        assertEquals("test-uuid-1234", resolved.identity().uuid());
        assertEquals("Gabriel", resolved.identity().givenName());
        assertEquals("Greeting-Java", resolved.identity().familyName());
        assertEquals("0.1.7", resolved.identity().version());
        assertEquals("gabriel-greeting-java", resolved.identity().slug());
        assertEquals("A Java greeting holon.", resolved.description());
        assertEquals("native", resolved.kind());
        assertEquals("gradle", resolved.buildRunner());
        assertEquals("./src/main/java/example/Main.java", resolved.buildMain());
        assertEquals("gabriel-greeting-java", resolved.artifactBinary());
        assertEquals(List.of("build.gradle", "settings.gradle"), resolved.requiredFiles());
        assertEquals(List.of("members/daemon", "members/app"), resolved.memberPaths());
        assertEquals(1, resolved.skills().size());
        assertEquals("multilingual-greeter", resolved.skills().get(0).name());
        assertEquals(1, resolved.sequences().size());
        assertEquals("say-hello", resolved.sequences().get(0).name());
        assertEquals(1, resolved.sequences().get(0).params().size());
        assertTrue(resolved.sequences().get(0).params().get(0).required());
    }

    @Test
    void resolveFindsNearbyHolonProto(@TempDir Path tmp) throws IOException {
        Path root = tmp.resolve("gabriel-holon");
        Path protoDir = root.resolve("protos");
        Files.createDirectories(protoDir);
        Path manifest = writeHolon(root.resolve("api").resolve("v1").resolve(Identity.PROTO_MANIFEST_FILE_NAME));

        Identity.ResolvedManifest resolved = Identity.resolve(protoDir);
        Identity.ManifestIdentity manifestIdentity = Identity.resolveManifest(protoDir);

        assertEquals(manifest.toAbsolutePath().normalize(), resolved.sourcePath());
        assertEquals("gabriel-greeting-java", resolved.identity().slug());
        assertEquals(resolved.identity(), manifestIdentity.identity());
        assertEquals(resolved.sourcePath(), manifestIdentity.sourcePath());
    }

    private static Path writeHolon(Path path) throws IOException {
        Files.createDirectories(path.getParent());
        Files.writeString(path, """
                syntax = "proto3";
                package holons.test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    uuid: "test-uuid-1234"
                    given_name: "Gabriel"
                    family_name: "Greeting-Java"
                    motto: "Greets in many languages."
                    composer: "identity-test"
                    status: "draft"
                    born: "2026-03-23"
                    version: "0.1.7"
                    aliases: ["gabriel", "greeting-java"]
                  }
                  description: "A Java greeting holon."
                  lang: "java"
                  kind: "native"
                  build: {
                    runner: "gradle"
                    main: "./src/main/java/example/Main.java"
                    members: [
                      {path: "members/daemon"},
                      {path: "members/app"}
                    ]
                  }
                  requires: {
                    files: ["build.gradle", "settings.gradle"]
                  }
                  artifacts: {
                    binary: "gabriel-greeting-java"
                  }
                  skills: [{
                    name: "multilingual-greeter"
                    description: "Greets a user."
                    when: "The user asks for a greeting."
                    steps: ["Call SayHello"]
                  }]
                  sequences: [{
                    name: "say-hello"
                    description: "Greet one user."
                    params: [{name: "name", description: "Person to greet", required: true}]
                    steps: ["op gabriel-greeting-java sayHello {{ .name }}"]
                  }]
                };
                """);
        return path;
    }
}
