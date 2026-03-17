import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile
import java.nio.file.Files

plugins {
    application
    kotlin("jvm") version "2.1.20"
}

group = "org.organicprogramming"
version = "0.4.2"

val daemonBinaryName = "gudule-daemon-greeting-kotlin"
val grpcVersion = "1.76.0"
val grpcKotlinVersion = "1.4.3"
val protobufVersion = "4.34.0"
val coroutinesVersion = "1.10.2"
val appJvmTarget = 20

repositories {
    mavenCentral()
}

dependencies {
    implementation("org.organicprogramming:kotlin-holons:0.1.0")
    implementation("io.grpc:grpc-kotlin-stub:$grpcKotlinVersion")
    implementation("io.grpc:grpc-protobuf:$grpcVersion")
    implementation("io.grpc:grpc-stub:$grpcVersion")
    implementation("io.grpc:grpc-netty-shaded:$grpcVersion")
    implementation("com.google.protobuf:protobuf-kotlin:$protobufVersion")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:$coroutinesVersion")
    implementation("javax.annotation:javax.annotation-api:1.3.2")

    testImplementation(kotlin("test"))
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:$coroutinesVersion")
}

sourceSets {
    main {
        java.srcDir("src/main/kotlin")
        java.srcDir("gen/kotlin")
    }
    test {
        java.srcDir("src/test/kotlin")
    }
}

application {
    mainClass.set("org.organicprogramming.gudule.greeting.daemon.kotlin.MainKt")
}

tasks.withType<Test>().configureEach {
    useJUnitPlatform()
}

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(appJvmTarget))
    }
}

tasks.withType<JavaCompile>().configureEach {
    options.release.set(appJvmTarget)
}

tasks.withType<KotlinJvmCompile>().configureEach {
    compilerOptions {
        jvmTarget.set(JvmTarget.fromTarget(appJvmTarget.toString()))
    }
}

val rewriteInstallLauncher by tasks.registering {
    dependsOn(tasks.installDist)

    doLast {
        val launcher = layout.buildDirectory.file("install/$daemonBinaryName/bin/$daemonBinaryName").get().asFile
        val recipeRoot = layout.projectDirectory.asFile.absolutePath
        val installRoot = launcher.parentFile.parentFile.absolutePath

        launcher.writeText(
            """
            #!/bin/sh
            set -eu
            APP_HOME=${'$'}{APP_HOME:-$installRoot}
            CLASSPATH="${'$'}APP_HOME/lib/*"
            exec java -Dgudule.recipe.root="$recipeRoot" -cp "${'$'}CLASSPATH" org.organicprogramming.gudule.greeting.daemon.kotlin.MainKt "${'$'}@"
            """.trimIndent() + "\n"
        )
        launcher.setExecutable(true)
    }
}

tasks.installDist {
    finalizedBy(rewriteInstallLauncher)
}
