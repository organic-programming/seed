#include "api/cli.h"

#include "api/public.h"
#include "internal/server.h"
#include "holons/holons.h"

#include <ctype.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

const char *gabriel_greeting_c_version = "gabriel-greeting-c 0.1.147";

typedef enum {
  GABRIEL_GREETING_C_FORMAT_TEXT = 0,
  GABRIEL_GREETING_C_FORMAT_JSON = 1
} gabriel_greeting_c_output_format;

typedef struct {
  gabriel_greeting_c_output_format format;
  const char *lang;
  int positional_count;
  const char *positionals[2];
} gabriel_greeting_c_command_options;

static void canonical_command(const char *raw, char *out, size_t out_len) {
  size_t i;
  size_t used = 0;
  if (out_len == 0) {
    return;
  }
  for (i = 0; raw != NULL && raw[i] != '\0'; ++i) {
    unsigned char ch = (unsigned char)raw[i];
    if (ch == '-' || ch == '_' || isspace(ch)) {
      continue;
    }
    if (used + 1 >= out_len) {
      break;
    }
    out[used++] = (char)tolower(ch);
  }
  out[used] = '\0';
}

static int parse_output_format(const char *raw,
                               gabriel_greeting_c_output_format *format,
                               char *err, size_t err_len) {
  char normalized[32];
  canonical_command(raw == NULL ? "" : raw, normalized, sizeof(normalized));
  if (normalized[0] == '\0' || strcmp(normalized, "text") == 0 ||
      strcmp(normalized, "txt") == 0) {
    *format = GABRIEL_GREETING_C_FORMAT_TEXT;
    return 0;
  }
  if (strcmp(normalized, "json") == 0) {
    *format = GABRIEL_GREETING_C_FORMAT_JSON;
    return 0;
  }
  snprintf(err, err_len, "unsupported format \"%s\"", raw);
  return -1;
}

static int parse_command_options(int argc, char **argv,
                                 gabriel_greeting_c_command_options *options,
                                 char *err, size_t err_len) {
  int i;
  memset(options, 0, sizeof(*options));
  options->format = GABRIEL_GREETING_C_FORMAT_TEXT;

  for (i = 0; i < argc; ++i) {
    const char *arg = argv[i];
    if (strcmp(arg, "--json") == 0) {
      options->format = GABRIEL_GREETING_C_FORMAT_JSON;
      continue;
    }
    if (strcmp(arg, "--format") == 0) {
      if (i + 1 >= argc) {
        snprintf(err, err_len, "--format requires a value");
        return -1;
      }
      if (parse_output_format(argv[++i], &options->format, err, err_len) != 0) {
        return -1;
      }
      continue;
    }
    if (strncmp(arg, "--format=", 9) == 0) {
      if (parse_output_format(arg + 9, &options->format, err, err_len) != 0) {
        return -1;
      }
      continue;
    }
    if (strcmp(arg, "--lang") == 0) {
      if (i + 1 >= argc) {
        snprintf(err, err_len, "--lang requires a value");
        return -1;
      }
      options->lang = argv[++i];
      continue;
    }
    if (strncmp(arg, "--lang=", 7) == 0) {
      options->lang = arg + 7;
      continue;
    }
    if (strncmp(arg, "--", 2) == 0) {
      snprintf(err, err_len, "unknown flag \"%s\"", arg);
      return -1;
    }
    if (options->positional_count >= 2) {
      snprintf(err, err_len, "accepts at most <name> [lang_code]");
      return -1;
    }
    options->positionals[options->positional_count++] = arg;
  }

  return 0;
}

static void write_json_string(FILE *output, const char *data, size_t size) {
  size_t i;
  fputc('"', output);
  for (i = 0; i < size; ++i) {
    unsigned char ch = (unsigned char)data[i];
    switch (ch) {
    case '\\':
      fputs("\\\\", output);
      break;
    case '"':
      fputs("\\\"", output);
      break;
    case '\b':
      fputs("\\b", output);
      break;
    case '\f':
      fputs("\\f", output);
      break;
    case '\n':
      fputs("\\n", output);
      break;
    case '\r':
      fputs("\\r", output);
      break;
    case '\t':
      fputs("\\t", output);
      break;
    default:
      fputc((int)ch, output);
      break;
    }
  }
  fputc('"', output);
}

