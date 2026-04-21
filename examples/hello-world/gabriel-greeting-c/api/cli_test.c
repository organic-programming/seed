#include "api/cli.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int expect(int condition, const char *message) {
  if (condition) {
    return 1;
  }
  fprintf(stderr, "%s\n", message);
  return 0;
}

static char *read_file(FILE *file) {
  long size;
  char *buffer;

  fflush(file);
  fseek(file, 0, SEEK_END);
  size = ftell(file);
  rewind(file);

  buffer = (char *)calloc((size_t)size + 1, 1);
  fread(buffer, 1, (size_t)size, file);
  buffer[size] = '\0';
  return buffer;
}

int main(void) {
  FILE *stdout_file;
  FILE *stderr_file;
  char *stdout_text;
  char *stderr_text;
  int exit_code;

  stdout_file = tmpfile();
  stderr_file = tmpfile();
  exit_code = gabriel_greeting_c_run_cli(
      1, (char *[]){"version"}, stdout_file, stderr_file);
  stdout_text = read_file(stdout_file);
  stderr_text = read_file(stderr_file);
  if (!expect(exit_code == 0, "version should succeed") ||
      !expect(strstr(stdout_text, gabriel_greeting_c_version) != NULL,
              "version output mismatch") ||
      !expect(stderr_text[0] == '\0', "version should not write stderr")) {
    return 1;
  }
  free(stdout_text);
  free(stderr_text);
  fclose(stdout_file);
  fclose(stderr_file);

  stdout_file = tmpfile();
  stderr_file = tmpfile();
  exit_code = gabriel_greeting_c_run_cli(
      1, (char *[]){"help"}, stdout_file, stderr_file);
  stdout_text = read_file(stdout_file);
  if (!expect(exit_code == 0, "help should succeed") ||
      !expect(strstr(stdout_text, "usage: gabriel-greeting-c") != NULL,
              "help should print usage") ||
      !expect(strstr(stdout_text, "listLanguages") != NULL,
              "help should mention listLanguages")) {
    return 1;
  }
  free(stdout_text);
  fclose(stdout_file);
  fclose(stderr_file);

  stdout_file = tmpfile();
  stderr_file = tmpfile();
  exit_code = gabriel_greeting_c_run_cli(
      3, (char *[]){"listLanguages", "--format", "json"}, stdout_file, stderr_file);
  stdout_text = read_file(stdout_file);
  if (!expect(exit_code == 0, "listLanguages json should succeed") ||
      !expect(strstr(stdout_text, "\"name\":\"English\"") != NULL,
              "listLanguages json should contain English")) {
    return 1;
  }
  free(stdout_text);
  fclose(stdout_file);
  fclose(stderr_file);

  stdout_file = tmpfile();
  stderr_file = tmpfile();
  exit_code = gabriel_greeting_c_run_cli(
      3, (char *[]){"sayHello", "Bob", "fr"}, stdout_file, stderr_file);
  stdout_text = read_file(stdout_file);
  if (!expect(exit_code == 0, "sayHello text should succeed") ||
      !expect(strcmp(stdout_text, "Bonjour Bob\n") == 0,
              "sayHello text should greet Bob in French")) {
    return 1;
  }
  free(stdout_text);
  fclose(stdout_file);
  fclose(stderr_file);

  stdout_file = tmpfile();
  stderr_file = tmpfile();
  exit_code = gabriel_greeting_c_run_cli(
      2, (char *[]){"sayHello", "--json"}, stdout_file, stderr_file);
  stdout_text = read_file(stdout_file);
  if (!expect(exit_code == 0, "sayHello json should succeed") ||
      !expect(strstr(stdout_text, "\"greeting\":\"Hello Mary\"") != NULL,
              "sayHello json should include default greeting") ||
      !expect(strstr(stdout_text, "\"langCode\":\"en\"") != NULL,
              "sayHello json should include camelCase langCode")) {
    return 1;
  }
  free(stdout_text);
  fclose(stdout_file);
  fclose(stderr_file);

  return 0;
}
