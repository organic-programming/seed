import Foundation
import Holons

public enum CoaxMemberState: Sendable {
    case available
    case connected
    case error
}

public struct CoaxMember: Identifiable, Hashable, Sendable {
    public let slug: String
    public let familyName: String
    public let displayName: String
    public let state: CoaxMemberState
    public let isOrganism: Bool

    public var id: String { slug }

    public init(
        slug: String,
        familyName: String,
        displayName: String,
        state: CoaxMemberState,
        isOrganism: Bool = false
    ) {
        self.slug = slug
        self.familyName = familyName
        self.displayName = displayName
        self.state = state
        self.isOrganism = isOrganism
    }
}

@MainActor
public protocol OrganismState: AnyObject {
    var coaxMembers: [CoaxMember] { get }
    func connectCoaxMember(slug: String, transport: String) async throws -> CoaxMember
    func disconnectCoaxMember(slug: String) async
    func tellCoaxMember(slug: String, method: String, payloadJSON: Data) async throws -> Data
}
