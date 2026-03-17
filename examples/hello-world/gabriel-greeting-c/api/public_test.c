#include "api/public.h"

#include <stdio.h>
#include <string.h>

static int expect(int condition, const char *message) {
  if (condition) {
    return 1;
  }
  fprintf(stderr, "%s\n", message);
  return 0;
}

static int equals_cstr(upb_StringView value, const char *expected) {
  return value.size == strlen(expected) &&
         memcmp(value.data, expected, value.size) == 0;
}

int main(void) {
  upb_Arena *arena = upb_Arena_New();
  greeting_v1_ListLanguagesResponse *languages =
      gabriel_greeting_c_list_languages(arena);
  size_t count = 0;
  const greeting_v1_Language *const *items =
      greeting_v1_ListLanguagesResponse_languages(languages, &count);
  greeting_v1_SayHelloRequest *request =
      greeting_v1_SayHelloRequest_new(arena);
  greeting_v1_SayHelloResponse *response;

  if (!expect(count == 56, "expected 56 languages")) {
    return 1;
  }
  if (!expect(equals_cstr(greeting_v1_Language_code(items[0]), "en"),
              "expected first language to be English")) {
    return 1;
  }
  if (!expect(equals_cstr(greeting_v1_Language_native(items[1]), "Français"),
              "expected French native label")) {
    return 1;
  }

  greeting_v1_SayHelloRequest_set_lang_code(request, upb_StringView_FromString("fr"));
  response = gabriel_greeting_c_say_hello(request, arena);
  if (!expect(equals_cstr(greeting_v1_SayHelloResponse_greeting(response),
                          "Bonjour Marie"),
              "expected localized French default")) {
    return 1;
  }

  greeting_v1_SayHelloRequest_set_name(request, upb_StringView_FromString("Alice"));
  greeting_v1_SayHelloRequest_set_lang_code(request, upb_StringView_FromString("xx"));
  response = gabriel_greeting_c_say_hello(request, arena);
  if (!expect(equals_cstr(greeting_v1_SayHelloResponse_greeting(response),
                          "Hello Alice"),
              "expected English fallback greeting")) {
    return 1;
  }

  upb_Arena_Free(arena);
  return 0;
}
