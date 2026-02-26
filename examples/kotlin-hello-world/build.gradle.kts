import org.gradle.api.JavaVersion
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    kotlin("jvm") version "2.1.20"
    application
}

group = "org.organicprogramming"
version = "0.1.0"

val sharedJvmTarget = JavaVersion.VERSION_23

java {
    sourceCompatibility = sharedJvmTarget
    targetCompatibility = sharedJvmTarget
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_23)
    }
}

application {
    mainClass.set("org.organicprogramming.hello.HelloKt")
}

repositories {
    mavenCentral()
}

dependencies {
    testImplementation(kotlin("test"))
}

tasks.test {
    useJUnitPlatform()
}
