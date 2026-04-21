import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile

plugins {
    kotlin("jvm") version "2.1.20"
}

group = "org.organicprogramming"
version = "0.1.0"
val libraryJvmTarget = 21



repositories {
    mavenCentral()
}

sourceSets {
    main {
        java.srcDir("src/main/kotlin/gen")
    }
}

dependencies {
    implementation("io.grpc:grpc-netty-shaded:1.60.0")
    implementation("io.grpc:grpc-protobuf:1.60.0")
    implementation("io.grpc:grpc-services:1.60.0")
    implementation("io.grpc:grpc-stub:1.60.0")
    implementation("com.google.protobuf:protobuf-java:4.34.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.8.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    testImplementation(kotlin("test"))
    testImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    testImplementation("com.squareup.okhttp3:okhttp-tls:4.12.0")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.8.1")
}

tasks.test {
    useJUnitPlatform()
}

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(libraryJvmTarget))
    }
}

tasks.withType<JavaCompile>().configureEach {
    options.release.set(libraryJvmTarget)
}

tasks.withType<KotlinJvmCompile>().configureEach {
    compilerOptions {
        jvmTarget.set(JvmTarget.fromTarget(libraryJvmTarget.toString()))
    }
}
