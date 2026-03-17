#include "api/public.h"

#include "internal/greetings.h"

#include <ctype.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>

static upb_StringView arena_copy_range(upb_Arena *arena, const char *data,
                                       size_t size) {
  char *copy = (char *)upb_Arena_Malloc(arena, size + 1);
  if (copy == NULL) {
    return upb_StringView_FromDataAndSize("", 0);
  }
  memcpy(copy, data, size);
  copy[size] = '\0';
  return upb_StringView_FromDataAndSize(copy, size);
}

static upb_StringView arena_copy_cstr(upb_Arena *arena, const char *data) {
  return arena_copy_range(arena, data, strlen(data));
}

static upb_StringView trim_name_copy(upb_Arena *arena, upb_StringView name) {
  size_t start = 0;
  size_t end = name.size;

  while (start < end && isspace((unsigned char)name.data[start])) {
    ++start;
  }
  while (end > start && isspace((unsigned char)name.data[end - 1])) {
    --end;
  }

  return arena_copy_range(arena, name.data + start, end - start);
}

static upb_StringView format_greeting(upb_Arena *arena, const char *pattern,
                                      upb_StringView name) {
  const char *marker = strstr(pattern, "%s");
  if (marker == NULL) {
    return arena_copy_cstr(arena, pattern);
  }

  {
    const size_t prefix_len = (size_t)(marker - pattern);
    const size_t suffix_len = strlen(marker + 2);
    const size_t out_len = prefix_len + name.size + suffix_len;
    char *buffer = (char *)upb_Arena_Malloc(arena, out_len + 1);
    if (buffer == NULL) {
      return upb_StringView_FromDataAndSize("", 0);
    }

    memcpy(buffer, pattern, prefix_len);
    memcpy(buffer + prefix_len, name.data, name.size);
    memcpy(buffer + prefix_len + name.size, marker + 2, suffix_len);
    buffer[out_len] = '\0';
    return upb_StringView_FromDataAndSize(buffer, out_len);
  }
}

greeting_v1_ListLanguagesResponse *
gabriel_greeting_c_list_languages(upb_Arena *arena) {
  size_t i;
  greeting_v1_ListLanguagesResponse *response =
      greeting_v1_ListLanguagesResponse_new(arena);
  if (response == NULL) {
    return NULL;
  }

  for (i = 0; i < gabriel_greeting_c_greetings_count(); ++i) {
    const gabriel_greeting_c_greeting *entry = gabriel_greeting_c_greeting_at(i);
    greeting_v1_Language *language =
        greeting_v1_ListLanguagesResponse_add_languages(response, arena);
    if (language == NULL) {
      return NULL;
    }
    greeting_v1_Language_set_code(language,
                                  arena_copy_cstr(arena, entry->lang_code));
    greeting_v1_Language_set_name(language,
                                  arena_copy_cstr(arena, entry->lang_english));
    greeting_v1_Language_set_native(language,
                                    arena_copy_cstr(arena, entry->lang_native));
  }

  return response;
}

greeting_v1_SayHelloResponse *
gabriel_greeting_c_say_hello(const greeting_v1_SayHelloRequest *request,
                             upb_Arena *arena) {
  greeting_v1_SayHelloResponse *response;
  upb_StringView trimmed_name;
  upb_StringView name_to_use;
  const gabriel_greeting_c_greeting *entry;

  if (request == NULL) {
    return NULL;
  }

  response = greeting_v1_SayHelloResponse_new(arena);
  if (response == NULL) {
    return NULL;
  }

  entry = gabriel_greeting_c_lookup(greeting_v1_SayHelloRequest_lang_code(request));
  trimmed_name = trim_name_copy(arena, greeting_v1_SayHelloRequest_name(request));
  if (trimmed_name.size == 0) {
    name_to_use = arena_copy_cstr(arena, entry->default_name);
  } else {
    name_to_use = trimmed_name;
  }

  greeting_v1_SayHelloResponse_set_greeting(
      response, format_greeting(arena, entry->template_string, name_to_use));
  greeting_v1_SayHelloResponse_set_language(
      response, arena_copy_cstr(arena, entry->lang_english));
  greeting_v1_SayHelloResponse_set_lang_code(
      response, arena_copy_cstr(arena, entry->lang_code));
  return response;
}
