#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "holons/holons.h"

#define MAX_GREETINGS 56

typedef struct {
    const char *code;
    const char *name;
    const char *native;
    const char *template_str;
} Greeting;

const Greeting greetings[MAX_GREETINGS] = {
    {"en", "English", "English", "Hello, %s!"},
    {"fr", "French", "Français", "Bonjour, %s !"},
    {"es", "Spanish", "Español", "Hola, %s!"},
    {"de", "German", "Deutsch", "Hallo, %s!"},
    {"it", "Italian", "Italiano", "Ciao, %s!"},
    {"pt", "Portuguese", "Português", "Olá, %s!"},
    {"nl", "Dutch", "Nederlands", "Hallo, %s!"},
    {"ru", "Russian", "Русский", "Привет, %s!"},
    {"ja", "Japanese", "日本語", "こんにちは、%sさん！"},
    {"zh", "Chinese", "中文", "你好，%s！"},
    {"ko", "Korean", "한국어", "안녕하세요, %s!"},
    {"ar", "Arabic", "العربية", "مرحبا، %s!"},
    {"hi", "Hindi", "हिन्दी", "नमस्ते, %s!"},
    {"tr", "Turkish", "Türkçe", "Merhaba, %s!"},
    {"pl", "Polish", "Polski", "Cześć, %s!"},
    {"sv", "Swedish", "Svenska", "Hej, %s!"},
    {"no", "Norwegian", "Norsk", "Hei, %s!"},
    {"da", "Danish", "Dansk", "Hej, %s!"},
    {"fi", "Finnish", "Suomi", "Hei, %s!"},
    {"cs", "Czech", "Čeština", "Ahoj, %s!"},
    {"ro", "Romanian", "Română", "Bună, %s!"},
    {"hu", "Hungarian", "Magyar", "Szia, %s!"},
    {"el", "Greek", "Ελληνικά", "Γεια σου, %s!"},
    {"th", "Thai", "ไทย", "สวัสดี, %s!"},
    {"vi", "Vietnamese", "Tiếng Việt", "Xin chào, %s!"},
    {"id", "Indonesian", "Bahasa Indonesia", "Halo, %s!"},
    {"ms", "Malay", "Bahasa Melayu", "Hai, %s!"},
    {"sw", "Swahili", "Kiswahili", "Habari, %s!"},
    {"he", "Hebrew", "עברית", "שלום, %s!"},
    {"uk", "Ukrainian", "Українська", "Привіт, %s!"},
    {"bn", "Bengali", "বাংলা", "নমস্কার, %s!"},
    {"ta", "Tamil", "தமிழ்", "வணக்கம், %s!"},
    {"fa", "Persian", "فارسی", "سلام، %s!"},
    {"ur", "Urdu", "اردو", "السلام علیکم، %s!"},
    {"fil", "Filipino", "Filipino", "Kumusta, %s!"},
    {"ca", "Catalan", "Català", "Hola, %s!"},
    {"eu", "Basque", "Euskara", "Kaixo, %s!"},
    {"gl", "Galician", "Galego", "Ola, %s!"},
    {"is", "Icelandic", "Íslenska", "Halló, %s!"},
    {"et", "Estonian", "Eesti", "Tere, %s!"},
    {"lv", "Latvian", "Latviešu", "Sveiki, %s!"},
    {"lt", "Lithuanian", "Lietuvių", "Sveiki, %s!"},
    {"sk", "Slovak", "Slovenčina", "Ahoj, %s!"},
    {"sl", "Slovenian", "Slovenščina", "Živjo, %s!"},
    {"hr", "Croatian", "Hrvatski", "Bok, %s!"},
    {"sr", "Serbian", "Српски", "Здраво, %s!"},
    {"bg", "Bulgarian", "Български", "Здравей, %s!"},
    {"ka", "Georgian", "ქართული", "გამარჯობა, %s!"},
    {"hy", "Armenian", "Հայերեն", "Բարև, %s!"},
    {"am", "Amharic", "አማርኛ", "ሰላም, %s!"},
    {"mn", "Mongolian", "Монгол", "Сайн уу, %s!"},
    {"ne", "Nepali", "नेपाली", "नमस्कार, %s!"},
    {"kk", "Kazakh", "Қазақша", "Сәлем, %s!"},
    {"uz", "Uzbek", "Oʻzbekcha", "Salom, %s!"},
    {"yo", "Yoruba", "Yorùbá", "Báwo, %s!"},
    {"zu", "Zulu", "isiZulu", "Sawubona, %s!"},
};

const Greeting* find_greeting(const char* code) {
    if (!code) code = "en";
    for (int i = 0; i < MAX_GREETINGS; ++i) {
        if (strcmp(greetings[i].code, code) == 0) {
            return &greetings[i];
        }
    }
    // fallback to English
    for (int i = 0; i < MAX_GREETINGS; ++i) {
        if (strcmp(greetings[i].code, "en") == 0) {
            return &greetings[i];
        }
    }
    return &greetings[0];
}

