using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using Grpc.Net.Client;
using Holons.V1;

namespace Holons.Tests;

internal sealed record PackageSeed(
    string Slug,
    string Uuid,
    string GivenName,
    string FamilyName,
    string Runner = "shell",
    string Entrypoint = "",
    string Kind = "native",
    string Transport = "stdio",
    IReadOnlyList<string>? Architectures = null,
    bool HasDist = false,
    bool HasSource = false,
    IReadOnlyList<string>? Aliases = null);

internal sealed class RuntimeEnvironment : IDisposable
{
    private readonly string _originalCwd = Directory.GetCurrentDirectory();
    private readonly string? _originalOpPath = Environment.GetEnvironmentVariable("OPPATH");
    private readonly string? _originalOpBin = Environment.GetEnvironmentVariable("OPBIN");
    private readonly string? _originalPath = Environment.GetEnvironmentVariable("PATH");
    private readonly string? _originalExecutablePath = Environment.GetEnvironmentVariable("HOLONS_EXECUTABLE_PATH");

    public RuntimeEnvironment()
    {
        Root = Directory.CreateTempSubdirectory("holons-csharp-discovery-").FullName;
        OpHome = Path.Combine(Root, "runtime");
        OpBin = Path.Combine(OpHome, "bin");
        Directory.CreateDirectory(OpBin);
        Environment.SetEnvironmentVariable("OPPATH", OpHome);
        Environment.SetEnvironmentVariable("OPBIN", OpBin);
    }

    public string Root { get; }

    public string OpHome { get; }

    public string OpBin { get; }

    public void SetCurrentDirectory(string path) => Directory.SetCurrentDirectory(path);

    public void SetExecutablePath(string? path) =>
        Environment.SetEnvironmentVariable("HOLONS_EXECUTABLE_PATH", path);

    public void PrependPath(string directory)
    {
        var current = Environment.GetEnvironmentVariable("PATH") ?? string.Empty;
        var updated = string.IsNullOrWhiteSpace(current)
            ? directory
            : directory + Path.PathSeparator + current;
        Environment.SetEnvironmentVariable("PATH", updated);
    }

    public void Dispose()
    {
        Directory.SetCurrentDirectory(Directory.Exists(_originalCwd) ? _originalCwd : AppContext.BaseDirectory);
        Environment.SetEnvironmentVariable("OPPATH", _originalOpPath);
        Environment.SetEnvironmentVariable("OPBIN", _originalOpBin);
        Environment.SetEnvironmentVariable("PATH", _originalPath);
        Environment.SetEnvironmentVariable("HOLONS_EXECUTABLE_PATH", _originalExecutablePath);

        try
        {
            if (Directory.Exists(Root))
                Directory.Delete(Root, recursive: true);
        }
        catch
        {
            // Best effort cleanup for temp fixtures.
        }
    }
}

internal static class DiscoveryTestSupport
{
    private static readonly Lazy<string> FixtureDll = new(ResolveFixtureDllPath);

    public static string FileUrl(string path) => new Uri(Path.GetFullPath(path)).AbsoluteUri;

    public static IReadOnlyList<string> SortedSlugs(DiscoverResult result) =>
        result.Found
            .Where(reference => reference.Info is not null)
            .Select(reference => reference.Info!.Slug)
            .OrderBy(slug => slug, StringComparer.Ordinal)
            .ToArray();

    public static async Task<DescribeResponse> InvokeDescribeAsync(GrpcChannel channel)
    {
        var client = new HolonMeta.HolonMetaClient(channel.CreateCallInvoker());
        return await client.DescribeAsync(
                new DescribeRequest(),
                deadline: DateTime.UtcNow.AddSeconds(5))
            .ResponseAsync
            .ConfigureAwait(false);
    }

    public static void WritePackageHolon(
        string directory,
        PackageSeed seed,
        bool withHolonJson = true,
        bool withBinary = false)
    {
        Directory.CreateDirectory(directory);

        var entrypoint = string.IsNullOrWhiteSpace(seed.Entrypoint) ? seed.Slug : seed.Entrypoint;
        var architectures = seed.Architectures?.Count > 0
            ? seed.Architectures
            : withBinary
            ? new[] { PackageArchDirectory() }
            : Array.Empty<string>();

        if (withBinary)
        {
            var binaryPath = Path.Combine(directory, "bin", PackageArchDirectory(), entrypoint);
            Directory.CreateDirectory(Path.GetDirectoryName(binaryPath)!);
            File.WriteAllText(binaryPath, StaticDescribeWrapperScript(), new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
            if (!OperatingSystem.IsWindows())
            {
                File.SetUnixFileMode(
                    binaryPath,
                    UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute
                    | UnixFileMode.GroupRead | UnixFileMode.GroupExecute
                    | UnixFileMode.OtherRead | UnixFileMode.OtherExecute);
            }
        }

        if (!withHolonJson)
            return;

        var payload = new Dictionary<string, object?>
        {
            ["schema"] = "holon-package/v1",
            ["slug"] = seed.Slug,
            ["uuid"] = seed.Uuid,
            ["identity"] = new Dictionary<string, object?>
            {
                ["given_name"] = seed.GivenName,
                ["family_name"] = seed.FamilyName,
                ["aliases"] = seed.Aliases ?? Array.Empty<string>(),
            },
            ["lang"] = "csharp",
            ["runner"] = seed.Runner,
            ["status"] = "draft",
            ["kind"] = seed.Kind,
            ["transport"] = seed.Transport,
            ["entrypoint"] = entrypoint,
            ["architectures"] = architectures,
            ["has_dist"] = seed.HasDist,
            ["has_source"] = seed.HasSource,
        };

        File.WriteAllText(
            Path.Combine(directory, ".holon.json"),
            JsonSerializer.Serialize(payload, new JsonSerializerOptions { WriteIndented = true }) + Environment.NewLine);
    }

    public static void WriteFakeOpScript(string directory, string cwdFile, string argsFile, string stdoutJson)
    {
        Directory.CreateDirectory(directory);
        var opPath = Path.Combine(directory, "op");
        File.WriteAllText(
            opPath,
            $$"""
            #!/usr/bin/env bash
            set -euo pipefail
            printf '%s\n' "$PWD" > '{{cwdFile}}'
            : > '{{argsFile}}'
            for arg in "$@"; do
              printf '%s\n' "$arg" >> '{{argsFile}}'
            done
            cat <<'JSON'
            {{stdoutJson}}
            JSON
            """,
            new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(
                opPath,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute
                | UnixFileMode.GroupRead | UnixFileMode.GroupExecute
                | UnixFileMode.OtherRead | UnixFileMode.OtherExecute);
        }
    }

    private static string StaticDescribeWrapperScript()
    {
        var fixtureDll = FixtureDll.Value.Replace("'", "'\"'\"'");
        return $$"""
            #!/usr/bin/env bash
            set -euo pipefail
            exec dotnet '{{fixtureDll}}' "$@"
            """;
    }

    private static string ResolveFixtureDllPath()
    {
        return TestPaths.StaticDescribeFixtureDll();
    }

    private static string PackageArchDirectory()
    {
        var system = OperatingSystem.IsMacOS()
            ? "darwin"
            : OperatingSystem.IsWindows()
            ? "windows"
            : "linux";
        var architecture = RuntimeInformation.ProcessArchitecture switch
        {
            Architecture.X64 => "amd64",
            Architecture.Arm64 => "arm64",
            var value => value.ToString().ToLowerInvariant(),
        };
        return $"{system}_{architecture}";
    }
}
