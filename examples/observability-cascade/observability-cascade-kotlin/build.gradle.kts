import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile

plugins {
    application
    kotlin("jvm") version "2.2.21"
}

group = "org.organicprogramming"
version = "0.1.0"
val sharedJvmTarget = JavaVersion.VERSION_20

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
    implementation("io.grpc:grpc-protobuf:1.76.0")
    implementation("io.grpc:grpc-stub:1.76.0")
    implementation("io.grpc:grpc-netty-shaded:1.76.0")
    implementation("com.google.protobuf:protobuf-java:4.34.1")
    implementation("com.google.protobuf:protobuf-kotlin:4.34.1")
    implementation("com.google.code.gson:gson:2.11.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
}

sourceSets {
    main {
        java.srcDirs("src/main/kotlin", "gen", "gen/kotlin")
    }
}

application {
    mainClass.set("org.organicprogramming.observability.cascade.kotlinapp.MainKt")
}

tasks.withType<KotlinJvmCompile>().configureEach {
    compilerOptions {
        jvmTarget.set(JvmTarget.fromTarget(sharedJvmTarget.majorVersion))
    }
}
