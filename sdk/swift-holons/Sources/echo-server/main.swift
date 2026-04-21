import Foundation
import Holons
#if os(Linux)
import Glibc
#else
import Darwin
#endif

let invocation = CertificationCLI.makeEchoServerInvocation(
    userArgs: Array(CommandLine.arguments.dropFirst())
)

do {
    let status = try CertificationCLI.run(invocation)
    exit(status)
} catch {
    fputs("echo-server failed: \(error)\n", stderr)
    exit(1)
}
