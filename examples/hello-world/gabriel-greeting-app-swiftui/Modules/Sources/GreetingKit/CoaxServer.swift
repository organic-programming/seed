import Foundation
#if os(macOS)
import Holons
import GRPC

// CoaxServer manages the in-process gRPC server that exposes the COAX
// interaction surface. It is OFF by default — a human toggles it on
// via the UI. The toggle state persists across launches (UserDefaults).
@MainActor
public final class CoaxServer: ObservableObject {
    @Published public var isEnabled: Bool {
        didSet {
            UserDefaults.standard.set(isEnabled, forKey: CoaxServer.enabledKey)
            if isEnabled {
                startServer()
            } else {
                stopServer()
            }
        }
    }

    @Published public private(set) var listenURI: String?

    private let holon: HolonProcess
    private var runningServer: Serve.RunningServer?

    private static let enabledKey = "coax.server.enabled"

    public init(holon: HolonProcess) {
        self.holon = holon
        self.isEnabled = UserDefaults.standard.bool(forKey: CoaxServer.enabledKey)
    }

    /// Start the server if enabled. Called once at app launch.
    public func startIfEnabled() {
        if isEnabled {
            startServer()
        }
    }

    public func stop() {
        stopServer()
    }

    // MARK: - Private

    private func startServer() {
        guard runningServer == nil else { return }
        listenURI = nil

        let coaxProvider = CoaxServiceProvider(holon: holon, coaxServer: self)
        let appProvider = GreetingAppServiceProvider(holon: holon)
        let metaProvider = CoaxDescribeProvider()
        let providers: [CallHandlerProvider] = [metaProvider, coaxProvider, appProvider]

        // Use a synchronous background thread (DispatchQueue) instead of Task.detached
        // to avoid Swift 6 sendability issues with Serve.RunningServer.
        DispatchQueue.global(qos: .utility).async { [weak self] in
            do {
                let server = try Serve.startWithOptions(
                    "tcp://:0",
                    serviceProviders: providers,
                    options: Serve.Options(
                        describe: false,
                        logger: { _ in }
                    )
                )
                let uri = server.publicURI
                DispatchQueue.main.async { [weak self] in
                    self?.runningServer = server
                    self?.listenURI = uri
                    self?.logCoax("[COAX] server listening on \(uri)")
                }
            } catch {
                DispatchQueue.main.async { [weak self] in
                    self?.listenURI = nil
                    self?.logCoax("[COAX] failed to start server: \(error)")
                }
            }
        }
    }

    private func stopServer() {
        guard let server = runningServer else { return }
        runningServer = nil
        listenURI = nil
        logCoax("[COAX] server stopped")
        DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + .milliseconds(250)) {
            server.stop()
        }
    }

    private func logCoax(_ line: String) {
        guard let data = (line + "\n").data(using: .utf8) else { return }
        FileHandle.standardError.write(data)
    }
}
#endif
