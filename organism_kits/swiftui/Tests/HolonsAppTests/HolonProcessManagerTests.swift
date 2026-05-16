#if os(macOS)
import XCTest

@testable import HolonsApp

final class HolonProcessManagerTests: XCTestCase {
    @MainActor
    func testStartSelectsPreferredHolonAndConnects() async {
        let recorder = ProcessRecorder()
        let manager = HolonProcessManager(
            holons: FakeHolons([
                FakeHolon(slug: "slow", title: "Slow", rank: 20),
                FakeHolon(slug: "fast", title: "Fast", rank: 1),
            ]),
            clientFactory: { slug, options in
                recorder.connect(slug: slug, transport: options.transport, lifecycle: options.lifecycle)
                return FakeProcessClient()
            },
            closeClient: { client in client.close() },
            slugOf: { $0.slug },
            displayNameOf: { $0.title },
            sortRankOf: { $0.rank },
            defaultTransport: "tcp"
        )

        await manager.start()

        XCTAssertTrue(manager.isRunning)
        XCTAssertEqual(manager.selectedHolon?.slug, "fast")
        XCTAssertEqual(recorder.calls.map(\.slug), ["fast"])
        XCTAssertEqual(recorder.calls.map(\.transport), ["tcp"])
        XCTAssertEqual(recorder.calls.map(\.lifecycle), ["ephemeral"])
    }

    @MainActor
    func testChangingSelectionStopsCurrentClient() async {
        let client = FakeProcessClient()
        let manager = HolonProcessManager(
            holons: FakeHolons([]),
            clientFactory: { _, _ in client },
            closeClient: { client in client.close() },
            slugOf: { $0.slug },
            displayNameOf: { $0.title },
            sortRankOf: { $0.rank },
            autoRefresh: false
        )
        manager.availableHolons = [
            FakeHolon(slug: "one", title: "One", rank: 1),
            FakeHolon(slug: "two", title: "Two", rank: 2),
        ]

        await manager.start()
        manager.selectedHolon = manager.availableHolons[1]

        XCTAssertFalse(manager.isRunning)
        XCTAssertTrue(client.didClose)
    }
}

private struct FakeHolon: Identifiable, Hashable, Sendable {
    let slug: String
    let title: String
    let rank: Int

    var id: String { slug }
}

private final class FakeHolons: Holons {
    private let values: [FakeHolon]

    init(_ values: [FakeHolon]) {
        self.values = values
    }

    func list() throws -> [FakeHolon] {
        values
    }
}

private final class FakeProcessClient: @unchecked Sendable {
    private(set) var didClose = false

    func close() {
        didClose = true
    }
}

private final class ProcessRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var storedCalls: [(slug: String, transport: String, lifecycle: String)] = []

    var calls: [(slug: String, transport: String, lifecycle: String)] {
        lock.lock()
        defer { lock.unlock() }
        return storedCalls
    }

    func connect(slug: String, transport: String, lifecycle: String) {
        lock.lock()
        storedCalls.append((slug: slug, transport: transport, lifecycle: lifecycle))
        lock.unlock()
    }
}
#endif
