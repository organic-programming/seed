using System.Text;
using System.Text.Json;
using Grpc.Core;
using Grpc.Net.Client;
using Holons;

namespace HelloWorld;

internal static class Program
{
    public static int Main(string[] args) => RunAsync(args).GetAwaiter().GetResult();

    private static async Task<int> RunAsync(string[] args)
    {
        if (args.Length > 0 && args[0] == "serve")
        {
            return RunServe(args.Skip(1).ToArray());
        }

        if (args.Length > 0 && args[0] == "connect-example")
        {
            await RunConnectExampleAsync();
            return 0;
        }

        var name = args.Length > 0 ? args[0] : "";
        Console.WriteLine(HelloService.Greet(name));
        return 0;
    }

    private static int RunServe(string[] args)
    {
        var listenUri = Serve.ParseFlags(args);
        Console.Error.WriteLine($"csharp-hello-world listening on {listenUri}");
        Console.WriteLine(JsonSerializer.Serialize(new { message = HelloService.Greet("") }));
        return 0;
    }

    private static async Task RunConnectExampleAsync()
    {
        var tempRoot = Path.Combine(Path.GetTempPath(), $"csharp-holons-connect-{Guid.NewGuid():N}");
        Directory.CreateDirectory(tempRoot);
        var previousCwd = Directory.GetCurrentDirectory();

        try
        {
            WriteEchoHolon(tempRoot, ResolveEchoServer());
            Directory.SetCurrentDirectory(tempRoot);

            var channel = Connect.ConnectTarget("echo-server");
            try
            {
                var response = await PingAsync(channel, "{\"message\":\"hello-from-csharp\"}");
                Console.WriteLine(response);
            }
            finally
            {
                Connect.Disconnect(channel);
            }
        }
        finally
        {
            Directory.SetCurrentDirectory(previousCwd);
            if (Directory.Exists(tempRoot))
                Directory.Delete(tempRoot, recursive: true);
        }
    }

    private static string ResolveEchoServer()
    {
        var path = Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "../../../../../../sdk/csharp-holons/bin/echo-server"));
        if (!File.Exists(path))
            throw new FileNotFoundException("echo-server not found", path);
        return path;
    }

    private static void WriteEchoHolon(string root, string binaryPath)
    {
        var holonDir = Path.Combine(root, "holons", "echo-server");
        Directory.CreateDirectory(holonDir);
        File.WriteAllText(
            Path.Combine(holonDir, "holon.yaml"),
            $$"""
            uuid: "echo-server-connect-example"
            given_name: Echo
            family_name: Server
            motto: Reply precisely.
            composer: "connect-example"
            kind: service
            build:
              runner: csharp
              main: bin/echo-server
            artifacts:
              binary: "{{binaryPath}}"
            """);
    }

    private static async Task<string> PingAsync(GrpcChannel channel, string payload)
    {
        var method = new Method<string, string>(
            MethodType.Unary,
            "echo.v1.Echo",
            "Ping",
            Marshallers.Create(
                value => Encoding.UTF8.GetBytes(value),
                bytes => Encoding.UTF8.GetString(bytes)),
            Marshallers.Create(
                value => Encoding.UTF8.GetBytes(value),
                bytes => Encoding.UTF8.GetString(bytes)));

        return await channel
            .CreateCallInvoker()
            .AsyncUnaryCall(
                method,
                null,
                new CallOptions(deadline: DateTime.UtcNow.AddSeconds(5)),
                payload)
            .ResponseAsync;
    }
}
