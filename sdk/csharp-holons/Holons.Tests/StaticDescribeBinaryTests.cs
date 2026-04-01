using System.Diagnostics;
using Grpc.Net.Client;
using Holons.V1;

namespace Holons.Tests;

public class StaticDescribeBinaryTests
{
    [Fact]
    public async Task PublishedBinaryServesStaticDescribeWithoutAdjacentProtoFiles()
    {
        var fixtureOutputDir = Path.GetDirectoryName(TestPaths.StaticDescribeFixtureDll())!;
        var runRoot = Directory.CreateTempSubdirectory("holons-csharp-static-describe-");
        var runDir = Path.Combine(runRoot.FullName, "run");

        try
        {
            CopyDirectory(fixtureOutputDir, runDir);

            Assert.Empty(Directory.EnumerateFiles(runDir, "*.proto", SearchOption.AllDirectories));

            var dllPath = Path.Combine(runDir, "StaticDescribeFixture.dll");
            using var process = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = "dotnet",
                    WorkingDirectory = runDir,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false,
                },
            };
            process.StartInfo.ArgumentList.Add(dllPath);
            process.StartInfo.ArgumentList.Add("--listen");
            process.StartInfo.ArgumentList.Add("tcp://127.0.0.1:0");

            process.Start();

            try
            {
                var uri = await ReadLineWithTimeoutAsync(process.StandardOutput, TimeSpan.FromSeconds(20));
                if (string.IsNullOrWhiteSpace(uri))
                {
                    var stderr = await process.StandardError.ReadToEndAsync();
                    throw new InvalidOperationException($"static describe fixture did not advertise a listen URI: {stderr}");
                }
                if (process.HasExited)
                {
                    var stderr = await process.StandardError.ReadToEndAsync();
                    throw new InvalidOperationException($"static describe fixture exited early ({process.ExitCode}): {stderr}");
                }

                using var channel = GrpcChannel.ForAddress($"http://{uri["tcp://".Length..]}");
                var client = new HolonMeta.HolonMetaClient(channel.CreateCallInvoker());
                var response = await client.DescribeAsync(
                    new DescribeRequest(),
                    deadline: DateTime.UtcNow.AddSeconds(5)).ResponseAsync;

                Assert.Equal("Static", response.Manifest.Identity.GivenName);
                Assert.Equal("Fixture", response.Manifest.Identity.FamilyName);
                Assert.Equal("Published without adjacent proto files.", response.Manifest.Identity.Motto);
                Assert.Equal("csharp", response.Manifest.Lang);
                Assert.Equal("fixture.v1.Ping", Assert.Single(response.Services).Name);
            }
            finally
            {
                await StopProcessAsync(process);
            }
        }
        finally
        {
            runRoot.Delete(recursive: true);
        }
    }

    private static async Task<string?> ReadLineWithTimeoutAsync(StreamReader reader, TimeSpan timeout)
    {
        var lineTask = reader.ReadLineAsync();
        var completed = await Task.WhenAny(lineTask, Task.Delay(timeout));
        if (completed != lineTask)
            throw new TimeoutException("timed out waiting for fixture output");
        return await lineTask;
    }

    private static async Task StopProcessAsync(Process process)
    {
        try
        {
            if (!process.HasExited)
                process.Kill(entireProcessTree: true);
        }
        catch
        {
            // ignored
        }

        try
        {
            await process.WaitForExitAsync();
        }
        catch
        {
            // ignored
        }
    }

    private static void CopyDirectory(string sourceDir, string destinationDir)
    {
        Directory.CreateDirectory(destinationDir);

        foreach (var directory in Directory.EnumerateDirectories(sourceDir, "*", SearchOption.AllDirectories))
        {
            var relative = Path.GetRelativePath(sourceDir, directory);
            Directory.CreateDirectory(Path.Combine(destinationDir, relative));
        }

        foreach (var file in Directory.EnumerateFiles(sourceDir, "*", SearchOption.AllDirectories))
        {
            var relative = Path.GetRelativePath(sourceDir, file);
            var destinationPath = Path.Combine(destinationDir, relative);
            Directory.CreateDirectory(Path.GetDirectoryName(destinationPath)!);
            File.Copy(file, destinationPath, overwrite: true);
        }
    }
}
