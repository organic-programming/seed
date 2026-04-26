import Combine
import Foundation
import HolonsApp

#if os(macOS)
  import Holons
#endif

@MainActor
public final class GreetingHolonManager: ObservableObject {
  public static let listLanguagesMethod = "/greeting.v1.GreetingService/ListLanguages"
  public static let sayHelloMethod = "/greeting.v1.GreetingService/SayHello"

  @Published public var isRunning = false
  @Published public var connectionError: String?
  @Published public var availableHolons: [GabrielHolonIdentity] = []
  @Published public var selectedHolon: GabrielHolonIdentity? = nil {
    didSet {
      guard oldValue != selectedHolon else { return }
      stop()
    }
  }
  @Published public var selectedLanguageCode: String = ""
  @Published public var availableLanguages: [Greeting_V1_Language] = []
  @Published public var userName: String = ""
  @Published public var greeting: String = ""
  @Published public var transport: String = {
    HolonTransportName.normalize(
      ProcessInfo.processInfo.environment["OP_ASSEMBLY_TRANSPORT"]
    ).rawValue
  }()
  {
    didSet {
      guard oldValue != transport else { return }
      stop()
    }
  }

  private var client: GreetingClientProtocol?
  private var startTask: Task<GreetingClientProtocol, Error>?
  private var startTaskID: UUID?
  #if os(macOS)
    private let holons: BundledHolons<GabrielHolonIdentity>
    private let clientFactory: GreetingClientFactory
    private var observability: Observability?
    private var logger: HolonLogger?
    private var sayHelloTotal: Counter?
    private var sayHelloDuration: Histogram?
  #endif

  #if os(macOS)
    public init(
      holons: BundledHolons<GabrielHolonIdentity> = BundledHolons<GabrielHolonIdentity>(
        fromDiscovered: GabrielHolonIdentity.fromDiscovered,
        slugOf: { $0.slug },
        sortRankOf: { $0.sortRank },
        displayNameOf: { $0.displayName }
      )
    ) {
      self.holons = holons
      self.clientFactory = connectClient
      refreshHolons()
    }

    init(
      holons: BundledHolons<GabrielHolonIdentity> = BundledHolons<GabrielHolonIdentity>(
        fromDiscovered: GabrielHolonIdentity.fromDiscovered,
        slugOf: { $0.slug },
        sortRankOf: { $0.sortRank },
        displayNameOf: { $0.displayName }
      ),
      clientFactory: @escaping GreetingClientFactory,
      autoRefresh: Bool = true
    ) {
      self.holons = holons
      self.clientFactory = clientFactory
      if autoRefresh {
        refreshHolons()
      }
    }
  #else
    public init() {
    }
  #endif

  #if os(macOS)
    public func attachObservability(_ observability: Observability) {
      self.observability = observability
      self.logger = observability.logger("greeting-controller")
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
  #endif

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
      #if os(macOS)
        if delay > 0 {
          try await Task.sleep(nanoseconds: delay)
        }
      #endif
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
    #if os(macOS)
      if availableHolons.isEmpty {
        refreshHolons()
      }
    #endif
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

    if client == nil {
      await start()
    }
    guard client != nil else {
      throw connectionFailure(HolonError.notConnected)
    }

    #if os(macOS)
      let startedAt = Date()
      logger?.info("Greeting request started", [
        "method": Self.sayHelloMethod,
        "lang": selectedLanguageCode,
        "holon": selectedHolon?.slug ?? "",
      ])
    #endif
    do {
      let response = try await sayHello(name: userName, langCode: selectedLanguageCode)
      greeting = response
      #if os(macOS)
        sayHelloTotal?.inc()
        sayHelloDuration?.observe(Date().timeIntervalSince(startedAt))
        logger?.info("Greeting response received", [
          "method": Self.sayHelloMethod,
          "lang": selectedLanguageCode,
          "holon": selectedHolon?.slug ?? "",
        ])
      #endif
      return response
    } catch {
      #if os(macOS)
        logger?.error("Greeting request failed", [
          "method": Self.sayHelloMethod,
          "lang": selectedLanguageCode,
          "holon": selectedHolon?.slug ?? "",
          "error": error.localizedDescription,
        ])
      #endif
      throw error
    }
  }

  public func start() async {
    guard client == nil else { return }
    if let startTask {
      do {
        _ = try await startTask.value
      } catch {
        if connectionError == nil {
          connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
        }
        isRunning = false
      }
      return
    }
    connectionError = nil

    #if os(macOS)
      do {
        if availableHolons.isEmpty {
          refreshHolons()
        }
        guard let holon = selectedHolon ?? preferredHolon(in: availableHolons) else {
          throw GreetingSelectionError.noHolonsFound
        }

        if selectedHolon != holon {
          selectedHolon = holon
        }

        logHostUI(
          "[HostUI] assembly=\(assemblyFamily) holon=\(holon.binaryName) transport=\(transport)")

        var options = ConnectOptions()
        options.transport = transport
        options.lifecycle = "ephemeral"
        options.timeout = 5.0

        let taskID = UUID()
        startTaskID = taskID
        let factory = clientFactory
        let connectTask = Task.detached(priority: .userInitiated) { [slug = holon.slug, options] in
          try factory(slug, options)
        }
        startTask = connectTask

        do {
          let connectedClient = try await connectTask.value
          guard startTaskID == taskID else {
            try? connectedClient.close()
            return
          }
          client = connectedClient
          logHostUI("[HostUI] connected to \(holon.binaryName) on \(connectionTarget())")
          isRunning = true
        } catch {
          guard startTaskID == taskID else {
            return
          }
          connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
          isRunning = false
        }

        if startTaskID == taskID {
          startTask = nil
          startTaskID = nil
        }
      } catch {
        connectionError = "Failed to start Gabriel holon: \(String(describing: error))"
        isRunning = false
      }
    #else
      connectionError = GreetingClientError.unsupportedPlatform.localizedDescription
      isRunning = false
    #endif
  }

