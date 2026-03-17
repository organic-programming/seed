'use strict';

class Greeting {
  constructor(langCode, langEnglish, langNative, template, defaultName) {
    this.langCode = langCode;
    this.langEnglish = langEnglish;
    this.langNative = langNative;
    this.template = template;
    this.defaultName = defaultName;
  }
}

const GREETINGS = Object.freeze([
  new Greeting('en', 'English', 'English', 'Hello %s', 'Mary'),
  new Greeting('fr', 'French', 'Français', 'Bonjour %s', 'Marie'),
  new Greeting('es', 'Spanish', 'Español', 'Hola %s', 'María'),
  new Greeting('de', 'German', 'Deutsch', 'Hallo %s', 'Maria'),
  new Greeting('it', 'Italian', 'Italiano', 'Ciao %s', 'Maria'),
  new Greeting('pt', 'Portuguese', 'Português', 'Olá %s', 'Maria'),
  new Greeting('nl', 'Dutch', 'Nederlands', 'Hallo %s', 'Maria'),
  new Greeting('ru', 'Russian', 'Русский', 'Привет %s', 'Мария'),
  new Greeting('ja', 'Japanese', '日本語', 'こんにちは、%sさん', 'マリア'),
  new Greeting('zh', 'Chinese', '中文', '你好，%s', '玛丽'),
  new Greeting('ko', 'Korean', '한국어', '안녕하세요 %s', '마리아'),
  new Greeting('ar', 'Arabic', 'العربية', 'مرحبا، %s', 'ماريا'),
  new Greeting('hi', 'Hindi', 'हिन्दी', 'नमस्ते %s', 'मारिया'),
  new Greeting('tr', 'Turkish', 'Türkçe', 'Merhaba %s', 'Meryem'),
  new Greeting('pl', 'Polish', 'Polski', 'Cześć %s', 'Maria'),
  new Greeting('sv', 'Swedish', 'Svenska', 'Hej %s', 'Maria'),
  new Greeting('no', 'Norwegian', 'Norsk', 'Hei %s', 'Maria'),
  new Greeting('da', 'Danish', 'Dansk', 'Hej %s', 'Maria'),
  new Greeting('fi', 'Finnish', 'Suomi', 'Hei %s', 'Maria'),
  new Greeting('cs', 'Czech', 'Čeština', 'Ahoj %s', 'Marie'),
  new Greeting('ro', 'Romanian', 'Română', 'Bună %s', 'Maria'),
  new Greeting('hu', 'Hungarian', 'Magyar', 'Szia %s', 'Mária'),
  new Greeting('el', 'Greek', 'Ελληνικά', 'Γεια σου %s', 'Μαρία'),
  new Greeting('th', 'Thai', 'ไทย', 'สวัสดี %s', 'มาเรีย'),
  new Greeting('vi', 'Vietnamese', 'Tiếng Việt', 'Xin chào %s', 'Mary'),
  new Greeting('id', 'Indonesian', 'Bahasa Indonesia', 'Halo %s', 'Maria'),
  new Greeting('ms', 'Malay', 'Bahasa Melayu', 'Hai %s', 'Maria'),
  new Greeting('sw', 'Swahili', 'Kiswahili', 'Habari %s', 'Maria'),
  new Greeting('he', 'Hebrew', 'עברית', 'שלום %s', 'מרים'),
  new Greeting('uk', 'Ukrainian', 'Українська', 'Привіт %s', 'Марія'),
  new Greeting('bn', 'Bengali', 'বাংলা', 'নমস্কার %s', 'মারিয়া'),
  new Greeting('ta', 'Tamil', 'தமிழ்', 'வணக்கம் %s', 'மரியா'),
  new Greeting('fa', 'Persian', 'فارسی', 'سلام، %s', 'ماریا'),
  new Greeting('ur', 'Urdu', 'اردو', 'السلام علیکم، %s', 'ماریہ'),
  new Greeting('fil', 'Filipino', 'Filipino', 'Kumusta %s', 'Maria'),
  new Greeting('ca', 'Catalan', 'Català', 'Hola %s', 'Maria'),
  new Greeting('eu', 'Basque', 'Euskara', 'Kaixo %s', 'Maria'),
  new Greeting('gl', 'Galician', 'Galego', 'Ola %s', 'María'),
  new Greeting('is', 'Icelandic', 'Íslenska', 'Halló %s', 'María'),
  new Greeting('et', 'Estonian', 'Eesti', 'Tere %s', 'Maria'),
  new Greeting('lv', 'Latvian', 'Latviešu', 'Sveiki %s', 'Marija'),
  new Greeting('lt', 'Lithuanian', 'Lietuvių', 'Sveiki %s', 'Marija'),
  new Greeting('sk', 'Slovak', 'Slovenčina', 'Ahoj %s', 'Mária'),
  new Greeting('sl', 'Slovenian', 'Slovenščina', 'Živjo %s', 'Marija'),
  new Greeting('hr', 'Croatian', 'Hrvatski', 'Bok %s', 'Marija'),
  new Greeting('sr', 'Serbian', 'Српски', 'Здраво %s', 'Марија'),
  new Greeting('bg', 'Bulgarian', 'Български', 'Здравей %s', 'Мария'),
  new Greeting('ka', 'Georgian', 'ქართული', 'გამარჯობა %s', 'მარიამ'),
  new Greeting('hy', 'Armenian', 'Հայերեն', 'Բարև %s', 'Մարիա'),
  new Greeting('am', 'Amharic', 'አማርኛ', 'ሰላም %s', 'ማሪያ'),
  new Greeting('mn', 'Mongolian', 'Монгол', 'Сайн уу %s', 'Мария'),
  new Greeting('ne', 'Nepali', 'नेपाली', 'नमस्कार %s', 'मारिया'),
  new Greeting('kk', 'Kazakh', 'Қазақша', 'Сәлем %s', 'Мария'),
  new Greeting('uz', 'Uzbek', 'Oʻzbekcha', 'Salom %s', 'Mariya'),
  new Greeting('yo', 'Yoruba', 'Yorùbá', 'Báwo %s', 'Maria'),
  new Greeting('zu', 'Zulu', 'isiZulu', 'Sawubona %s', 'uMaria'),
]);

const GREETING_BY_CODE = new Map(GREETINGS.map((greeting) => [greeting.langCode, greeting]));

function lookup(langCode) {
  return GREETING_BY_CODE.get(langCode) || GREETING_BY_CODE.get('en');
}

module.exports = {
  Greeting,
  GREETINGS,
  lookup,
};
