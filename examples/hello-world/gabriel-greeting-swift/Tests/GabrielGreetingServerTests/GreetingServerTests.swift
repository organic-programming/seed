import GabrielGreeting
import GabrielGreetingServer
import GRPC
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
        request.name = "Alice"
        request.langCode = "fr"

        let response = try client.stub.sayHello(request).response.wait()

        XCTAssertEqual(response.greeting, "Bonjour Alice")
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

    private func startClient() throws -> RunningClient {
        let serverGroup = MultiThreadedEventLoopGroup(numberOfThreads: 1)
        let clientGroup = MultiThreadedEventLoopGroup(numberOfThreads: 1)
        let server = try Server.insecure(group: serverGroup)
            .withServiceProviders([GreetingServiceProvider()])
            .bind(host: "127.0.0.1", port: 0)
            .wait()
        let channel = ClientConnection.insecure(group: clientGroup)
            .connect(host: "127.0.0.1", port: server.channel.localAddress?.port ?? 0)
        let stub = Greeting_V1_GreetingServiceClient(channel: channel)

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
    let stub: Greeting_V1_GreetingServiceClient

    func stop() {
        _ = try? channel.close().wait()
        _ = try? server.initiateGracefulShutdown().wait()
        try? clientGroup.syncShutdownGracefully()
        try? serverGroup.syncShutdownGracefully()
    }
}
