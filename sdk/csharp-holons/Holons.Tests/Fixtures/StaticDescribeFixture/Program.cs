using Gen;
using Holons;

Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());

var options = Serve.ParseOptions(args);
using var server = Serve.StartWithOptions(
    options.ListenUri,
    Array.Empty<Serve.GrpcServiceRegistration>(),
    new Serve.ServeOptions
    {
        Reflect = options.Reflect,
        Logger = Console.Error.WriteLine,
    });

Console.WriteLine(server.PublicUri);
await server.AwaitAsync();
