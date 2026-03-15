import Foundation

public enum GreetingAPI {
    public static func listLanguages() -> Greeting_V1_ListLanguagesResponse {
        var response = Greeting_V1_ListLanguagesResponse()
        response.languages = Greetings.all.map { greeting in
            var language = Greeting_V1_Language()
            language.code = greeting.langCode
            language.name = greeting.langEnglish
            language.native = greeting.langNative
            return language
        }
        return response
    }

    public static func sayHello(_ request: Greeting_V1_SayHelloRequest) -> Greeting_V1_SayHelloResponse {
        let greeting = Greetings.lookup(request.langCode)
        let trimmedName = request.name.trimmingCharacters(in: .whitespacesAndNewlines)

        var response = Greeting_V1_SayHelloResponse()
        response.greeting = String(format: greeting.template, trimmedName.isEmpty ? greeting.defaultName : trimmedName)
        response.language = greeting.langEnglish
        response.langCode = greeting.langCode
        return response
    }
}
