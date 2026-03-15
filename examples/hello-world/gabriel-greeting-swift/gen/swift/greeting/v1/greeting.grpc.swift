//
// DO NOT EDIT.
//
// Manually authored to match grpc-swift 1.9 generated structure because
// protoc-gen-grpc-swift is not available in this workspace.
//

import GRPC
import NIO
import SwiftProtobuf

public protocol Greeting_V1_GreetingServiceClientProtocol: GRPCClient {
    var serviceName: String { get }

    func listLanguages(
        _ request: Greeting_V1_ListLanguagesRequest,
        callOptions: CallOptions?
    ) -> UnaryCall<Greeting_V1_ListLanguagesRequest, Greeting_V1_ListLanguagesResponse>

    func sayHello(
        _ request: Greeting_V1_SayHelloRequest,
        callOptions: CallOptions?
    ) -> UnaryCall<Greeting_V1_SayHelloRequest, Greeting_V1_SayHelloResponse>
}

extension Greeting_V1_GreetingServiceClientProtocol {
    public var serviceName: String {
        "greeting.v1.GreetingService"
    }

    public func listLanguages(
        _ request: Greeting_V1_ListLanguagesRequest,
        callOptions: CallOptions? = nil
    ) -> UnaryCall<Greeting_V1_ListLanguagesRequest, Greeting_V1_ListLanguagesResponse> {
        makeUnaryCall(
            path: Greeting_V1_GreetingServiceClientMetadata.Methods.listLanguages.path,
            request: request,
            callOptions: callOptions ?? defaultCallOptions
        )
    }

    public func sayHello(
        _ request: Greeting_V1_SayHelloRequest,
        callOptions: CallOptions? = nil
    ) -> UnaryCall<Greeting_V1_SayHelloRequest, Greeting_V1_SayHelloResponse> {
        makeUnaryCall(
            path: Greeting_V1_GreetingServiceClientMetadata.Methods.sayHello.path,
            request: request,
            callOptions: callOptions ?? defaultCallOptions
        )
    }
}

public struct Greeting_V1_GreetingServiceClient: Greeting_V1_GreetingServiceClientProtocol {
    public var channel: GRPCChannel
    public var defaultCallOptions: CallOptions

    public init(
        channel: GRPCChannel,
        defaultCallOptions: CallOptions = CallOptions()
    ) {
        self.channel = channel
        self.defaultCallOptions = defaultCallOptions
    }
}

public enum Greeting_V1_GreetingServiceClientMetadata {
    public static let serviceDescriptor = GRPCServiceDescriptor(
        name: "GreetingService",
        fullName: "greeting.v1.GreetingService",
        methods: [
            Methods.listLanguages,
            Methods.sayHello,
        ]
    )

    public enum Methods {
        public static let listLanguages = GRPCMethodDescriptor(
            name: "ListLanguages",
            path: "/greeting.v1.GreetingService/ListLanguages",
            type: .unary
        )

        public static let sayHello = GRPCMethodDescriptor(
            name: "SayHello",
            path: "/greeting.v1.GreetingService/SayHello",
            type: .unary
        )
    }
}

public protocol Greeting_V1_GreetingServiceProvider: CallHandlerProvider {
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
    public var serviceName: Substring {
        Greeting_V1_GreetingServiceServerMetadata.serviceDescriptor.fullName[...]
    }

    public func handle(
        method name: Substring,
        context: CallHandlerContext
    ) -> GRPCServerHandlerProtocol? {
        switch name {
        case "ListLanguages":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_ListLanguagesRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_ListLanguagesResponse>(),
                interceptors: [],
                userFunction: listLanguages(request:context:)
            )
        case "SayHello":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Greeting_V1_SayHelloRequest>(),
                responseSerializer: ProtobufSerializer<Greeting_V1_SayHelloResponse>(),
                interceptors: [],
                userFunction: sayHello(request:context:)
            )
        default:
            return nil
        }
    }
}

public enum Greeting_V1_GreetingServiceServerMetadata {
    public static let serviceDescriptor = Greeting_V1_GreetingServiceClientMetadata.serviceDescriptor
}
