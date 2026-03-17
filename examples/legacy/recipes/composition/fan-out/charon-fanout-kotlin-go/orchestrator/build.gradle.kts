import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.gradle.api.tasks.compile.JavaCompile

plugins {
    kotlin("jvm") version "2.2.10"
    application
}

repositories {
    mavenCentral()
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_20)
    }
}

tasks.withType<JavaCompile>().configureEach {
    options.release.set(20)
}

application {
    mainClass.set("MainKt")
}

tasks.test {
    enabled = false
}
