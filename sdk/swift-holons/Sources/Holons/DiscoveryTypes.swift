import GRPC

public let LOCAL = 0
public let PROXY = 1
public let DELEGATED = 2

public let SIBLINGS = 0x01
public let CWD = 0x02
public let SOURCE = 0x04
public let BUILT = 0x08
public let INSTALLED = 0x10
public let CACHED = 0x20
public let ALL = 0x3F

public let NO_LIMIT = 0
public let NO_TIMEOUT = 0

public struct IdentityInfo: Codable, Equatable {
  public let givenName: String
  public let familyName: String
  public let motto: String
  public let aliases: [String]

  private enum CodingKeys: String, CodingKey {
    case givenName = "given_name"
    case familyName = "family_name"
    case motto
    case aliases
  }

  public init(
    givenName: String = "",
    familyName: String = "",
    motto: String = "",
    aliases: [String] = []
  ) {
    self.givenName = givenName
    self.familyName = familyName
    self.motto = motto
    self.aliases = aliases
  }

  public init(from decoder: Decoder) throws {
    let container = try decoder.container(keyedBy: CodingKeys.self)
    self.init(
      givenName: try container.decodeIfPresent(String.self, forKey: .givenName) ?? "",
      familyName: try container.decodeIfPresent(String.self, forKey: .familyName) ?? "",
      motto: try container.decodeIfPresent(String.self, forKey: .motto) ?? "",
      aliases: try container.decodeIfPresent([String].self, forKey: .aliases) ?? []
    )
  }
}

public struct HolonInfo: Codable, Equatable {
  public let slug: String
  public let uuid: String
  public let identity: IdentityInfo
  public let lang: String
  public let runner: String
  public let status: String
  public let kind: String
  public let transport: String
  public let entrypoint: String
  public let architectures: [String]
  public let hasDist: Bool
  public let hasSource: Bool

  private enum CodingKeys: String, CodingKey {
    case slug
    case uuid
    case identity
    case lang
    case runner
    case status
    case kind
    case transport
    case entrypoint
    case architectures
    case hasDist = "has_dist"
    case hasSource = "has_source"
  }

  public init(
    slug: String = "",
    uuid: String = "",
    identity: IdentityInfo = IdentityInfo(),
    lang: String = "",
    runner: String = "",
    status: String = "",
    kind: String = "",
    transport: String = "",
    entrypoint: String = "",
    architectures: [String] = [],
    hasDist: Bool = false,
    hasSource: Bool = false
  ) {
    self.slug = slug
    self.uuid = uuid
    self.identity = identity
    self.lang = lang
    self.runner = runner
    self.status = status
    self.kind = kind
    self.transport = transport
    self.entrypoint = entrypoint
    self.architectures = architectures
    self.hasDist = hasDist
    self.hasSource = hasSource
  }

  public init(from decoder: Decoder) throws {
    let container = try decoder.container(keyedBy: CodingKeys.self)
    self.init(
      slug: try container.decodeIfPresent(String.self, forKey: .slug) ?? "",
      uuid: try container.decodeIfPresent(String.self, forKey: .uuid) ?? "",
      identity: try container.decodeIfPresent(IdentityInfo.self, forKey: .identity) ?? IdentityInfo(),
      lang: try container.decodeIfPresent(String.self, forKey: .lang) ?? "",
      runner: try container.decodeIfPresent(String.self, forKey: .runner) ?? "",
      status: try container.decodeIfPresent(String.self, forKey: .status) ?? "",
      kind: try container.decodeIfPresent(String.self, forKey: .kind) ?? "",
      transport: try container.decodeIfPresent(String.self, forKey: .transport) ?? "",
      entrypoint: try container.decodeIfPresent(String.self, forKey: .entrypoint) ?? "",
      architectures: try container.decodeIfPresent([String].self, forKey: .architectures) ?? [],
      hasDist: try container.decodeIfPresent(Bool.self, forKey: .hasDist) ?? false,
      hasSource: try container.decodeIfPresent(Bool.self, forKey: .hasSource) ?? false
    )
  }
}

public struct HolonRef: Equatable {
  public let url: String
  public let info: HolonInfo?
  public let error: String?

  public init(url: String, info: HolonInfo? = nil, error: String? = nil) {
    self.url = url
    self.info = info
    self.error = error
  }
}

public struct DiscoverResult: Equatable {
  public let found: [HolonRef]
  public let error: String?

  public init(found: [HolonRef] = [], error: String? = nil) {
    self.found = found
    self.error = error
  }
}

public struct ResolveResult: Equatable {
  public let ref: HolonRef?
  public let error: String?

  public init(ref: HolonRef? = nil, error: String? = nil) {
    self.ref = ref
    self.error = error
  }
}

public struct ConnectResult {
  public let channel: GRPCChannel?
  public let uid: String
  public let origin: HolonRef?
  public let error: String?

  public init(
    channel: GRPCChannel? = nil,
    uid: String = "",
    origin: HolonRef? = nil,
    error: String? = nil
  ) {
    self.channel = channel
    self.uid = uid
    self.origin = origin
    self.error = error
  }
}
