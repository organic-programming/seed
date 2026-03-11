// Package internal contains the greeting data and server logic.
package internal

// Greeting holds a language entry with its greeting template.
type Greeting struct {
	Code     string // ISO 639-1
	Name     string // English name
	Native   string // Native script name
	Template string // fmt.Sprintf template with one %s for the name
}

// Greetings contains all 56 supported languages.
var Greetings = []Greeting{
	{"en", "English", "English", "Hello, %s!"},
	{"fr", "French", "Français", "Bonjour, %s !"},
	{"es", "Spanish", "Español", "¡Hola, %s!"},
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
}

// index builds a code→Greeting lookup map (called once at init).
var greetingIndex map[string]Greeting

func init() {
	greetingIndex = make(map[string]Greeting, len(Greetings))
	for _, g := range Greetings {
		greetingIndex[g.Code] = g
	}
}

// Lookup returns the Greeting for the given lang code.
// Returns English if the code is unknown.
func Lookup(code string) Greeting {
	if g, ok := greetingIndex[code]; ok {
		return g
	}
	return greetingIndex["en"]
}
