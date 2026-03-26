using System.Text;
using System.Text.Json.Nodes;
using System.Diagnostics;
using Grpc.Core;
using Grpc.Net.Client;

namespace Holons.Tests;

public class ConnectTests
{
    private static readonly string ProjectRoot =
        Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", ".."));
    private static readonly Method<string, string> PingMethod = new(
        MethodType.Unary,
        "echo.v1.Echo",
        "Ping",
        new Marshaller<string>(
            value => Encoding.UTF8.GetBytes(value),
            data => Encoding.UTF8.GetString(data)),
        new Marshaller<string>(
            value => Encoding.UTF8.GetBytes(value),
            data => Encoding.UTF8.GetString(data)));

    [Fact]
    public async Task ConnectStartsSlugOverStdioByDefault()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = Directory.CreateTempSubdirectory("holons-csharp-connect-");
        var fixture = await CreateFixtureAsync(root.FullName, "Connect", "Stdio");

        try
        {
            await WithConnectEnvironmentAsync(root.FullName, async () =>
            {
                using var channel = Connect.ConnectTarget(fixture.Slug);
                var pid = await WaitForPidFileAsync(fixture.PidFile);
                var args = await WaitForArgsFileAsync(fixture.ArgsFile);

                try
                {
                    var outJson = await InvokePingAsync(channel, "csharp-connect-stdio");
                    Assert.Equal("csharp-connect-stdio", outJson["message"]?.GetValue<string>());
                    Assert.Equal("csharp-holons", outJson["sdk"]?.GetValue<string>());
                    Assert.Equal(new[] { "serve", "--listen", "stdio://" }, args);
                    Assert.False(File.Exists(fixture.PortFile));
                }
                finally
                {
                    Connect.Disconnect(channel);
                }

                await WaitForPidExitAsync(pid);
            });
        }
        finally
        {
            root.Delete(recursive: true);
        }
    }

    [Fact]
    public async Task ConnectWritesUnixPortFileInPersistentMode()
    {
        if (OperatingSystem.IsWindows())
            return;

        var root = Directory.CreateTempSubdirectory("holons-csharp-connect-");
        var fixture = await CreateFixtureAsync(root.FullName, "Connect", "Unix");

        try
        {
            await WithConnectEnvironmentAsync(root.FullName, async () =>
            {
                using var channel = Connect.ConnectTarget(
                    fixture.Slug,
                    new Connect.ConnectOptions { Timeout = TimeSpan.FromSeconds(5), Transport = "unix", Start = true });
                var pid = await WaitForPidFileAsync(fixture.PidFile);

                try
                {
                    var outJson = await InvokePingAsync(channel, "csharp-connect-unix");
                    Assert.Equal("csharp-connect-unix", outJson["message"]?.GetValue<string>());
                }
                finally
                {
                    Connect.Disconnect(channel);
                }

                var target = (await File.ReadAllTextAsync(fixture.PortFile)).Trim();
                Assert.StartsWith("unix:///tmp/holons-", target);
                Assert.True(ProcessExists(pid));

                using var reused = Connect.ConnectTarget(fixture.Slug);
                try
                {
                    var outJson = await InvokePingAsync(reused, "csharp-connect-unix-reuse");
                    Assert.Equal("csharp-connect-unix-reuse", outJson["message"]?.GetValue<string>());
                }
                finally
                {
                    Connect.Disconnect(reused);
                    try { Process.GetProcessById(pid).Kill(entireProcessTree: true); } catch { }
                    await WaitForPidExitAsync(pid);
                }
            });
        }
        finally
        {
            root.Delete(recursive: true);
        }
    }

    private static async Task<JsonObject> InvokePingAsync(GrpcChannel channel, string message)
    {
        var response = await channel.CreateCallInvoker().AsyncUnaryCall(
                PingMethod,
                null,
                new CallOptions(deadline: DateTime.UtcNow.AddSeconds(2)),
                $$"""{"message":"{{message}}"}""")
            .ResponseAsync.ConfigureAwait(false);
        return JsonNode.Parse(response)!.AsObject();
    }

    private static async Task<ConnectFixture> CreateFixtureAsync(string root, string givenName, string familyName)
    {
        var slug = $"{givenName}-{familyName}".ToLowerInvariant();
        var holonDir = Path.Combine(root, "holons", slug);
        var binaryDir = Path.Combine(holonDir, ".op", "build", "bin");
        Directory.CreateDirectory(binaryDir);

        var wrapper = Path.Combine(binaryDir, "echo-wrapper");
        var pidFile = Path.Combine(root, $"{slug}.pid");
        var argsFile = Path.Combine(root, $"{slug}.args");
        var portFile = Path.Combine(root, ".op", "run", $"{slug}.port");

        await File.WriteAllTextAsync(wrapper, $$"""
            #!/usr/bin/env bash
            set -euo pipefail
            printf '%s\n' "$$" > '{{pidFile}}'
            : > '{{argsFile}}'
            for arg in "$@"; do
              printf '%s\n' "$arg" >> '{{argsFile}}'
            done
            exec '{{Path.Combine(ProjectRoot, "bin", "echo-server")}}' "$@"
            """);
        File.SetUnixFileMode(
            wrapper,
            UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);

        Directory.CreateDirectory(holonDir);
        await File.WriteAllTextAsync(Path.Combine(holonDir, "holon.proto"), $$"""
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                uuid: "{{slug}}-uuid"
                given_name: "{{givenName}}"
                family_name: "{{familyName}}"
                composer: "connect-test"
              }
              kind: "service"
              build: {
                runner: "shell"
              }
              artifacts: {
                binary: "echo-wrapper"
              }
            };
            """);

        return new ConnectFixture(root, slug, pidFile, argsFile, portFile);
    }

    private static async Task WithConnectEnvironmentAsync(string root, Func<Task> action)
    {
        var previousCwd = Directory.GetCurrentDirectory();
        var previousOpPath = Environment.GetEnvironmentVariable("OPPATH");
        var previousOpBin = Environment.GetEnvironmentVariable("OPBIN");

        Directory.SetCurrentDirectory(root);
        Environment.SetEnvironmentVariable("OPPATH", Path.Combine(root, ".op-home"));
        Environment.SetEnvironmentVariable("OPBIN", Path.Combine(root, ".op-bin"));

        try
        {
            await action().ConfigureAwait(false);
        }
        finally
        {
            Directory.SetCurrentDirectory(Directory.Exists(previousCwd) ? previousCwd : ProjectRoot);
            Environment.SetEnvironmentVariable("OPPATH", previousOpPath);
            Environment.SetEnvironmentVariable("OPBIN", previousOpBin);
        }
    }

    private static async Task<int> WaitForPidFileAsync(string path)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                var pid = int.Parse((await File.ReadAllTextAsync(path).ConfigureAwait(false)).Trim());
                if (pid > 0)
                    return pid;
            }
            catch
            {
                // Wrapper is still starting.
            }
            await Task.Delay(25).ConfigureAwait(false);
        }

        throw new Xunit.Sdk.XunitException($"timed out waiting for pid file {path}");
    }

    private static async Task<IReadOnlyList<string>> WaitForArgsFileAsync(string path)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                var lines = (await File.ReadAllLinesAsync(path).ConfigureAwait(false))
                    .Where(line => !string.IsNullOrWhiteSpace(line))
                    .ToArray();
                if (lines.Length > 0)
                    return lines;
            }
            catch
            {
                // Wrapper is still starting.
            }
            await Task.Delay(25).ConfigureAwait(false);
        }

        throw new Xunit.Sdk.XunitException($"timed out waiting for args file {path}");
    }

    private static async Task WaitForPidExitAsync(int pid)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(2);
        while (DateTime.UtcNow < deadline)
        {
            if (!ProcessExists(pid))
                return;
            await Task.Delay(25).ConfigureAwait(false);
        }

        throw new Xunit.Sdk.XunitException($"process {pid} did not exit");
    }

    private static bool ProcessExists(int pid) =>
        Process.GetProcesses().Any(process => process.Id == pid);

    private sealed record ConnectFixture(
        string Root,
        string Slug,
        string PidFile,
        string ArgsFile,
        string PortFile);
}
