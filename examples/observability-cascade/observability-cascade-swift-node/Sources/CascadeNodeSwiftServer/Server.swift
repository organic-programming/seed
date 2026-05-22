import CascadeNodeSwift
import GRPC
import Holons

public func listenAndServe(_ listenURI: String, transport: String = "stdio", children: [ChildSpec] = []) throws {
    try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    _ = try fromEnv(ObsConfig(slug: "observability-cascade-swift-node"))
    let downstream: SpawnedMember?
    if let first = children.first {
        downstream = try Composite.spawnMember(SpawnOptions(
            slug: first.slug,
            binaryPath: first.binary,
            transport: transport,
            downstreamChain: Array(children.dropFirst())
        ))
    } else {
        downstream = nil
    }
    defer { downstream?.stop() }

    try Serve.runWithOptions(
        normalizedListenURI(listenURI),
        serviceProviders: [canonicalRelayServiceProvider(downstream: downstream?.channel)],
        options: Serve.Options(
            reflect: false,
            slug: "observability-cascade-swift-node"
        )
    )
}

private func normalizedListenURI(_ listenURI: String) -> String {
    if listenURI.hasPrefix("tcp://:") {
        return "tcp://0.0.0.0:\(listenURI.dropFirst("tcp://:".count))"
    }
    return listenURI
}
