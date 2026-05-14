package observability_cascade.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ObservabilityCascadeServiceGrpc {

  private ObservabilityCascadeServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "observability_cascade.v1.ObservabilityCascadeService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest,
      observability_cascade.v1.Service.CascadeReport> getRunDefaultMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RunDefault",
      requestType = observability_cascade.v1.Service.RunRequest.class,
      responseType = observability_cascade.v1.Service.CascadeReport.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest,
      observability_cascade.v1.Service.CascadeReport> getRunDefaultMethod() {
    io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest, observability_cascade.v1.Service.CascadeReport> getRunDefaultMethod;
    if ((getRunDefaultMethod = ObservabilityCascadeServiceGrpc.getRunDefaultMethod) == null) {
      synchronized (ObservabilityCascadeServiceGrpc.class) {
        if ((getRunDefaultMethod = ObservabilityCascadeServiceGrpc.getRunDefaultMethod) == null) {
          ObservabilityCascadeServiceGrpc.getRunDefaultMethod = getRunDefaultMethod =
              io.grpc.MethodDescriptor.<observability_cascade.v1.Service.RunRequest, observability_cascade.v1.Service.CascadeReport>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RunDefault"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  observability_cascade.v1.Service.RunRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  observability_cascade.v1.Service.CascadeReport.getDefaultInstance()))
              .setSchemaDescriptor(new ObservabilityCascadeServiceMethodDescriptorSupplier("RunDefault"))
              .build();
        }
      }
    }
    return getRunDefaultMethod;
  }

  private static volatile io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest,
      observability_cascade.v1.Service.CascadeReport> getRunLiveStreamMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RunLiveStream",
      requestType = observability_cascade.v1.Service.RunRequest.class,
      responseType = observability_cascade.v1.Service.CascadeReport.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest,
      observability_cascade.v1.Service.CascadeReport> getRunLiveStreamMethod() {
    io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest, observability_cascade.v1.Service.CascadeReport> getRunLiveStreamMethod;
    if ((getRunLiveStreamMethod = ObservabilityCascadeServiceGrpc.getRunLiveStreamMethod) == null) {
      synchronized (ObservabilityCascadeServiceGrpc.class) {
        if ((getRunLiveStreamMethod = ObservabilityCascadeServiceGrpc.getRunLiveStreamMethod) == null) {
          ObservabilityCascadeServiceGrpc.getRunLiveStreamMethod = getRunLiveStreamMethod =
              io.grpc.MethodDescriptor.<observability_cascade.v1.Service.RunRequest, observability_cascade.v1.Service.CascadeReport>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RunLiveStream"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  observability_cascade.v1.Service.RunRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  observability_cascade.v1.Service.CascadeReport.getDefaultInstance()))
              .setSchemaDescriptor(new ObservabilityCascadeServiceMethodDescriptorSupplier("RunLiveStream"))
              .build();
        }
      }
    }
    return getRunLiveStreamMethod;
  }

  private static volatile io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest,
      observability_cascade.v1.Service.MultiPatternReport> getRunMultiPatternMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RunMultiPattern",
      requestType = observability_cascade.v1.Service.RunRequest.class,
      responseType = observability_cascade.v1.Service.MultiPatternReport.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest,
      observability_cascade.v1.Service.MultiPatternReport> getRunMultiPatternMethod() {
    io.grpc.MethodDescriptor<observability_cascade.v1.Service.RunRequest, observability_cascade.v1.Service.MultiPatternReport> getRunMultiPatternMethod;
    if ((getRunMultiPatternMethod = ObservabilityCascadeServiceGrpc.getRunMultiPatternMethod) == null) {
      synchronized (ObservabilityCascadeServiceGrpc.class) {
        if ((getRunMultiPatternMethod = ObservabilityCascadeServiceGrpc.getRunMultiPatternMethod) == null) {
          ObservabilityCascadeServiceGrpc.getRunMultiPatternMethod = getRunMultiPatternMethod =
              io.grpc.MethodDescriptor.<observability_cascade.v1.Service.RunRequest, observability_cascade.v1.Service.MultiPatternReport>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RunMultiPattern"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  observability_cascade.v1.Service.RunRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  observability_cascade.v1.Service.MultiPatternReport.getDefaultInstance()))
              .setSchemaDescriptor(new ObservabilityCascadeServiceMethodDescriptorSupplier("RunMultiPattern"))
              .build();
        }
      }
    }
    return getRunMultiPatternMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ObservabilityCascadeServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceStub>() {
        @java.lang.Override
        public ObservabilityCascadeServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ObservabilityCascadeServiceStub(channel, callOptions);
        }
      };
    return ObservabilityCascadeServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ObservabilityCascadeServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceBlockingV2Stub>() {
        @java.lang.Override
        public ObservabilityCascadeServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ObservabilityCascadeServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ObservabilityCascadeServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ObservabilityCascadeServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceBlockingStub>() {
        @java.lang.Override
        public ObservabilityCascadeServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ObservabilityCascadeServiceBlockingStub(channel, callOptions);
        }
      };
    return ObservabilityCascadeServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ObservabilityCascadeServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ObservabilityCascadeServiceFutureStub>() {
        @java.lang.Override
        public ObservabilityCascadeServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ObservabilityCascadeServiceFutureStub(channel, callOptions);
        }
      };
    return ObservabilityCascadeServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     * <pre>
     * Run the default 4-deep chain in this composite's own language.
     * &#64;example {}
     * </pre>
     */
    default void runDefault(observability_cascade.v1.Service.RunRequest request,
        io.grpc.stub.StreamObserver<observability_cascade.v1.Service.CascadeReport> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRunDefaultMethod(), responseObserver);
    }

    /**
     * <pre>
     * Run with long-lived Follow:true streams.
     * &#64;example {}
     * </pre>
     */
    default void runLiveStream(observability_cascade.v1.Service.RunRequest request,
        io.grpc.stub.StreamObserver<observability_cascade.v1.Service.CascadeReport> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRunLiveStreamMethod(), responseObserver);
    }

    /**
     * <pre>
     * Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
     * &#64;example {}
     * </pre>
     */
    default void runMultiPattern(observability_cascade.v1.Service.RunRequest request,
        io.grpc.stub.StreamObserver<observability_cascade.v1.Service.MultiPatternReport> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRunMultiPatternMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ObservabilityCascadeService.
   */
  public static abstract class ObservabilityCascadeServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ObservabilityCascadeServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ObservabilityCascadeService.
   */
  public static final class ObservabilityCascadeServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ObservabilityCascadeServiceStub> {
    private ObservabilityCascadeServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ObservabilityCascadeServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ObservabilityCascadeServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Run the default 4-deep chain in this composite's own language.
     * &#64;example {}
     * </pre>
     */
    public void runDefault(observability_cascade.v1.Service.RunRequest request,
        io.grpc.stub.StreamObserver<observability_cascade.v1.Service.CascadeReport> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRunDefaultMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Run with long-lived Follow:true streams.
     * &#64;example {}
     * </pre>
     */
    public void runLiveStream(observability_cascade.v1.Service.RunRequest request,
        io.grpc.stub.StreamObserver<observability_cascade.v1.Service.CascadeReport> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRunLiveStreamMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
     * &#64;example {}
     * </pre>
     */
    public void runMultiPattern(observability_cascade.v1.Service.RunRequest request,
        io.grpc.stub.StreamObserver<observability_cascade.v1.Service.MultiPatternReport> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRunMultiPatternMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ObservabilityCascadeService.
   */
  public static final class ObservabilityCascadeServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ObservabilityCascadeServiceBlockingV2Stub> {
    private ObservabilityCascadeServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ObservabilityCascadeServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ObservabilityCascadeServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Run the default 4-deep chain in this composite's own language.
     * &#64;example {}
     * </pre>
     */
    public observability_cascade.v1.Service.CascadeReport runDefault(observability_cascade.v1.Service.RunRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRunDefaultMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Run with long-lived Follow:true streams.
     * &#64;example {}
     * </pre>
     */
    public observability_cascade.v1.Service.CascadeReport runLiveStream(observability_cascade.v1.Service.RunRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRunLiveStreamMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
     * &#64;example {}
     * </pre>
     */
    public observability_cascade.v1.Service.MultiPatternReport runMultiPattern(observability_cascade.v1.Service.RunRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRunMultiPatternMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ObservabilityCascadeService.
   */
  public static final class ObservabilityCascadeServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ObservabilityCascadeServiceBlockingStub> {
    private ObservabilityCascadeServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ObservabilityCascadeServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ObservabilityCascadeServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Run the default 4-deep chain in this composite's own language.
     * &#64;example {}
     * </pre>
     */
    public observability_cascade.v1.Service.CascadeReport runDefault(observability_cascade.v1.Service.RunRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRunDefaultMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Run with long-lived Follow:true streams.
     * &#64;example {}
     * </pre>
     */
    public observability_cascade.v1.Service.CascadeReport runLiveStream(observability_cascade.v1.Service.RunRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRunLiveStreamMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
     * &#64;example {}
     * </pre>
     */
    public observability_cascade.v1.Service.MultiPatternReport runMultiPattern(observability_cascade.v1.Service.RunRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRunMultiPatternMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ObservabilityCascadeService.
   */
  public static final class ObservabilityCascadeServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ObservabilityCascadeServiceFutureStub> {
    private ObservabilityCascadeServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ObservabilityCascadeServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ObservabilityCascadeServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Run the default 4-deep chain in this composite's own language.
     * &#64;example {}
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<observability_cascade.v1.Service.CascadeReport> runDefault(
        observability_cascade.v1.Service.RunRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRunDefaultMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Run with long-lived Follow:true streams.
     * &#64;example {}
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<observability_cascade.v1.Service.CascadeReport> runLiveStream(
        observability_cascade.v1.Service.RunRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRunLiveStreamMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Run the full alter-language pattern matrix (3 patterns x 12 ticks = 36 ticks).
     * &#64;example {}
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<observability_cascade.v1.Service.MultiPatternReport> runMultiPattern(
        observability_cascade.v1.Service.RunRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRunMultiPatternMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_RUN_DEFAULT = 0;
  private static final int METHODID_RUN_LIVE_STREAM = 1;
  private static final int METHODID_RUN_MULTI_PATTERN = 2;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_RUN_DEFAULT:
          serviceImpl.runDefault((observability_cascade.v1.Service.RunRequest) request,
              (io.grpc.stub.StreamObserver<observability_cascade.v1.Service.CascadeReport>) responseObserver);
          break;
        case METHODID_RUN_LIVE_STREAM:
          serviceImpl.runLiveStream((observability_cascade.v1.Service.RunRequest) request,
              (io.grpc.stub.StreamObserver<observability_cascade.v1.Service.CascadeReport>) responseObserver);
          break;
        case METHODID_RUN_MULTI_PATTERN:
          serviceImpl.runMultiPattern((observability_cascade.v1.Service.RunRequest) request,
              (io.grpc.stub.StreamObserver<observability_cascade.v1.Service.MultiPatternReport>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getRunDefaultMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              observability_cascade.v1.Service.RunRequest,
              observability_cascade.v1.Service.CascadeReport>(
                service, METHODID_RUN_DEFAULT)))
        .addMethod(
          getRunLiveStreamMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              observability_cascade.v1.Service.RunRequest,
              observability_cascade.v1.Service.CascadeReport>(
                service, METHODID_RUN_LIVE_STREAM)))
        .addMethod(
          getRunMultiPatternMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              observability_cascade.v1.Service.RunRequest,
              observability_cascade.v1.Service.MultiPatternReport>(
                service, METHODID_RUN_MULTI_PATTERN)))
        .build();
  }

  private static abstract class ObservabilityCascadeServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ObservabilityCascadeServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return observability_cascade.v1.Service.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ObservabilityCascadeService");
    }
  }

  private static final class ObservabilityCascadeServiceFileDescriptorSupplier
      extends ObservabilityCascadeServiceBaseDescriptorSupplier {
    ObservabilityCascadeServiceFileDescriptorSupplier() {}
  }

  private static final class ObservabilityCascadeServiceMethodDescriptorSupplier
      extends ObservabilityCascadeServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ObservabilityCascadeServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (ObservabilityCascadeServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ObservabilityCascadeServiceFileDescriptorSupplier())
              .addMethod(getRunDefaultMethod())
              .addMethod(getRunLiveStreamMethod())
              .addMethod(getRunMultiPatternMethod())
              .build();
        }
      }
    }
    return result;
  }
}
