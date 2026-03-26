using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;

namespace Holons;

/// <summary>Discover holons by scanning for holon.proto manifests.</summary>
public static class Discover
{
    private static readonly Regex ApiVersionDir = new(
        @"^v[0-9]+(?:[A-Za-z0-9._-]*)?$",
        RegexOptions.Compiled);

    public record HolonBuild
    {
        public string Runner { get; init; } = "";
        public string Main { get; init; } = "";
    }

    public record HolonArtifacts
    {
        public string Binary { get; init; } = "";
        public string Primary { get; init; } = "";
    }

    public record HolonManifest
    {
        public string Kind { get; init; } = "";
        public HolonBuild Build { get; init; } = new();
        public HolonArtifacts Artifacts { get; init; } = new();
    }

    public record HolonEntry
    {
        public string Slug { get; init; } = "";
        public string Uuid { get; init; } = "";
        public string Dir { get; init; } = "";
        public string RelativePath { get; init; } = "";
        public string Origin { get; init; } = "";
        public IdentityParser.HolonIdentity Identity { get; init; } = new();
        public HolonManifest? Manifest { get; init; }
    }

    public static List<HolonEntry> DiscoverRoot(string root) => DiscoverInRoot(root, "local");

    public static List<HolonEntry> DiscoverLocal() => DiscoverRoot(Directory.GetCurrentDirectory());

    public static List<HolonEntry> DiscoverAll()
    {
        var entries = new List<HolonEntry>();
        var seen = new HashSet<string>(StringComparer.Ordinal);

        foreach (var spec in new[]
        {
            (Root: Directory.GetCurrentDirectory(), Origin: "local"),
            (Root: OpBin(), Origin: "$OPBIN"),
            (Root: CacheDir(), Origin: "cache"),
        })
        {
            foreach (var entry in DiscoverInRoot(spec.Root, spec.Origin))
            {
                var key = string.IsNullOrWhiteSpace(entry.Uuid) ? entry.Dir : entry.Uuid;
                if (seen.Add(key))
                    entries.Add(entry);
            }
        }

        return entries;
    }

    public static HolonEntry? FindBySlug(string slug)
    {
        var needle = (slug ?? "").Trim();
        if (needle.Length == 0)
            return null;

        HolonEntry? match = null;
        foreach (var entry in DiscoverAll())
        {
            if (!string.Equals(entry.Slug, needle, StringComparison.Ordinal))
                continue;
            if (match is not null && !string.Equals(match.Uuid, entry.Uuid, StringComparison.Ordinal))
                throw new InvalidOperationException($"ambiguous holon \"{needle}\"");
            match = entry;
        }

        return match;
    }

    public static HolonEntry? FindByUUID(string prefix)
    {
        var needle = (prefix ?? "").Trim();
        if (needle.Length == 0)
            return null;

        HolonEntry? match = null;
        foreach (var entry in DiscoverAll())
        {
            if (!entry.Uuid.StartsWith(needle, StringComparison.Ordinal))
                continue;
            if (match is not null && !string.Equals(match.Uuid, entry.Uuid, StringComparison.Ordinal))
                throw new InvalidOperationException($"ambiguous UUID prefix \"{needle}\"");
            match = entry;
        }

        return match;
    }

    private static List<HolonEntry> DiscoverInRoot(string root, string origin)
    {
        var resolvedRoot = Path.GetFullPath(string.IsNullOrWhiteSpace(root) ? Directory.GetCurrentDirectory() : root);
        if (!Directory.Exists(resolvedRoot))
            return new List<HolonEntry>();

        var entriesByKey = new Dictionary<string, HolonEntry>(StringComparer.Ordinal);
        var orderedKeys = new List<string>();
        ScanDirectory(resolvedRoot, resolvedRoot, origin, entriesByKey, orderedKeys);

        return orderedKeys
            .Where(entriesByKey.ContainsKey)
            .Select(key => entriesByKey[key])
            .OrderBy(entry => entry.RelativePath, StringComparer.Ordinal)
            .ThenBy(entry => entry.Uuid, StringComparer.Ordinal)
            .ToList();
    }

