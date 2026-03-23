import GabrielGreeting
import GRPC
import Holons
import NIO

public final class GreetingServiceProvider: Greeting_V1_GreetingServiceProvider {
    public init() {}

    public func listLanguages(
        request: Greeting_V1_ListLanguagesRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_ListLanguagesResponse> {
        _ = request
        return context.eventLoop.makeSucceededFuture(GreetingAPI.listLanguages())
    }

    public func sayHello(
        request: Greeting_V1_SayHelloRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_SayHelloResponse> {
        context.eventLoop.makeSucceededFuture(GreetingAPI.sayHello(request))
    }
}

public func listenAndServe(_ listenURI: String, reflect: Bool = false) throws {
    try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    try Serve.runWithOptions(
        listenURI,
        serviceProviders: [GreetingServiceProvider()],
        options: Serve.Options(reflect: reflect)
    )
}
