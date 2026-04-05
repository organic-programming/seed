import groovy.json.JsonOutput
import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import org.jetbrains.kotlin.gradle.tasks.KotlinJvmCompile
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption
import java.nio.file.attribute.PosixFilePermission

plugins {
    kotlin("jvm") version "2.2.21"
    id("org.jetbrains.kotlin.plugin.compose") version "2.2.21"
    id("org.jetbrains.compose") version "1.9.2"
    id("com.google.protobuf") version "0.9.5"
}

group = "org.organicprogramming"
version = "0.1.0"

val sharedJvmTarget = JavaVersion.VERSION_20
val appSlug = "gabriel-greeting-app-kotlin-compose"
val appUuid = "1201adc8-c8a5-48ad-9f9d-b375a12ab93e"
val appGivenName = "Gabriel"
val appFamilyName = "Greeting-App-Kotlin-Compose"
val appMotto = "Kotlin Compose HostUI for the Gabriel greeting service."
val memberSlugs = listOf(
    "gabriel-greeting-go",
    "gabriel-greeting-swift",
    "gabriel-greeting-rust",
    "gabriel-greeting-python",
    "gabriel-greeting-c",
    "gabriel-greeting-cpp",
    "gabriel-greeting-csharp",
    "gabriel-greeting-dart",
    "gabriel-greeting-java",
    "gabriel-greeting-kotlin",
    "gabriel-greeting-node",
    "gabriel-greeting-ruby",
)

java {
    sourceCompatibility = sharedJvmTarget
    targetCompatibility = sharedJvmTarget
}

dependencies {
    implementation(compose.desktop.currentOs)
    implementation(compose.material3)
    implementation(compose.foundation)
    implementation(compose.materialIconsExtended)
    implementation("org.organicprogramming:kotlin-holons:0.1.0")
    implementation("io.grpc:grpc-protobuf:1.60.0")
    implementation("io.grpc:grpc-stub:1.60.0")
    implementation("io.grpc:grpc-netty-shaded:1.60.0")
    implementation("com.google.protobuf:protobuf-java:4.34.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-swing:1.10.2")
    implementation("com.google.code.gson:gson:2.11.0")
    compileOnly("javax.annotation:javax.annotation-api:1.3.2")

    testImplementation(kotlin("test-junit"))
    testImplementation(compose.desktop.uiTestJUnit4)
    testImplementation(compose.desktop.currentOs)
    testImplementation("junit:junit:4.13.2")
    testImplementation("io.grpc:grpc-testing:1.60.0")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.10.2")
    testImplementation("com.google.code.gson:gson:2.11.0")
}

compose.desktop {
    application {
        mainClass = "org.organicprogramming.gabriel.greeting.kotlincompose.MainKt"
    }
}

sourceSets {
    main {
        proto {
            srcDir("../api")
            srcDir("../../../_protos")
            srcDir("../../../../holons/grace-op/_protos")
        }
    }
}

protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:4.34.0"
    }
    plugins {
        create("grpc") {
            artifact = "io.grpc:protoc-gen-grpc-java:1.60.0"
        }
    }
    generateProtoTasks {
        all().configureEach {
            plugins {
                create("grpc")
            }
        }
    }
}

tasks.withType<KotlinJvmCompile>().configureEach {
    compilerOptions {
        jvmTarget.set(JvmTarget.fromTarget(sharedJvmTarget.majorVersion))
    }
}

tasks.withType<Test>().configureEach {
    systemProperty("skiko.renderApi", "SOFTWARE")
    testLogging {
        events("passed", "skipped", "failed")
    }
}

val composeSmokeTest by tasks.registering(Test::class) {
    group = LifecycleBasePlugin.VERIFICATION_GROUP
    description = "Runs the Compose smoke UI test only."
    testClassesDirs = sourceSets["test"].output.classesDirs
    classpath = sourceSets["test"].runtimeClasspath
    systemProperty("skiko.renderApi", "SOFTWARE")
    filter {
        includeTestsMatching("*.ComposeSmokeTest")
        includeTestsMatching("*.ComposeSmokeTest.*")
    }
}

val packageHolonDesktop by tasks.registering {
    group = LifecycleBasePlugin.BUILD_GROUP
    description = "Builds the Kotlin Compose desktop app and wraps it as a .holon package."
    dependsOn(tasks.named("test"))
    dependsOn(tasks.named("jar"))

    doLast {
        val holonTarget = (project.findProperty("holonTarget") as String? ?: currentHolonTarget()).lowercase()
        val runtimeArch = runtimeArchitectureForTarget(holonTarget)
        val exampleRoot = projectDir.parentFile
        val workspaceRoot = exampleRoot.parentFile.parentFile.parentFile
        val packageDir = exampleRoot.toPath().resolve(".op/build/$appSlug.holon")
        val runtimeDir = packageDir.resolve("bin").resolve(runtimeArch)
        val libDir = packageDir.resolve("lib")
        val holonsDir = packageDir.resolve("Holons")
        val appProtoDir = packageDir.resolve("AppProto")

        packageDir.toFile().deleteRecursively()
        Files.createDirectories(runtimeDir)
        Files.createDirectories(libDir)
        Files.createDirectories(holonsDir)
        Files.createDirectories(appProtoDir)

        copyRuntimeClasspath(libDir)
        copyMemberHolons(exampleRoot.parentFile.toPath(), holonsDir)
        copyAppProto(exampleRoot.toPath().resolve("api"), appProtoDir)

        val entrypoint = launcherNameForTarget(holonTarget)
        val launcherPath = runtimeDir.resolve(entrypoint)
        if (holonTarget == "windows") {
            writeWindowsLauncher(launcherPath)
        } else {
            writeUnixLauncher(launcherPath)
        }

        writeHolonJson(packageDir, runtimeArch, entrypoint)
    }
}

