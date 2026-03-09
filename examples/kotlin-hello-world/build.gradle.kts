import org.gradle.api.JavaVersion
import org.gradle.jvm.tasks.Jar
import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.gradle.api.tasks.JavaExec

plugins {
    kotlin("jvm") version "2.1.20"
    application
}

group = "org.organicprogramming"
version = "0.1.0"

val sharedJvmTarget = JavaVersion.VERSION_21

java {
    sourceCompatibility = sharedJvmTarget
    targetCompatibility = sharedJvmTarget
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_21)
    }
}

application {
    mainClass.set("org.organicprogramming.hello.HelloKt")
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("org.organicprogramming:kotlin-holons:0.1.0")
    implementation("io.grpc:grpc-netty-shaded:1.60.0")
    implementation("io.grpc:grpc-stub:1.60.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.8.1")
    testImplementation(kotlin("test"))
}

tasks.test {
    useJUnitPlatform()
}

tasks.withType<Jar>().configureEach {
    manifest {
        attributes["Main-Class"] = "org.organicprogramming.hello.HelloKt"
    }
}

tasks.register<JavaExec>("runConnectExample") {
    classpath = sourceSets.main.get().runtimeClasspath
    mainClass.set("org.organicprogramming.hello.ConnectExampleKt")
    workingDir = projectDir
}
