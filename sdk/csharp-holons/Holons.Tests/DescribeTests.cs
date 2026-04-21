using Grpc.Core;
using Holons.V1;

namespace Holons.Tests;

public class DescribeTests
{
    [Fact]
    public void BuildResponseFromEchoProto()
    {
        var root = CreateTempHolon();
        try
        {
            var response = Describe.BuildResponse(
                Path.Combine(root, "protos"));

            Assert.Equal("Echo", response.Manifest.Identity.GivenName);
            Assert.Equal("Server", response.Manifest.Identity.FamilyName);
            Assert.Equal("Reply precisely.", response.Manifest.Identity.Motto);
            var service = Assert.Single(response.Services);
            Assert.Equal("echo.v1.Echo", service.Name);
            Assert.Equal("Echo echoes request payloads for documentation tests.", service.Description);

            var method = Assert.Single(service.Methods);
            Assert.Equal("Ping", method.Name);
            Assert.Equal("echo.v1.PingRequest", method.InputType);
            Assert.Equal("echo.v1.PingResponse", method.OutputType);
            Assert.Equal("""{"message":"hello","sdk":"go-holons"}""", method.ExampleInput);

            var field = method.InputFields[0];
            Assert.Equal("message", field.Name);
            Assert.Equal("string", field.Type);
            Assert.Equal(1, field.Number);
            Assert.Equal("Message to echo back.", field.Description);
            Assert.Equal(FieldLabel.Optional, field.Label);
            Assert.True(field.Required);
            Assert.Equal("\"hello\"", field.Example);
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task RegistersWorkingDescribeRpc()
    {
        var root = CreateTempHolon();
        var binder = new CapturingBinder<DescribeRequest, DescribeResponse>();

        try
        {
            Describe.UseStaticResponse(Describe.BuildResponse(Path.Combine(root, "protos")));
            Describe.BindService().BindService(binder);
            var response = await binder.InvokeAsync(new DescribeRequest());

            Assert.Equal("Echo", response.Manifest.Identity.GivenName);
            Assert.Equal("echo.v1.Echo", Assert.Single(response.Services).Name);
            Assert.Equal("Ping", Assert.Single(response.Services.Single().Methods).Name);
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void BindServiceRequiresStaticDescribeResponse()
    {
        Describe.UseStaticResponse(null);

        var error = Assert.Throws<InvalidOperationException>(() => Describe.BindService());
        Assert.Equal(Describe.NoIncodeDescriptionMessage, error.Message);
    }

    [Fact]
    public void HandlesMissingProtoDirectory()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-empty-{Guid.NewGuid():N}");
        Directory.CreateDirectory(root);
        File.WriteAllText(
            Path.Combine(root, "holon.proto"),
            """
            syntax = "proto3";
            package test.v1;

            option (holons.v1.manifest) = {
              identity: {
                given_name: "Silent"
                family_name: "Holon"
                motto: "Quietly available."
              }
            };
            """);

        try
        {
            var response = Describe.BuildResponse(Path.Combine(root, "protos"));
            Assert.Equal("Silent", response.Manifest.Identity.GivenName);
            Assert.Equal("Holon", response.Manifest.Identity.FamilyName);
            Assert.Equal("Quietly available.", response.Manifest.Identity.Motto);
            Assert.Empty(response.Services);
        }
        finally
        {
            Directory.Delete(root, recursive: true);
        }
    }

    private static string CreateTempHolon()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-describe-{Guid.NewGuid():N}");
        var protoDir = Path.Combine(root, "protos", "echo", "v1");
        Directory.CreateDirectory(protoDir);

        File.WriteAllText(
            Path.Combine(root, "holon.proto"),
            """
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                given_name: "Echo"
                family_name: "Server"
                motto: "Reply precisely."
              }
            };
            """);
        File.WriteAllText(
            Path.Combine(protoDir, "echo.proto"),
            """
            syntax = "proto3";
            package echo.v1;

            // Echo echoes request payloads for documentation tests.
            service Echo {
              // Ping echoes the inbound message.
              // @example {"message":"hello","sdk":"go-holons"}
              rpc Ping(PingRequest) returns (PingResponse);
            }

            message PingRequest {
              // Message to echo back.
              // @required
              // @example "hello"
              string message = 1;

              // SDK marker included in the response.
              // @example "go-holons"
              string sdk = 2;
            }

            message PingResponse {
              // Echoed message.
              string message = 1;

              // SDK marker from the server.
              string sdk = 2;
            }
            """);

        return root;
    }

    private sealed class CapturingBinder<TRequest, TResponse> : ServiceBinderBase
        where TRequest : class
        where TResponse : class
    {
        private UnaryServerMethod<TRequest, TResponse>? _handler;

        public override void AddMethod<TReq, TRes>(
            Method<TReq, TRes> method,
            UnaryServerMethod<TReq, TRes> handler)
        {
            if (method.FullName == Describe.DescribeMethod.FullName)
            {
                _handler = async (request, context) =>
                    await handler((TReq)(object)request, context).ConfigureAwait(false) as TResponse
                    ?? throw new InvalidOperationException("unexpected response type");
            }
        }

        public Task<TResponse> InvokeAsync(TRequest request)
        {
            var handler = _handler ?? throw new InvalidOperationException("Describe method was not bound");
            return handler(request, new TestServerCallContext());
        }
    }

    private sealed class TestServerCallContext : ServerCallContext
    {
        private readonly Metadata _responseTrailers = [];
        private readonly Dictionary<object, object> _userState = [];

        protected override string MethodCore => Describe.DescribeMethod.FullName;
        protected override string HostCore => "localhost";
        protected override string PeerCore => "test://local";
        protected override DateTime DeadlineCore => DateTime.UtcNow.AddMinutes(1);
        protected override Metadata RequestHeadersCore => [];
        protected override CancellationToken CancellationTokenCore => CancellationToken.None;
        protected override Metadata ResponseTrailersCore => _responseTrailers;
        protected override Status StatusCore { get; set; } = Status.DefaultSuccess;
        protected override WriteOptions? WriteOptionsCore { get; set; }
        protected override AuthContext AuthContextCore => new(string.Empty, new Dictionary<string, List<AuthProperty>>());
        protected override ContextPropagationToken CreatePropagationTokenCore(ContextPropagationOptions? options) =>
            throw new NotSupportedException();
        protected override Task WriteResponseHeadersAsyncCore(Metadata responseHeaders) => Task.CompletedTask;
        protected override IDictionary<object, object> UserStateCore => _userState;
    }
}
