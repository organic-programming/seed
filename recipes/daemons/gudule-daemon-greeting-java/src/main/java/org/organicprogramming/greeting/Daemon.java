package org.organicprogramming.greeting;

import greeting.v1.Greeting.*;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import org.organicprogramming.holons.Serve;
import org.organicprogramming.holons.Transport;

import java.io.IOException;
import java.util.HashMap;
import java.util.Map;

public class Daemon {

    private static class GreetingData {
        String code;
        String name;
        String nativeName;
        String template;

        GreetingData(String code, String name, String nativeName, String template) {
            this.code = code;
            this.name = name;
            this.nativeName = nativeName;
            this.template = template;
        }
    }

    private static final Map<String, GreetingData> greetings = new HashMap<>();
    private static final GreetingData[] greetingsList = new GreetingData[]{
        new GreetingData("en", "English", "English", "Hello, %s!"),
        new GreetingData("fr", "French", "Français", "Bonjour, %s !"),
        new GreetingData("es", "Spanish", "Español", "¡Hola, %s!"),
        new GreetingData("de", "German", "Deutsch", "Hallo, %s!"),
        new GreetingData("it", "Italian", "Italiano", "Ciao, %s!"),
        new GreetingData("pt", "Portuguese", "Português", "Olá, %s!"),
        new GreetingData("nl", "Dutch", "Nederlands", "Hallo, %s!"),
        new GreetingData("ru", "Russian", "Русский", "Привет, %s!"),
        new GreetingData("ja", "Japanese", "日本語", "こんにちは、%sさん！"),
        new GreetingData("zh", "Chinese", "中文", "你好，%s！"),
        new GreetingData("ko", "Korean", "한국어", "안녕하세요, %s!"),
        new GreetingData("ar", "Arabic", "العربية", "مرحبا، %s!"),
        new GreetingData("hi", "Hindi", "हिन्दी", "नमस्ते, %s!"),
        new GreetingData("tr", "Turkish", "Türkçe", "Merhaba, %s!"),
        new GreetingData("pl", "Polish", "Polski", "Cześć, %s!"),
        new GreetingData("sv", "Swedish", "Svenska", "Hej, %s!"),
        new GreetingData("no", "Norwegian", "Norsk", "Hei, %s!"),
        new GreetingData("da", "Danish", "Dansk", "Hej, %s!"),
        new GreetingData("fi", "Finnish", "Suomi", "Hei, %s!"),
        new GreetingData("cs", "Czech", "Čeština", "Ahoj, %s!"),
        new GreetingData("ro", "Romanian", "Română", "Bună, %s!"),
        new GreetingData("hu", "Hungarian", "Magyar", "Szia, %s!"),
        new GreetingData("el", "Greek", "Ελληνικά", "Γεια σου, %s!"),
        new GreetingData("th", "Thai", "ไทย", "สวัสดี, %s!"),
        new GreetingData("vi", "Vietnamese", "Tiếng Việt", "Xin chào, %s!"),
        new GreetingData("id", "Indonesian", "Bahasa Indonesia", "Halo, %s!"),
        new GreetingData("ms", "Malay", "Bahasa Melayu", "Hai, %s!"),
        new GreetingData("sw", "Swahili", "Kiswahili", "Habari, %s!"),
        new GreetingData("he", "Hebrew", "עברית", "שלום, %s!"),
        new GreetingData("uk", "Ukrainian", "Українська", "Привіт, %s!"),
        new GreetingData("bn", "Bengali", "বাংলা", "নমস্কার, %s!"),
        new GreetingData("ta", "Tamil", "தமிழ்", "வணக்கம், %s!"),
        new GreetingData("fa", "Persian", "فارسی", "سلام، %s!"),
        new GreetingData("ur", "Urdu", "اردو", "السلام علیکم، %s!"),
        new GreetingData("fil", "Filipino", "Filipino", "Kumusta, %s!"),
        new GreetingData("ca", "Catalan", "Català", "Hola, %s!"),
        new GreetingData("eu", "Basque", "Euskara", "Kaixo, %s!"),
        new GreetingData("gl", "Galician", "Galego", "Ola, %s!"),
        new GreetingData("is", "Icelandic", "Íslenska", "Halló, %s!"),
        new GreetingData("et", "Estonian", "Eesti", "Tere, %s!"),
        new GreetingData("lv", "Latvian", "Latviešu", "Sveiki, %s!"),
        new GreetingData("lt", "Lithuanian", "Lietuvių", "Sveiki, %s!"),
        new GreetingData("sk", "Slovak", "Slovenčina", "Ahoj, %s!"),
        new GreetingData("sl", "Slovenian", "Slovenščina", "Živjo, %s!"),
        new GreetingData("hr", "Croatian", "Hrvatski", "Bok, %s!"),
        new GreetingData("sr", "Serbian", "Српски", "Здраво, %s!"),
        new GreetingData("bg", "Bulgarian", "Български", "Здравей, %s!"),
        new GreetingData("ka", "Georgian", "ქართული", "გამარჯობა, %s!"),
        new GreetingData("hy", "Armenian", "Հայերեն", "Բարև, %s!"),
        new GreetingData("am", "Amharic", "አማርኛ", "ሰላም, %s!"),
        new GreetingData("mn", "Mongolian", "Монгол", "Сайн уу, %s!"),
        new GreetingData("ne", "Nepali", "नेपाली", "नमस्कार, %s!"),
        new GreetingData("kk", "Kazakh", "Қазақша", "Сәлем, %s!"),
        new GreetingData("uz", "Uzbek", "Oʻzbekcha", "Salom, %s!"),
        new GreetingData("yo", "Yoruba", "Yorùbá", "Báwo, %s!"),
        new GreetingData("zu", "Zulu", "isiZulu", "Sawubona, %s!"),
    };

