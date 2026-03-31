using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Holons;

/// <summary>Uniform holon discovery for csharp-holons.</summary>
public static partial class Discovery
{
    private static readonly HashSet<string> ExcludedDirs = new(StringComparer.Ordinal)
    {
        ".git",
        ".op",
        "node_modules",
        "vendor",
        "build",
        "testdata",
    };

    private sealed record DiscoveredEntry(HolonRef Ref, string DirPath, string RelativePath);

    private sealed class PackageJson
    {
        [JsonPropertyName("schema")]
        public string? Schema { get; init; }

        [JsonPropertyName("slug")]
        public string? Slug { get; init; }

        [JsonPropertyName("uuid")]
        public string? Uuid { get; init; }

        [JsonPropertyName("identity")]
        public PackageIdentityJson? Identity { get; init; }

        [JsonPropertyName("lang")]
        public string? Lang { get; init; }

        [JsonPropertyName("runner")]
        public string? Runner { get; init; }

        [JsonPropertyName("status")]
        public string? Status { get; init; }

        [JsonPropertyName("kind")]
        public string? Kind { get; init; }

        [JsonPropertyName("transport")]
        public string? Transport { get; init; }

        [JsonPropertyName("entrypoint")]
        public string? Entrypoint { get; init; }

        [JsonPropertyName("architectures")]
        public List<string>? Architectures { get; init; }

        [JsonPropertyName("has_dist")]
        public bool HasDist { get; init; }

        [JsonPropertyName("has_source")]
        public bool HasSource { get; init; }
    }

    private sealed class PackageIdentityJson
    {
        [JsonPropertyName("given_name")]
        public string? GivenName { get; init; }

        [JsonPropertyName("family_name")]
        public string? FamilyName { get; init; }

        [JsonPropertyName("motto")]
        public string? Motto { get; init; }

        [JsonPropertyName("aliases")]
        public List<string>? Aliases { get; init; }
    }

    public static DiscoverResult Discover(
        int scope,
        string? expression,
        string? root,
        int specifiers,
        int limit,
        int timeout)
    {
        if (scope != LOCAL)
            return new DiscoverResult(Array.Empty<HolonRef>(), $"scope {scope} not supported");

        if (specifiers < 0 || (specifiers & ~ALL) != 0)
            return new DiscoverResult(Array.Empty<HolonRef>(), $"invalid specifiers 0x{specifiers:X2}: valid range is 0x00-0x3F");

        if (specifiers == 0)
            specifiers = ALL;

        if (limit < 0)
            return new DiscoverResult(Array.Empty<HolonRef>(), null);

        var normalizedExpression = NormalizedExpression(expression);
        string? searchRoot = null;

        string ResolveRoot()
        {
            searchRoot ??= ResolveDiscoverRoot(root);
            return searchRoot;
        }

        try
        {
            if (normalizedExpression is not null)
            {
                var (refs, handled) = DiscoverPathExpression(normalizedExpression, ResolveRoot, timeout);
                if (handled)
                    return new DiscoverResult(ApplyRefLimit(refs, limit), null);
            }

            var entries = DiscoverEntries(ResolveRoot(), specifiers, timeout);
            var found = new List<HolonRef>();

            foreach (var entry in entries)
            {
                if (!MatchesExpression(entry, normalizedExpression))
                    continue;

                found.Add(entry.Ref);
                if (limit > 0 && found.Count >= limit)
                    break;
            }

            return new DiscoverResult(found, null);
        }
        catch (Exception ex)
        {
            return new DiscoverResult(Array.Empty<HolonRef>(), ex.Message);
        }
    }

    public static ResolveResult Resolve(
        int scope,
        string expression,
        string? root,
        int specifiers,
        int timeout)
    {
        var result = Discover(scope, expression, root, specifiers, 1, timeout);
        if (result.Error is not null)
            return new ResolveResult(null, result.Error);

        if (result.Found.Count == 0)
            return new ResolveResult(null, $"holon \"{expression}\" not found");

        var reference = result.Found[0];
        return reference.Error is null
            ? new ResolveResult(reference, null)
            : new ResolveResult(reference, reference.Error);
    }

