#if os(macOS)
  import Foundation
  import GRPC
  import Holons
  import func Holons.connect

  public protocol HolonConnector<Holon> {
    associatedtype Holon
    func connect(_ holon: Holon, transport: String) throws -> GRPCChannel
  }

  public final class BundledHolonConnector<Holon>: HolonConnector {
    private let slugOf: (Holon) -> String
    private let buildRunnerOf: ((Holon) -> String)?

    public init(
      slugOf: @escaping (Holon) -> String,
      buildRunnerOf: ((Holon) -> String)? = nil
    ) {
      self.slugOf = slugOf
      self.buildRunnerOf = buildRunnerOf
    }

    public func connect(_ holon: Holon, transport: String) throws -> GRPCChannel {
      var options = ConnectOptions()
      options.transport = effectiveHolonTransport(
        requestedTransport: transport,
        buildRunner: buildRunnerOf?(holon)
      )
      options.lifecycle = "ephemeral"
      options.timeout = 5.0
      return try sdkConnect(slugOf(holon), options: options)
    }
  }

  public func effectiveHolonTransport(
    requestedTransport: String,
    buildRunner: String? = nil
  ) -> String {
    let normalized = HolonTransportName.normalize(requestedTransport).rawValue
    guard isBundledMacOSHost() else {
      return normalized
    }
    if normalized == HolonTransportName.unix.rawValue {
      return HolonTransportName.tcp.rawValue
    }
    if normalized == HolonTransportName.stdio.rawValue,
      buildRunner == "cmake"
    {
      return HolonTransportName.tcp.rawValue
    }
    return normalized
  }

  private func sdkConnect(_ target: String, options: ConnectOptions) throws -> GRPCChannel {
    try connect(target, options: options)
  }

  private func isBundledMacOSHost() -> Bool {
    Bundle.main.bundleURL.path.contains(".app/Contents/")
  }
#endif