static void write_json_string_view(FILE *output, upb_StringView value) {
  write_json_string(output, value.data, value.size);
}

static int write_list_languages_output(FILE *output,
                                       const greeting_v1_ListLanguagesResponse *response,
                                       gabriel_greeting_c_output_format format) {
  size_t count = 0;
  size_t i;
  const greeting_v1_Language *const *languages =
      greeting_v1_ListLanguagesResponse_languages(response, &count);

  if (format == GABRIEL_GREETING_C_FORMAT_TEXT) {
    for (i = 0; i < count; ++i) {
      const upb_StringView code = greeting_v1_Language_code(languages[i]);
      const upb_StringView name = greeting_v1_Language_name(languages[i]);
      const upb_StringView native = greeting_v1_Language_native(languages[i]);
      fprintf(output, "%.*s\t%.*s\t%.*s\n", (int)code.size, code.data,
              (int)name.size, name.data, (int)native.size, native.data);
    }
    return 0;
  }

  fputs("{\"languages\":[", output);
  for (i = 0; i < count; ++i) {
    if (i != 0) {
      fputc(',', output);
    }
    fputs("{\"code\":", output);
    write_json_string_view(output, greeting_v1_Language_code(languages[i]));
    fputs(",\"name\":", output);
    write_json_string_view(output, greeting_v1_Language_name(languages[i]));
    fputs(",\"native\":", output);
    write_json_string_view(output, greeting_v1_Language_native(languages[i]));
    fputc('}', output);
  }
  fputs("]}\n", output);
  return 0;
}

static int write_say_hello_output(FILE *output,
                                  const greeting_v1_SayHelloResponse *response,
                                  gabriel_greeting_c_output_format format) {
  if (format == GABRIEL_GREETING_C_FORMAT_TEXT) {
    const upb_StringView greeting = greeting_v1_SayHelloResponse_greeting(response);
    fprintf(output, "%.*s\n", (int)greeting.size, greeting.data);
    return 0;
  }

  fputs("{\"greeting\":", output);
  write_json_string_view(output, greeting_v1_SayHelloResponse_greeting(response));
  fputs(",\"language\":", output);
  write_json_string_view(output, greeting_v1_SayHelloResponse_language(response));
  fputs(",\"langCode\":", output);
  write_json_string_view(output, greeting_v1_SayHelloResponse_lang_code(response));
  fputs("}\n", output);
  return 0;
}

static int run_list_languages(int argc, char **argv, FILE *stdout_stream,
                              FILE *stderr_stream) {
  gabriel_greeting_c_command_options options;
  char err[256];
  upb_Arena *arena;
  greeting_v1_ListLanguagesResponse *response;

  if (parse_command_options(argc, argv, &options, err, sizeof(err)) != 0) {
    fprintf(stderr_stream, "listLanguages: %s\n", err);
    return 1;
  }
  if (options.positional_count != 0) {
    fprintf(stderr_stream, "listLanguages: accepts no positional arguments\n");
    return 1;
  }

  arena = upb_Arena_New();
  response = gabriel_greeting_c_list_languages(arena);
  if (response == NULL) {
    upb_Arena_Free(arena);
    fprintf(stderr_stream, "listLanguages: failed to build response\n");
    return 1;
  }

  write_list_languages_output(stdout_stream, response, options.format);
  upb_Arena_Free(arena);
  return 0;
}

