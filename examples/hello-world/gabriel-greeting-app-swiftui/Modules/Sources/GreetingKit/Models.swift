import SwiftProtobuf

/// Make the generated Language type usable in SwiftUI lists.
extension Greeting_V1_Language: @retroactive Identifiable {
    public var id: String { code }
}
