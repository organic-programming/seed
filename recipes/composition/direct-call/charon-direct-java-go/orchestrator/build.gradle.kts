import org.gradle.api.tasks.compile.JavaCompile

plugins {
    application
}

repositories {
    mavenCentral()
}

tasks.withType<JavaCompile>().configureEach {
    options.release.set(20)
}

application {
    mainClass.set("Main")
}

tasks.test {
    enabled = false
}
