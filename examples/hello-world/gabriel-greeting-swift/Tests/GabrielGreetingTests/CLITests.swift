import XCTest
@testable import GabrielGreeting

final class CLITests: XCTestCase {
    func testRunCLIVersion() {
        var stdout = StringOutputStream()
        var stderr = StringOutputStream()

        let code = CLI.run(["version"], stdout: &stdout, stderr: &stderr)

        XCTAssertEqual(code, 0)
        XCTAssertEqual(stdout.string.trimmingCharacters(in: .whitespacesAndNewlines), CLI.version)
        XCTAssertTrue(stderr.string.isEmpty)
    }

    func testRunCLIListLanguagesJSON() throws {
        var stdout = StringOutputStream()
        var stderr = StringOutputStream()

        let code = CLI.run(["list-languages", "--format", "json"], stdout: &stdout, stderr: &stderr)

        XCTAssertEqual(code, 0)
        XCTAssertTrue(stderr.string.isEmpty)

        let response = try Greeting_V1_ListLanguagesResponse(jsonUTF8Data: Data(stdout.string.utf8))
        XCTAssertEqual(response.languages.count, 56)
        XCTAssertEqual(response.languages.first?.code, "en")
        XCTAssertEqual(response.languages.first?.name, "English")
    }

    func testRunCLISayHelloText() {
        var stdout = StringOutputStream()
        var stderr = StringOutputStream()

        let code = CLI.run(["say-hello", "Alice", "fr"], stdout: &stdout, stderr: &stderr)

        XCTAssertEqual(code, 0)
        XCTAssertEqual(stdout.string.trimmingCharacters(in: .whitespacesAndNewlines), "Bonjour Alice")
        XCTAssertTrue(stderr.string.isEmpty)
    }

    func testRunCLISayHelloDefaultsToEnglishJSON() throws {
        var stdout = StringOutputStream()
        var stderr = StringOutputStream()

        let code = CLI.run(["say-hello", "--json"], stdout: &stdout, stderr: &stderr)

        XCTAssertEqual(code, 0)
        XCTAssertTrue(stderr.string.isEmpty)

        let response = try Greeting_V1_SayHelloResponse(jsonUTF8Data: Data(stdout.string.utf8))
        XCTAssertEqual(response.greeting, "Hello Mary")
        XCTAssertEqual(response.language, "English")
        XCTAssertEqual(response.langCode, "en")
    }
}

private struct StringOutputStream: TextOutputStream {
    var string = ""

    mutating func write(_ string: String) {
        self.string += string
    }
}