  public func stop() {
    startTaskID = nil
    startTask?.cancel()
    startTask = nil

    let currentClient = client
    client = nil

    do {
      try currentClient?.close()
    } catch {
      connectionError = "Failed to stop Gabriel holon connection: \(error.localizedDescription)"
    }

    #if os(macOS)
    #endif
    isRunning = false
  }

  public func listLanguages() async throws -> [Greeting_V1_Language] {
    if client == nil { await start() }
    guard let client else {
      throw HolonError.notConnected
    }
    return try await client.listLanguages()
  }

  public func sayHello(name: String, langCode: String) async throws -> String {
    guard let client else {
      throw HolonError.notConnected
    }
    return try await client.sayHello(name: name, langCode: langCode)
  }

  public func tellMember(
    slug: String,
    method: String,
    payloadJSON: Data
  ) async throws -> Data {
    let canonicalMethod = canonicalGRPCMethodPath(method)
    let normalizedPayload = payloadJSON.isEmpty ? Data("{}".utf8) : payloadJSON

    #if os(macOS)
      if availableHolons.isEmpty {
        refreshHolons()
      }
    #endif

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
      if !request.name.isEmpty {
        userName = request.name
      }
      if !request.langCode.isEmpty {
        _ = try await selectLanguage(request.langCode)
      }
      let greeting = try await greetCurrentSelection(
        name: request.name.isEmpty ? nil : request.name,
        langCode: request.langCode.isEmpty ? nil : request.langCode
      )
      var response = Greeting_V1_SayHelloResponse()
      response.greeting = greeting
      return try response.jsonUTF8Data()

    default:
      if client == nil {
        await start()
      }
      guard let client else {
        throw connectionFailure(HolonError.notConnected)
      }
      return try await client.tell(method: canonicalMethod, payloadJSON: normalizedPayload)
    }
  }

  deinit {
    try? client?.close()
  }

  public var assemblyFamily: String {
    let value = ProcessInfo.processInfo.environment["OP_ASSEMBLY_FAMILY"]?
      .trimmingCharacters(in: .whitespacesAndNewlines)
    return value?.isEmpty == false ? value! : "Gabriel-Greeting-App-SwiftUI"
  }

  public var holonBinaryName: String {
    selectedHolon?.binaryName ?? "gabriel-greeting-swift"
  }

  private var languageLoadRetryDelays: [UInt64] {
    HolonTransportName.normalize(transport) == .stdio
      ? [0, 80_000_000, 180_000_000]
      : [120_000_000, 300_000_000, 600_000_000]
  }

  private func connectionTarget() -> String {
    HolonTransportName.normalize(transport).rawValue
  }

  private func connectionFailure(_ fallback: Error?) -> Error {
    if let connectionError, !connectionError.isEmpty {
      return NSError(
        domain: "GreetingKit.GreetingHolonManager",
        code: 1,
        userInfo: [NSLocalizedDescriptionKey: connectionError]
      )
    }
    return fallback ?? HolonError.notConnected
  }

  private func logHostUI(_ line: String) {
    guard let data = (line + "\n").data(using: .utf8) else {
      return
    }
    FileHandle.standardError.write(data)
  }
}

@available(*, deprecated, renamed: "GreetingHolonManager")
public typealias HolonProcess = GreetingHolonManager

#if os(macOS)
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

      await start()
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
#endif

protocol GreetingClientProtocol: AnyObject, Sendable {
  func listLanguages() async throws -> [Greeting_V1_Language]
  func sayHello(name: String, langCode: String) async throws -> String
  func tell(method: String, payloadJSON: Data) async throws -> Data
  func close() throws
}

extension GreetingClient: GreetingClientProtocol {}

typealias GreetingClientFactory = @Sendable (String, ConnectOptions) throws -> GreetingClientProtocol

private let connectClientLock = NSLock()

private func connectClient(
  holonSlug: String,
  options: ConnectOptions
) throws -> GreetingClientProtocol {
  connectClientLock.lock()
  defer { connectClientLock.unlock() }
  return try GreetingClient.connected(to: holonSlug, options: options)
}

public enum HolonError: LocalizedError {
  case notConnected

  public var errorDescription: String? {
    switch self {
    case .notConnected:
      return "Not connected to the Gabriel greeting holon"
    }
  }
}

public struct GabrielHolonIdentity: Identifiable, Hashable {
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
  extension GreetingHolonManager {
    fileprivate func preferredHolon(in holons: [GabrielHolonIdentity]) -> GabrielHolonIdentity? {
      holons.sorted(by: holonSort).first
    }

    fileprivate func holonSort(_ lhs: GabrielHolonIdentity, _ rhs: GabrielHolonIdentity) -> Bool {
      if lhs.sortRank != rhs.sortRank {
        return lhs.sortRank < rhs.sortRank
      }
      return lhs.displayName.localizedCaseInsensitiveCompare(rhs.displayName) == .orderedAscending
    }

    fileprivate func refreshHolons() {
      let previousSelection = selectedHolon?.slug

      do {
        let results = try holons.list()

        availableHolons = results
        if let previousSelection,
          let holon = availableHolons.first(where: { $0.slug == previousSelection })
        {
          selectedHolon = holon
        } else {
          selectedHolon = preferredHolon(in: availableHolons)
        }
      } catch {
        availableHolons = []
        selectedHolon = nil
        connectionError = "Failed to discover Gabriel holons: \(error.localizedDescription)"
      }
    }
  }
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
