import Dispatch
import Foundation
import GRPC
import Holons
#if os(Linux)
import Glibc
#else
import Darwin
#endif

@main
enum GreetingDaemonMain {
    static func main() throws {
        let args = Array(CommandLine.arguments.dropFirst())
        guard let command = args.first else {
            usage()
        }

        switch command {
        case "serve":
            try runServe(Array(args.dropFirst()))
        case "version":
            print("gudule-daemon-greeting-swift v0.4.2")
        default:
            usage()
        }
    }

    private static func runServe(_ args: [String]) throws {
        let recipeRoot = try findRecipeRoot()
        let listenURI = Serve.parseFlags(args)
        let options = Serve.Options(
            protoDir: recipeRoot.appendingPathComponent("protos").path,
            holonYAMLPath: recipeRoot.appendingPathComponent("holon.yaml").path
        )
        let running = try Serve.startWithOptions(
            listenURI,
            serviceProviders: [GreetingServiceProvider()],
            options: options
        )

        announce(running.publicURI)

        signal(SIGTERM, SIG_IGN)
        signal(SIGINT, SIG_IGN)

        let queue = DispatchQueue(label: "gudule.greeting.swift.signal-forwarding")
        let stopServer = {
            running.stop(gracePeriodSeconds: options.shutdownGracePeriodSeconds)
        }

        let termSource = DispatchSource.makeSignalSource(signal: SIGTERM, queue: queue)
        termSource.setEventHandler(handler: stopServer)
        termSource.resume()

        let intSource = DispatchSource.makeSignalSource(signal: SIGINT, queue: queue)
        intSource.setEventHandler(handler: stopServer)
        intSource.resume()

        defer {
            termSource.cancel()
            intSource.cancel()
            signal(SIGTERM, SIG_DFL)
            signal(SIGINT, SIG_DFL)
        }

        try running.await()
    }

    private static func announce(_ uri: String) {
        guard let data = "gRPC server listening on \(uri)\n".data(using: .utf8) else {
            return
        }
        try? FileHandle.standardError.write(contentsOf: data)
    }

    private static func usage() -> Never {
        fputs("usage: gudule-daemon-greeting-swift <serve|version> [flags]\n", stderr)
        exit(1)
    }
}
