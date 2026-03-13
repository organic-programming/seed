#include <iostream>
#include <string>
#include <vector>
#include <unordered_map>

#include "holons/serve.hpp"
#include "greeting/v1/greeting.grpc.pb.h"

struct Greeting {
    std::string code;
    std::string name;
    std::string native;
    std::string template_str;
};

const std::vector<Greeting> greetings = {
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
};

std::string format_greeting(const std::string& template_str, const std::string& name) {
    size_t pos = template_str.find("%s");
    if (pos == std::string::npos) {
        return template_str;
    }
    std::string result = template_str;
    result.replace(pos, 2, name);
    return result;
}

class GreetingServiceImpl final : public greeting::v1::GreetingService::Service {
    grpc::Status ListLanguages(grpc::ServerContext* context, const greeting::v1::ListLanguagesRequest* request,
                               greeting::v1::ListLanguagesResponse* response) override {
        for (const auto& g : greetings) {
            auto* lang = response->add_languages();
            lang->set_code(g.code);
            lang->set_name(g.name);
            lang->set_native(g.native);
        }
        return grpc::Status::OK;
    }

    grpc::Status SayHello(grpc::ServerContext* context, const greeting::v1::SayHelloRequest* request,
                          greeting::v1::SayHelloResponse* response) override {
        std::string name = request->name().empty() ? "World" : request->name();
        std::string req_code = request->lang_code();
        
        const Greeting* found = nullptr;
        for (const auto& g : greetings) {
            if (g.code == req_code) {
                found = &g;
                break;
            }
        }
        if (!found) {
            for (const auto& g : greetings) {
                if (g.code == "en") {
                    found = &g;
                    break;
                }
            }
        }
        
        if (found) {
            response->set_greeting(format_greeting(found->template_str, name));
            response->set_language(found->name);
            response->set_lang_code(found->code);
        }
        return grpc::Status::OK;
    }
};

int main(int argc, char** argv) {
    std::vector<std::string> args(argv + 1, argv + argc);
    if (!args.empty() && args[0] == "version") {
        std::cout << "gudule-daemon-greeting-cpp v0.4.5\n";
        return 0;
    }
    
    // serve.hpp parse_flags ignores unknown args except --listen and --port,
    // so we can just pass args to it.
    auto listeners = holons::serve::parse_flags(args);

    GreetingServiceImpl service;
    
    holons::serve::options opts;
    auto register_services = [&](grpc::ServerBuilder& builder) {
        builder.RegisterService(&service);
    };

    try {
        holons::serve::serve(listeners, register_services, opts);
    } catch (const std::exception& e) {
        std::cerr << "Error: " << e.what() << std::endl;
        return 1;
    }
    return 0;
}
