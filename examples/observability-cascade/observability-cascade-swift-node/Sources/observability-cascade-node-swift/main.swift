import CascadeNodeSwift
import CascadeNodeSwiftServer
import Foundation

exit(Int32(CLI.run(Array(CommandLine.arguments.dropFirst()), serve: listenAndServe)))
