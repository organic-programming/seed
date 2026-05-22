import Combine
import Foundation
import HolonsApp

#if os(macOS)
  import Holons
#endif

#if os(macOS)
  @MainActor
  public final class GreetingHolonManager:
    HolonProcessManager<GabrielHolonIdentity, any GreetingClientProtocol>
  {
    public static let listLanguagesMethod = "/greeting.v1.GreetingService/ListLanguages"
    public static let sayHelloMethod = "/greeting.v1.GreetingService/SayHello"

    @Published public var selectedLanguageCode: String = ""
    @Published public var availableLanguages: [Greeting_V1_Language] = []
    @Published public var userName: String = ""
    @Published public var greeting: String = ""

    private var sayHelloTotal: Counter?
    private var sayHelloDuration: Histogram?

    public convenience init() {
      self.init(
        holons: Self.defaultHolons(),
        clientFactory: connectClient,
        autoRefresh: true
      )
    }

    init(
      holons: any HolonsApp.Holons<GabrielHolonIdentity> = GreetingHolonManager.defaultHolons(),
      clientFactory: @escaping GreetingClientFactory,
      autoRefresh: Bool = true
    ) {
      super.init(
        holons: holons,
        clientFactory: clientFactory,
        closeClient: { try $0.close() },
        slugOf: { $0.slug },
        displayNameOf: { $0.displayName },
        sortRankOf: { $0.sortRank },
        connectionName: "Gabriel holon",
        noHolonsError: { GreetingSelectionError.noHolonsFound },
        notConnectedError: { HolonError.notConnected },
        autoRefresh: autoRefresh
      )
    }

    public override var observabilityLoggerName: String {
      "greeting-controller"
    }

    public override func attachObservability(_ observability: Observability) {
      super.attachObservability(observability)
      self.sayHelloTotal = observability.counter(
        "gabriel_greeting_say_hello_total",
        help: "Greeting requests sent from the SwiftUI app",
        labels: ["origin": "app"]
      )
      self.sayHelloDuration = observability.histogram(
        "gabriel_greeting_say_hello_duration_seconds",
        help: "Greeting request duration observed by the SwiftUI app",
        labels: ["origin": "app"]
      )
    }

    @discardableResult
    public func reloadLanguages(greetAfterLoad: Bool = false) async throws -> [Greeting_V1_Language] {
      greeting = ""
      availableLanguages = []

      await start()
      guard isRunning else {
        throw connectionFailure(nil)
      }

      var lastError: Error?
      for delay in languageLoadRetryDelays {
        if delay > 0 {
          try await Task.sleep(nanoseconds: delay)
        }
        do {
          let languages = try await listLanguages()
          availableLanguages = languages
          selectedLanguageCode = resolvedLanguageSelection(
            availableLanguages: languages,
            preferredCode: selectedLanguageCode
          )
          if greetAfterLoad, !selectedLanguageCode.isEmpty {
            _ = try await greetCurrentSelection()
          }
          return languages
        } catch {
          lastError = error
        }
      }

      throw connectionFailure(lastError)
    }

    @discardableResult
    public func selectHolon(
      slug: String,
      greetAfterLoad: Bool = false
    ) async throws -> GabrielHolonIdentity {
      if availableHolons.isEmpty {
        refreshHolons()
      }
      let identity = try resolvedHolonSelection(slug: slug, availableHolons: availableHolons)
      if selectedHolon != identity {
        selectedHolon = identity
      }
      try await reloadLanguages(greetAfterLoad: greetAfterLoad)
      return identity
    }

    @discardableResult
    public func selectTransport(
      _ value: String,
      greetAfterLoad: Bool = false
    ) async throws -> String {
      let transport = try validatedTransportSelection(value)
      if self.transport != transport.rawValue {
        self.transport = transport.rawValue
      }
      try await reloadLanguages(greetAfterLoad: greetAfterLoad)
      return transport.rawValue
    }

    @discardableResult
    public func selectLanguage(_ value: String) async throws -> String {
      if availableLanguages.isEmpty {
        try await reloadLanguages(greetAfterLoad: false)
      }
      let code = try validatedLanguageSelection(value, availableLanguages: availableLanguages)
      selectedLanguageCode = code
      return code
    }

    @discardableResult
    public func selectLanguageAndGreet(_ value: String) async throws -> String {
      let code = try await selectLanguage(value)
      _ = try await greetCurrentSelection()
      return code
    }

    @discardableResult
    public func greetCurrentSelection(
      name: String? = nil,
      langCode: String? = nil
    ) async throws -> String {
      if let name, !name.isEmpty {
        userName = name
      }
      if let langCode, !langCode.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
        _ = try await selectLanguage(langCode)
      } else if selectedLanguageCode.isEmpty {
        if availableLanguages.isEmpty {
          try await reloadLanguages(greetAfterLoad: false)
        }
        guard !selectedLanguageCode.isEmpty else {
          throw GreetingSelectionError.noLanguageSelected
        }
      }

      if connectedClient == nil {
        await start()
      }
      guard connectedClient != nil else {
        throw connectionFailure(HolonError.notConnected)
      }

      let startedAt = Date()
      do {
        let response = try await sayHello(name: userName, langCode: selectedLanguageCode)
        greeting = response
        sayHelloTotal?.inc()
        let elapsed = Date().timeIntervalSince(startedAt)
        sayHelloDuration?.observe(elapsed)
        processLogger?.info("Greeting request completed", [
          "method": Self.sayHelloMethod,
          "lang": selectedLanguageCode,
          "holon": selectedHolon?.slug ?? "",
          "elapsed_ms": String(format: "%.1f", elapsed * 1000),
        ])
        return response
      } catch {
        processLogger?.error("Greeting request failed", [
          "method": Self.sayHelloMethod,
          "lang": selectedLanguageCode,
          "holon": selectedHolon?.slug ?? "",
          "error": error.localizedDescription,
        ])
        throw error
      }
    }

    public func listLanguages() async throws -> [Greeting_V1_Language] {
      if connectedClient == nil { await start() }
      guard let client = connectedClient else {
        throw HolonError.notConnected
      }
      return try await client.listLanguages()
    }

    public func sayHello(name: String, langCode: String) async throws -> String {
      guard let client = connectedClient else {
        throw HolonError.notConnected
      }
      return try await client.sayHello(name: name, langCode: langCode)
    }

    public override func invokeRPC(
      on client: any GreetingClientProtocol,
      method: String,
      payload: Data
    ) async throws -> Data {
      try await client.tell(method: method, payloadJSON: payload)
    }

    public func tellMember(
      slug: String,
      method: String,
      payloadJSON: Data
    ) async throws -> Data {
      let canonicalMethod = canonicalGRPCMethodPath(method)
      let normalizedPayload = payloadJSON.isEmpty ? Data("{}".utf8) : payloadJSON

      if availableHolons.isEmpty {
        refreshHolons()
      }

      if selectedHolon?.slug != slug {
        _ = try await selectHolon(slug: slug, greetAfterLoad: false)
      }

      switch canonicalMethod {
      case Self.listLanguagesMethod:
        let languages = try await reloadLanguages(greetAfterLoad: false)
        var response = Greeting_V1_ListLanguagesResponse()
        response.languages = languages
        return try response.jsonUTF8Data()

      case Self.sayHelloMethod:
        let request = try Greeting_V1_SayHelloRequest(jsonUTF8Data: normalizedPayload)
        let greeting = try await greetCurrentSelection(
          name: request.name.isEmpty ? nil : request.name,
          langCode: request.langCode.isEmpty ? nil : request.langCode
        )
        var response = Greeting_V1_SayHelloResponse()
        response.greeting = greeting
        return try response.jsonUTF8Data()

      default:
        if connectedClient == nil {
          await start()
        }
        guard let client = connectedClient else {
          throw connectionFailure(HolonError.notConnected)
        }
        return try await invokeRPC(on: client, method: canonicalMethod, payload: normalizedPayload)
      }
    }

    private var languageLoadRetryDelays: [UInt64] {
      HolonTransportName.normalize(transport) == .stdio
        ? [0, 80_000_000, 180_000_000]
        : [120_000_000, 300_000_000, 600_000_000]
    }

    private static func defaultHolons() -> BundledHolons<GabrielHolonIdentity> {
      BundledHolons<GabrielHolonIdentity>(
        fromDiscovered: GabrielHolonIdentity.fromDiscovered,
        slugOf: { $0.slug },
        sortRankOf: { $0.sortRank },
        displayNameOf: { $0.displayName }
      )
    }
  }

  extension GreetingHolonManager: HolonManager {
    public func listMembers() async -> [CoaxMember] {
      availableHolons.map { coaxMember(for: $0) }
    }

    public func memberStatus(slug: String) async -> CoaxMember? {
      availableHolons.first(where: { $0.slug == slug }).map { coaxMember(for: $0) }
    }

    public func connectMember(slug: String, transport: String) async throws -> CoaxMember {
      if availableHolons.isEmpty {
        refreshHolons()
      }
      guard let identity = availableHolons.first(where: { $0.slug == slug }) else {
        throw GreetingSelectionError.holonNotFound(slug)
      }

      if !transport.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
        self.transport = try validatedTransportSelection(transport).rawValue
      }
      if selectedHolon != identity {
        selectedHolon = identity
      }

      try await reloadLanguages(greetAfterLoad: true)
      return coaxMember(
        for: identity,
        overrideState: isRunning ? .connected : .error
      )
    }

    public func disconnectMember(slug: String) async {
      if !slug.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
        selectedHolon?.slug != slug
      {
        return
      }
      stop()
    }

    private func coaxMember(
      for identity: GabrielHolonIdentity,
      overrideState: CoaxMemberState? = nil
    ) -> CoaxMember {
      CoaxMember(
        slug: identity.slug,
        familyName: identity.familyName,
        displayName: identity.displayName,
        state: overrideState ?? memberState(for: identity)
      )
    }

    private func memberState(
      for identity: GabrielHolonIdentity
    ) -> CoaxMemberState {
      if selectedHolon?.slug == identity.slug && isRunning {
        return .connected
      }
      return .available
    }
  }
