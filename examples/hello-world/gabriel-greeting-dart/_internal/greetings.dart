class Greeting {
  const Greeting(
    this.langCode,
    this.langEnglish,
    this.langNative,
    this.template,
    this.defaultName,
  );

  final String langCode;
  final String langEnglish;
  final String langNative;
  final String template;
  final String defaultName;
}

const List<Greeting> greetings = <Greeting>[
  Greeting('en', 'English', 'English', 'Hello %s', 'Mary'),
  Greeting('fr', 'French', 'Français', 'Bonjour %s', 'Marie'),
  Greeting('es', 'Spanish', 'Español', 'Hola %s', 'María'),
  Greeting('de', 'German', 'Deutsch', 'Hallo %s', 'Maria'),
  Greeting('it', 'Italian', 'Italiano', 'Ciao %s', 'Maria'),
  Greeting('pt', 'Portuguese', 'Português', 'Olá %s', 'Maria'),
  Greeting('nl', 'Dutch', 'Nederlands', 'Hallo %s', 'Maria'),
  Greeting('ru', 'Russian', 'Русский', 'Привет %s', 'Мария'),
  Greeting('ja', 'Japanese', '日本語', 'こんにちは、%sさん', 'マリア'),
  Greeting('zh', 'Chinese', '中文', '你好，%s', '玛丽'),
  Greeting('ko', 'Korean', '한국어', '안녕하세요 %s', '마리아'),
  Greeting('ar', 'Arabic', 'العربية', 'مرحبا، %s', 'ماريا'),
  Greeting('hi', 'Hindi', 'हिन्दी', 'नमस्ते %s', 'मारिया'),
  Greeting('tr', 'Turkish', 'Türkçe', 'Merhaba %s', 'Meryem'),
  Greeting('pl', 'Polish', 'Polski', 'Cześć %s', 'Maria'),
  Greeting('sv', 'Swedish', 'Svenska', 'Hej %s', 'Maria'),
  Greeting('no', 'Norwegian', 'Norsk', 'Hei %s', 'Maria'),
  Greeting('da', 'Danish', 'Dansk', 'Hej %s', 'Maria'),
  Greeting('fi', 'Finnish', 'Suomi', 'Hei %s', 'Maria'),
  Greeting('cs', 'Czech', 'Čeština', 'Ahoj %s', 'Marie'),
  Greeting('ro', 'Romanian', 'Română', 'Bună %s', 'Maria'),
  Greeting('hu', 'Hungarian', 'Magyar', 'Szia %s', 'Mária'),
  Greeting('el', 'Greek', 'Ελληνικά', 'Γεια σου %s', 'Μαρία'),
  Greeting('th', 'Thai', 'ไทย', 'สวัสดี %s', 'มาเรีย'),
  Greeting('vi', 'Vietnamese', 'Tiếng Việt', 'Xin chào %s', 'Mary'),
  Greeting('id', 'Indonesian', 'Bahasa Indonesia', 'Halo %s', 'Maria'),
  Greeting('ms', 'Malay', 'Bahasa Melayu', 'Hai %s', 'Maria'),
  Greeting('sw', 'Swahili', 'Kiswahili', 'Habari %s', 'Maria'),
  Greeting('he', 'Hebrew', 'עברית', 'שלום %s', 'מרים'),
  Greeting('uk', 'Ukrainian', 'Українська', 'Привіт %s', 'Марія'),
  Greeting('bn', 'Bengali', 'বাংলা', 'নমস্কার %s', 'মারিয়া'),
  Greeting('ta', 'Tamil', 'தமிழ்', 'வணக்கம் %s', 'மரியா'),
  Greeting('fa', 'Persian', 'فارسی', 'سلام، %s', 'ماریا'),
  Greeting('ur', 'Urdu', 'اردو', 'السلام علیکم، %s', 'ماریہ'),
  Greeting('fil', 'Filipino', 'Filipino', 'Kumusta %s', 'Maria'),
  Greeting('ca', 'Catalan', 'Català', 'Hola %s', 'Maria'),
  Greeting('eu', 'Basque', 'Euskara', 'Kaixo %s', 'Maria'),
  Greeting('gl', 'Galician', 'Galego', 'Ola %s', 'María'),
  Greeting('is', 'Icelandic', 'Íslenska', 'Halló %s', 'María'),
  Greeting('et', 'Estonian', 'Eesti', 'Tere %s', 'Maria'),
  Greeting('lv', 'Latvian', 'Latviešu', 'Sveiki %s', 'Marija'),
  Greeting('lt', 'Lithuanian', 'Lietuvių', 'Sveiki %s', 'Marija'),
  Greeting('sk', 'Slovak', 'Slovenčina', 'Ahoj %s', 'Mária'),
  Greeting('sl', 'Slovenian', 'Slovenščina', 'Živjo %s', 'Marija'),
  Greeting('hr', 'Croatian', 'Hrvatski', 'Bok %s', 'Marija'),
  Greeting('sr', 'Serbian', 'Српски', 'Здраво %s', 'Марија'),
  Greeting('bg', 'Bulgarian', 'Български', 'Здравей %s', 'Мария'),
  Greeting('ka', 'Georgian', 'ქართული', 'გამარჯობა %s', 'მარიამ'),
  Greeting('hy', 'Armenian', 'Հայերեն', 'Բարև %s', 'Մարիա'),
  Greeting('am', 'Amharic', 'አማርኛ', 'ሰላም %s', 'ማሪያ'),
  Greeting('mn', 'Mongolian', 'Монгол', 'Сайн уу %s', 'Мария'),
  Greeting('ne', 'Nepali', 'नेपाली', 'नमस्कार %s', 'मारिया'),
  Greeting('kk', 'Kazakh', 'Қазақша', 'Сәлем %s', 'Мария'),
  Greeting('uz', 'Uzbek', 'Oʻzbekcha', 'Salom %s', 'Mariya'),
  Greeting('yo', 'Yoruba', 'Yorùbá', 'Báwo %s', 'Maria'),
  Greeting('zu', 'Zulu', 'isiZulu', 'Sawubona %s', 'uMaria'),
];

final Map<String, Greeting> _byCode = <String, Greeting>{
  for (final greeting in greetings) greeting.langCode: greeting,
};

Greeting lookup(String code) => _byCode[code] ?? _byCode['en']!;
