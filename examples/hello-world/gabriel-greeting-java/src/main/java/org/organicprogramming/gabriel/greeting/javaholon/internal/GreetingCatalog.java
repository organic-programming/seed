package org.organicprogramming.gabriel.greeting.javaholon.internal;

import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;

public final class GreetingCatalog {
    public static final GreetingData[] GREETINGS = new GreetingData[] {
            new GreetingData("en", "English", "English", "Hello %s", "Mary"),
            new GreetingData("fr", "French", "Français", "Bonjour %s", "Marie"),
            new GreetingData("es", "Spanish", "Español", "Hola %s", "María"),
            new GreetingData("de", "German", "Deutsch", "Hallo %s", "Maria"),
            new GreetingData("it", "Italian", "Italiano", "Ciao %s", "Maria"),
            new GreetingData("pt", "Portuguese", "Português", "Olá %s", "Maria"),
            new GreetingData("nl", "Dutch", "Nederlands", "Hallo %s", "Maria"),
            new GreetingData("ru", "Russian", "Русский", "Привет %s", "Мария"),
            new GreetingData("ja", "Japanese", "日本語", "こんにちは、%sさん", "マリア"),
            new GreetingData("zh", "Chinese", "中文", "你好，%s", "玛丽"),
            new GreetingData("ko", "Korean", "한국어", "안녕하세요 %s", "마리아"),
            new GreetingData("ar", "Arabic", "العربية", "مرحبا، %s", "ماريا"),
            new GreetingData("hi", "Hindi", "हिन्दी", "नमस्ते %s", "मारिया"),
            new GreetingData("tr", "Turkish", "Türkçe", "Merhaba %s", "Meryem"),
            new GreetingData("pl", "Polish", "Polski", "Cześć %s", "Maria"),
            new GreetingData("sv", "Swedish", "Svenska", "Hej %s", "Maria"),
            new GreetingData("no", "Norwegian", "Norsk", "Hei %s", "Maria"),
            new GreetingData("da", "Danish", "Dansk", "Hej %s", "Maria"),
            new GreetingData("fi", "Finnish", "Suomi", "Hei %s", "Maria"),
            new GreetingData("cs", "Czech", "Čeština", "Ahoj %s", "Marie"),
            new GreetingData("ro", "Romanian", "Română", "Bună %s", "Maria"),
            new GreetingData("hu", "Hungarian", "Magyar", "Szia %s", "Mária"),
            new GreetingData("el", "Greek", "Ελληνικά", "Γεια σου %s", "Μαρία"),
            new GreetingData("th", "Thai", "ไทย", "สวัสดี %s", "มาเรีย"),
            new GreetingData("vi", "Vietnamese", "Tiếng Việt", "Xin chào %s", "Mary"),
            new GreetingData("id", "Indonesian", "Bahasa Indonesia", "Halo %s", "Maria"),
            new GreetingData("ms", "Malay", "Bahasa Melayu", "Hai %s", "Maria"),
            new GreetingData("sw", "Swahili", "Kiswahili", "Habari %s", "Maria"),
            new GreetingData("he", "Hebrew", "עברית", "שלום %s", "מרים"),
            new GreetingData("uk", "Ukrainian", "Українська", "Привіт %s", "Марія"),
            new GreetingData("bn", "Bengali", "বাংলা", "নমস্কার %s", "মারিয়া"),
            new GreetingData("ta", "Tamil", "தமிழ்", "வணக்கம் %s", "மரியா"),
            new GreetingData("fa", "Persian", "فارسی", "سلام، %s", "ماریا"),
            new GreetingData("ur", "Urdu", "اردو", "السلام علیکم، %s", "ماریہ"),
            new GreetingData("fil", "Filipino", "Filipino", "Kumusta %s", "Maria"),
            new GreetingData("ca", "Catalan", "Català", "Hola %s", "Maria"),
            new GreetingData("eu", "Basque", "Euskara", "Kaixo %s", "Maria"),
            new GreetingData("gl", "Galician", "Galego", "Ola %s", "María"),
            new GreetingData("is", "Icelandic", "Íslenska", "Halló %s", "María"),
            new GreetingData("et", "Estonian", "Eesti", "Tere %s", "Maria"),
            new GreetingData("lv", "Latvian", "Latviešu", "Sveiki %s", "Marija"),
            new GreetingData("lt", "Lithuanian", "Lietuvių", "Sveiki %s", "Marija"),
            new GreetingData("sk", "Slovak", "Slovenčina", "Ahoj %s", "Mária"),
            new GreetingData("sl", "Slovenian", "Slovenščina", "Živjo %s", "Marija"),
            new GreetingData("hr", "Croatian", "Hrvatski", "Bok %s", "Marija"),
            new GreetingData("sr", "Serbian", "Српски", "Здраво %s", "Марија"),
            new GreetingData("bg", "Bulgarian", "Български", "Здравей %s", "Мария"),
            new GreetingData("ka", "Georgian", "ქართული", "გამარჯობა %s", "მარიამ"),
            new GreetingData("hy", "Armenian", "Հայերեն", "Բարև %s", "Մարիա"),
            new GreetingData("am", "Amharic", "አማርኛ", "ሰላም %s", "ማሪያ"),
            new GreetingData("mn", "Mongolian", "Монгол", "Сайн уу %s", "Мария"),
            new GreetingData("ne", "Nepali", "नेपाली", "नमस्कार %s", "मारिया"),
            new GreetingData("kk", "Kazakh", "Қазақша", "Сәлем %s", "Мария"),
            new GreetingData("uz", "Uzbek", "Oʻzbekcha", "Salom %s", "Mariya"),
            new GreetingData("yo", "Yoruba", "Yorùbá", "Báwo %s", "Maria"),
            new GreetingData("zu", "Zulu", "isiZulu", "Sawubona %s", "uMaria"),
    };

    private static final Map<String, GreetingData> BY_CODE = Stream.of(GREETINGS)
            .collect(Collectors.toUnmodifiableMap(GreetingData::langCode, greeting -> greeting));

    private GreetingCatalog() {
    }

    public static GreetingData lookup(String code) {
        return BY_CODE.getOrDefault(code, BY_CODE.get("en"));
    }
}