    private static IReadOnlyList<DiscoveredEntry> DiscoverEntries(string root, int specifiers, int timeout)
    {
        var layers = new (int Flag, Func<string, IReadOnlyList<DiscoveredEntry>> Scan)[]
        {
            (SIBLINGS, _ => DiscoverPackagesDirect(BundleHolonsRoot(), timeout)),
            (CWD, currentRoot => DiscoverPackagesRecursive(currentRoot, timeout)),
            (SOURCE, currentRoot => EntriesFromRefs(
                currentRoot,
                RequireSourceDiscoveryResult(
                    DiscoverSourceWithLocalOp(LOCAL, null, currentRoot, SOURCE, NO_LIMIT, timeout)).Found)),
            (BUILT, currentRoot => DiscoverPackagesDirect(Path.Combine(currentRoot, ".op", "build"), timeout)),
            (INSTALLED, _ => DiscoverPackagesDirect(OpBin(), timeout)),
            (CACHED, _ => DiscoverPackagesRecursive(CacheDir(), timeout)),
        };

        var seen = new HashSet<string>(StringComparer.Ordinal);
        var found = new List<DiscoveredEntry>();

        foreach (var layer in layers)
        {
            if ((specifiers & layer.Flag) == 0)
                continue;

            foreach (var entry in layer.Scan(root))
            {
                var key = EntryKey(entry);
                if (!seen.Add(key))
                    continue;
                found.Add(entry);
            }
        }

        return found;
    }

    private static IReadOnlyList<DiscoveredEntry> DiscoverPackagesDirect(string root, int timeout) =>
        DiscoverPackagesFromDirs(root, PackageDirsDirect(root), timeout);

    private static IReadOnlyList<DiscoveredEntry> DiscoverPackagesRecursive(string root, int timeout) =>
        DiscoverPackagesFromDirs(root, PackageDirsRecursive(root), timeout);

    private static IReadOnlyList<DiscoveredEntry> DiscoverPackagesFromDirs(
        string root,
        IReadOnlyList<string> directories,
        int timeout)
    {
        var resolvedRoot = NormalizeSearchRoot(root);
        var entriesByKey = new Dictionary<string, DiscoveredEntry>(StringComparer.Ordinal);
        var orderedKeys = new List<string>();

        foreach (var packageDir in directories)
        {
            DiscoveredEntry entry;
            try
            {
                entry = LoadPackageEntry(resolvedRoot, packageDir);
            }
            catch
            {
                try
                {
                    entry = ProbePackageEntry(resolvedRoot, packageDir, timeout);
                }
                catch
                {
                    continue;
                }
            }

            var key = EntryKey(entry);
            if (entriesByKey.TryGetValue(key, out var existing))
            {
                if (ShouldReplace(existing, entry))
                    entriesByKey[key] = entry;
                continue;
            }

            entriesByKey[key] = entry;
            orderedKeys.Add(key);
        }

        var entries = orderedKeys
            .Where(entriesByKey.ContainsKey)
            .Select(key => entriesByKey[key])
            .OrderBy(entry => entry.RelativePath, StringComparer.Ordinal)
            .ThenBy(EntrySortKey, StringComparer.Ordinal)
            .ToArray();

        return entries;
    }

