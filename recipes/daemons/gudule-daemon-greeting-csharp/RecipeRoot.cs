namespace GreetingDaemon.Csharp;

internal static class RecipeRoot
{
    public static string Find() =>
        TryFind() ?? throw new DirectoryNotFoundException("could not locate gudule-daemon-greeting-csharp recipe root");

    public static string? TryFind()
    {
        var configured = (Environment.GetEnvironmentVariable("GUDULE_RECIPE_ROOT") ?? string.Empty).Trim();
        if (configured.Length > 0)
            return Path.GetFullPath(configured);

        var starts = new List<string>
        {
            Directory.GetCurrentDirectory(),
            AppContext.BaseDirectory
        };

        if (!string.IsNullOrWhiteSpace(Environment.ProcessPath))
            starts.Add(Path.GetDirectoryName(Environment.ProcessPath!)!);

        var fallback = FindHolonManifestRoot(starts);
        if (!string.IsNullOrEmpty(fallback))
            return fallback;

        return FindManifestOnlyRoot(starts);
    }

    private static string? FindHolonManifestRoot(IEnumerable<string> starts)
    {
        foreach (var start in starts)
        {
            var current = new DirectoryInfo(Path.GetFullPath(start));
            while (current is not null)
            {
                var holonYaml = Path.Combine(current.FullName, "holon.yaml");
                var projectFile = Path.Combine(current.FullName, "gudule-daemon-greeting-csharp.csproj");
                if (File.Exists(holonYaml) && File.Exists(projectFile))
                    return current.FullName;

                current = current.Parent;
            }
        }

        return null;
    }

    private static string? FindManifestOnlyRoot(IEnumerable<string> starts)
    {
        foreach (var start in starts)
        {
            var current = new DirectoryInfo(Path.GetFullPath(start));
            while (current is not null)
            {
                var holonYaml = Path.Combine(current.FullName, "holon.yaml");
                if (File.Exists(holonYaml))
                    return current.FullName;

                current = current.Parent;
            }
        }

        return null;
    }
}
