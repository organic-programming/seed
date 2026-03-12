import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile
import org.jetbrains.compose.desktop.application.dsl.TargetFormat

plugins {
    kotlin("jvm") version "2.2.21"
    kotlin("plugin.compose") version "2.2.21"
    id("org.jetbrains.compose") version "1.9.2"
    id("com.google.protobuf") version "0.9.5"
}

repositories {
    google()
    mavenCentral()
    maven("https://maven.pkg.jetbrains.space/public/p/compose/dev")
}

val daemonBinaryName = "gudule-daemon-greeting-gokotlin"
val grpcVersion = "1.76.0"
val grpcKotlinVersion = "1.4.3"
val protobufVersion = "4.32.1"
val coroutinesVersion = "1.10.2"
val appJvmTarget = 20

dependencies {
    implementation(compose.desktop.currentOs)
    implementation(compose.material3)
    implementation("org.organicprogramming:kotlin-holons:0.1.0")
    implementation("io.grpc:grpc-kotlin-stub:$grpcKotlinVersion")
    implementation("io.grpc:grpc-protobuf:$grpcVersion")
    implementation("io.grpc:grpc-stub:$grpcVersion")
    implementation("io.grpc:grpc-netty-shaded:$grpcVersion")
    implementation("com.google.protobuf:protobuf-kotlin:$protobufVersion")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-swing:$coroutinesVersion")
    implementation("javax.annotation:javax.annotation-api:1.3.2")
}

sourceSets {
    named("main") {
        java.srcDir("build/generated/sources/proto/main/java")
        java.srcDir("build/generated/sources/proto/main/grpc")
        java.srcDir("build/generated/sources/proto/main/grpckt")
        java.srcDir("build/generated/sources/proto/main/kotlin")
    }
}

protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:$protobufVersion"
    }
    plugins {
        create("grpc") {
            artifact = "io.grpc:protoc-gen-grpc-java:$grpcVersion"
        }
        create("grpckt") {
            artifact = "io.grpc:protoc-gen-grpc-kotlin:$grpcKotlinVersion:jdk8@jar"
        }
    }
    generateProtoTasks {
        all().configureEach {
            builtins {
                create("kotlin")
            }
            plugins {
                create("grpc")
                create("grpckt")
            }
        }
    }
}

val stageDaemon by tasks.registering(Copy::class) {
    from(layout.projectDirectory.dir("../greeting-daemon"))
    include(daemonBinaryName)
    into(layout.buildDirectory.dir("generated/embedded-daemon"))
}

tasks.processResources {
    dependsOn(stageDaemon)
    from(stageDaemon) {
        into("embedded")
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

compose.desktop {
    application {
        mainClass = "greeting.gokotlin.MainKt"

        nativeDistributions {
            targetFormats(TargetFormat.Dmg)
            packageName = "gudule-greeting-gokotlin"
            packageVersion = "1.0.0"
        }
    }
}
