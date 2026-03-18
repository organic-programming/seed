import SwiftUI

/// Domain model for a language entry returned by the greeting service.
public struct Language: Identifiable, Sendable {
    public let code: String
    public let name: String
    public let native: String

    public var id: String { code }

    public init(code: String, name: String, native: String) {
        self.code = code
        self.name = name
        self.native = native
    }
}