    private static void ScanDirectory(
        string root,
        string dir,
        string origin,
        Dictionary<string, HolonEntry> entriesByKey,
        List<string> orderedKeys)
    {
        IEnumerable<string> children;
        try
        {
            children = Directory.EnumerateFileSystemEntries(dir);
        }
        catch
        {
            return;
        }

        foreach (var child in children)
        {
            var name = Path.GetFileName(child);
            if (Directory.Exists(child))
            {
                if (ShouldSkipDir(root, child, name))
                    continue;
                ScanDirectory(root, child, origin, entriesByKey, orderedKeys);
                continue;
            }

            if (!File.Exists(child) || !string.Equals(name, Identity.ProtoManifestFileName, StringComparison.Ordinal))
                continue;

            try
            {
                var resolved = Identity.ResolveProtoFile(child);
                var holonDir = Path.GetFullPath(ManifestRoot(child));
                var entry = new HolonEntry
                {
                    Slug = resolved.Identity.Slug(),
                    Uuid = resolved.Identity.Uuid,
                    Dir = holonDir,
                    RelativePath = RelativePath(root, holonDir),
                    Origin = origin,
                    Identity = resolved.Identity,
                    Manifest = ManifestFromResolved(resolved),
                };

                var key = string.IsNullOrWhiteSpace(entry.Uuid) ? entry.Dir : entry.Uuid;
                if (entriesByKey.TryGetValue(key, out var existing))
                {
                    if (PathDepth(entry.RelativePath) < PathDepth(existing.RelativePath))
                        entriesByKey[key] = entry;
                    continue;
                }

                entriesByKey[key] = entry;
                orderedKeys.Add(key);
            }
            catch
            {
                // Skip invalid holon manifests.
            }
        }
    }

    private static HolonManifest ManifestFromResolved(IdentityParser.ResolvedManifest resolved)
    {
        return new HolonManifest
        {
            Kind = resolved.Kind,
            Build = new HolonBuild
            {
                Runner = resolved.BuildRunner,
                Main = resolved.BuildMain,
            },
            Artifacts = new HolonArtifacts
            {
                Binary = resolved.ArtifactBinary,
                Primary = resolved.ArtifactPrimary,
            },
        };
    }

    private static string ManifestRoot(string manifestPath)
    {
        var manifestDir = Path.GetDirectoryName(manifestPath);
        if (string.IsNullOrWhiteSpace(manifestDir))
            return ".";

        var versionDir = Path.GetFileName(manifestDir);
        var apiDir = Path.GetFileName(Path.GetDirectoryName(manifestDir));
        if (ApiVersionDir.IsMatch(versionDir) && string.Equals(apiDir, "api", StringComparison.Ordinal))
        {
            var holonRoot = Path.GetDirectoryName(Path.GetDirectoryName(manifestDir));
            if (!string.IsNullOrWhiteSpace(holonRoot))
                return holonRoot;
        }
        return manifestDir;
    }

    private static bool ShouldSkipDir(string root, string dir, string name)
    {
        if (string.Equals(Path.GetFullPath(dir), Path.GetFullPath(root), StringComparison.Ordinal))
            return false;

        return name is ".git" or ".op" or "node_modules" or "vendor" or "build"
            || name.StartsWith(".", StringComparison.Ordinal);
    }

    private static string RelativePath(string root, string dir)
    {
        var relative = Path.GetRelativePath(root, dir);
        return string.IsNullOrEmpty(relative) ? "." : relative.Replace('\\', '/');
    }

    private static int PathDepth(string relativePath)
    {
        var trimmed = (relativePath ?? "").Trim().Trim('/');
        if (trimmed.Length == 0 || trimmed == ".")
            return 0;
        return trimmed.Split('/', StringSplitOptions.RemoveEmptyEntries).Length;
    }

    private static string OpPath()
    {
        var configured = Environment.GetEnvironmentVariable("OPPATH");
        if (!string.IsNullOrWhiteSpace(configured))
            return Path.GetFullPath(configured);
        return Path.GetFullPath(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".op"));
    }

    private static string OpBin()
    {
        var configured = Environment.GetEnvironmentVariable("OPBIN");
        if (!string.IsNullOrWhiteSpace(configured))
            return Path.GetFullPath(configured);
        return Path.Combine(OpPath(), "bin");
    }

    private static string CacheDir() => Path.Combine(OpPath(), "cache");
}
