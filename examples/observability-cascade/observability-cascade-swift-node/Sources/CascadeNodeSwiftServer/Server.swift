import CascadeNodeSwift
import GRPC
import Holons
import NIO

public final class RelayServiceProvider: Relay_V1_RelayServiceProvider {
    public let interceptors: Relay_V1_RelayServiceServerInterceptorFactoryProtocol? = nil

    public init() {}

    public func tick(
        request: Relay_V1_TickRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Relay_V1_TickResponse> {
        context.eventLoop.makeSucceededFuture(PublicAPI.tick(request))
    }
}

public func listenAndServe(_ listenURI: String, reflect: Bool = false, members: [Serve.MemberRef] = []) throws {
    try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    try Serve.runWithOptions(
        normalizedListenURI(listenURI),
        serviceProviders: [RelayServiceProvider()],
        options: Serve.Options(
            reflect: reflect,
            slug: "observability-cascade-swift-node",
            memberEndpoints: members
        )
    )
}

private func normalizedListenURI(_ listenURI: String) -> String {
    if listenURI.hasPrefix("tcp://:") {
        return "tcp://0.0.0.0:\(listenURI.dropFirst("tcp://:".count))"
    }
    return listenURI
}
