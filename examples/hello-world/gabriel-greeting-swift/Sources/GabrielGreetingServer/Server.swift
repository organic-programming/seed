import Dispatch
import Foundation
import GabrielGreeting
import GRPC
import Holons
import NIO

public final class GreetingServiceProvider: Greeting_V1_GreetingServiceProvider {
    public let interceptors: Greeting_V1_GreetingServiceServerInterceptorFactoryProtocol? = nil

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
        let start = DispatchTime.now().uptimeNanoseconds
        let response = GreetingAPI.sayHello(request)
        let durationNS = DispatchTime.now().uptimeNanoseconds &- start
        // Swift Serve does not yet expose a handler-visible current transport.
        let transport = "unknown"
        let name = resolvedGreetingName(request: request, response: response)

        emitGreetingObservability(
            response: response,
            name: name,
            transport: transport,
            durationNS: durationNS
        )

        return context.eventLoop.makeSucceededFuture(response)
    }
}

private let nameMarker = "__holons_name_marker__"

private func resolvedGreetingName(
    request: Greeting_V1_SayHelloRequest,
    response: Greeting_V1_SayHelloResponse
) -> String {
    let trimmedName = request.name.trimmingCharacters(in: .whitespacesAndNewlines)
    if !trimmedName.isEmpty {
        return trimmedName
    }

    var markerRequest = Greeting_V1_SayHelloRequest()
    markerRequest.name = nameMarker
    markerRequest.langCode = response.langCode
    return extractName(
        from: response.greeting,
        markerGreeting: GreetingAPI.sayHello(markerRequest).greeting
    )
}

private func extractName(from greeting: String, markerGreeting: String) -> String {
    guard let markerRange = markerGreeting.range(of: nameMarker) else {
        return greeting
    }

    let prefix = String(markerGreeting[..<markerRange.lowerBound])
    let suffix = String(markerGreeting[markerRange.upperBound...])
    var name = greeting
    if !prefix.isEmpty {
        guard name.hasPrefix(prefix) else {
            return greeting
        }
        name.removeFirst(prefix.count)
    }
    if !suffix.isEmpty {
        guard name.hasSuffix(suffix) else {
            return greeting
        }
        name.removeLast(suffix.count)
    }
    return name
}

private func emitGreetingObservability(
    response: Greeting_V1_SayHelloResponse,
    name: String,
    transport: String,
    durationNS: UInt64
) {
    let obs = current()
    let message = "Greeted \(name) in \(response.language) (\(response.langCode))"
    obs.logger("greeting").info(message, [
        "lang_code": response.langCode,
        "language": response.language,
        "name": name,
        "greeting": response.greeting,
        "transport": transport,
        "duration_ns": String(durationNS),
    ])
    obs.counter(
        "greeting_emitted_total",
        help: "Greetings emitted, partitioned by language and transport.",
        labels: [
            "lang_code": response.langCode,
            "language": response.language,
            "transport": transport,
        ]
    )?.inc()
}

public func listenAndServe(_ listenURI: String, reflect: Bool = false) throws {
    try Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    try Serve.runWithOptions(
        listenURI,
        serviceProviders: [GreetingServiceProvider()],
        options: Serve.Options(reflect: reflect)
    )
}
