using Holons;
using Holons.Observability;
using Gen;

namespace CascadeNode.Csharp.Api;

public static class Cli
{
    public const string Version = "observability-cascade-csharp-node {{ .Version }}";

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
                    Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());
                    var childFlags = Composite.ParseChildFlags(args.Skip(1).ToArray());
                    var transport = ParseTransport(childFlags.Remaining);
                    var parsed = Serve.ParseOptions(childFlags.Remaining);
                    ObservabilityRegistry.FromEnv(new ObsConfig { Slug = "observability-cascade-csharp-node" });
                    SpawnedMember? downstream = null;
                    try
                    {
                        if (childFlags.Children.Count > 0)
                        {
                            var child = childFlags.Children[0];
                            downstream = await Composite.SpawnMember(
                                new SpawnOptions
                                {
                                    Slug = child.Slug,
                                    BinaryPath = child.Binary,
                                    Transport = transport,
                                    DownstreamChain = childFlags.Children.Skip(1).ToArray(),
                                    ExtraEnv = new Dictionary<string, string>
                                    {
                                        ["OP_OBS"] = "logs,events,metrics,prom",
                                        ["OP_PROM_ADDR"] = "127.0.0.1:0",
                                    },
                                });
                        }

                        Serve.RunWithOptions(
                            parsed.ListenUri,
                            [Holons.Relay.Service(downstream?.Conn)],
                            new Serve.ServeOptions
                            {
                                Reflect = parsed.Reflect,
                                Slug = "observability-cascade-csharp-node",
                            });
                    }
                    finally
                    {
                        if (downstream is not null)
                            await downstream.StopAsync();
                    }
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
            default:
                await stderr.WriteLineAsync($"unknown command \"{args[0]}\"");
                await PrintUsage(stderr);
                return 1;
        }
    }

    private static string ParseTransport(IReadOnlyList<string> args)
    {
        for (var index = 0; index < args.Count; index++)
        {
            var arg = args[index];
            if (arg == "--transport" && index + 1 < args.Count)
                return args[index + 1].Trim().ToLowerInvariant();
            if (arg.StartsWith("--transport=", StringComparison.Ordinal))
                return arg["--transport=".Length..].Trim().ToLowerInvariant();
        }
        return "stdio";
    }

    private static string CanonicalCommand(string raw) =>
        raw.Trim().ToLowerInvariant().Replace("-", "", StringComparison.Ordinal).Replace("_", "", StringComparison.Ordinal).Replace(" ", "", StringComparison.Ordinal);

    private static async Task PrintUsage(TextWriter output)
    {
        await output.WriteLineAsync("usage: observability-cascade-csharp-node <command> [args] [flags]");
        await output.WriteLineAsync();
        await output.WriteLineAsync("commands:");
        await output.WriteLineAsync("  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server");
        await output.WriteLineAsync("  version                                                          Print version and exit");
        await output.WriteLineAsync("  help                                                             Print usage");
    }
}