    private static DiscoveredEntry LoadPackageEntry(string root, string packageDir)
    {
        var manifestPath = Path.Combine(packageDir, ".holon.json");
        using var stream = File.OpenRead(manifestPath);
        var payload = JsonSerializer.Deserialize<PackageJson>(stream)
            ?? throw new InvalidDataException($"invalid .holon.json at {manifestPath}");

        var schema = Trimmed(payload.Schema);
        if (schema.Length > 0 && !string.Equals(schema, "holon-package/v1", StringComparison.Ordinal))
            throw new InvalidDataException($"unsupported package schema \"{schema}\"");

        var identityPayload = payload.Identity ?? new PackageIdentityJson();
        var identity = new IdentityInfo(
            Trimmed(identityPayload.GivenName),
            Trimmed(identityPayload.FamilyName),
            Trimmed(identityPayload.Motto),
            StringList(identityPayload.Aliases));

        var slug = Trimmed(payload.Slug);
        if (slug.Length == 0)
            slug = SlugFor(identity.GivenName, identity.FamilyName);

        var info = new HolonInfo(
            slug,
            Trimmed(payload.Uuid),
            identity,
            Trimmed(payload.Lang),
            Trimmed(payload.Runner),
            Trimmed(payload.Status),
            Trimmed(payload.Kind),
            Trimmed(payload.Transport),
            Trimmed(payload.Entrypoint),
            StringList(payload.Architectures),
            payload.HasDist,
            payload.HasSource);

        var absoluteDirectory = Path.GetFullPath(packageDir);
        return new DiscoveredEntry(
            new HolonRef(FileUrl(absoluteDirectory), info, null),
            absoluteDirectory,
            RelativePath(root, absoluteDirectory));
    }

    private static DiscoveredEntry ProbePackageEntry(string root, string packageDir, int timeout)
    {
        var absoluteDirectory = Path.GetFullPath(packageDir);
        var info = ConnectionInternals.DescribePackageDirectory(absoluteDirectory, timeout);
        var architectures = info.Architectures.Count > 0
            ? info.Architectures
            : PackageArchitectures(absoluteDirectory);
        info = info with
        {
            HasDist = Directory.Exists(Path.Combine(absoluteDirectory, "dist")) || info.HasDist,
            HasSource = Directory.Exists(Path.Combine(absoluteDirectory, "git")) || info.HasSource,
            Architectures = architectures.ToArray(),
        };

        return new DiscoveredEntry(
            new HolonRef(FileUrl(absoluteDirectory), info, null),
            absoluteDirectory,
            RelativePath(root, absoluteDirectory));
    }

    private static (IReadOnlyList<HolonRef> Refs, bool Handled) DiscoverPathExpression(
        string expression,
        Func<string> resolveRoot,
        int timeout)
    {
        var candidate = PathExpressionCandidate(expression, resolveRoot);
        if (candidate is null)
            return (Array.Empty<HolonRef>(), false);

        var reference = DiscoverRefAtPath(candidate, timeout);
        return reference is null
            ? (Array.Empty<HolonRef>(), true)
            : (new[] { reference }, true);
    }

    private static string? PathExpressionCandidate(string expression, Func<string> resolveRoot)
    {
        var trimmed = expression.Trim();
        if (trimmed.Length == 0)
            return null;

        if (trimmed.StartsWith("file://", StringComparison.OrdinalIgnoreCase))
            return PathFromFileUrl(trimmed);

        if (trimmed.Contains("://", StringComparison.Ordinal))
            return null;

        var isPathLike = Path.IsPathRooted(trimmed)
            || trimmed.StartsWith(".", StringComparison.Ordinal)
            || trimmed.Contains(Path.DirectorySeparatorChar)
            || trimmed.Contains(Path.AltDirectorySeparatorChar)
            || trimmed.EndsWith(".holon", StringComparison.OrdinalIgnoreCase);
        if (!isPathLike)
            return null;

        return Path.IsPathRooted(trimmed)
            ? trimmed
            : Path.Combine(resolveRoot(), trimmed);
    }

