fun main() {
    val script = System.getenv("CHARON_RUN_SCRIPT")
        ?.takeIf { it.isNotBlank() }
        ?: "scripts/run.sh"
    val code = ProcessBuilder("/bin/sh", script)
        .inheritIO()
        .start()
        .waitFor()
    kotlin.system.exitProcess(code)
}
