import XCTest
@testable import GreetingKit

final class GreetingSelectionLogicTests: XCTestCase {
    func testResolvedLanguageSelectionPrefersCurrentThenEnglishThenFirst() {
        let english = language(code: "en", name: "English", native: "English")
        let french = language(code: "fr", name: "French", native: "Francais")
        let spanish = language(code: "es", name: "Spanish", native: "Espanol")

        XCTAssertEqual(
            resolvedLanguageSelection(
                availableLanguages: [english, french, spanish],
                preferredCode: "fr"
            ),
            "fr"
        )
        XCTAssertEqual(
            resolvedLanguageSelection(
                availableLanguages: [english, french, spanish],
                preferredCode: "de"
            ),
            "en"
        )
        XCTAssertEqual(
            resolvedLanguageSelection(
                availableLanguages: [spanish, french],
                preferredCode: "de"
            ),
            "es"
        )
    }

    func testValidatedLanguageSelectionRejectsUnknownCode() throws {
        let english = language(code: "en", name: "English", native: "English")
        let french = language(code: "fr", name: "French", native: "Francais")

        XCTAssertEqual(
            try validatedLanguageSelection("fr", availableLanguages: [english, french]),
            "fr"
        )
        XCTAssertThrowsError(
            try validatedLanguageSelection("zz", availableLanguages: [english, french])
        ) { error in
            XCTAssertEqual(
                (error as? GreetingSelectionError)?.localizedDescription,
                "Unsupported language 'zz'"
            )
        }
    }

    func testResolvedHolonSelectionRejectsUnknownSlug() throws {
        let available = [
            GabrielHolonIdentity(
                slug: "gabriel-greeting-go",
                familyName: "Greeting-Go",
                binaryName: "gabriel-greeting-go",
                buildRunner: "go-module",
                displayName: "Gabriel (Go)",
                sortRank: 0,
                holonUUID: "go-uuid",
                born: "2026-01-01",
                sourceKind: "source",
                discoveryPath: "/tmp/gabriel-greeting-go",
                hasSource: true
            ),
        ]

        XCTAssertEqual(
            try resolvedHolonSelection(slug: "gabriel-greeting-go", availableHolons: available).slug,
            "gabriel-greeting-go"
        )
        XCTAssertThrowsError(
            try resolvedHolonSelection(slug: "gabriel-greeting-swift", availableHolons: available)
        ) { error in
            XCTAssertEqual(
                (error as? GreetingSelectionError)?.localizedDescription,
                "Holon 'gabriel-greeting-swift' not found"
            )
        }
    }
}

private func language(code: String, name: String, native: String) -> Greeting_V1_Language {
    var value = Greeting_V1_Language()
    value.code = code
    value.name = name
    value.native = native
    return value
}
