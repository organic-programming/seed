using System;
using System.Diagnostics;
using System.IO;
using System.Threading.Tasks;

namespace CharonDirectCsharpGoOrchestrator;

internal static class Program
{
    private static async Task<int> Main()
    {
        var script = Environment.GetEnvironmentVariable("CHARON_RUN_SCRIPT");
        if (string.IsNullOrWhiteSpace(script))
        {
            script = Path.Combine(AppContext.BaseDirectory, "scripts", "run.sh");
        }

        var start = new ProcessStartInfo("/bin/sh")
        {
            UseShellExecute = false,
        };
        start.ArgumentList.Add(script);
        using var process = Process.Start(start) ?? throw new InvalidOperationException("failed to start /bin/sh");
        await process.WaitForExitAsync();
        return process.ExitCode;
    }
}
