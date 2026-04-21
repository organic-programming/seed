import SwiftProtobuf

/// Make the generated Language type usable in SwiftUI lists.
extension Greeting_V1_Language: Identifiable {
    public var id: String { code }
}
