import GRPC
import NIOCore

protocol Grpc_Reflection_V1alpha_ServerReflectionProvider: CallHandlerProvider {
    func serverReflectionInfo(
        context: StreamingResponseCallContext<Grpc_Reflection_V1alpha_ServerReflectionResponse>
    ) -> EventLoopFuture<(StreamEvent<Grpc_Reflection_V1alpha_ServerReflectionRequest>) -> Void>
}

extension Grpc_Reflection_V1alpha_ServerReflectionProvider {
    var serviceName: Substring {
        Grpc_Reflection_V1alpha_ServerReflectionServerMetadata.serviceDescriptor.fullName[...]
    }

    func handle(
        method name: Substring,
        context: CallHandlerContext
    ) -> GRPCServerHandlerProtocol? {
        switch name {
        case "ServerReflectionInfo":
            return BidirectionalStreamingServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Grpc_Reflection_V1alpha_ServerReflectionRequest>(),
                responseSerializer: ProtobufSerializer<Grpc_Reflection_V1alpha_ServerReflectionResponse>(),
                interceptors: [],
                observerFactory: serverReflectionInfo(context:)
            )
        default:
            return nil
        }
    }
}

enum Grpc_Reflection_V1alpha_ServerReflectionServerMetadata {
    static let serviceDescriptor = GRPCServiceDescriptor(
        name: "ServerReflection",
        fullName: "grpc.reflection.v1alpha.ServerReflection",
        methods: [
            Methods.serverReflectionInfo,
        ]
    )

    enum Methods {
        static let serverReflectionInfo = GRPCMethodDescriptor(
            name: "ServerReflectionInfo",
            path: "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
            type: .bidirectionalStreaming
        )
    }
}