#else
  @MainActor
  public final class GreetingHolonManager: ObservableObject {
    @Published public var isRunning = false
    @Published public var connectionError: String?
    @Published public var availableHolons: [GabrielHolonIdentity] = []
    @Published public var selectedHolon: GabrielHolonIdentity?
    @Published public var selectedLanguageCode: String = ""
    @Published public var availableLanguages: [Greeting_V1_Language] = []
    @Published public var userName: String = ""
    @Published public var greeting: String = ""
    @Published public var transport: String = HolonTransportName.stdio.rawValue

    public init() {}

    public func attachObservability(_ observability: Any) {
      _ = observability
    }

    public func stop() {}
  }
#endif

@available(*, deprecated, renamed: "GreetingHolonManager")
public typealias HolonProcess = GreetingHolonManager

public protocol GreetingClientProtocol: AnyObject, Sendable {
  func listLanguages() async throws -> [Greeting_V1_Language]
  func sayHello(name: String, langCode: String) async throws -> String
  func tell(method: String, payloadJSON: Data) async throws -> Data
  func close() throws
}

extension GreetingClient: GreetingClientProtocol {}

#if os(macOS)
  typealias GreetingClientFactory = @Sendable (String, ConnectOptions) throws -> any GreetingClientProtocol

  private let connectClientLock = NSLock()

  private func connectClient(
    holonSlug: String,
    options: ConnectOptions
  ) throws -> any GreetingClientProtocol {
    connectClientLock.lock()
    defer { connectClientLock.unlock() }
    return try GreetingClient.connected(to: holonSlug, options: options)
  }
