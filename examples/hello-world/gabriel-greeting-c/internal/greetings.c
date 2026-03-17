#include "internal/greetings.h"

#include <string.h>

static const gabriel_greeting_c_greeting gabriel_greeting_c_greetings[] = {
    {"en", "English", "English", "Hello %s", "Mary"},
    {"fr", "French", "Français", "Bonjour %s", "Marie"},
    {"es", "Spanish", "Español", "Hola %s", "María"},
    {"de", "German", "Deutsch", "Hallo %s", "Maria"},
    {"it", "Italian", "Italiano", "Ciao %s", "Maria"},
    {"pt", "Portuguese", "Português", "Olá %s", "Maria"},
    {"nl", "Dutch", "Nederlands", "Hallo %s", "Maria"},
    {"ru", "Russian", "Русский", "Привет %s", "Мария"},
    {"ja", "Japanese", "日本語", "こんにちは、%sさん", "マリア"},
    {"zh", "Chinese", "中文", "你好，%s", "玛丽"},
    {"ko", "Korean", "한국어", "안녕하세요 %s", "마리아"},
    {"ar", "Arabic", "العربية", "مرحبا، %s", "ماريا"},
    {"hi", "Hindi", "हिन्दी", "नमस्ते %s", "मारिया"},
    {"tr", "Turkish", "Türkçe", "Merhaba %s", "Meryem"},
    {"pl", "Polish", "Polski", "Cześć %s", "Maria"},
    {"sv", "Swedish", "Svenska", "Hej %s", "Maria"},
    {"no", "Norwegian", "Norsk", "Hei %s", "Maria"},
    {"da", "Danish", "Dansk", "Hej %s", "Maria"},
    {"fi", "Finnish", "Suomi", "Hei %s", "Maria"},
    {"cs", "Czech", "Čeština", "Ahoj %s", "Marie"},
    {"ro", "Romanian", "Română", "Bună %s", "Maria"},
    {"hu", "Hungarian", "Magyar", "Szia %s", "Mária"},
    {"el", "Greek", "Ελληνικά", "Γεια σου %s", "Μαρία"},
    {"th", "Thai", "ไทย", "สวัสดี %s", "มาเรีย"},
    {"vi", "Vietnamese", "Tiếng Việt", "Xin chào %s", "Mary"},
    {"id", "Indonesian", "Bahasa Indonesia", "Halo %s", "Maria"},
    {"ms", "Malay", "Bahasa Melayu", "Hai %s", "Maria"},
    {"sw", "Swahili", "Kiswahili", "Habari %s", "Maria"},
    {"he", "Hebrew", "עברית", "שלום %s", "מרים"},
    {"uk", "Ukrainian", "Українська", "Привіт %s", "Марія"},
    {"bn", "Bengali", "বাংলা", "নমস্কার %s", "মারিয়া"},
    {"ta", "Tamil", "தமிழ்", "வணக்கம் %s", "மரியா"},
    {"fa", "Persian", "فارسی", "سلام، %s", "ماریا"},
    {"ur", "Urdu", "اردو", "السلام علیکم، %s", "ماریہ"},
    {"fil", "Filipino", "Filipino", "Kumusta %s", "Maria"},
    {"ca", "Catalan", "Català", "Hola %s", "Maria"},
    {"eu", "Basque", "Euskara", "Kaixo %s", "Maria"},
    {"gl", "Galician", "Galego", "Ola %s", "María"},
    {"is", "Icelandic", "Íslenska", "Halló %s", "María"},
    {"et", "Estonian", "Eesti", "Tere %s", "Maria"},
    {"lv", "Latvian", "Latviešu", "Sveiki %s", "Marija"},
    {"lt", "Lithuanian", "Lietuvių", "Sveiki %s", "Marija"},
    {"sk", "Slovak", "Slovenčina", "Ahoj %s", "Mária"},
    {"sl", "Slovenian", "Slovenščina", "Živjo %s", "Marija"},
    {"hr", "Croatian", "Hrvatski", "Bok %s", "Marija"},
    {"sr", "Serbian", "Српски", "Здраво %s", "Марија"},
    {"bg", "Bulgarian", "Български", "Здравей %s", "Мария"},
    {"ka", "Georgian", "ქართული", "გამარჯობა %s", "მარიამ"},
    {"hy", "Armenian", "Հայերեն", "Բարև %s", "Մարիա"},
    {"am", "Amharic", "አማርኛ", "ሰላም %s", "ማሪያ"},
    {"mn", "Mongolian", "Монгол", "Сайн уу %s", "Мария"},
    {"ne", "Nepali", "नेपाली", "नमस्कार %s", "मारिया"},
    {"kk", "Kazakh", "Қазақша", "Сәлем %s", "Мария"},
    {"uz", "Uzbek", "Oʻzbekcha", "Salom %s", "Mariya"},
    {"yo", "Yoruba", "Yorùbá", "Báwo %s", "Maria"},
    {"zu", "Zulu", "isiZulu", "Sawubona %s", "uMaria"},
};

size_t gabriel_greeting_c_greetings_count(void) {
  return sizeof(gabriel_greeting_c_greetings) /
         sizeof(gabriel_greeting_c_greetings[0]);
}

const gabriel_greeting_c_greeting *gabriel_greeting_c_greeting_at(size_t index) {
  return &gabriel_greeting_c_greetings[index];
}

const gabriel_greeting_c_greeting *
gabriel_greeting_c_lookup(upb_StringView lang_code) {
  size_t i;
  for (i = 0; i < gabriel_greeting_c_greetings_count(); ++i) {
    const gabriel_greeting_c_greeting *entry = &gabriel_greeting_c_greetings[i];
    if (strlen(entry->lang_code) == lang_code.size &&
        memcmp(entry->lang_code, lang_code.data, lang_code.size) == 0) {
      return entry;
    }
  }
  return &gabriel_greeting_c_greetings[0];
}
