import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile

plugins {
    application
    kotlin("jvm") version "2.2.21"
}

group = "org.organicprogramming"
version = "0.1.0"
val sharedJvmTarget = JavaVersion.VERSION_21

repositories {
    mavenCentral()
}

java {
    sourceCompatibility = sharedJvmTarget
    targetCompatibility = sharedJvmTarget
}

dependencies {
    implementation("org.organicprogramming:kotlin-holons:0.1.0")
    implementation("io.grpc:grpc-kotlin-stub:1.4.3")
    implementation("io.grpc:grpc-protobuf:1.60.0")
    implementation("io.grpc:grpc-stub:1.60.0")
    implementation("io.grpc:grpc-netty-shaded:1.60.0")
    implementation("io.grpc:grpc-services:1.60.0")
    implementation("com.google.protobuf:protobuf-java:4.34.0")
    implementation("com.google.protobuf:protobuf-java-util:4.34.0")
    implementation("com.google.protobuf:protobuf-kotlin:4.34.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
    implementation("javax.annotation:javax.annotation-api:1.3.2")

    testImplementation(kotlin("test"))
    testImplementation("io.grpc:grpc-testing:1.60.0")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.10.2")
    testImplementation("com.google.code.gson:gson:2.11.0")
}

sourceSets {
    main {
        java.srcDirs("src/main/kotlin", "gen", "gen/kotlin")
    }
    test {
        java.srcDirs("src/test/kotlin")
    }
}

application {
    mainClass.set("org.organicprogramming.gabriel.greeting.kotlinholon.cmd.MainKt")
}

tasks.test {
    useJUnitPlatform()
}

tasks.withType<KotlinJvmCompile>().configureEach {
    compilerOptions {
        jvmTarget.set(JvmTarget.fromTarget(sharedJvmTarget.majorVersion))
    }
}