    private static HolonRef? DiscoverRefAtPath(string path, int timeout)
    {
        var absolutePath = Path.GetFullPath(path);
        if (!Directory.Exists(absolutePath) && !File.Exists(absolutePath))
            return null;

        if (Directory.Exists(absolutePath))
        {
            if (absolutePath.EndsWith(".holon", StringComparison.OrdinalIgnoreCase)
                || File.Exists(Path.Combine(absolutePath, ".holon.json")))
            {
                try
                {
                    return LoadPackageEntry(Path.GetDirectoryName(absolutePath) ?? absolutePath, absolutePath).Ref;
                }
                catch
                {
                    try
                    {
                        return ProbePackageEntry(Path.GetDirectoryName(absolutePath) ?? absolutePath, absolutePath, timeout).Ref;
                    }
                    catch (Exception ex)
                    {
                        return new HolonRef(FileUrl(absolutePath), null, ex.Message);
                    }
                }
            }

            var result = DiscoverSourceWithLocalOp(LOCAL, null, absolutePath, SOURCE, NO_LIMIT, timeout);
            if (result.Error is not null)
                throw new InvalidOperationException(result.Error);

            if (result.Found.Count == 1)
                return result.Found[0];

            return result.Found.FirstOrDefault(reference =>
                string.Equals(PathFromRefUrl(reference.Url), absolutePath, StringComparison.Ordinal));
        }

        if (string.Equals(Path.GetFileName(absolutePath), Identity.ProtoManifestFileName, StringComparison.Ordinal))
        {
            var root = Path.GetDirectoryName(absolutePath) ?? absolutePath;
            var result = DiscoverSourceWithLocalOp(LOCAL, null, root, SOURCE, NO_LIMIT, timeout);
            if (result.Error is not null)
                throw new InvalidOperationException(result.Error);

            if (result.Found.Count == 1)
                return result.Found[0];

            return result.Found.FirstOrDefault(reference =>
                string.Equals(PathFromRefUrl(reference.Url), root, StringComparison.Ordinal));
        }

        try
        {
            var info = ConnectionInternals.DescribeBinaryTarget(absolutePath, timeout);
            return new HolonRef(FileUrl(absolutePath), info, null);
        }
        catch (Exception ex)
        {
            return new HolonRef(FileUrl(absolutePath), null, ex.Message);
        }
    }

    private static DiscoverResult RequireSourceDiscoveryResult(DiscoverResult result)
    {
        if (result.Error is not null)
            throw new InvalidOperationException(result.Error);
        return result;
    }

    private static IReadOnlyList<DiscoveredEntry> EntriesFromRefs(string root, IReadOnlyList<HolonRef> references)
    {
        var entries = new List<DiscoveredEntry>(references.Count);
        foreach (var reference in references)
        {
            var path = PathFromRefUrl(reference.Url);
            var dirPath = path.Length > 0 ? path : reference.Url;
            entries.Add(new DiscoveredEntry(reference, dirPath, RelativePath(root, dirPath)));
        }

        return entries;
    }

