class GreetingEntry {
  const GreetingEntry({
    required this.code,
    required this.name,
    required this.native,
    required this.template,
  });

  final String code;
  final String name;
  final String native;
  final String template;
}

const List<GreetingEntry> greetings = <GreetingEntry>[
  GreetingEntry(code: 'en', name: 'English', native: 'English', template: 'Hello, %s!'),
  GreetingEntry(code: 'fr', name: 'French', native: 'Français', template: 'Bonjour, %s !'),
  GreetingEntry(code: 'es', name: 'Spanish', native: 'Español', template: '¡Hola, %s!'),
  GreetingEntry(code: 'de', name: 'German', native: 'Deutsch', template: 'Hallo, %s!'),
  GreetingEntry(code: 'it', name: 'Italian', native: 'Italiano', template: 'Ciao, %s!'),
  GreetingEntry(code: 'pt', name: 'Portuguese', native: 'Português', template: 'Olá, %s!'),
  GreetingEntry(code: 'nl', name: 'Dutch', native: 'Nederlands', template: 'Hallo, %s!'),
  GreetingEntry(code: 'ru', name: 'Russian', native: 'Русский', template: 'Привет, %s!'),
  GreetingEntry(code: 'ja', name: 'Japanese', native: '日本語', template: 'こんにちは、%sさん！'),
  GreetingEntry(code: 'zh', name: 'Chinese', native: '中文', template: '你好，%s！'),
  GreetingEntry(code: 'ko', name: 'Korean', native: '한국어', template: '안녕하세요, %s!'),
  GreetingEntry(code: 'ar', name: 'Arabic', native: 'العربية', template: 'مرحبا، %s!'),
  GreetingEntry(code: 'hi', name: 'Hindi', native: 'हिन्दी', template: 'नमस्ते, %s!'),
  GreetingEntry(code: 'tr', name: 'Turkish', native: 'Türkçe', template: 'Merhaba, %s!'),
  GreetingEntry(code: 'pl', name: 'Polish', native: 'Polski', template: 'Cześć, %s!'),
  GreetingEntry(code: 'sv', name: 'Swedish', native: 'Svenska', template: 'Hej, %s!'),
  GreetingEntry(code: 'no', name: 'Norwegian', native: 'Norsk', template: 'Hei, %s!'),
  GreetingEntry(code: 'da', name: 'Danish', native: 'Dansk', template: 'Hej, %s!'),
  GreetingEntry(code: 'fi', name: 'Finnish', native: 'Suomi', template: 'Hei, %s!'),
  GreetingEntry(code: 'cs', name: 'Czech', native: 'Čeština', template: 'Ahoj, %s!'),
  GreetingEntry(code: 'ro', name: 'Romanian', native: 'Română', template: 'Bună, %s!'),
  GreetingEntry(code: 'hu', name: 'Hungarian', native: 'Magyar', template: 'Szia, %s!'),
  GreetingEntry(code: 'el', name: 'Greek', native: 'Ελληνικά', template: 'Γεια σου, %s!'),
  GreetingEntry(code: 'th', name: 'Thai', native: 'ไทย', template: 'สวัสดี, %s!'),
  GreetingEntry(code: 'vi', name: 'Vietnamese', native: 'Tiếng Việt', template: 'Xin chào, %s!'),
  GreetingEntry(code: 'id', name: 'Indonesian', native: 'Bahasa Indonesia', template: 'Halo, %s!'),
  GreetingEntry(code: 'ms', name: 'Malay', native: 'Bahasa Melayu', template: 'Hai, %s!'),
  GreetingEntry(code: 'sw', name: 'Swahili', native: 'Kiswahili', template: 'Habari, %s!'),
  GreetingEntry(code: 'he', name: 'Hebrew', native: 'עברית', template: 'שלום, %s!'),
  GreetingEntry(code: 'uk', name: 'Ukrainian', native: 'Українська', template: 'Привіт, %s!'),
  GreetingEntry(code: 'bn', name: 'Bengali', native: 'বাংলা', template: 'নমস্কার, %s!'),
  GreetingEntry(code: 'ta', name: 'Tamil', native: 'தமிழ்', template: 'வணக்கம், %s!'),
  GreetingEntry(code: 'fa', name: 'Persian', native: 'فارسی', template: 'سلام، %s!'),
  GreetingEntry(code: 'ur', name: 'Urdu', native: 'اردو', template: 'السلام علیکم، %s!'),
  GreetingEntry(code: 'fil', name: 'Filipino', native: 'Filipino', template: 'Kumusta, %s!'),
  GreetingEntry(code: 'ca', name: 'Catalan', native: 'Català', template: 'Hola, %s!'),
  GreetingEntry(code: 'eu', name: 'Basque', native: 'Euskara', template: 'Kaixo, %s!'),
  GreetingEntry(code: 'gl', name: 'Galician', native: 'Galego', template: 'Ola, %s!'),
  GreetingEntry(code: 'is', name: 'Icelandic', native: 'Íslenska', template: 'Halló, %s!'),
  GreetingEntry(code: 'et', name: 'Estonian', native: 'Eesti', template: 'Tere, %s!'),
  GreetingEntry(code: 'lv', name: 'Latvian', native: 'Latviešu', template: 'Sveiki, %s!'),
  GreetingEntry(code: 'lt', name: 'Lithuanian', native: 'Lietuvių', template: 'Sveiki, %s!'),
  GreetingEntry(code: 'sk', name: 'Slovak', native: 'Slovenčina', template: 'Ahoj, %s!'),
  GreetingEntry(code: 'sl', name: 'Slovenian', native: 'Slovenščina', template: 'Živjo, %s!'),
  GreetingEntry(code: 'hr', name: 'Croatian', native: 'Hrvatski', template: 'Bok, %s!'),
  GreetingEntry(code: 'sr', name: 'Serbian', native: 'Српски', template: 'Здраво, %s!'),
  GreetingEntry(code: 'bg', name: 'Bulgarian', native: 'Български', template: 'Здравей, %s!'),
  GreetingEntry(code: 'ka', name: 'Georgian', native: 'ქართული', template: 'გამარჯობა, %s!'),
  GreetingEntry(code: 'hy', name: 'Armenian', native: 'Հայերեն', template: 'Բարև, %s!'),
  GreetingEntry(code: 'am', name: 'Amharic', native: 'አማርኛ', template: 'ሰላም, %s!'),
  GreetingEntry(code: 'mn', name: 'Mongolian', native: 'Монгол', template: 'Сайн уу, %s!'),
  GreetingEntry(code: 'ne', name: 'Nepali', native: 'नेपाली', template: 'नमस्कार, %s!'),
  GreetingEntry(code: 'kk', name: 'Kazakh', native: 'Қазақша', template: 'Сәлем, %s!'),
  GreetingEntry(code: 'uz', name: 'Uzbek', native: 'Oʻzbekcha', template: 'Salom, %s!'),
  GreetingEntry(code: 'yo', name: 'Yoruba', native: 'Yorùbá', template: 'Báwo, %s!'),
  GreetingEntry(code: 'zu', name: 'Zulu', native: 'isiZulu', template: 'Sawubona, %s!'),
];

final Map<String, GreetingEntry> _greetingIndex = <String, GreetingEntry>{
  for (final entry in greetings) entry.code: entry,
};

GreetingEntry lookupGreeting(String code) => _greetingIndex[code.trim()] ?? _greetingIndex['en']!;
