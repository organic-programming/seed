struct Greeting {
    let langCode: String
    let langEnglish: String
    let langNative: String
    let template: String
    let defaultName: String
}

enum Greetings {
    static let all: [Greeting] = [
        .init(langCode: "en", langEnglish: "English", langNative: "English", template: "Hello %@", defaultName: "Mary"),
        .init(langCode: "fr", langEnglish: "French", langNative: "Français", template: "Bonjour %@", defaultName: "Marie"),
        .init(langCode: "es", langEnglish: "Spanish", langNative: "Español", template: "Hola %@", defaultName: "María"),
        .init(langCode: "de", langEnglish: "German", langNative: "Deutsch", template: "Hallo %@", defaultName: "Maria"),
        .init(langCode: "it", langEnglish: "Italian", langNative: "Italiano", template: "Ciao %@", defaultName: "Maria"),
        .init(langCode: "pt", langEnglish: "Portuguese", langNative: "Português", template: "Olá %@", defaultName: "Maria"),
        .init(langCode: "nl", langEnglish: "Dutch", langNative: "Nederlands", template: "Hallo %@", defaultName: "Maria"),
        .init(langCode: "ru", langEnglish: "Russian", langNative: "Русский", template: "Привет %@", defaultName: "Мария"),
        .init(langCode: "ja", langEnglish: "Japanese", langNative: "日本語", template: "こんにちは、%@さん", defaultName: "マリア"),
        .init(langCode: "zh", langEnglish: "Chinese", langNative: "中文", template: "你好，%@", defaultName: "玛丽"),
        .init(langCode: "ko", langEnglish: "Korean", langNative: "한국어", template: "안녕하세요 %@", defaultName: "마리아"),
        .init(langCode: "ar", langEnglish: "Arabic", langNative: "العربية", template: "مرحبا، %@", defaultName: "ماريا"),
        .init(langCode: "hi", langEnglish: "Hindi", langNative: "हिन्दी", template: "नमस्ते %@", defaultName: "मारिया"),
        .init(langCode: "tr", langEnglish: "Turkish", langNative: "Türkçe", template: "Merhaba %@", defaultName: "Meryem"),
        .init(langCode: "pl", langEnglish: "Polish", langNative: "Polski", template: "Cześć %@", defaultName: "Maria"),
        .init(langCode: "sv", langEnglish: "Swedish", langNative: "Svenska", template: "Hej %@", defaultName: "Maria"),
        .init(langCode: "no", langEnglish: "Norwegian", langNative: "Norsk", template: "Hei %@", defaultName: "Maria"),
        .init(langCode: "da", langEnglish: "Danish", langNative: "Dansk", template: "Hej %@", defaultName: "Maria"),
        .init(langCode: "fi", langEnglish: "Finnish", langNative: "Suomi", template: "Hei %@", defaultName: "Maria"),
        .init(langCode: "cs", langEnglish: "Czech", langNative: "Čeština", template: "Ahoj %@", defaultName: "Marie"),
        .init(langCode: "ro", langEnglish: "Romanian", langNative: "Română", template: "Bună %@", defaultName: "Maria"),
        .init(langCode: "hu", langEnglish: "Hungarian", langNative: "Magyar", template: "Szia %@", defaultName: "Mária"),
        .init(langCode: "el", langEnglish: "Greek", langNative: "Ελληνικά", template: "Γεια σου %@", defaultName: "Μαρία"),
        .init(langCode: "th", langEnglish: "Thai", langNative: "ไทย", template: "สวัสดี %@", defaultName: "มาเรีย"),
        .init(langCode: "vi", langEnglish: "Vietnamese", langNative: "Tiếng Việt", template: "Xin chào %@", defaultName: "Mary"),
        .init(langCode: "id", langEnglish: "Indonesian", langNative: "Bahasa Indonesia", template: "Halo %@", defaultName: "Maria"),
        .init(langCode: "ms", langEnglish: "Malay", langNative: "Bahasa Melayu", template: "Hai %@", defaultName: "Maria"),
        .init(langCode: "sw", langEnglish: "Swahili", langNative: "Kiswahili", template: "Habari %@", defaultName: "Maria"),
        .init(langCode: "he", langEnglish: "Hebrew", langNative: "עברית", template: "שלום %@", defaultName: "מרים"),
        .init(langCode: "uk", langEnglish: "Ukrainian", langNative: "Українська", template: "Привіт %@", defaultName: "Марія"),
        .init(langCode: "bn", langEnglish: "Bengali", langNative: "বাংলা", template: "নমস্কার %@", defaultName: "মারিয়া"),
        .init(langCode: "ta", langEnglish: "Tamil", langNative: "தமிழ்", template: "வணக்கம் %@", defaultName: "மரியா"),
        .init(langCode: "fa", langEnglish: "Persian", langNative: "فارسی", template: "سلام، %@", defaultName: "ماریا"),
        .init(langCode: "ur", langEnglish: "Urdu", langNative: "اردو", template: "السلام علیکم، %@", defaultName: "ماریہ"),
        .init(langCode: "fil", langEnglish: "Filipino", langNative: "Filipino", template: "Kumusta %@", defaultName: "Maria"),
        .init(langCode: "ca", langEnglish: "Catalan", langNative: "Català", template: "Hola %@", defaultName: "Maria"),
        .init(langCode: "eu", langEnglish: "Basque", langNative: "Euskara", template: "Kaixo %@", defaultName: "Maria"),
        .init(langCode: "gl", langEnglish: "Galician", langNative: "Galego", template: "Ola %@", defaultName: "María"),
        .init(langCode: "is", langEnglish: "Icelandic", langNative: "Íslenska", template: "Halló %@", defaultName: "María"),
        .init(langCode: "et", langEnglish: "Estonian", langNative: "Eesti", template: "Tere %@", defaultName: "Maria"),
        .init(langCode: "lv", langEnglish: "Latvian", langNative: "Latviešu", template: "Sveiki %@", defaultName: "Marija"),
        .init(langCode: "lt", langEnglish: "Lithuanian", langNative: "Lietuvių", template: "Sveiki %@", defaultName: "Marija"),
        .init(langCode: "sk", langEnglish: "Slovak", langNative: "Slovenčina", template: "Ahoj %@", defaultName: "Mária"),
        .init(langCode: "sl", langEnglish: "Slovenian", langNative: "Slovenščina", template: "Živjo %@", defaultName: "Marija"),
        .init(langCode: "hr", langEnglish: "Croatian", langNative: "Hrvatski", template: "Bok %@", defaultName: "Marija"),
        .init(langCode: "sr", langEnglish: "Serbian", langNative: "Српски", template: "Здраво %@", defaultName: "Марија"),
        .init(langCode: "bg", langEnglish: "Bulgarian", langNative: "Български", template: "Здравей %@", defaultName: "Мария"),
        .init(langCode: "ka", langEnglish: "Georgian", langNative: "ქართული", template: "გამარჯობა %@", defaultName: "მარიამ"),
        .init(langCode: "hy", langEnglish: "Armenian", langNative: "Հայերեն", template: "Բարև %@", defaultName: "Մարիա"),
        .init(langCode: "am", langEnglish: "Amharic", langNative: "አማርኛ", template: "ሰላም %@", defaultName: "ማሪያ"),
        .init(langCode: "mn", langEnglish: "Mongolian", langNative: "Монгол", template: "Сайн уу %@", defaultName: "Мария"),
        .init(langCode: "ne", langEnglish: "Nepali", langNative: "नेपाली", template: "नमस्कार %@", defaultName: "मारिया"),
        .init(langCode: "kk", langEnglish: "Kazakh", langNative: "Қазақша", template: "Сәлем %@", defaultName: "Мария"),
        .init(langCode: "uz", langEnglish: "Uzbek", langNative: "Oʻzbekcha", template: "Salom %@", defaultName: "Mariya"),
        .init(langCode: "yo", langEnglish: "Yoruba", langNative: "Yorùbá", template: "Báwo %@", defaultName: "Maria"),
        .init(langCode: "zu", langEnglish: "Zulu", langNative: "isiZulu", template: "Sawubona %@", defaultName: "uMaria"),
    ]

    private static let byCode = Dictionary(uniqueKeysWithValues: all.map { ($0.langCode, $0) })

    static func lookup(_ code: String) -> Greeting {
        byCode[code] ?? byCode["en"]!
    }
}