    static {
        for (GreetingData g : greetingsList) {
            greetings.put(g.code, g);
        }
    }

    private static GreetingData lookup(String code) {
        if (code == null) code = "en";
        GreetingData g = greetings.get(code);
        return g != null ? g : greetings.get("en");
    }

    public static void main(String[] args) throws IOException, InterruptedException {
        if (args.length > 0 && args[0].equals("version")) {
            System.out.println("gudule-daemon-greeting-java v0.4.5");
            return;
        }

        String listenUri = Serve.parseFlags(args);
        Transport.ParsedURI uri = Transport.parseURI(listenUri);

        if (!"tcp".equals(uri.scheme()) && !"unix".equals(uri.scheme())) {
            System.err.println("Unsupported scheme for Java gRPC yet: " + uri.scheme());
            System.exit(1);
        }

        // We run on typical standard ServerBuilder since java-holons lifecycle wrapper isn't complete yet
        // However we respect the port parsed by java-holons
        int port = uri.port() > 0 ? uri.port() : 9090;

        Server server = ServerBuilder.forPort(port)
                .addService(new GreetingServiceImpl())
                // .addService(ProtoReflectionService.newInstance()) // omitted to reduce dependency bloat
                .build()
                .start();

        System.err.println("gRPC server listening on tcp://:" + server.getPort());
        
        // Let it run
        server.awaitTermination();
    }

    static class GreetingServiceImpl extends GreetingServiceGrpc.GreetingServiceImplBase {
        @Override
        public void listLanguages(ListLanguagesRequest request, StreamObserver<ListLanguagesResponse> responseObserver) {
            ListLanguagesResponse.Builder responseBuilder = ListLanguagesResponse.newBuilder();
            for (GreetingData g : greetingsList) {
                Language lang = Language.newBuilder()
                        .setCode(g.code)
                        .setName(g.name)
                        .setNative(g.nativeName)
                        .build();
                responseBuilder.addLanguages(lang);
            }
            responseObserver.onNext(responseBuilder.build());
            responseObserver.onCompleted();
        }

        @Override
        public void sayHello(SayHelloRequest request, StreamObserver<SayHelloResponse> responseObserver) {
            String name = request.getName();
            if (name == null || name.isEmpty()) {
                name = "World";
            }
            GreetingData g = lookup(request.getLangCode());
            
            String greetingStr = g.template.replace("%s", name);
            
            SayHelloResponse response = SayHelloResponse.newBuilder()
                    .setGreeting(greetingStr)
                    .setLanguage(g.name)
                    .setLangCode(g.code)
                    .build();
                    
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        }
    }
}
