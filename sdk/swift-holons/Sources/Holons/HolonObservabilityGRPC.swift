import GRPC
import NIOCore

public protocol Holons_V1_HolonObservabilityClientProtocol: GRPCClient {
    var serviceName: String { get }
    var interceptors: Holons_V1_HolonObservabilityClientInterceptorFactoryProtocol? { get }

    func logs(
        _ request: Holons_V1_LogsRequest,
        callOptions: CallOptions?,
        handler: @escaping (Holons_V1_LogEntry) -> Void
    ) -> ServerStreamingCall<Holons_V1_LogsRequest, Holons_V1_LogEntry>

    func metrics(
        _ request: Holons_V1_MetricsRequest,
        callOptions: CallOptions?
    ) -> UnaryCall<Holons_V1_MetricsRequest, Holons_V1_MetricsSnapshot>

    func events(
        _ request: Holons_V1_EventsRequest,
        callOptions: CallOptions?,
        handler: @escaping (Holons_V1_EventInfo) -> Void
    ) -> ServerStreamingCall<Holons_V1_EventsRequest, Holons_V1_EventInfo>
}

extension Holons_V1_HolonObservabilityClientProtocol {
    public var serviceName: String { "holons.v1.HolonObservability" }

    public func logs(
        _ request: Holons_V1_LogsRequest,
        callOptions: CallOptions? = nil,
        handler: @escaping (Holons_V1_LogEntry) -> Void
    ) -> ServerStreamingCall<Holons_V1_LogsRequest, Holons_V1_LogEntry> {
        makeServerStreamingCall(
            path: "/holons.v1.HolonObservability/Logs",
            request: request,
            callOptions: callOptions ?? defaultCallOptions,
            interceptors: interceptors?.makeLogsInterceptors() ?? [],
            handler: handler
        )
    }

    public func metrics(
        _ request: Holons_V1_MetricsRequest,
        callOptions: CallOptions? = nil
    ) -> UnaryCall<Holons_V1_MetricsRequest, Holons_V1_MetricsSnapshot> {
        makeUnaryCall(
            path: "/holons.v1.HolonObservability/Metrics",
            request: request,
            callOptions: callOptions ?? defaultCallOptions,
            interceptors: interceptors?.makeMetricsInterceptors() ?? []
        )
    }

    public func events(
        _ request: Holons_V1_EventsRequest,
        callOptions: CallOptions? = nil,
        handler: @escaping (Holons_V1_EventInfo) -> Void
    ) -> ServerStreamingCall<Holons_V1_EventsRequest, Holons_V1_EventInfo> {
        makeServerStreamingCall(
            path: "/holons.v1.HolonObservability/Events",
            request: request,
            callOptions: callOptions ?? defaultCallOptions,
            interceptors: interceptors?.makeEventsInterceptors() ?? [],
            handler: handler
        )
    }
}

public final class Holons_V1_HolonObservabilityClient: Holons_V1_HolonObservabilityClientProtocol {
    public let channel: GRPCChannel
    public var defaultCallOptions: CallOptions
    public var interceptors: Holons_V1_HolonObservabilityClientInterceptorFactoryProtocol?

    public init(
        channel: GRPCChannel,
        defaultCallOptions: CallOptions = CallOptions(),
        interceptors: Holons_V1_HolonObservabilityClientInterceptorFactoryProtocol? = nil
    ) {
        self.channel = channel
        self.defaultCallOptions = defaultCallOptions
        self.interceptors = interceptors
    }
}

public protocol Holons_V1_HolonObservabilityClientInterceptorFactoryProtocol {
    func makeLogsInterceptors() -> [ClientInterceptor<Holons_V1_LogsRequest, Holons_V1_LogEntry>]
    func makeMetricsInterceptors() -> [ClientInterceptor<Holons_V1_MetricsRequest, Holons_V1_MetricsSnapshot>]
    func makeEventsInterceptors() -> [ClientInterceptor<Holons_V1_EventsRequest, Holons_V1_EventInfo>]
}

public protocol Holons_V1_HolonObservabilityProvider: CallHandlerProvider {
    var interceptors: Holons_V1_HolonObservabilityServerInterceptorFactoryProtocol? { get }

    func logs(
        request: Holons_V1_LogsRequest,
        context: StreamingResponseCallContext<Holons_V1_LogEntry>
    ) -> EventLoopFuture<GRPCStatus>

    func metrics(
        request: Holons_V1_MetricsRequest,
        context: StatusOnlyCallContext
    ) -> EventLoopFuture<Holons_V1_MetricsSnapshot>

    func events(
        request: Holons_V1_EventsRequest,
        context: StreamingResponseCallContext<Holons_V1_EventInfo>
    ) -> EventLoopFuture<GRPCStatus>
}

extension Holons_V1_HolonObservabilityProvider {
    public var serviceName: Substring {
        Holons_V1_HolonObservabilityServerMetadata.serviceDescriptor.fullName[...]
    }

    public var interceptors: Holons_V1_HolonObservabilityServerInterceptorFactoryProtocol? {
        nil
    }

    public func handle(
        method name: Substring,
        context: CallHandlerContext
    ) -> GRPCServerHandlerProtocol? {
        switch name {
        case "Logs":
            return ServerStreamingServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_LogsRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_LogEntry>(),
                interceptors: interceptors?.makeLogsInterceptors() ?? [],
                userFunction: logs(request:context:)
            )
        case "Metrics":
            return UnaryServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_MetricsRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_MetricsSnapshot>(),
                interceptors: interceptors?.makeMetricsInterceptors() ?? [],
                userFunction: metrics(request:context:)
            )
        case "Events":
            return ServerStreamingServerHandler(
                context: context,
                requestDeserializer: ProtobufDeserializer<Holons_V1_EventsRequest>(),
                responseSerializer: ProtobufSerializer<Holons_V1_EventInfo>(),
                interceptors: interceptors?.makeEventsInterceptors() ?? [],
                userFunction: events(request:context:)
            )
        default:
            return nil
        }
    }
}

public protocol Holons_V1_HolonObservabilityServerInterceptorFactoryProtocol {
    func makeLogsInterceptors() -> [ServerInterceptor<Holons_V1_LogsRequest, Holons_V1_LogEntry>]
    func makeMetricsInterceptors() -> [ServerInterceptor<Holons_V1_MetricsRequest, Holons_V1_MetricsSnapshot>]
    func makeEventsInterceptors() -> [ServerInterceptor<Holons_V1_EventsRequest, Holons_V1_EventInfo>]
}

public enum Holons_V1_HolonObservabilityServerMetadata {
    public static let serviceDescriptor = GRPCServiceDescriptor(
        name: "HolonObservability",
        fullName: "holons.v1.HolonObservability",
        methods: [
            Methods.logs,
            Methods.metrics,
            Methods.events,
        ]
    )

    public enum Methods {
        public static let logs = GRPCMethodDescriptor(
            name: "Logs",
            path: "/holons.v1.HolonObservability/Logs",
            type: .serverStreaming
        )
        public static let metrics = GRPCMethodDescriptor(
            name: "Metrics",
            path: "/holons.v1.HolonObservability/Metrics",
            type: .unary
        )
        public static let events = GRPCMethodDescriptor(
            name: "Events",
            path: "/holons.v1.HolonObservability/Events",
            type: .serverStreaming
        )
    }
}
