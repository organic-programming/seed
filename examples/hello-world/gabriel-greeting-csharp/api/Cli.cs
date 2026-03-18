using Google.Protobuf;
using Greeting.V1;
using Holons;

namespace GabrielGreeting.Csharp.Api;

public static class Cli
{
    public const string Version = "gabriel-greeting-csharp {{ .Version }}";

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
                    await _Internal.GreetingServer.ListenAndServeAsync(Serve.ParseFlags(args.Skip(1).ToArray()));
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
            case "listlanguages":
                return await RunListLanguagesAsync(args.Skip(1).ToArray(), stdout, stderr);
            case "sayhello":
                return await RunSayHelloAsync(args.Skip(1).ToArray(), stdout, stderr);
            default:
                await stderr.WriteLineAsync($"unknown command \"{args[0]}\"");
                await PrintUsage(stderr);
                return 1;
        }
    }

    private static async Task<int> RunListLanguagesAsync(string[] args, TextWriter stdout, TextWriter stderr)
    {
        try
        {
            var options = ParseCommandOptions(args);
            if (options.Positional.Count != 0)
            {
                await stderr.WriteLineAsync("listLanguages: accepts no positional arguments");
                return 1;
            }

            await WriteResponseAsync(stdout, PublicApi.ListLanguages(new ListLanguagesRequest()), options.Format);
            return 0;
        }
        catch (Exception error)
        {
            await stderr.WriteLineAsync($"listLanguages: {error.Message}");
            return 1;
        }
    }

    private static async Task<int> RunSayHelloAsync(string[] args, TextWriter stdout, TextWriter stderr)
    {
        try
        {
            var options = ParseCommandOptions(args);
            if (options.Positional.Count > 2)
            {
                await stderr.WriteLineAsync("sayHello: accepts at most <name> [lang_code]");
                return 1;
            }

            var request = new SayHelloRequest { LangCode = "en" };
            if (options.Positional.Count >= 1)
                request.Name = options.Positional[0];
            if (options.Positional.Count >= 2)
            {
                if (!string.IsNullOrWhiteSpace(options.Lang))
                {
                    await stderr.WriteLineAsync("sayHello: use either a positional lang_code or --lang, not both");
                    return 1;
                }
                request.LangCode = options.Positional[1];
            }
            if (!string.IsNullOrWhiteSpace(options.Lang))
                request.LangCode = options.Lang;

            await WriteResponseAsync(stdout, PublicApi.SayHello(request), options.Format);
            return 0;
        }
        catch (Exception error)
        {
            await stderr.WriteLineAsync($"sayHello: {error.Message}");
            return 1;
        }
    }

    private static CommandOptions ParseCommandOptions(string[] args)
    {
        var options = new CommandOptions();
        for (var index = 0; index < args.Length; index++)
        {
            var arg = args[index];
            if (arg == "--json")
            {
                options.Format = "json";
            }
            else if (arg == "--format")
            {
                index += 1;
                if (index >= args.Length)
                    throw new ArgumentException("--format requires a value");
                options.Format = ParseOutputFormat(args[index]);
            }
            else if (arg.StartsWith("--format=", StringComparison.Ordinal))
            {
                options.Format = ParseOutputFormat(arg["--format=".Length..]);
            }
            else if (arg == "--lang")
            {
                index += 1;
                if (index >= args.Length)
                    throw new ArgumentException("--lang requires a value");
                options.Lang = args[index].Trim();
            }
            else if (arg.StartsWith("--lang=", StringComparison.Ordinal))
            {
                options.Lang = arg["--lang=".Length..].Trim();
            }
            else if (arg.StartsWith("--", StringComparison.Ordinal))
            {
                throw new ArgumentException($"unknown flag \"{arg}\"");
            }
            else
            {
                options.Positional.Add(arg);
            }
        }
        return options;
    }

    private static string ParseOutputFormat(string raw)
    {
        return raw.Trim().ToLowerInvariant() switch
        {
            "" or "text" or "txt" => "text",
            "json" => "json",
            _ => throw new ArgumentException($"unsupported format \"{raw}\""),
        };
    }

    private static async Task WriteResponseAsync(TextWriter stdout, IMessage message, string format)
    {
        switch (format)
        {
            case "json":
                await stdout.WriteLineAsync(JsonFormatter.Default.Format(message));
                return;
            case "text":
                await WriteTextAsync(stdout, message);
                return;
            default:
                throw new ArgumentException($"unsupported format \"{format}\"");
        }
    }

    private static async Task WriteTextAsync(TextWriter stdout, IMessage message)
    {
        switch (message)
        {
            case SayHelloResponse response:
                await stdout.WriteLineAsync(response.Greeting);
                return;
            case ListLanguagesResponse response:
                foreach (var language in response.Languages)
                    await stdout.WriteLineAsync($"{language.Code}\t{language.Name}\t{language.Native}");
                return;
            default:
                throw new ArgumentException($"unsupported text output for {message.GetType().Name}");
        }
    }

    private static string CanonicalCommand(string raw) =>
        raw.Trim().ToLowerInvariant().Replace("-", "").Replace("_", "").Replace(" ", "");

    private static async Task PrintUsage(TextWriter output)
    {
        await output.WriteLineAsync("usage: gabriel-greeting-csharp <command> [args] [flags]");
        await output.WriteLineAsync();
        await output.WriteLineAsync("commands:");
        await output.WriteLineAsync("  serve [--listen <uri>]                    Start the gRPC server");
        await output.WriteLineAsync("  version                                   Print version and exit");
        await output.WriteLineAsync("  help                                      Print usage");
        await output.WriteLineAsync("  listLanguages [--format text|json]        List supported languages");
        await output.WriteLineAsync("  sayHello [name] [lang_code] [--format text|json] [--lang <code>]");
        await output.WriteLineAsync();
        await output.WriteLineAsync("examples:");
        await output.WriteLineAsync("  gabriel-greeting-csharp serve --listen tcp://:9090");
        await output.WriteLineAsync("  gabriel-greeting-csharp listLanguages --format json");
        await output.WriteLineAsync("  gabriel-greeting-csharp sayHello Alice fr");
        await output.WriteLineAsync("  gabriel-greeting-csharp sayHello Alice --lang fr --format json");
    }

    private sealed class CommandOptions
    {
        public string Format { get; set; } = "text";
        public string Lang { get; set; } = string.Empty;
        public List<string> Positional { get; } = [];
    }
}
