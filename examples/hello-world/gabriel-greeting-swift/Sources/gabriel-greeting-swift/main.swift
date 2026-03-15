import Foundation
import GabrielGreeting
import GabrielGreetingServer

exit(Int32(CLI.run(Array(CommandLine.arguments.dropFirst()), serve: listenAndServe)))
