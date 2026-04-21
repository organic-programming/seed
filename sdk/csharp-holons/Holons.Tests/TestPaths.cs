using System.Runtime.CompilerServices;

namespace Holons.Tests;

internal static class TestPaths
{
    public static string CSharpHolonsRoot([CallerFilePath] string sourceFilePath = "")
    {
        var configured = Environment.GetEnvironmentVariable("HOLONS_CSHARP_SOURCE_ROOT");
        if (!string.IsNullOrWhiteSpace(configured))
            return Path.GetFullPath(configured);

        return Path.GetFullPath(Path.Combine(Path.GetDirectoryName(sourceFilePath)!, ".."));
    }

    public static string SdkRoot([CallerFilePath] string sourceFilePath = "") =>
        Path.GetFullPath(Path.Combine(CSharpHolonsRoot(sourceFilePath), ".."));

    public static string StaticDescribeFixtureDll([CallerFilePath] string sourceFilePath = "")
    {
        var configuredDll = Environment.GetEnvironmentVariable("HOLONS_CSHARP_FIXTURE_DLL");
        if (!string.IsNullOrWhiteSpace(configuredDll) && File.Exists(configuredDll))
            return Path.GetFullPath(configuredDll);

        var configuredDir = Environment.GetEnvironmentVariable("HOLONS_CSHARP_FIXTURE_OUTPUT_DIR");
        if (!string.IsNullOrWhiteSpace(configuredDir))
        {
            var configuredPath = Path.Combine(configuredDir, "StaticDescribeFixture.dll");
            if (File.Exists(configuredPath))
                return Path.GetFullPath(configuredPath);
        }

        var searchRoots = new[]
        {
            Path.Combine(CSharpHolonsRoot(sourceFilePath), "Holons.Tests", "Fixtures", "StaticDescribeFixture", "bin"),
            Path.Combine(CSharpHolonsRoot(sourceFilePath), "Holons.Tests", "bin"),
        };
        foreach (var root in searchRoots)
        {
            if (!Directory.Exists(root))
                continue;

            var candidate = Directory.EnumerateFiles(root, "StaticDescribeFixture.dll", SearchOption.AllDirectories)
                .OrderByDescending(File.GetLastWriteTimeUtc)
                .FirstOrDefault();
            if (!string.IsNullOrWhiteSpace(candidate))
                return Path.GetFullPath(candidate);
        }

        throw new FileNotFoundException("Static describe fixture build output was not found", searchRoots[0]);
    }
}