static int run_say_hello(int argc, char **argv, FILE *stdout_stream,
                         FILE *stderr_stream) {
  gabriel_greeting_c_command_options options;
  char err[256];
  upb_Arena *arena;
  greeting_v1_SayHelloRequest *request;
  greeting_v1_SayHelloResponse *response;

  if (parse_command_options(argc, argv, &options, err, sizeof(err)) != 0) {
    fprintf(stderr_stream, "sayHello: %s\n", err);
    return 1;
  }

  arena = upb_Arena_New();
  request = greeting_v1_SayHelloRequest_new(arena);
  greeting_v1_SayHelloRequest_set_lang_code(request, upb_StringView_FromString("en"));

  if (options.positional_count >= 1) {
    greeting_v1_SayHelloRequest_set_name(
        request, upb_StringView_FromString(options.positionals[0]));
  }
  if (options.positional_count >= 2) {
    if (options.lang != NULL) {
      upb_Arena_Free(arena);
      fprintf(stderr_stream,
              "sayHello: use either a positional lang_code or --lang, not both\n");
      return 1;
    }
    greeting_v1_SayHelloRequest_set_lang_code(
        request, upb_StringView_FromString(options.positionals[1]));
  }
  if (options.lang != NULL) {
    greeting_v1_SayHelloRequest_set_lang_code(
        request, upb_StringView_FromString(options.lang));
  }

  response = gabriel_greeting_c_say_hello(request, arena);
  if (response == NULL) {
    upb_Arena_Free(arena);
    fprintf(stderr_stream, "sayHello: failed to build response\n");
    return 1;
  }

  write_say_hello_output(stdout_stream, response, options.format);
  upb_Arena_Free(arena);
  return 0;
}

int gabriel_greeting_c_run_cli(int argc, char **argv, FILE *stdout_stream,
                               FILE *stderr_stream) {
  char command[64];
  char listen_uri[HOLONS_MAX_URI_LEN];

  if (argc <= 0) {
    gabriel_greeting_c_print_usage(stderr_stream);
    return 1;
  }

  canonical_command(argv[0], command, sizeof(command));
  if (strcmp(command, "serve") == 0) {
    if (holons_parse_flags(argc - 1, argv + 1, listen_uri, sizeof(listen_uri)) != 0) {
      fprintf(stderr_stream, "serve: failed to parse listen options\n");
      return 1;
    }
    fflush(stdout_stream);
    fflush(stderr_stream);
    return gabriel_greeting_c_serve(listen_uri, stderr_stream);
  }
  if (strcmp(command, "version") == 0) {
    fprintf(stdout_stream, "%s\n", gabriel_greeting_c_version);
    return 0;
  }
  if (strcmp(command, "help") == 0) {
    gabriel_greeting_c_print_usage(stdout_stream);
    return 0;
  }
  if (strcmp(command, "listlanguages") == 0) {
    return run_list_languages(argc - 1, argv + 1, stdout_stream, stderr_stream);
  }
  if (strcmp(command, "sayhello") == 0) {
    return run_say_hello(argc - 1, argv + 1, stdout_stream, stderr_stream);
  }

  fprintf(stderr_stream, "unknown command \"%s\"\n", argv[0]);
  gabriel_greeting_c_print_usage(stderr_stream);
  return 1;
}

void gabriel_greeting_c_print_usage(FILE *output) {
  fprintf(output, "usage: gabriel-greeting-c <command> [args] [flags]\n\n");
  fprintf(output, "commands:\n");
  fprintf(output, "  serve [--listen <uri>]                    Start the gRPC server\n");
  fprintf(output, "  version                                  Print version and exit\n");
  fprintf(output, "  help                                     Print usage\n");
  fprintf(output, "  listLanguages [--format text|json]       List supported languages\n");
  fprintf(output,
          "  sayHello [name] [lang_code] [--format text|json] [--lang <code>]\n\n");
  fprintf(output, "examples:\n");
  fprintf(output, "  gabriel-greeting-c serve --listen tcp://:9090\n");
  fprintf(output, "  gabriel-greeting-c listLanguages --format json\n");
  fprintf(output, "  gabriel-greeting-c sayHello Bob fr\n");
  fprintf(output,
          "  gabriel-greeting-c sayHello Bob --lang fr --format json\n");
}
