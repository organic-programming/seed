import XCTest

@testable import HolonsApp

final class SelectionValidatorsTests: XCTestCase {
    func testResolvedSelectionReturnsMatchingItem() throws {
        let items = [
            SelectionItem(id: "swift", title: "Swift"),
            SelectionItem(id: "zig", title: "Zig"),
        ]

        let selected = try resolvedSelection(
            "zig",
            availableItems: items,
            idOf: { $0.id }
        )

        XCTAssertEqual(selected.title, "Zig")
    }

    func testValidatedSelectionTrimsAndRejectsMissingItem() throws {
        let items = [
            SelectionItem(id: "en", title: "English"),
            SelectionItem(id: "fr", title: "French"),
        ]

        XCTAssertEqual(
            try validatedSelection(" fr ", availableItems: items, idOf: { $0.id }),
            "fr"
        )

        XCTAssertThrowsError(
            try validatedSelection("de", availableItems: items, idOf: { $0.id })
        ) { error in
            XCTAssertEqual(error as? SelectionValidationError, .itemNotFound("de"))
        }
    }

    func testResolvedPreferredSelectionUsesFallbackThenFirst() {
        let items = [
            SelectionItem(id: "es", title: "Spanish"),
            SelectionItem(id: "fr", title: "French"),
        ]

        XCTAssertEqual(
            resolvedPreferredSelection(
                availableItems: items,
                preferredID: "en",
                fallbackIDs: ["fr"],
                idOf: { $0.id }
            ),
            "fr"
        )

        XCTAssertEqual(
            resolvedPreferredSelection(
                availableItems: items,
                preferredID: "en",
                fallbackIDs: ["de"],
                idOf: { $0.id }
            ),
            "es"
        )
    }

    func testValidatedTransportSelectionRequiresCanonicalName() throws {
        XCTAssertEqual(try validatedTransportSelection("tcp"), .tcp)
        XCTAssertThrowsError(try validatedTransportSelection("tcp://")) { error in
            XCTAssertEqual(error as? SelectionValidationError, .unsupportedTransport("tcp://"))
        }
    }
}

private struct SelectionItem {
    let id: String
    let title: String
}
