public final class Main {
    public static void main(String[] args) throws Exception {
        String script = System.getenv("CHARON_RUN_SCRIPT");
        if (script == null || script.isBlank()) {
            script = "scripts/run.sh";
        }
        int code = new ProcessBuilder("/bin/sh", script)
            .inheritIO()
            .start()
            .waitFor();
        System.exit(code);
    }
}
