namespace Holons;

/// <summary>Helpers for composite holons.</summary>
public static partial class Composite
{
    public static string Member(string id)
    {
        var executable = Environment.GetEnvironmentVariable("OP_HOLON_EXECUTABLE");
        if (string.IsNullOrWhiteSpace(executable))
            executable = Environment.ProcessPath;
        if (string.IsNullOrWhiteSpace(executable))
            throw new InvalidOperationException("OP_HOLON_EXECUTABLE is not set");
        return MemberFromExecutable(executable, id);
    }

    public static string MemberFromExecutable(string executable, string id)
    {
        if (string.IsNullOrWhiteSpace(id))
            throw new ArgumentException("member id is required", nameof(id));
        if (string.IsNullOrWhiteSpace(executable))
            throw new ArgumentException("executable path is required", nameof(executable));

        var launcherDir = Path.GetDirectoryName(Path.GetFullPath(executable))
            ?? throw new InvalidOperationException($"executable path has no parent: {executable}");
        var memberDir = Path.Combine(launcherDir, "holons", id);
        if (!Directory.Exists(memberDir))
            throw new DirectoryNotFoundException($"member directory not found: {memberDir}");

        foreach (var candidate in Directory.EnumerateFiles(memberDir).OrderBy(path => path, StringComparer.Ordinal))
        {
            if (IsExecutable(candidate))
                return candidate;
        }

        throw new FileNotFoundException($"no executable found in {memberDir}", memberDir);
    }

    private static bool IsExecutable(string path)
    {
        if (OperatingSystem.IsWindows())
            return string.Equals(Path.GetExtension(path), ".exe", StringComparison.OrdinalIgnoreCase);

        if (Path.GetExtension(path).Length != 0)
            return false;

        try
        {
            var mode = File.GetUnixFileMode(path);
            return (mode & (UnixFileMode.UserExecute | UnixFileMode.GroupExecute | UnixFileMode.OtherExecute)) != 0;
        }
        catch (PlatformNotSupportedException)
        {
            return true;
        }
    }
}
