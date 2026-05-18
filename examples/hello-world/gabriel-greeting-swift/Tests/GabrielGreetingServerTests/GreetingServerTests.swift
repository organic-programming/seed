import GabrielGreeting
import GabrielGreetingServer
import GRPC
import Holons
import NIOPosix
import XCTest

final class GreetingServerTests: XCTestCase {
    func testListLanguagesReturnsAll() throws {
        let client = try startClient()
        defer { client.stop() }

        let response = try client.stub.listLanguages(.init()).response.wait()

        XCTAssertEqual(response.languages.count, 56)
    }

    func testListLanguagesHasRequiredFields() throws {
        let client = try startClient()
        defer { client.stop() }

        let response = try client.stub.listLanguages(.init()).response.wait()

        for language in response.languages {
            XCTAssertFalse(language.code.isEmpty)
            XCTAssertFalse(language.name.isEmpty)
            XCTAssertFalse(language.native.isEmpty)
        }
    }

    func testSayHelloNominal() throws {
        let client = try startClient()
        defer { client.stop() }

        var request = Greeting_V1_SayHelloRequest()
        request.name = "Bob"
        request.langCode = "fr"

        let response = try client.stub.sayHello(request).response.wait()

        XCTAssertEqual(response.greeting, "Bonjour Bob")
        XCTAssertEqual(response.language, "French")
        XCTAssertEqual(response.langCode, "fr")
    }

    func testSayHelloEmptyName() throws {
        let client = try startClient()
        defer { client.stop() }

        var request = Greeting_V1_SayHelloRequest()
        request.langCode = "en"

        let response = try client.stub.sayHello(request).response.wait()

        XCTAssertEqual(response.greeting, "Hello Mary")
    }

    func testSayHelloUnknownLanguageFallsBackToEnglish() throws {
        let client = try startClient()
        defer { client.stop() }

        var request = Greeting_V1_SayHelloRequest()
        request.name = "Bob"
        request.langCode = "xx"

        let response = try client.stub.sayHello(request).response.wait()

        XCTAssertEqual(response.langCode, "en")
        XCTAssertEqual(response.greeting, "Hello Bob")
    }

    func testSayHelloEmitsObservabilitySignals() throws {
        reset()
        let obs = try configure(
            ObsConfig(slug: "gabriel-greeting-swift"),
            env: ["OP_OBS": "logs,metrics"]
        )
        defer {
            obs.close()
            reset()
        }

        let client = try startClient()
        defer { client.stop() }

        var request = Greeting_V1_SayHelloRequest()
        request.name = " Bob "
        request.langCode = "en"

        let response = try client.stub.sayHello(request).response.wait()

        XCTAssertEqual(response.greeting, "Hello Bob")

        let counters = obs.registry?.listCounters() ?? []
        let counter = counters.first { sample in
            sample.name == "greeting_emitted_total"
                && sample.labels["lang_code"] == "en"
                && sample.labels["language"] == "English"
                && sample.labels["transport"] == "unknown"
        }
        XCTAssertEqual(counter?.read(), 1)

        let entry = obs.logRing?.drain().first { entry in
            entry.bodyString == "Greeted Bob in English (en)"
        }
        XCTAssertEqual(entry?.record.severityNumber, .info)
        XCTAssertEqual(entry?.attribute(AttrHolonsSlug), "gabriel-greeting-swift")
        XCTAssertEqual(entry?.attribute(AttrServiceName), "gabriel-greeting-swift")
        XCTAssertEqual(entry?.attribute("lang_code"), "en")
        XCTAssertEqual(entry?.attribute("language"), "English")
        XCTAssertEqual(entry?.attribute("name"), "Bob")
        XCTAssertEqual(entry?.attribute("greeting"), "Hello Bob")
        XCTAssertEqual(entry?.attribute("transport"), "unknown")
        guard let durationValue = entry?.record.attributes.first(where: { $0.key == "duration_ns" })?.value,
              case .intValue(let durationNS)? = durationValue.value else {
            XCTFail("duration_ns should be encoded as int_value")
            return
        }
        XCTAssertGreaterThanOrEqual(durationNS, 0)
    }

    private func startClient() throws -> RunningClient {
        let serverGroup = MultiThreadedEventLoopGroup(numberOfThreads: 1)
        let clientGroup = MultiThreadedEventLoopGroup(numberOfThreads: 1)
        let server = try Server.insecure(group: serverGroup)
            .withServiceProviders([GreetingServiceProvider()])
            .bind(host: "127.0.0.1", port: 0)
            .wait()
        let channel = ClientConnection.insecure(group: clientGroup)
            .withConnectionReestablishment(enabled: false)
            .connect(host: "127.0.0.1", port: server.channel.localAddress?.port ?? 0)
        let stub = Greeting_V1_GreetingServiceNIOClient(channel: channel)

        return RunningClient(
            server: server,
            serverGroup: serverGroup,
            clientGroup: clientGroup,
            channel: channel,
            stub: stub
        )
    }
}

private struct RunningClient {
    let server: Server
    let serverGroup: MultiThreadedEventLoopGroup
    let clientGroup: MultiThreadedEventLoopGroup
    let channel: ClientConnection
    let stub: Greeting_V1_GreetingServiceNIOClient

    func stop() {
        _ = try? server.initiateGracefulShutdown().wait()
        _ = try? server.onClose.wait()
        _ = try? channel.close().wait()
        try? serverGroup.syncShutdownGracefully()
        try? clientGroup.syncShutdownGracefully()
    }
}
