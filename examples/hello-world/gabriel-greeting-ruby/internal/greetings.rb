# frozen_string_literal: true

module GabrielGreetingRuby
  module Internal
    GreetingEntry = Struct.new(
      :lang_code,
      :lang_english,
      :lang_native,
      :template,
      :default_name,
      keyword_init: true
    )

    GREETINGS = [
      GreetingEntry.new(lang_code: "en", lang_english: "English", lang_native: "English", template: "Hello %s", default_name: "Mary"),
      GreetingEntry.new(lang_code: "fr", lang_english: "French", lang_native: "Français", template: "Bonjour %s", default_name: "Marie"),
      GreetingEntry.new(lang_code: "es", lang_english: "Spanish", lang_native: "Español", template: "Hola %s", default_name: "María"),
      GreetingEntry.new(lang_code: "de", lang_english: "German", lang_native: "Deutsch", template: "Hallo %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "it", lang_english: "Italian", lang_native: "Italiano", template: "Ciao %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "pt", lang_english: "Portuguese", lang_native: "Português", template: "Olá %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "nl", lang_english: "Dutch", lang_native: "Nederlands", template: "Hallo %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "ru", lang_english: "Russian", lang_native: "Русский", template: "Привет %s", default_name: "Мария"),
      GreetingEntry.new(lang_code: "ja", lang_english: "Japanese", lang_native: "日本語", template: "こんにちは、%sさん", default_name: "マリア"),
      GreetingEntry.new(lang_code: "zh", lang_english: "Chinese", lang_native: "中文", template: "你好，%s", default_name: "玛丽"),
      GreetingEntry.new(lang_code: "ko", lang_english: "Korean", lang_native: "한국어", template: "안녕하세요 %s", default_name: "마리아"),
      GreetingEntry.new(lang_code: "ar", lang_english: "Arabic", lang_native: "العربية", template: "مرحبا، %s", default_name: "ماريا"),
      GreetingEntry.new(lang_code: "hi", lang_english: "Hindi", lang_native: "हिन्दी", template: "नमस्ते %s", default_name: "मारिया"),
      GreetingEntry.new(lang_code: "tr", lang_english: "Turkish", lang_native: "Türkçe", template: "Merhaba %s", default_name: "Meryem"),
      GreetingEntry.new(lang_code: "pl", lang_english: "Polish", lang_native: "Polski", template: "Cześć %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "sv", lang_english: "Swedish", lang_native: "Svenska", template: "Hej %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "no", lang_english: "Norwegian", lang_native: "Norsk", template: "Hei %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "da", lang_english: "Danish", lang_native: "Dansk", template: "Hej %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "fi", lang_english: "Finnish", lang_native: "Suomi", template: "Hei %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "cs", lang_english: "Czech", lang_native: "Čeština", template: "Ahoj %s", default_name: "Marie"),
      GreetingEntry.new(lang_code: "ro", lang_english: "Romanian", lang_native: "Română", template: "Bună %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "hu", lang_english: "Hungarian", lang_native: "Magyar", template: "Szia %s", default_name: "Mária"),
      GreetingEntry.new(lang_code: "el", lang_english: "Greek", lang_native: "Ελληνικά", template: "Γεια σου %s", default_name: "Μαρία"),
      GreetingEntry.new(lang_code: "th", lang_english: "Thai", lang_native: "ไทย", template: "สวัสดี %s", default_name: "มาเรีย"),
      GreetingEntry.new(lang_code: "vi", lang_english: "Vietnamese", lang_native: "Tiếng Việt", template: "Xin chào %s", default_name: "Mary"),
      GreetingEntry.new(lang_code: "id", lang_english: "Indonesian", lang_native: "Bahasa Indonesia", template: "Halo %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "ms", lang_english: "Malay", lang_native: "Bahasa Melayu", template: "Hai %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "sw", lang_english: "Swahili", lang_native: "Kiswahili", template: "Habari %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "he", lang_english: "Hebrew", lang_native: "עברית", template: "שלום %s", default_name: "מרים"),
      GreetingEntry.new(lang_code: "uk", lang_english: "Ukrainian", lang_native: "Українська", template: "Привіт %s", default_name: "Марія"),
      GreetingEntry.new(lang_code: "bn", lang_english: "Bengali", lang_native: "বাংলা", template: "নমস্কার %s", default_name: "মারিয়া"),
      GreetingEntry.new(lang_code: "ta", lang_english: "Tamil", lang_native: "தமிழ்", template: "வணக்கம் %s", default_name: "மரியா"),
      GreetingEntry.new(lang_code: "fa", lang_english: "Persian", lang_native: "فارسی", template: "سلام، %s", default_name: "ماریا"),
      GreetingEntry.new(lang_code: "ur", lang_english: "Urdu", lang_native: "اردو", template: "السلام علیکم، %s", default_name: "ماریہ"),
      GreetingEntry.new(lang_code: "fil", lang_english: "Filipino", lang_native: "Filipino", template: "Kumusta %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "ca", lang_english: "Catalan", lang_native: "Català", template: "Hola %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "eu", lang_english: "Basque", lang_native: "Euskara", template: "Kaixo %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "gl", lang_english: "Galician", lang_native: "Galego", template: "Ola %s", default_name: "María"),
      GreetingEntry.new(lang_code: "is", lang_english: "Icelandic", lang_native: "Íslenska", template: "Halló %s", default_name: "María"),
      GreetingEntry.new(lang_code: "et", lang_english: "Estonian", lang_native: "Eesti", template: "Tere %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "lv", lang_english: "Latvian", lang_native: "Latviešu", template: "Sveiki %s", default_name: "Marija"),
      GreetingEntry.new(lang_code: "lt", lang_english: "Lithuanian", lang_native: "Lietuvių", template: "Sveiki %s", default_name: "Marija"),
      GreetingEntry.new(lang_code: "sk", lang_english: "Slovak", lang_native: "Slovenčina", template: "Ahoj %s", default_name: "Mária"),
      GreetingEntry.new(lang_code: "sl", lang_english: "Slovenian", lang_native: "Slovenščina", template: "Živjo %s", default_name: "Marija"),
      GreetingEntry.new(lang_code: "hr", lang_english: "Croatian", lang_native: "Hrvatski", template: "Bok %s", default_name: "Marija"),
      GreetingEntry.new(lang_code: "sr", lang_english: "Serbian", lang_native: "Српски", template: "Здраво %s", default_name: "Марија"),
      GreetingEntry.new(lang_code: "bg", lang_english: "Bulgarian", lang_native: "Български", template: "Здравей %s", default_name: "Мария"),
      GreetingEntry.new(lang_code: "ka", lang_english: "Georgian", lang_native: "ქართული", template: "გამარჯობა %s", default_name: "მარიამ"),
      GreetingEntry.new(lang_code: "hy", lang_english: "Armenian", lang_native: "Հայերեն", template: "Բարև %s", default_name: "Մարիա"),
      GreetingEntry.new(lang_code: "am", lang_english: "Amharic", lang_native: "አማርኛ", template: "ሰላም %s", default_name: "ማሪያ"),
      GreetingEntry.new(lang_code: "mn", lang_english: "Mongolian", lang_native: "Монгол", template: "Сайн уу %s", default_name: "Мария"),
      GreetingEntry.new(lang_code: "ne", lang_english: "Nepali", lang_native: "नेपाली", template: "नमस्कार %s", default_name: "मारिया"),
      GreetingEntry.new(lang_code: "kk", lang_english: "Kazakh", lang_native: "Қазақша", template: "Сәлем %s", default_name: "Мария"),
      GreetingEntry.new(lang_code: "uz", lang_english: "Uzbek", lang_native: "Oʻzbekcha", template: "Salom %s", default_name: "Mariya"),
      GreetingEntry.new(lang_code: "yo", lang_english: "Yoruba", lang_native: "Yorùbá", template: "Báwo %s", default_name: "Maria"),
      GreetingEntry.new(lang_code: "zu", lang_english: "Zulu", lang_native: "isiZulu", template: "Sawubona %s", default_name: "uMaria")
    ].freeze

    INDEX = GREETINGS.each_with_object({}) do |entry, index|
      index[entry.lang_code] = entry
    end.freeze

    class << self
      def lookup(code)
        INDEX.fetch(code.to_s, INDEX.fetch("en"))
      end
    end
  end
end
