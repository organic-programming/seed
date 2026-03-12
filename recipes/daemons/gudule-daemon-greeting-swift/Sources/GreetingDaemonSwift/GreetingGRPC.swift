import GRPC
import GreetingGenerated
import NIOCore

protocol Greeting_V1_GreetingServiceProvider: CallHandlerProvider {
    var interceptors: Greeting_V1_GreetingServiceServerInterceptorFactoryProtocol? { get }

    func listLanguages(
        request: Greeting_V1_ListLanguagesRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_ListLanguagesResponse>

    func sayHello(
        request: Greeting_V1_SayHelloRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_SayHelloResponse>
}

extension Greeting_V1_GreetingServiceProvider {
    var serviceName: Substring {
        Greeting_V1_GreetingServiceServerMetadata.serviceDescriptor.fullName[...]
    }

    var interceptors: Greeting_V1_GreetingServiceServerInterceptorFactoryProtocol? {
        nil
    }

    func handle(
        method name: Substring,
        context: CallHandlerContext
    ) -> GRPCServerHandlerProtocol? {
        switch name {
        case "ListLanguages":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_ListLanguagesRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_ListLanguagesResponse>(),
                interceptors: interceptors?.makeListLanguagesInterceptors() ?? [],
                userFunction: listLanguages(request:context:)
            )
        case "SayHello":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_SayHelloRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_SayHelloResponse>(),
                interceptors: interceptors?.makeSayHelloInterceptors() ?? [],
                userFunction: sayHello(request:context:)
            )
        default:
            return nil
        }
    }
}

protocol Greeting_V1_GreetingServiceServerInterceptorFactoryProtocol {
    func makeListLanguagesInterceptors() -> [ServerInterceptor<Greeting_V1_ListLanguagesRequest, Greeting_V1_ListLanguagesResponse>]
    func makeSayHelloInterceptors() -> [ServerInterceptor<Greeting_V1_SayHelloRequest, Greeting_V1_SayHelloResponse>]
}

enum Greeting_V1_GreetingServiceServerMetadata {
    static let serviceDescriptor = GRPCServiceDescriptor(
        name: "GreetingService",
        fullName: "greeting.v1.GreetingService",
        methods: [
            Methods.listLanguages,
            Methods.sayHello,
        ]
    )

    enum Methods {
        static let listLanguages = GRPCMethodDescriptor(
            name: "ListLanguages",
            path: "/greeting.v1.GreetingService/ListLanguages",
            type: .unary
        )

        static let sayHello = GRPCMethodDescriptor(
            name: "SayHello",
            path: "/greeting.v1.GreetingService/SayHello",
            type: .unary
        )
    }
}

final class GreetingServiceProvider: Greeting_V1_GreetingServiceProvider {
    private let service = GreetingService()

    func listLanguages(
        request: Greeting_V1_ListLanguagesRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_ListLanguagesResponse> {
        _ = request
        return context.eventLoop.makeSucceededFuture(service.listLanguages())
    }

    func sayHello(
        request: Greeting_V1_SayHelloRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Greeting_V1_SayHelloResponse> {
        context.eventLoop.makeSucceededFuture(
            service.sayHello(name: request.name, langCode: request.langCode)
        )
    }
}
