#pragma once

#include "upb/message/message.h"

#include <stddef.h>

typedef struct {
  const char *lang_code;
  const char *lang_english;
  const char *lang_native;
  const char *template_string;
  const char *default_name;
} gabriel_greeting_c_greeting;

size_t gabriel_greeting_c_greetings_count(void);
const gabriel_greeting_c_greeting *gabriel_greeting_c_greeting_at(size_t index);
const gabriel_greeting_c_greeting *
gabriel_greeting_c_lookup(upb_StringView lang_code);
