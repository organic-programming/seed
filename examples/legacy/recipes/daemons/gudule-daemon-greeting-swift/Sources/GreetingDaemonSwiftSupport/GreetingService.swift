import Foundation
import GRPC
import GreetingGenerated

struct GreetingService {
    func listLanguages() -> Greeting_V1_ListLanguagesResponse {
        var response = Greeting_V1_ListLanguagesResponse()
        response.languages = greetings.map { entry in
            var language = Greeting_V1_Language()
            language.code = entry.code
            language.name = entry.name
            language.native = entry.nativeLabel
            return language
        }
        return response
    }

    func sayHello(name: String, langCode: String) -> Greeting_V1_SayHelloResponse {
        let entry = lookupGreeting(langCode)
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)

        var response = Greeting_V1_SayHelloResponse()
        response.greeting = entry.template.replacingOccurrences(
            of: "%s",
            with: trimmedName.isEmpty ? "World" : trimmedName
        )
        response.language = entry.name
        response.langCode = entry.code
        return response
    }
}

public enum GreetingDaemonSwiftSupport {
    public static func makeServiceProviders() -> [CallHandlerProvider] {
        [GreetingServiceProvider()]
    }

    public static func locateRecipeRoot() -> URL? {
        locateGreetingDaemonSwiftRecipeRoot()
    }

    public static func findRecipeRoot() throws -> URL {
        try findGreetingDaemonSwiftRecipeRoot()
    }
}
