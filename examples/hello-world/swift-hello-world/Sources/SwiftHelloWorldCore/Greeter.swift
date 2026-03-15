import Foundation
import Holons

public enum HelloService {
    public static func greet(name: String?) -> String {
        let value = (name ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        let finalName = value.isEmpty ? "World" : value
        return "Hello, \(finalName)!"
    }

    public static func listenURI(from args: [String]) -> String {
        Serve.parseFlags(args)
    }
}
