const std = @import("std");
const holons = @import("zig_holons");

const runtime = holons.protobuf.runtime;

pub const Greeting = struct {
    lang_code: []const u8,
    lang_english: []const u8,
    lang_native: []const u8,
    template: []const u8,
    default_name: []const u8,
};

pub const greetings = [_]Greeting{
    .{ .lang_code = "en", .lang_english = "English", .lang_native = "English", .template = "Hello %s", .default_name = "Mary" },
    .{ .lang_code = "fr", .lang_english = "French", .lang_native = "Français", .template = "Bonjour %s", .default_name = "Marie" },
    .{ .lang_code = "es", .lang_english = "Spanish", .lang_native = "Español", .template = "Hola %s", .default_name = "María" },
    .{ .lang_code = "de", .lang_english = "German", .lang_native = "Deutsch", .template = "Hallo %s", .default_name = "Maria" },
    .{ .lang_code = "it", .lang_english = "Italian", .lang_native = "Italiano", .template = "Ciao %s", .default_name = "Maria" },
    .{ .lang_code = "pt", .lang_english = "Portuguese", .lang_native = "Português", .template = "Olá %s", .default_name = "Maria" },
    .{ .lang_code = "nl", .lang_english = "Dutch", .lang_native = "Nederlands", .template = "Hallo %s", .default_name = "Maria" },
    .{ .lang_code = "ru", .lang_english = "Russian", .lang_native = "Русский", .template = "Привет %s", .default_name = "Мария" },
    .{ .lang_code = "ja", .lang_english = "Japanese", .lang_native = "日本語", .template = "こんにちは、%sさん", .default_name = "マリア" },
    .{ .lang_code = "zh", .lang_english = "Chinese", .lang_native = "中文", .template = "你好，%s", .default_name = "玛丽" },
    .{ .lang_code = "ko", .lang_english = "Korean", .lang_native = "한국어", .template = "안녕하세요 %s", .default_name = "마리아" },
    .{ .lang_code = "ar", .lang_english = "Arabic", .lang_native = "العربية", .template = "مرحبا، %s", .default_name = "ماريا" },
    .{ .lang_code = "hi", .lang_english = "Hindi", .lang_native = "हिन्दी", .template = "नमस्ते %s", .default_name = "मारिया" },
    .{ .lang_code = "tr", .lang_english = "Turkish", .lang_native = "Türkçe", .template = "Merhaba %s", .default_name = "Meryem" },
    .{ .lang_code = "pl", .lang_english = "Polish", .lang_native = "Polski", .template = "Cześć %s", .default_name = "Maria" },
    .{ .lang_code = "sv", .lang_english = "Swedish", .lang_native = "Svenska", .template = "Hej %s", .default_name = "Maria" },
    .{ .lang_code = "no", .lang_english = "Norwegian", .lang_native = "Norsk", .template = "Hei %s", .default_name = "Maria" },
    .{ .lang_code = "da", .lang_english = "Danish", .lang_native = "Dansk", .template = "Hej %s", .default_name = "Maria" },
    .{ .lang_code = "fi", .lang_english = "Finnish", .lang_native = "Suomi", .template = "Hei %s", .default_name = "Maria" },
    .{ .lang_code = "cs", .lang_english = "Czech", .lang_native = "Čeština", .template = "Ahoj %s", .default_name = "Marie" },
    .{ .lang_code = "ro", .lang_english = "Romanian", .lang_native = "Română", .template = "Bună %s", .default_name = "Maria" },
    .{ .lang_code = "hu", .lang_english = "Hungarian", .lang_native = "Magyar", .template = "Szia %s", .default_name = "Mária" },
    .{ .lang_code = "el", .lang_english = "Greek", .lang_native = "Ελληνικά", .template = "Γεια σου %s", .default_name = "Μαρία" },
    .{ .lang_code = "th", .lang_english = "Thai", .lang_native = "ไทย", .template = "สวัสดี %s", .default_name = "มาเรีย" },
    .{ .lang_code = "vi", .lang_english = "Vietnamese", .lang_native = "Tiếng Việt", .template = "Xin chào %s", .default_name = "Mary" },
    .{ .lang_code = "id", .lang_english = "Indonesian", .lang_native = "Bahasa Indonesia", .template = "Halo %s", .default_name = "Maria" },
    .{ .lang_code = "ms", .lang_english = "Malay", .lang_native = "Bahasa Melayu", .template = "Hai %s", .default_name = "Maria" },
    .{ .lang_code = "sw", .lang_english = "Swahili", .lang_native = "Kiswahili", .template = "Habari %s", .default_name = "Maria" },
    .{ .lang_code = "he", .lang_english = "Hebrew", .lang_native = "עברית", .template = "שלום %s", .default_name = "מרים" },
    .{ .lang_code = "uk", .lang_english = "Ukrainian", .lang_native = "Українська", .template = "Привіт %s", .default_name = "Марія" },
    .{ .lang_code = "bn", .lang_english = "Bengali", .lang_native = "বাংলা", .template = "নমস্কার %s", .default_name = "মারিয়া" },
    .{ .lang_code = "ta", .lang_english = "Tamil", .lang_native = "தமிழ்", .template = "வணக்கம் %s", .default_name = "மரியா" },
    .{ .lang_code = "fa", .lang_english = "Persian", .lang_native = "فارسی", .template = "سلام، %s", .default_name = "ماریا" },
    .{ .lang_code = "ur", .lang_english = "Urdu", .lang_native = "اردو", .template = "السلام علیکم، %s", .default_name = "ماریہ" },
    .{ .lang_code = "fil", .lang_english = "Filipino", .lang_native = "Filipino", .template = "Kumusta %s", .default_name = "Maria" },
    .{ .lang_code = "ca", .lang_english = "Catalan", .lang_native = "Català", .template = "Hola %s", .default_name = "Maria" },
    .{ .lang_code = "eu", .lang_english = "Basque", .lang_native = "Euskara", .template = "Kaixo %s", .default_name = "Maria" },
    .{ .lang_code = "gl", .lang_english = "Galician", .lang_native = "Galego", .template = "Ola %s", .default_name = "María" },
    .{ .lang_code = "is", .lang_english = "Icelandic", .lang_native = "Íslenska", .template = "Halló %s", .default_name = "María" },
    .{ .lang_code = "et", .lang_english = "Estonian", .lang_native = "Eesti", .template = "Tere %s", .default_name = "Maria" },
    .{ .lang_code = "lv", .lang_english = "Latvian", .lang_native = "Latviešu", .template = "Sveiki %s", .default_name = "Marija" },
    .{ .lang_code = "lt", .lang_english = "Lithuanian", .lang_native = "Lietuvių", .template = "Sveiki %s", .default_name = "Marija" },
    .{ .lang_code = "sk", .lang_english = "Slovak", .lang_native = "Slovenčina", .template = "Ahoj %s", .default_name = "Mária" },
    .{ .lang_code = "sl", .lang_english = "Slovenian", .lang_native = "Slovenščina", .template = "Živjo %s", .default_name = "Marija" },
    .{ .lang_code = "hr", .lang_english = "Croatian", .lang_native = "Hrvatski", .template = "Bok %s", .default_name = "Marija" },
    .{ .lang_code = "sr", .lang_english = "Serbian", .lang_native = "Српски", .template = "Здраво %s", .default_name = "Марија" },
    .{ .lang_code = "bg", .lang_english = "Bulgarian", .lang_native = "Български", .template = "Здравей %s", .default_name = "Мария" },
    .{ .lang_code = "ka", .lang_english = "Georgian", .lang_native = "ქართული", .template = "გამარჯობა %s", .default_name = "მარიამ" },
    .{ .lang_code = "hy", .lang_english = "Armenian", .lang_native = "Հայերեն", .template = "Բարև %s", .default_name = "Մարիա" },
    .{ .lang_code = "am", .lang_english = "Amharic", .lang_native = "አማርኛ", .template = "ሰላም %s", .default_name = "ማሪያ" },
    .{ .lang_code = "mn", .lang_english = "Mongolian", .lang_native = "Монгол", .template = "Сайн уу %s", .default_name = "Мария" },
    .{ .lang_code = "ne", .lang_english = "Nepali", .lang_native = "नेपाली", .template = "नमस्कार %s", .default_name = "मारिया" },
    .{ .lang_code = "kk", .lang_english = "Kazakh", .lang_native = "Қазақша", .template = "Сәлем %s", .default_name = "Мария" },
    .{ .lang_code = "uz", .lang_english = "Uzbek", .lang_native = "Oʻzbekcha", .template = "Salom %s", .default_name = "Mariya" },
    .{ .lang_code = "yo", .lang_english = "Yoruba", .lang_native = "Yorùbá", .template = "Báwo %s", .default_name = "Maria" },
    .{ .lang_code = "zu", .lang_english = "Zulu", .lang_native = "isiZulu", .template = "Sawubona %s", .default_name = "uMaria" },
};

