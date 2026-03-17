#include <arpa/inet.h>
#include <netinet/in.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#ifndef GABRIEL_GREETING_C_PUBLIC_BINARY
#error "GABRIEL_GREETING_C_PUBLIC_BINARY must be defined"
#endif

#ifndef GABRIEL_GREETING_C_PROTO_DIR
#error "GABRIEL_GREETING_C_PROTO_DIR must be defined"
#endif

static int expect(int condition, const char *message) {
  if (condition) {
    return 1;
  }
  fprintf(stderr, "%s\n", message);
  return 0;
}

static int pick_port(void) {
  int fd;
  int port;
  struct sockaddr_in addr;
  socklen_t len = sizeof(addr);

  fd = socket(AF_INET, SOCK_STREAM, 0);
  memset(&addr, 0, sizeof(addr));
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
  addr.sin_port = 0;
  bind(fd, (struct sockaddr *)&addr, sizeof(addr));
  getsockname(fd, (struct sockaddr *)&addr, &len);
  port = ntohs(addr.sin_port);
  close(fd);
  return port;
}

static char *capture_command(const char *command) {
  FILE *pipe = popen(command, "r");
  char *buffer = NULL;
  size_t used = 0;
  size_t capacity = 0;
  int ch;

  if (pipe == NULL) {
    return NULL;
  }

  while ((ch = fgetc(pipe)) != EOF) {
    if (used + 2 > capacity) {
      size_t next_capacity = capacity == 0 ? 256 : capacity * 2;
      char *next = realloc(buffer, next_capacity);
      if (next == NULL) {
        free(buffer);
        pclose(pipe);
        return NULL;
      }
      buffer = next;
      capacity = next_capacity;
    }
    buffer[used++] = (char)ch;
  }

  if (buffer == NULL) {
    buffer = calloc(1, 1);
  } else {
    buffer[used] = '\0';
  }

  pclose(pipe);
  return buffer;
}

static int wait_for_server(int port) {
  int attempt;
  char command[512];

  for (attempt = 0; attempt < 60; ++attempt) {
    char *output;
    snprintf(command, sizeof(command),
             "grpcurl -plaintext -import-path %s -proto v1/greeting.proto "
             "-d '{}' 127.0.0.1:%d greeting.v1.GreetingService/ListLanguages 2>/dev/null",
             GABRIEL_GREETING_C_PROTO_DIR, port);
    output = capture_command(command);
    if (output != NULL && strstr(output, "\"code\": \"en\"") != NULL) {
      free(output);
      return 1;
    }
    free(output);
    usleep(250000);
  }

  return 0;
}

static void stop_server_group(pid_t pid) {
  int attempt;
  int status;

  if (pid <= 0) {
    return;
  }

  kill(-pid, SIGTERM);
  for (attempt = 0; attempt < 20; ++attempt) {
    if (waitpid(pid, &status, WNOHANG) == pid) {
      return;
    }
    usleep(100000);
  }

  kill(-pid, SIGKILL);
  waitpid(pid, &status, 0);
}

int main(void) {
  const int port = pick_port();
  char port_text[16];
  pid_t pid;
  char command[512];
  char *output;

  snprintf(port_text, sizeof(port_text), "%d", port);

  pid = fork();
  if (pid == 0) {
    setpgid(0, 0);
    execl(GABRIEL_GREETING_C_PUBLIC_BINARY, GABRIEL_GREETING_C_PUBLIC_BINARY,
          "serve", "--port", port_text, (char *)NULL);
    _exit(127);
  }

  if (!expect(wait_for_server(port), "server failed to start")) {
    stop_server_group(pid);
    return 1;
  }

  snprintf(command, sizeof(command),
           "grpcurl -plaintext -import-path %s -proto v1/greeting.proto "
           "-d '{}' 127.0.0.1:%d greeting.v1.GreetingService/ListLanguages 2>/dev/null",
           GABRIEL_GREETING_C_PROTO_DIR, port);
  output = capture_command(command);
  if (!expect(output != NULL && strstr(output, "\"code\": \"en\"") != NULL,
              "ListLanguages grpcurl call failed")) {
    free(output);
    stop_server_group(pid);
    return 1;
  }
  free(output);

  snprintf(command, sizeof(command),
           "grpcurl -plaintext -import-path %s -proto v1/greeting.proto "
           "-d '{\"name\":\"Alice\",\"lang_code\":\"fr\"}' "
           "127.0.0.1:%d greeting.v1.GreetingService/SayHello 2>/dev/null",
           GABRIEL_GREETING_C_PROTO_DIR, port);
  output = capture_command(command);
  if (!expect(output != NULL && strstr(output, "Bonjour Alice") != NULL,
              "SayHello grpcurl call failed")) {
    free(output);
    stop_server_group(pid);
    return 1;
  }
  free(output);

  stop_server_group(pid);
  return 0;
}
