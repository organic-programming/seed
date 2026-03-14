#include <stdlib.h>
#include <stdio.h>
#include <string.h>

int main(int argc, char** argv) {
    char script_path[512] = "scripts/run.sh";
    if (argc > 0) {
        char* last_slash = strrchr(argv[0], '/');
        if (last_slash) {
            snprintf(script_path, sizeof(script_path), "%.*s/scripts/run.sh", (int)(last_slash - argv[0]), argv[0]);
        }
    }
    
    const char* script = getenv("CHARON_RUN_SCRIPT");
    if (!script || script[0] == '\0') {
        script = script_path;
    }
    
    char command[512];
    snprintf(command, sizeof(command), "/bin/sh %s", script);
    return system(command);
}
