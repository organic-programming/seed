import Foundation
import Holons
#if os(Linux)
import Glibc
#else
import Darwin
#endif

let invocation = CertificationCLI.makeEchoClientInvocation(
    userArgs: Array(CommandLine.arguments.dropFirst())
)

do {
    let status = try CertificationCLI.run(invocation)
    exit(status)
} catch {
    fputs("echo-client failed: \(error)\n", stderr)
    exit(1)
}
