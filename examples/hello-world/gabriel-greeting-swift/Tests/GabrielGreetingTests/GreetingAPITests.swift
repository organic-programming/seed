import XCTest
@testable import GabrielGreeting

final class GreetingAPITests: XCTestCase {
    func testListLanguagesIncludesEnglish() {
        let response = GreetingAPI.listLanguages()

        XCTAssertEqual(response.languages.count, 56)

        guard let english = response.languages.first(where: { $0.code == "en" }) else {
            XCTFail("expected English entry")
            return
        }

        XCTAssertEqual(english.name, "English")
        XCTAssertEqual(english.native, "English")
    }

    func testSayHelloUsesRequestedLanguage() {
        var request = Greeting_V1_SayHelloRequest()
        request.name = "Alice"
        request.langCode = "fr"

        let response = GreetingAPI.sayHello(request)

        XCTAssertEqual(response.greeting, "Bonjour Alice")
        XCTAssertEqual(response.language, "French")
        XCTAssertEqual(response.langCode, "fr")
    }

    func testSayHelloFallsBackToEnglishDefaultName() {
        var request = Greeting_V1_SayHelloRequest()
        request.langCode = "unknown"

        let response = GreetingAPI.sayHello(request)

        XCTAssertEqual(response.greeting, "Hello Mary")
        XCTAssertEqual(response.language, "English")
        XCTAssertEqual(response.langCode, "en")
    }
}
