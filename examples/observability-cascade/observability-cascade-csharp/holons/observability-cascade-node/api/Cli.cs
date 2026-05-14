using Google.Protobuf;
using Holons;
using Relay.V1;

namespace CascadeNode.Csharp.Api;

public static class Cli
{
    public const string Version = "observability-cascade-node-csharp {{ .Version }}";

    public static async Task<int> RunAsync(
        string[] args,
        TextWriter? stdout = null,
        TextWriter? stderr = null)
    {
        stdout ??= Console.Out;
        stderr ??= Console.Error;

        if (args.Length == 0)
        {
            await PrintUsage(stderr);
            return 1;
        }

        switch (CanonicalCommand(args[0]))
        {
            case "serve":
                try
                {
                    var parsed = Serve.ParseOptions(args.Skip(1).ToArray());
                    await _Internal.RelayServer.ListenAndServeAsync(
                        parsed.ListenUri,
                        parsed.Reflect,
                        parsed.MemberEndpoints);
                    return 0;
                }
                catch (Exception error)
                {
                    await stderr.WriteLineAsync($"serve: {error.Message}");
                    return 1;
                }
            case "version":
                await stdout.WriteLineAsync(Version);
                return 0;
            case "help":
                await PrintUsage(stdout);
                return 0;
            case "tick":
                return await RunTickAsync(args.Skip(1).ToArray(), stdout, stderr);
            default:
                await stderr.WriteLineAsync($"unknown command \"{args[0]}\"");
                await PrintUsage(stderr);
                return 1;
        }
    }

    private static async Task<int> RunTickAsync(string[] args, TextWriter stdout, TextWriter stderr)
    {
        try
        {
            var request = new TickRequest();
            var positional = new List<string>();
            for (var index = 0; index < args.Length; index++)
            {
                var arg = args[index];
                if (arg == "--sender")
                {
                    index += 1;
                    if (index >= args.Length)
                        throw new ArgumentException("--sender requires a value");
                    request.Sender = args[index];
                }
                else if (arg.StartsWith("--sender=", StringComparison.Ordinal))
                {
                    request.Sender = arg["--sender=".Length..];
                }
                else if (arg == "--note")
                {
                    index += 1;
                    if (index >= args.Length)
                        throw new ArgumentException("--note requires a value");
                    request.Note = args[index];
                }
                else if (arg.StartsWith("--note=", StringComparison.Ordinal))
                {
                    request.Note = arg["--note=".Length..];
                }
                else if (arg.StartsWith("--", StringComparison.Ordinal))
                {
                    throw new ArgumentException($"unknown flag \"{arg}\"");
                }
                else
                {
                    positional.Add(arg);
                }
            }
            if (string.IsNullOrWhiteSpace(request.Sender) && positional.Count >= 1)
                request.Sender = positional[0];
            if (string.IsNullOrWhiteSpace(request.Note) && positional.Count >= 2)
                request.Note = positional[1];

            await WriteResponseAsync(stdout, PublicApi.Tick(request));
            return 0;
        }
        catch (Exception error)
        {
            await stderr.WriteLineAsync($"tick: {error.Message}");
            return 1;
        }
    }

    private static async Task WriteResponseAsync(TextWriter stdout, IMessage message)
    {
        await stdout.WriteLineAsync(JsonFormatter.Default.Format(message));
    }

    private static string CanonicalCommand(string raw) =>
        raw.Trim().ToLowerInvariant().Replace("-", "", StringComparison.Ordinal).Replace("_", "", StringComparison.Ordinal).Replace(" ", "", StringComparison.Ordinal);

    private static async Task PrintUsage(TextWriter output)
    {
        await output.WriteLineAsync("usage: observability-cascade-node-csharp <command> [args] [flags]");
        await output.WriteLineAsync();
        await output.WriteLineAsync("commands:");
        await output.WriteLineAsync("  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server");
        await output.WriteLineAsync("  tick [sender] [note]                                Emit one local tick");
        await output.WriteLineAsync("  version                                             Print version and exit");
        await output.WriteLineAsync("  help                                                Print usage");
    }
}
