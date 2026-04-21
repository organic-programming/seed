package greeting.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.60.0)",
    comments = "Source: v1/greeting.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class GreetingServiceGrpc {

  private GreetingServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "greeting.v1.GreetingService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<greeting.v1.Greeting.ListLanguagesRequest,
      greeting.v1.Greeting.ListLanguagesResponse> getListLanguagesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListLanguages",
      requestType = greeting.v1.Greeting.ListLanguagesRequest.class,
      responseType = greeting.v1.Greeting.ListLanguagesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<greeting.v1.Greeting.ListLanguagesRequest,
      greeting.v1.Greeting.ListLanguagesResponse> getListLanguagesMethod() {
    io.grpc.MethodDescriptor<greeting.v1.Greeting.ListLanguagesRequest, greeting.v1.Greeting.ListLanguagesResponse> getListLanguagesMethod;
    if ((getListLanguagesMethod = GreetingServiceGrpc.getListLanguagesMethod) == null) {
      synchronized (GreetingServiceGrpc.class) {
        if ((getListLanguagesMethod = GreetingServiceGrpc.getListLanguagesMethod) == null) {
          GreetingServiceGrpc.getListLanguagesMethod = getListLanguagesMethod =
              io.grpc.MethodDescriptor.<greeting.v1.Greeting.ListLanguagesRequest, greeting.v1.Greeting.ListLanguagesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListLanguages"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  greeting.v1.Greeting.ListLanguagesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  greeting.v1.Greeting.ListLanguagesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new GreetingServiceMethodDescriptorSupplier("ListLanguages"))
              .build();
        }
      }
    }
    return getListLanguagesMethod;
  }

  private static volatile io.grpc.MethodDescriptor<greeting.v1.Greeting.SayHelloRequest,
      greeting.v1.Greeting.SayHelloResponse> getSayHelloMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "SayHello",
      requestType = greeting.v1.Greeting.SayHelloRequest.class,
      responseType = greeting.v1.Greeting.SayHelloResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<greeting.v1.Greeting.SayHelloRequest,
      greeting.v1.Greeting.SayHelloResponse> getSayHelloMethod() {
    io.grpc.MethodDescriptor<greeting.v1.Greeting.SayHelloRequest, greeting.v1.Greeting.SayHelloResponse> getSayHelloMethod;
    if ((getSayHelloMethod = GreetingServiceGrpc.getSayHelloMethod) == null) {
      synchronized (GreetingServiceGrpc.class) {
        if ((getSayHelloMethod = GreetingServiceGrpc.getSayHelloMethod) == null) {
          GreetingServiceGrpc.getSayHelloMethod = getSayHelloMethod =
              io.grpc.MethodDescriptor.<greeting.v1.Greeting.SayHelloRequest, greeting.v1.Greeting.SayHelloResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "SayHello"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  greeting.v1.Greeting.SayHelloRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  greeting.v1.Greeting.SayHelloResponse.getDefaultInstance()))
              .setSchemaDescriptor(new GreetingServiceMethodDescriptorSupplier("SayHello"))
              .build();
        }
      }
    }
    return getSayHelloMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static GreetingServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<GreetingServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<GreetingServiceStub>() {
        @java.lang.Override
        public GreetingServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new GreetingServiceStub(channel, callOptions);
        }
      };
    return GreetingServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static GreetingServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<GreetingServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<GreetingServiceBlockingStub>() {
        @java.lang.Override
        public GreetingServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new GreetingServiceBlockingStub(channel, callOptions);
        }
      };
    return GreetingServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static GreetingServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<GreetingServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<GreetingServiceFutureStub>() {
        @java.lang.Override
        public GreetingServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new GreetingServiceFutureStub(channel, callOptions);
        }
      };
    return GreetingServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     * <pre>
     * Returns all available greeting languages.
     * &#64;example {}
     * </pre>
     */
    default void listLanguages(greeting.v1.Greeting.ListLanguagesRequest request,
        io.grpc.stub.StreamObserver<greeting.v1.Greeting.ListLanguagesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListLanguagesMethod(), responseObserver);
    }

    /**
     * <pre>
     * Greets the user in the chosen language.
     * &#64;example {"name":"Bob","lang_code":"fr"}
     * </pre>
     */
    default void sayHello(greeting.v1.Greeting.SayHelloRequest request,
        io.grpc.stub.StreamObserver<greeting.v1.Greeting.SayHelloResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getSayHelloMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service GreetingService.
   */
  public static abstract class GreetingServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return GreetingServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service GreetingService.
   */
  public static final class GreetingServiceStub
      extends io.grpc.stub.AbstractAsyncStub<GreetingServiceStub> {
    private GreetingServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected GreetingServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new GreetingServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Returns all available greeting languages.
     * &#64;example {}
     * </pre>
     */
    public void listLanguages(greeting.v1.Greeting.ListLanguagesRequest request,
        io.grpc.stub.StreamObserver<greeting.v1.Greeting.ListLanguagesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListLanguagesMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Greets the user in the chosen language.
     * &#64;example {"name":"Bob","lang_code":"fr"}
     * </pre>
     */
    public void sayHello(greeting.v1.Greeting.SayHelloRequest request,
        io.grpc.stub.StreamObserver<greeting.v1.Greeting.SayHelloResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getSayHelloMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service GreetingService.
   */
  public static final class GreetingServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<GreetingServiceBlockingStub> {
    private GreetingServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected GreetingServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new GreetingServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Returns all available greeting languages.
     * &#64;example {}
     * </pre>
     */
    public greeting.v1.Greeting.ListLanguagesResponse listLanguages(greeting.v1.Greeting.ListLanguagesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListLanguagesMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Greets the user in the chosen language.
     * &#64;example {"name":"Bob","lang_code":"fr"}
     * </pre>
     */
    public greeting.v1.Greeting.SayHelloResponse sayHello(greeting.v1.Greeting.SayHelloRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getSayHelloMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service GreetingService.
   */
  public static final class GreetingServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<GreetingServiceFutureStub> {
    private GreetingServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected GreetingServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new GreetingServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Returns all available greeting languages.
     * &#64;example {}
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<greeting.v1.Greeting.ListLanguagesResponse> listLanguages(
        greeting.v1.Greeting.ListLanguagesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListLanguagesMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Greets the user in the chosen language.
     * &#64;example {"name":"Bob","lang_code":"fr"}
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<greeting.v1.Greeting.SayHelloResponse> sayHello(
        greeting.v1.Greeting.SayHelloRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getSayHelloMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_LIST_LANGUAGES = 0;
  private static final int METHODID_SAY_HELLO = 1;

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
        case METHODID_LIST_LANGUAGES:
          serviceImpl.listLanguages((greeting.v1.Greeting.ListLanguagesRequest) request,
              (io.grpc.stub.StreamObserver<greeting.v1.Greeting.ListLanguagesResponse>) responseObserver);
          break;
        case METHODID_SAY_HELLO:
          serviceImpl.sayHello((greeting.v1.Greeting.SayHelloRequest) request,
              (io.grpc.stub.StreamObserver<greeting.v1.Greeting.SayHelloResponse>) responseObserver);
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
          getListLanguagesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              greeting.v1.Greeting.ListLanguagesRequest,
              greeting.v1.Greeting.ListLanguagesResponse>(
                service, METHODID_LIST_LANGUAGES)))
        .addMethod(
          getSayHelloMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              greeting.v1.Greeting.SayHelloRequest,
              greeting.v1.Greeting.SayHelloResponse>(
                service, METHODID_SAY_HELLO)))
        .build();
  }

  private static abstract class GreetingServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    GreetingServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return greeting.v1.Greeting.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("GreetingService");
    }
  }

  private static final class GreetingServiceFileDescriptorSupplier
      extends GreetingServiceBaseDescriptorSupplier {
    GreetingServiceFileDescriptorSupplier() {}
  }

  private static final class GreetingServiceMethodDescriptorSupplier
      extends GreetingServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    GreetingServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (GreetingServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new GreetingServiceFileDescriptorSupplier())
              .addMethod(getListLanguagesMethod())
              .addMethod(getSayHelloMethod())
              .build();
        }
      }
    }
    return result;
  }
}