// In-place str replace for the simple "%s"
void format_greeting(const char* tmpl, const char* name, char* out, size_t out_len) {
    const char* pos = strstr(tmpl, "%s");
    if (!pos) {
        snprintf(out, out_len, "%s", tmpl);
        return;
    }
    size_t prefix_len = pos - tmpl;
    snprintf(out, out_len, "%.*s%s%s", (int)prefix_len, tmpl, name, pos + 2);
}

// Very basic extractor assuming simple JSON struct
void extract_json_field(const char* json, const char* field, char* out, size_t out_len) {
    out[0] = '\0';
    char search[64];
    snprintf(search, sizeof(search), "\"%s\":", field);
    char *pos = strstr(json, search);
    if (!pos) return;
    
    pos += strlen(search);
    while (*pos == ' ' || *pos == '\n') pos++;
    
    if (*pos == '"') {
        pos++; // skip quote
        char *end = strchr(pos, '"');
        if (end) {
            size_t len = end - pos;
            if (len >= out_len) len = out_len - 1;
            strncpy(out, pos, len);
            out[len] = '\0';
        }
    }
}

int handle_connection(const holons_conn_t *conn, void *ctx) {
    (void)ctx;
    char buffer[4096];
    ssize_t n = holons_conn_read(conn, buffer, sizeof(buffer) - 1);
    if (n <= 0) return 0;
    
    buffer[n] = '\0'; // null terminate for string ops
    
    // Very simple HTTP parser
    char method[16], path[256], protocol[16];
    if (sscanf(buffer, "%15s %255s %15s", method, path, protocol) != 3) {
        return 0; // Not an HTTP request
    }
    
    if (strcmp(method, "POST") != 0 && strcmp(method, "GET") != 0) {
        const char* res = "HTTP/1.1 405 Method Not Allowed\r\n\r\n";
        holons_conn_write(conn, res, strlen(res));
        return 0;
    }

    // Find body (after \r\n\r\n)
    char *body = strstr(buffer, "\r\n\r\n");
    if (body) {
        body += 4;
    }

    if (strcmp(path, "/greeting.v1.GreetingService/ListLanguages") == 0) {
        char response[8192] = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"languages\":[";
        for (int i = 0; i < MAX_GREETINGS; i++) {
            char item[256];
            snprintf(item, sizeof(item), "{\"code\":\"%s\",\"name\":\"%s\",\"native\":\"%s\"}",
                greetings[i].code, greetings[i].name, greetings[i].native);
            strncat(response, item, sizeof(response) - strlen(response) - 1);
            if (i < MAX_GREETINGS - 1) {
                strncat(response, ",", sizeof(response) - strlen(response) - 1);
            }
        }
        strncat(response, "]}", sizeof(response) - strlen(response) - 1);
        holons_conn_write(conn, response, strlen(response));
    } else if (strcmp(path, "/greeting.v1.GreetingService/SayHello") == 0) {
        char name[128] = "World";
        char lang_code[16] = "en";
        
        if (body) {
            extract_json_field(body, "name", name, sizeof(name));
            if (strlen(name) == 0) strcpy(name, "World");
            extract_json_field(body, "lang_code", lang_code, sizeof(lang_code));
        }

        const Greeting *g = find_greeting(lang_code);
        char greeting_str[256];
        format_greeting(g->template_str, name, greeting_str, sizeof(greeting_str));

        char response[1024];
        snprintf(response, sizeof(response), 
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n"
            "{\"greeting\":\"%s\",\"language\":\"%s\",\"lang_code\":\"%s\"}",
            greeting_str, g->name, g->code);
            
        holons_conn_write(conn, response, strlen(response));
    } else {
        const char *not_found = "HTTP/1.1 404 Not Found\r\n\r\n";
        holons_conn_write(conn, not_found, strlen(not_found));
    }

    return 0;
}

int main(int argc, char** argv) {
    if (argc >= 2 && strcmp(argv[1], "version") == 0) {
        printf("gudule-daemon-greeting-c v0.4.5\n");
        return 0;
    }

    char uri[HOLONS_MAX_URI_LEN];
    holons_parse_flags(argc, argv, uri, sizeof(uri));

    char err[256];
    printf("gRPC (Connect JSON) server listening on %s\n", uri);
    
    // We only answer 1 connection if stdio, else loop forever.
    int res = holons_serve(uri, handle_connection, NULL, 0, 1, err, sizeof(err));
    if (res != 0) {
        fprintf(stderr, "serve error: %s\n", err);
        return 1;
    }
    
    return 0;
}
