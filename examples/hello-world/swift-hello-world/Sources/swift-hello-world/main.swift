import Foundation
import SwiftHelloWorldCore

let args = Array(CommandLine.arguments.dropFirst())
let listenURI = HelloService.listenURI(from: args)

var name: String?
if let idx = args.firstIndex(of: "--name"), idx + 1 < args.count {
    name = args[idx + 1]
}

let message = HelloService.greet(name: name)

FileHandle.standardError.write(Data("swift-hello-world listening on \(listenURI)\n".utf8))
print("{\"message\":\"\(message)\"}")
