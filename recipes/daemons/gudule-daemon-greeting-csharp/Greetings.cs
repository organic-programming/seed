namespace GreetingDaemon.Csharp;

public sealed record GreetingEntry(string Code, string Name, string Native, string Template);

public static class Greetings
{
    public static IReadOnlyList<GreetingEntry> All { get; } = new GreetingEntry[]
    {
        new("en", "English", "English", "Hello, %s!"),
        new("fr", "French", "Français", "Bonjour, %s !"),
        new("es", "Spanish", "Español", "Hola, %s!"),
        new("de", "German", "Deutsch", "Hallo, %s!"),
        new("it", "Italian", "Italiano", "Ciao, %s!"),
        new("pt", "Portuguese", "Português", "Olá, %s!"),
        new("nl", "Dutch", "Nederlands", "Hallo, %s!"),
        new("ru", "Russian", "Русский", "Привет, %s!"),
        new("ja", "Japanese", "日本語", "こんにちは、%sさん！"),
        new("zh", "Chinese", "中文", "你好，%s！"),
        new("ko", "Korean", "한국어", "안녕하세요, %s!"),
        new("ar", "Arabic", "العربية", "مرحبا، %s!"),
        new("hi", "Hindi", "हिन्दी", "नमस्ते, %s!"),
        new("tr", "Turkish", "Türkçe", "Merhaba, %s!"),
        new("pl", "Polish", "Polski", "Cześć, %s!"),
        new("sv", "Swedish", "Svenska", "Hej, %s!"),
        new("no", "Norwegian", "Norsk", "Hei, %s!"),
        new("da", "Danish", "Dansk", "Hej, %s!"),
        new("fi", "Finnish", "Suomi", "Hei, %s!"),
        new("cs", "Czech", "Čeština", "Ahoj, %s!"),
        new("ro", "Romanian", "Română", "Bună, %s!"),
        new("hu", "Hungarian", "Magyar", "Szia, %s!"),
        new("el", "Greek", "Ελληνικά", "Γεια σου, %s!"),
        new("th", "Thai", "ไทย", "สวัสดี, %s!"),
        new("vi", "Vietnamese", "Tiếng Việt", "Xin chào, %s!"),
        new("id", "Indonesian", "Bahasa Indonesia", "Halo, %s!"),
        new("ms", "Malay", "Bahasa Melayu", "Hai, %s!"),
        new("sw", "Swahili", "Kiswahili", "Habari, %s!"),
        new("he", "Hebrew", "עברית", "שלום, %s!"),
        new("uk", "Ukrainian", "Українська", "Привіт, %s!"),
        new("bn", "Bengali", "বাংলা", "নমস্কার, %s!"),
        new("ta", "Tamil", "தமிழ்", "வணக்கம், %s!"),
        new("fa", "Persian", "فارسی", "سلام، %s!"),
        new("ur", "Urdu", "اردو", "السلام علیکم، %s!"),
        new("fil", "Filipino", "Filipino", "Kumusta, %s!"),
        new("ca", "Catalan", "Català", "Hola, %s!"),
        new("eu", "Basque", "Euskara", "Kaixo, %s!"),
        new("gl", "Galician", "Galego", "Ola, %s!"),
        new("is", "Icelandic", "Íslenska", "Halló, %s!"),
        new("et", "Estonian", "Eesti", "Tere, %s!"),
        new("lv", "Latvian", "Latviešu", "Sveiki, %s!"),
        new("lt", "Lithuanian", "Lietuvių", "Sveiki, %s!"),
        new("sk", "Slovak", "Slovenčina", "Ahoj, %s!"),
        new("sl", "Slovenian", "Slovenščina", "Živjo, %s!"),
        new("hr", "Croatian", "Hrvatski", "Bok, %s!"),
        new("sr", "Serbian", "Српски", "Здраво, %s!"),
        new("bg", "Bulgarian", "Български", "Здравей, %s!"),
        new("ka", "Georgian", "ქართული", "გამარჯობა, %s!"),
        new("hy", "Armenian", "Հայերեն", "Բարև, %s!"),
        new("am", "Amharic", "አማርኛ", "ሰላም, %s!"),
        new("mn", "Mongolian", "Монгол", "Сайн уу, %s!"),
        new("ne", "Nepali", "नेपाली", "नमस्कार, %s!"),
        new("kk", "Kazakh", "Қазақша", "Сәлем, %s!"),
        new("uz", "Uzbek", "Oʻzbekcha", "Salom, %s!"),
        new("yo", "Yoruba", "Yorùbá", "Báwo, %s!"),
        new("zu", "Zulu", "isiZulu", "Sawubona, %s!"),
    };

    private static readonly IReadOnlyDictionary<string, GreetingEntry> Index =
        All.ToDictionary(entry => entry.Code, StringComparer.Ordinal);

    public static GreetingEntry Lookup(string code)
    {
        var trimmed = (code ?? string.Empty).Trim();
        return Index.TryGetValue(trimmed, out var entry) ? entry : Index["en"];
    }
}
