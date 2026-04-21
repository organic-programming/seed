import GRPC
import NIOCore

protocol Holons_V1_HolonMetaProvider: CallHandlerProvider {
    var interceptors: Holons_V1_HolonMetaServerInterceptorFactoryProtocol? { get }

    func describe(
        request: Holons_V1_DescribeRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_DescribeResponse>
}

extension Holons_V1_HolonMetaProvider {
    var serviceName: Substring {
        Holons_V1_HolonMetaServerMetadata.serviceDescriptor.fullName[...]
    }

    var interceptors: Holons_V1_HolonMetaServerInterceptorFactoryProtocol? {
        nil
    }

    func handle(
        method name: Substring,
        context: CallHandlerContext
    ) -> GRPCServerHandlerProtocol? {
        switch name {
        case "Describe":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_DescribeRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_DescribeResponse>(),
                interceptors: self.interceptors?.makeDescribeInterceptors() ?? [],
                userFunction: self.describe(request:context:)
            )
        default:
            return nil
        }
    }
}

protocol Holons_V1_HolonMetaServerInterceptorFactoryProtocol {
    func makeDescribeInterceptors() -> [ServerInterceptor<Holons_V1_DescribeRequest, Holons_V1_DescribeResponse>]
}

enum Holons_V1_HolonMetaServerMetadata {
    static let serviceDescriptor = GRPCServiceDescriptor(
        name: "HolonMeta",
        fullName: "holons.v1.HolonMeta",
        methods: [
            Methods.describe,
        ]
    )

    enum Methods {
        static let describe = GRPCMethodDescriptor(
            name: "Describe",
            path: "/holons.v1.HolonMeta/Describe",
            type: .unary
        )
    }
}
