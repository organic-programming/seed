import Foundation
#if os(macOS)
import GabrielGreetingServer
import Holons

final class EmbeddedSwiftMemHolon {
    private var runningServer: Serve.RunningServer?

    deinit {
        runningServer?.stop()
    }

    func start(
        slug: String,
        protoDir: URL,
        logger: @escaping (String) -> Void
    ) throws {
        stop(logger: logger)

        let options = Serve.Options(
            logger: logger,
            protoDir: protoDir.path
        )
        let running = try Serve.startWithOptions(
            "mem://\(slug)",
            serviceProviders: [GreetingServiceProvider()],
            options: options
        )

        logger("[HostUI] embedded Swift mem holon listening on \(running.publicURI)")
        runningServer = running
    }

    func stop(logger: ((String) -> Void)? = nil) {
        guard let runningServer else { return }
        self.runningServer = nil
        runningServer.stop()
        logger?("[HostUI] embedded Swift mem holon stopped")
    }
}
#endif