#endif

public enum HolonError: LocalizedError {
  case notConnected

  public var errorDescription: String? {
    switch self {
    case .notConnected:
      return "Not connected to the Gabriel greeting holon"
    }
  }
}

public struct GabrielHolonIdentity: Identifiable, Hashable, Sendable {
  public let slug: String
  public let familyName: String
  public let binaryName: String
  public let buildRunner: String
  public let displayName: String
  public let sortRank: Int
  public let holonUUID: String
  public let born: String
  public let sourceKind: String
  public let discoveryPath: String
  public let hasSource: Bool

  public var id: String { slug }
  public var variant: String { slug.replacingOccurrences(of: "gabriel-greeting-", with: "") }
}

#if os(macOS)
  extension GabrielHolonIdentity {
    public static func fromDiscovered(_ entry: HolonEntry) -> GabrielHolonIdentity? {
      guard entry.slug.hasPrefix("gabriel-greeting-"),
        entry.slug != "gabriel-greeting-app-swiftui"
      else {
        return nil
      }
      return GabrielHolonIdentity(entry: entry)
    }

    fileprivate init(entry: HolonEntry) {
      let runner = entry.runner.isEmpty ? (entry.manifest?.build.runner ?? "") : entry.runner
      let binaryName = {
        let candidate =
          entry.entrypoint.isEmpty
          ? (entry.manifest?.artifacts.binary ?? entry.slug) : entry.entrypoint
        return (candidate as NSString).lastPathComponent
      }()

      self.init(
        slug: entry.slug,
        familyName: entry.identity.familyName,
        binaryName: binaryName,
        buildRunner: runner,
        displayName: Self.displayName(for: entry.slug),
        sortRank: Self.sortRank(for: entry.slug),
        holonUUID: entry.uuid,
        born: entry.identity.born,
        sourceKind: entry.sourceKind,
        discoveryPath: entry.dir.path,
        hasSource: entry.hasSource
      )
    }

    fileprivate static func displayName(for slug: String) -> String {
      switch slug.replacingOccurrences(of: "gabriel-greeting-", with: "") {
      case "cpp":
        return "Gabriel (C++)"
      case "csharp":
        return "Gabriel (C#)"
      case "node":
        return "Gabriel (Node.js)"
      default:
        let variant =
          slug
          .replacingOccurrences(of: "gabriel-greeting-", with: "")
          .split(separator: "-")
          .map { $0.capitalized }
          .joined(separator: " ")
        return "Gabriel (\(variant))"
      }
    }

    fileprivate static func sortRank(for slug: String) -> Int {
      let order = [
        "gabriel-greeting-swift": 0,
        "gabriel-greeting-go": 1,
        "gabriel-greeting-rust": 2,
        "gabriel-greeting-zig": 3,
        "gabriel-greeting-python": 4,
        "gabriel-greeting-c": 5,
        "gabriel-greeting-cpp": 6,
        "gabriel-greeting-csharp": 7,
        "gabriel-greeting-dart": 8,
        "gabriel-greeting-java": 9,
        "gabriel-greeting-kotlin": 10,
        "gabriel-greeting-node": 11,
        "gabriel-greeting-ruby": 12,
      ]
      return order[slug] ?? 999
    }
  }
#endif
