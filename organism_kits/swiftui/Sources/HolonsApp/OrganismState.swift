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
public protocol HolonManager: AnyObject {
    func listMembers() async -> [CoaxMember]
    func memberStatus(slug: String) async -> CoaxMember?
    func connectMember(slug: String, transport: String) async throws -> CoaxMember
    func disconnectMember(slug: String) async
    func tellMember(slug: String, method: String, payloadJSON: Data) async throws -> Data
}

@MainActor
public protocol OrganismState: HolonManager {
    var coaxMembers: [CoaxMember] { get }
    func connectCoaxMember(slug: String, transport: String) async throws -> CoaxMember
    func disconnectCoaxMember(slug: String) async
    func tellCoaxMember(slug: String, method: String, payloadJSON: Data) async throws -> Data
}

public extension OrganismState {
    func listMembers() async -> [CoaxMember] {
        coaxMembers
    }

    func memberStatus(slug: String) async -> CoaxMember? {
        coaxMembers.first(where: { $0.slug == slug })
    }

    func connectMember(slug: String, transport: String) async throws -> CoaxMember {
        try await connectCoaxMember(slug: slug, transport: transport)
    }

    func disconnectMember(slug: String) async {
        await disconnectCoaxMember(slug: slug)
    }

    func tellMember(slug: String, method: String, payloadJSON: Data) async throws -> Data {
        try await tellCoaxMember(slug: slug, method: method, payloadJSON: payloadJSON)
    }
}