fun currentHolonTarget(): String {
    val name = System.getProperty("os.name").lowercase()
    return when {
        "mac" in name || "darwin" in name -> "macos"
        "win" in name -> "windows"
        else -> "linux"
    }
}

fun runtimeArchitectureForTarget(target: String): String {
    val arch = when (System.getProperty("os.arch").lowercase()) {
        "aarch64", "arm64" -> "arm64"
        else -> "amd64"
    }
    val os = when (target) {
        "macos" -> "darwin"
        "windows" -> "windows"
        else -> "linux"
    }
    return "${os}_$arch"
}

fun launcherNameForTarget(target: String): String = if (target == "windows") "$appSlug.cmd" else appSlug

fun copyRuntimeClasspath(libDir: Path) {
    val jarFile = tasks.named<Jar>("jar").get().archiveFile.get().asFile.toPath()
    Files.copy(jarFile, libDir.resolve(jarFile.fileName.toString()), StandardCopyOption.REPLACE_EXISTING)

    val runtimeFiles = configurations.runtimeClasspath.get().resolvedConfiguration.resolvedArtifacts
        .map { it.file.toPath() }
        .distinctBy { it.fileName.toString() }
    runtimeFiles.forEach { artifact ->
        Files.copy(artifact, libDir.resolve(artifact.fileName.toString()), StandardCopyOption.REPLACE_EXISTING)
    }
}

fun copyMemberHolons(examplesRoot: Path, destination: Path) {
    memberSlugs.forEach { slug ->
        val source = examplesRoot.resolve(slug).resolve(".op/build/$slug.holon")
        require(Files.exists(source)) { "missing built member package: $source" }
        copyRecursively(source, destination.resolve("$slug.holon"))
    }
}

fun copyAppProto(source: Path, destination: Path) {
    require(Files.exists(source)) { "missing app proto directory: $source" }
    copyRecursively(source, destination)
}

fun copyRecursively(source: Path, destination: Path) {
    if (Files.isDirectory(source)) {
        Files.createDirectories(destination)
        Files.walk(source).use { walk ->
            walk.forEach { path ->
                val relative = source.relativize(path)
                val target = destination.resolve(relative.toString())
                if (Files.isDirectory(path)) {
                    Files.createDirectories(target)
                } else {
                    Files.createDirectories(target.parent)
                    Files.copy(path, target, StandardCopyOption.REPLACE_EXISTING)
                }
            }
        }
        return
    }

    Files.createDirectories(destination.parent)
    Files.copy(source, destination, StandardCopyOption.REPLACE_EXISTING)
}

fun writeUnixLauncher(path: Path) {
    val script = """
        |#!/usr/bin/env bash
        |set -euo pipefail
        |SCRIPT_DIR="${'$'}(cd -- "${'$'}(dirname "${'$'}{BASH_SOURCE[0]}")" && pwd)"
        |PACKAGE_DIR="${'$'}(cd -- "${'$'}SCRIPT_DIR/../.." && pwd)"
        |if [[ -n "${'$'}{JAVA_HOME:-}" ]]; then
        |  JAVA_BIN="${'$'}JAVA_HOME/bin/java"
        |else
        |  JAVA_BIN="${'$'}{JAVA:-java}"
        |fi
        |exec "${'$'}JAVA_BIN" -cp "${'$'}PACKAGE_DIR/lib/*" org.organicprogramming.gabriel.greeting.kotlincompose.MainKt "${'$'}@"
        |""".trimMargin()
    Files.writeString(path, script)
    val permissions = mutableSetOf(
        PosixFilePermission.OWNER_READ,
        PosixFilePermission.OWNER_WRITE,
        PosixFilePermission.OWNER_EXECUTE,
        PosixFilePermission.GROUP_READ,
        PosixFilePermission.GROUP_EXECUTE,
        PosixFilePermission.OTHERS_READ,
        PosixFilePermission.OTHERS_EXECUTE,
    )
    runCatching { Files.setPosixFilePermissions(path, permissions) }
}

fun writeWindowsLauncher(path: Path) {
    val script = """
        |@echo off
        |set SCRIPT_DIR=%~dp0
        |set PACKAGE_DIR=%SCRIPT_DIR%..\..\
        |if not "%JAVA_HOME%"=="" (
        |  set JAVA_BIN=%JAVA_HOME%\bin\java.exe
        |) else (
        |  set JAVA_BIN=java
        |)
        |"%JAVA_BIN%" -cp "%PACKAGE_DIR%lib\*" org.organicprogramming.gabriel.greeting.kotlincompose.MainKt %*
        |""".trimMargin()
    Files.writeString(path, script)
}

fun writeHolonJson(packageDir: Path, runtimeArch: String, entrypoint: String) {
    val payload = mapOf(
        "schema" to "holon-package/v1",
        "slug" to appSlug,
        "uuid" to appUuid,
        "identity" to mapOf(
            "given_name" to appGivenName,
            "family_name" to appFamilyName,
            "motto" to appMotto,
        ),
        "lang" to "kotlin",
        "runner" to "recipe",
        "status" to "draft",
        "kind" to "composite",
        "transport" to "stdio",
        "entrypoint" to entrypoint,
        "architectures" to listOf(runtimeArch),
        "has_dist" to false,
        "has_source" to false,
    )
    Files.writeString(
        packageDir.resolve(".holon.json"),
        JsonOutput.prettyPrint(JsonOutput.toJson(payload)) + "\n",
    )
}
