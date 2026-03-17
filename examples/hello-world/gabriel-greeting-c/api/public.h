#pragma once

#include "greeting.upb.h"

#include "upb/mem/arena.h"

greeting_v1_ListLanguagesResponse *
gabriel_greeting_c_list_languages(upb_Arena *arena);

greeting_v1_SayHelloResponse *
gabriel_greeting_c_say_hello(const greeting_v1_SayHelloRequest *request,
                             upb_Arena *arena);
