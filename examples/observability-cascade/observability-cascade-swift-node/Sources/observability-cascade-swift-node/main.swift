import CascadeNodeSwift
import CascadeNodeSwiftServer
import Foundation

exit(Int32(CLI.run(Array(CommandLine.arguments.dropFirst())) { listenURI, transport, children in
    try listenAndServe(listenURI, transport: transport, children: children)
}))
