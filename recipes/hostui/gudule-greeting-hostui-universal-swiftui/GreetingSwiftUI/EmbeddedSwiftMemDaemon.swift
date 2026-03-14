import Foundation
#if os(macOS)
import GreetingDaemonSwiftSupport
import Holons

/// Hosts the Swift greeting daemon in-process so `connect(slug)` can reach it over `mem://`.
final class EmbeddedSwiftMemDaemon {
    private var runningServer: Serve.RunningServer?

    deinit {
        runningServer?.stop()
    }

    func start(
        slug: String,
        stageRoot: URL,
        logger: @escaping (String) -> Void
    ) throws {
        stop(logger: logger)

        let holonRoot = stageRoot
            .appendingPathComponent("holons", isDirectory: true)
            .appendingPathComponent(slug, isDirectory: true)
        let options = Serve.Options(
            logger: logger,
            protoDir: holonRoot.appendingPathComponent("protos", isDirectory: true).path,
            holonYAMLPath: holonRoot.appendingPathComponent("holon.yaml").path
        )
        let running = try Serve.startWithOptions(
            "mem://\(slug)",
            serviceProviders: GreetingDaemonSwiftSupport.makeServiceProviders(),
            options: options
        )

        logger("[HostUI] embedded swift mem daemon listening on \(running.publicURI)")
        runningServer = running
    }

    func stop(logger: ((String) -> Void)? = nil) {
        guard let runningServer else { return }
        self.runningServer = nil
        runningServer.stop()
        logger?("[HostUI] embedded swift mem daemon stopped")
    }
}
#endif