pub const methods = [_]holons.grpc.server.Method{
    .{ .path = "/greeting.v1.GreetingService/SayHello", .handler = sayHello },
    .{ .path = "/greeting.v1.GreetingService/ListLanguages", .handler = listLanguages },
};

pub fn lookup(code: []const u8) Greeting {
    for (greetings) |entry| {
        if (std.mem.eql(u8, entry.lang_code, code)) return entry;
    }
    return greetings[0];
}

pub fn sayHello(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    var request = try runtime.unpackSayHelloRequest(bytes);
    defer request.deinit();

    const requested_lang = if (request.langCode().len == 0) "en" else request.langCode();
    const selected = lookup(requested_lang);
    const raw_name = std.mem.trim(u8, request.name(), " \t\r\n");
    const name = if (raw_name.len == 0) selected.default_name else raw_name;
    const greeting = try renderTemplate(allocator, selected.template, name);
    defer allocator.free(greeting);

    return runtime.packSayHelloResponse(
        allocator,
        greeting,
        selected.lang_english,
        selected.lang_code,
    );
}

pub fn listLanguages(allocator: std.mem.Allocator, bytes: []const u8) ![]u8 {
    try runtime.unpackListLanguagesRequest(bytes);
    var values: [greetings.len]runtime.LanguageValue = undefined;
    for (greetings, 0..) |entry, index| {
        values[index] = .{
            .code = entry.lang_code,
            .name = entry.lang_english,
            .native = entry.lang_native,
        };
    }
    return runtime.packListLanguagesResponse(allocator, values[0..]);
}

fn renderTemplate(allocator: std.mem.Allocator, template: []const u8, name: []const u8) ![]u8 {
    const marker = std.mem.indexOf(u8, template, "%s") orelse
        return allocator.dupe(u8, template);
    return std.fmt.allocPrint(
        allocator,
        "{s}{s}{s}",
        .{ template[0..marker], name, template[marker + 2 ..] },
    );
}

test "lookup falls back to English" {
    try std.testing.expectEqualStrings("English", lookup("missing").lang_english);
}

test "render greeting template" {
    const greeting = try renderTemplate(std.testing.allocator, lookup("ja").template, "Bob");
    defer std.testing.allocator.free(greeting);
    try std.testing.expectEqualStrings("こんにちは、Bobさん", greeting);
}