    private static bool MatchesExpression(DiscoveredEntry entry, string? expression)
    {
        if (expression is null)
            return true;

        var needle = expression.Trim();
        if (needle.Length == 0)
            return false;

        if (entry.Ref.Info is not null)
        {
            if (string.Equals(entry.Ref.Info.Slug, needle, StringComparison.Ordinal))
                return true;
            if (entry.Ref.Info.Uuid.StartsWith(needle, StringComparison.Ordinal))
                return true;
            if (entry.Ref.Info.Identity.Aliases.Contains(needle, StringComparer.Ordinal))
                return true;
        }

        var baseName = Path.GetFileName(entry.DirPath.TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar));
        if (baseName.EndsWith(".holon", StringComparison.OrdinalIgnoreCase))
            baseName = baseName[..^".holon".Length];
        return string.Equals(baseName, needle, StringComparison.Ordinal);
    }

    private static IReadOnlyList<string> PackageDirsDirect(string root)
    {
        var absoluteRoot = NormalizeSearchRoot(root);
        if (!Directory.Exists(absoluteRoot))
            return Array.Empty<string>();

        return Directory.EnumerateDirectories(absoluteRoot, "*.holon", SearchOption.TopDirectoryOnly)
            .OrderBy(path => path, StringComparer.Ordinal)
            .ToArray();
    }

    private static IReadOnlyList<string> PackageDirsRecursive(string root)
    {
        var absoluteRoot = NormalizeSearchRoot(root);
        if (!Directory.Exists(absoluteRoot))
            return Array.Empty<string>();

        var directories = new List<string>();
        WalkDirectories(absoluteRoot, absoluteRoot, directories);
        directories.Sort(StringComparer.Ordinal);
        return directories;
    }

    private static void WalkDirectories(string root, string current, List<string> directories)
    {
        IEnumerable<string> children;
        try
        {
            children = Directory.EnumerateDirectories(current);
        }
        catch
        {
            return;
        }

        foreach (var child in children.OrderBy(path => path, StringComparer.Ordinal))
        {
            var name = Path.GetFileName(child);
            if (name.EndsWith(".holon", StringComparison.OrdinalIgnoreCase))
            {
                directories.Add(child);
                continue;
            }

            if (ShouldSkipDirectory(root, child, name))
                continue;

            WalkDirectories(root, child, directories);
        }
    }

    private static bool ShouldSkipDirectory(string root, string path, string name)
    {
        if (string.Equals(Path.GetFullPath(path), Path.GetFullPath(root), StringComparison.Ordinal))
            return false;
        if (name.EndsWith(".holon", StringComparison.OrdinalIgnoreCase))
            return false;
        if (ExcludedDirs.Contains(name))
            return true;
        return name.StartsWith(".", StringComparison.Ordinal);
    }

    private static string ResolveDiscoverRoot(string? root)
    {
        if (root is null)
            return Path.GetFullPath(Directory.GetCurrentDirectory());

        var trimmed = root.Trim();
        if (trimmed.Length == 0)
            throw new InvalidOperationException("root cannot be empty");

        var resolved = Path.GetFullPath(trimmed);
        if (!Directory.Exists(resolved))
            throw new InvalidOperationException($"root \"{trimmed}\" is not a directory");
        return resolved;
    }

    private static string NormalizeSearchRoot(string root)
    {
        var trimmed = root.Trim();
        return trimmed.Length == 0
            ? Path.GetFullPath(Directory.GetCurrentDirectory())
            : Path.GetFullPath(trimmed);
    }

    private static string RelativePath(string root, string path)
    {
        try
        {
            var relative = Path.GetRelativePath(root, path).Replace('\\', '/');
            return string.IsNullOrEmpty(relative) ? "." : relative;
        }
        catch
        {
            return path.Replace('\\', '/');
        }
    }

    private static int PathDepth(string relativePath)
    {
        var trimmed = (relativePath ?? string.Empty).Trim().Trim('/');
        if (trimmed.Length == 0 || trimmed == ".")
            return 0;
        return trimmed.Split('/', StringSplitOptions.RemoveEmptyEntries).Length;
    }

    private static string BundleHolonsRoot()
    {
        var executable = Environment.GetEnvironmentVariable("HOLONS_EXECUTABLE_PATH");
        if (string.IsNullOrWhiteSpace(executable))
            executable = Environment.ProcessPath;
        if (string.IsNullOrWhiteSpace(executable))
            return string.Empty;

        var current = new DirectoryInfo(Path.GetFullPath(executable)).Parent;
        while (current is not null)
        {
            if (current.Name.EndsWith(".app", StringComparison.OrdinalIgnoreCase))
            {
                var candidate = Path.Combine(current.FullName, "Contents", "Resources", "Holons");
                if (Directory.Exists(candidate))
                    return candidate;
            }
            current = current.Parent;
        }

        return string.Empty;
    }

    private static string OpPath()
    {
        var configured = Environment.GetEnvironmentVariable("OPPATH");
        if (!string.IsNullOrWhiteSpace(configured))
            return Path.GetFullPath(configured);
        return Path.GetFullPath(Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
            ".op"));
    }

    private static string OpBin()
    {
        var configured = Environment.GetEnvironmentVariable("OPBIN");
        if (!string.IsNullOrWhiteSpace(configured))
            return Path.GetFullPath(configured);
        return Path.GetFullPath(Path.Combine(OpPath(), "bin"));
    }

    private static string CacheDir() => Path.Combine(OpPath(), "cache");

    private static IReadOnlyList<string> PackageArchitectures(string packageDir)
    {
        var binRoot = Path.Combine(packageDir, "bin");
        if (!Directory.Exists(binRoot))
            return Array.Empty<string>();

        return Directory.EnumerateDirectories(binRoot)
            .Select(Path.GetFileName)
            .Where(name => !string.IsNullOrWhiteSpace(name))
            .Cast<string>()
            .OrderBy(name => name, StringComparer.Ordinal)
            .ToArray();
    }

    internal static string PackageArchDirectory()
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

    private static string FileUrl(string path) => new Uri(Path.GetFullPath(path)).AbsoluteUri;

    private static string PathFromRefUrl(string rawUrl)
    {
        if (rawUrl.StartsWith("file://", StringComparison.OrdinalIgnoreCase))
            return PathFromFileUrl(rawUrl);
        return string.Empty;
    }

    internal static string PathFromFileUrl(string rawUrl)
    {
        var uri = new Uri(rawUrl, UriKind.Absolute);
        if (!string.Equals(uri.Scheme, "file", StringComparison.OrdinalIgnoreCase))
            throw new InvalidOperationException($"holon URL \"{rawUrl}\" is not a local file target");
        if (string.IsNullOrWhiteSpace(uri.LocalPath))
            throw new InvalidOperationException($"holon URL \"{rawUrl}\" has no path");
        return Path.GetFullPath(Uri.UnescapeDataString(uri.LocalPath));
    }

    private static string SlugFor(string givenName, string familyName)
    {
        var given = (givenName ?? string.Empty).Trim();
        var family = (familyName ?? string.Empty).Trim().TrimEnd('?');
        if (given.Length == 0 && family.Length == 0)
            return string.Empty;

        return $"{given}-{family}"
            .Trim()
            .ToLowerInvariant()
            .Replace(" ", "-", StringComparison.Ordinal)
            .Trim('-');
    }

    private static IReadOnlyList<string> StringList(IEnumerable<string>? values)
    {
        if (values is null)
            return Array.Empty<string>();

        return values
            .Select(Trimmed)
            .Where(value => value.Length > 0)
            .ToArray();
    }

    private static string Trimmed(string? value) => (value ?? string.Empty).Trim();

    private static IReadOnlyList<HolonRef> ApplyRefLimit(IReadOnlyList<HolonRef> references, int limit)
    {
        if (limit <= 0 || references.Count <= limit)
            return references;
        return references.Take(limit).ToArray();
    }

    private static string? NormalizedExpression(string? expression) => expression?.Trim();

    private static string EntryKey(DiscoveredEntry entry)
    {
        if (entry.Ref.Info is not null && !string.IsNullOrWhiteSpace(entry.Ref.Info.Uuid))
            return entry.Ref.Info.Uuid.Trim();
        if (!string.IsNullOrWhiteSpace(entry.DirPath))
            return Path.GetFullPath(entry.DirPath);
        return entry.Ref.Url.Trim();
    }

    private static string EntrySortKey(DiscoveredEntry entry)
    {
        if (entry.Ref.Info is not null && !string.IsNullOrWhiteSpace(entry.Ref.Info.Uuid))
            return entry.Ref.Info.Uuid.Trim();
        return entry.Ref.Url;
    }

    private static bool ShouldReplace(DiscoveredEntry current, DiscoveredEntry nextEntry) =>
        PathDepth(nextEntry.RelativePath) < PathDepth(current.RelativePath);

    private static DiscoverResult DiscoverSourceWithLocalOp(
        int scope,
        string? expression,
        string root,
        int specifiers,
        int limit,
        int timeout)
    {
        if (scope != LOCAL)
            return new DiscoverResult(Array.Empty<HolonRef>(), $"scope {scope} not supported");

        if (specifiers != SOURCE)
            return new DiscoverResult(Array.Empty<HolonRef>(), $"invalid source bridge specifiers 0x{specifiers:X2}");

        var startInfo = new ProcessStartInfo("op")
        {
            WorkingDirectory = Path.GetFullPath(root),
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        startInfo.ArgumentList.Add("discover");
        startInfo.ArgumentList.Add("--json");

        try
        {
            using var process = new Process { StartInfo = startInfo };
            if (!process.Start())
                return new DiscoverResult(Array.Empty<HolonRef>(), null);

            var timeoutMilliseconds = timeout <= 0 ? Timeout.Infinite : timeout;
            if (!process.WaitForExit(timeoutMilliseconds))
            {
                TryKillProcess(process);
                return new DiscoverResult(Array.Empty<HolonRef>(), null);
            }

            var stdout = process.StandardOutput.ReadToEnd();
            if (process.ExitCode != 0)
                return new DiscoverResult(Array.Empty<HolonRef>(), null);

            using var document = JsonDocument.Parse(string.IsNullOrWhiteSpace(stdout) ? "{}" : stdout);
            if (!document.RootElement.TryGetProperty("entries", out var entriesElement)
                || entriesElement.ValueKind != JsonValueKind.Array)
            {
                return new DiscoverResult(Array.Empty<HolonRef>(), null);
            }

            var absoluteRoot = Path.GetFullPath(root);
            var references = new List<HolonRef>();

            foreach (var entry in entriesElement.EnumerateArray())
            {
                if (entry.ValueKind != JsonValueKind.Object)
                    continue;

                var identityElement = entry.TryGetProperty("identity", out var value) && value.ValueKind == JsonValueKind.Object
                    ? value
                    : default;
                var givenName = JsonString(identityElement, "givenName", "given_name");
                var familyName = JsonString(identityElement, "familyName", "family_name");
                var identity = new IdentityInfo(
                    givenName,
                    familyName,
                    JsonString(identityElement, "motto"),
                    JsonStringArray(identityElement, "aliases"));

                var relativePath = JsonString(entry, "relativePath", "relative_path");
                var url = relativePath.Length == 0
                    ? FileUrl(absoluteRoot)
                    : FileUrl(Path.Combine(absoluteRoot, relativePath));

                references.Add(new HolonRef(
                    url,
                    new HolonInfo(
                        SlugFor(givenName, familyName),
                        JsonString(identityElement, "uuid"),
                        identity,
                        JsonString(identityElement, "lang"),
                        string.Empty,
                        JsonString(identityElement, "status"),
                        string.Empty,
                        string.Empty,
                        string.Empty,
                        Array.Empty<string>(),
                        false,
                        true),
                    null));
            }

            if (expression is not null)
            {
                references = references
                    .Where(reference => MatchesExpression(EntriesFromRefs(absoluteRoot, new[] { reference })[0], expression))
                    .ToList();
            }

            return new DiscoverResult(ApplyRefLimit(references, limit), null);
        }
        catch
        {
            return new DiscoverResult(Array.Empty<HolonRef>(), null);
        }
    }

    private static string JsonString(JsonElement element, params string[] names)
    {
        foreach (var name in names)
        {
            if (element.ValueKind == JsonValueKind.Object
                && element.TryGetProperty(name, out var value)
                && value.ValueKind == JsonValueKind.String)
            {
                return value.GetString()?.Trim() ?? string.Empty;
            }
        }

        return string.Empty;
    }

    private static IReadOnlyList<string> JsonStringArray(JsonElement element, string name)
    {
        if (element.ValueKind != JsonValueKind.Object
            || !element.TryGetProperty(name, out var value)
            || value.ValueKind != JsonValueKind.Array)
        {
            return Array.Empty<string>();
        }

        return value.EnumerateArray()
            .Where(item => item.ValueKind == JsonValueKind.String)
            .Select(item => item.GetString()?.Trim() ?? string.Empty)
            .Where(item => item.Length > 0)
            .ToArray();
    }

    private static void TryKillProcess(Process process)
    {
        try
        {
            if (!process.HasExited)
                process.Kill(entireProcessTree: true);
        }
        catch
        {
            // Best effort.
        }
    }
}
