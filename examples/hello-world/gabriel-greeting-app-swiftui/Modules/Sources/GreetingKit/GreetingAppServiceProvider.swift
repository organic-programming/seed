import GRPC
import NIOCore
import SwiftProtobuf

// GreetingAppServiceProvider implements the COAX domain surface
// (greeting.v1.GreetingAppService) for the Gabriel Greeting App.
// These are the same actions a human performs through the UI,
// expressed as RPCs an agent can call equivalently.
final class GreetingAppServiceProvider: CallHandlerProvider, @unchecked Sendable {
    let serviceName: Substring = "greeting.v1.GreetingAppService"

    private let holon: HolonProcess

    init(holon: HolonProcess) {
        self.holon = holon
    }

    func handle(method name: Substring, context: CallHandlerContext) -> GRPCServerHandlerProtocol? {
        switch name {
        case "SelectHolon":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_SelectHolonRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_SelectHolonResponse>(),
                interceptors: [],
                userFunction: selectHolon(request:context:)
            )
        case "SelectTransport":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_SelectTransportRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_SelectTransportResponse>(),
                interceptors: [],
                userFunction: selectTransport(request:context:)
            )
        case "SelectLanguage":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_SelectLanguageRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_SelectLanguageResponse>(),
                interceptors: [],
                userFunction: selectLanguage(request:context:)
            )
        case "Greet":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_GreetRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_GreetResponse>(),
                interceptors: [],
                userFunction: greet(request:context:)
            )
        default:
            return nil
        }
    }

    // MARK: - RPC Implementations

    private func selectHolon(
        request: Greeting_V1_SelectHolonRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_SelectHolonResponse> {
        let promise = context.eventLoop.makePromise(of: Greeting_V1_SelectHolonResponse.self)
        Task { @MainActor [holon] in
            guard let identity = holon.availableHolons.first(where: { $0.slug == request.slug }) else {
                promise.fail(GRPCStatus(code: .notFound, message: "Holon '\(request.slug)' not found"))
                return
            }
            holon.selectedHolon = identity
            await holon.start()
            var response = Greeting_V1_SelectHolonResponse()
            response.slug = identity.slug
            response.displayName = identity.displayName
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func selectTransport(
        request: Greeting_V1_SelectTransportRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_SelectTransportResponse> {
        let promise = context.eventLoop.makePromise(of: Greeting_V1_SelectTransportResponse.self)
        Task { @MainActor [holon] in
            guard let transport = GreetingTransportName.validatedRPCName(request.transport) else {
                promise.fail(
                    GRPCStatus(
                        code: .invalidArgument,
                        message: "Unsupported transport '\(request.transport)'. Expected one of: stdio, tcp, unix"
                    )
                )
                return
            }
            holon.transport = transport.rawValue
            await holon.start()
            if let connectionError = holon.connectionError {
                promise.fail(
                    GRPCStatus(
                        code: .unavailable,
                        message: connectionError
                    )
                )
                return
            }
            guard holon.isRunning else {
                promise.fail(
                    GRPCStatus(
                        code: .unavailable,
                        message: "Holon did not become ready"
                    )
                )
                return
            }
            var response = Greeting_V1_SelectTransportResponse()
            response.transport = transport.rawValue
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func selectLanguage(
        request: Greeting_V1_SelectLanguageRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_SelectLanguageResponse> {
        let promise = context.eventLoop.makePromise(of: Greeting_V1_SelectLanguageResponse.self)
        Task { @MainActor [holon] in
            holon.selectedLanguageCode = request.code
            var response = Greeting_V1_SelectLanguageResponse()
            response.code = request.code
            promise.succeed(response)
        }
        return promise.futureResult
    }

    private func greet(
        request: Greeting_V1_GreetRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_GreetResponse> {
        let promise = context.eventLoop.makePromise(of: Greeting_V1_GreetResponse.self)
        Task { @MainActor [holon] in
            do {
                if !request.name.isEmpty {
                    holon.userName = request.name
                }
                if !request.langCode.isEmpty {
                    holon.selectedLanguageCode = request.langCode
                }

                let name = request.name.isEmpty ? holon.userName : request.name
                let langCode = holon.selectedLanguageCode
                guard !langCode.isEmpty else {
                    promise.fail(GRPCStatus(code: .invalidArgument, message: "No language selected"))
                    return
                }
                let greeting = try await holon.sayHello(name: name, langCode: langCode)
                holon.greeting = greeting
                var response = Greeting_V1_GreetResponse()
                response.greeting = greeting
                promise.succeed(response)
            } catch {
                promise.fail(GRPCStatus(code: .unavailable, message: error.localizedDescription))
            }
        }
        return promise.futureResult
    }
}
