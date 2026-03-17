namespace GabrielGreeting.Csharp._Internal;

public sealed record GreetingEntry(
    string Code,
    string Name,
    string Native,
    string Template,
    string DefaultName);

public static class Greetings
{
    public static IReadOnlyList<GreetingEntry> All { get; } = new GreetingEntry[]
    {
        new("en", "English", "English", "Hello %s", "Mary"),
        new("fr", "French", "Français", "Bonjour %s", "Marie"),
        new("es", "Spanish", "Español", "Hola %s", "María"),
        new("de", "German", "Deutsch", "Hallo %s", "Maria"),
        new("it", "Italian", "Italiano", "Ciao %s", "Maria"),
        new("pt", "Portuguese", "Português", "Olá %s", "Maria"),
        new("nl", "Dutch", "Nederlands", "Hallo %s", "Maria"),
        new("ru", "Russian", "Русский", "Привет %s", "Мария"),
        new("ja", "Japanese", "日本語", "こんにちは、%sさん", "マリア"),
        new("zh", "Chinese", "中文", "你好，%s", "玛丽"),
        new("ko", "Korean", "한국어", "안녕하세요 %s", "마리아"),
        new("ar", "Arabic", "العربية", "مرحبا، %s", "ماريا"),
        new("hi", "Hindi", "हिन्दी", "नमस्ते %s", "मारिया"),
        new("tr", "Turkish", "Türkçe", "Merhaba %s", "Meryem"),
        new("pl", "Polish", "Polski", "Cześć %s", "Maria"),
        new("sv", "Swedish", "Svenska", "Hej %s", "Maria"),
        new("no", "Norwegian", "Norsk", "Hei %s", "Maria"),
        new("da", "Danish", "Dansk", "Hej %s", "Maria"),
        new("fi", "Finnish", "Suomi", "Hei %s", "Maria"),
        new("cs", "Czech", "Čeština", "Ahoj %s", "Marie"),
        new("ro", "Romanian", "Română", "Bună %s", "Maria"),
        new("hu", "Hungarian", "Magyar", "Szia %s", "Mária"),
        new("el", "Greek", "Ελληνικά", "Γεια σου %s", "Μαρία"),
        new("th", "Thai", "ไทย", "สวัสดี %s", "มาเรีย"),
        new("vi", "Vietnamese", "Tiếng Việt", "Xin chào %s", "Mary"),
        new("id", "Indonesian", "Bahasa Indonesia", "Halo %s", "Maria"),
        new("ms", "Malay", "Bahasa Melayu", "Hai %s", "Maria"),
        new("sw", "Swahili", "Kiswahili", "Habari %s", "Maria"),
        new("he", "Hebrew", "עברית", "שלום %s", "מרים"),
        new("uk", "Ukrainian", "Українська", "Привіт %s", "Марія"),
        new("bn", "Bengali", "বাংলা", "নমস্কার %s", "মারিয়া"),
        new("ta", "Tamil", "தமிழ்", "வணக்கம் %s", "மரியா"),
        new("fa", "Persian", "فارسی", "سلام، %s", "ماریا"),
        new("ur", "Urdu", "اردو", "السلام علیکم، %s", "ماریہ"),
        new("fil", "Filipino", "Filipino", "Kumusta %s", "Maria"),
        new("ca", "Catalan", "Català", "Hola %s", "Maria"),
        new("eu", "Basque", "Euskara", "Kaixo %s", "Maria"),
        new("gl", "Galician", "Galego", "Ola %s", "María"),
        new("is", "Icelandic", "Íslenska", "Halló %s", "María"),
        new("et", "Estonian", "Eesti", "Tere %s", "Maria"),
        new("lv", "Latvian", "Latviešu", "Sveiki %s", "Marija"),
        new("lt", "Lithuanian", "Lietuvių", "Sveiki %s", "Marija"),
        new("sk", "Slovak", "Slovenčina", "Ahoj %s", "Mária"),
        new("sl", "Slovenian", "Slovenščina", "Živjo %s", "Marija"),
        new("hr", "Croatian", "Hrvatski", "Bok %s", "Marija"),
        new("sr", "Serbian", "Српски", "Здраво %s", "Марија"),
        new("bg", "Bulgarian", "Български", "Здравей %s", "Мария"),
        new("ka", "Georgian", "ქართული", "გამარჯობა %s", "მარიამ"),
        new("hy", "Armenian", "Հայերեն", "Բարև %s", "Մարիա"),
        new("am", "Amharic", "አማርኛ", "ሰላም %s", "ማሪያ"),
        new("mn", "Mongolian", "Монгол", "Сайн уу %s", "Мария"),
        new("ne", "Nepali", "नेपाली", "नमस्कार %s", "मारिया"),
        new("kk", "Kazakh", "Қазақша", "Сәлем %s", "Мария"),
        new("uz", "Uzbek", "Oʻzbekcha", "Salom %s", "Mariya"),
        new("yo", "Yoruba", "Yorùbá", "Báwo %s", "Maria"),
        new("zu", "Zulu", "isiZulu", "Sawubona %s", "uMaria"),
    };

    private static readonly IReadOnlyDictionary<string, GreetingEntry> Index =
        All.ToDictionary(entry => entry.Code, StringComparer.Ordinal);

    public static GreetingEntry Lookup(string code)
    {
        var trimmed = (code ?? string.Empty).Trim();
        return Index.TryGetValue(trimmed, out var entry) ? entry : Index["en"];
    }
}
