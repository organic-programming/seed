using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;

namespace Holons;

public static class Identity
{
    public const string ProtoManifestFileName = "holon.proto";

    public static IdentityParser.HolonIdentity ParseHolon(string path)
    {
        return IdentityParser.ParseHolon(path);
    }

    public static IdentityParser.ResolvedManifest Resolve(string root)
    {
        return ResolveProtoFile(ResolveManifestPath(root));
    }

    public static IdentityParser.ResolvedManifest ResolveProtoFile(string path)
    {
        var resolvedPath = Path.GetFullPath(path);
        if (!string.Equals(Path.GetFileName(resolvedPath), ProtoManifestFileName, StringComparison.Ordinal))
            throw new IOException($"{resolvedPath} is not a {ProtoManifestFileName} file");

        return IdentityParser.ParseManifest(resolvedPath) with { SourcePath = resolvedPath };
    }

    public static string? FindHolonProto(string root)
    {
        return IdentityParser.FindHolonProto(root);
    }

    public static string ResolveManifestPath(string root)
    {
        return IdentityParser.ResolveManifestPath(root);
    }
}

/// <summary>Parse holon.proto identity files.</summary>
public static class IdentityParser
{
    private static readonly Regex ManifestPattern = new(
        @"option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{",
        RegexOptions.Compiled);

    /// <summary>Parsed holon identity.</summary>
    public record HolonIdentity
    {
        public string Uuid { get; init; } = "";
        public string GivenName { get; init; } = "";
        public string FamilyName { get; init; } = "";
        public string Motto { get; init; } = "";
        public string Composer { get; init; } = "";
        public string Clade { get; init; } = "";
        public string Status { get; init; } = "";
        public string Born { get; init; } = "";
        public string Lang { get; init; } = "";
        public List<string> Parents { get; init; } = new();
        public string Reproduction { get; init; } = "";
        public string GeneratedBy { get; init; } = "";
        public string ProtoStatus { get; init; } = "";
        public List<string> Aliases { get; init; } = new();

        public string Slug()
        {
            var given = (GivenName ?? "").Trim();
            var family = (FamilyName ?? "").Trim().TrimEnd('?');
            if (given.Length == 0 && family.Length == 0)
                return "";

            return $"{given}-{family}"
                .Trim()
                .ToLowerInvariant()
                .Replace(" ", "-", StringComparison.Ordinal)
                .Trim('-');
        }
    }

    public record ResolvedManifest
    {
        public HolonIdentity Identity { get; init; } = new();
        public string SourcePath { get; init; } = "";
        public string Kind { get; init; } = "";
        public string BuildRunner { get; init; } = "";
        public string BuildMain { get; init; } = "";
        public string ArtifactBinary { get; init; } = "";
        public string ArtifactPrimary { get; init; } = "";
    }

    /// <summary>Parse a holon.proto file.</summary>
    public static HolonIdentity ParseHolon(string path)
    {
        return ParseManifest(path).Identity;
    }

    public static ResolvedManifest ParseManifest(string path)
    {
        var text = File.ReadAllText(path);
        var manifestBlock = ExtractManifestBlock(text)
            ?? throw new FormatException($"{path}: missing holons.v1.manifest option in holon.proto");

        var identityBlock = ExtractBlock("identity", manifestBlock) ?? "";
        var lineageBlock = ExtractBlock("lineage", manifestBlock) ?? "";
        var buildBlock = ExtractBlock("build", manifestBlock) ?? "";
        var artifactsBlock = ExtractBlock("artifacts", manifestBlock) ?? "";

        return new ResolvedManifest
        {
            Identity = new HolonIdentity
            {
                Uuid = Scalar("uuid", identityBlock),
                GivenName = Scalar("given_name", identityBlock),
                FamilyName = Scalar("family_name", identityBlock),
                Motto = Scalar("motto", identityBlock),
                Composer = Scalar("composer", identityBlock),
                Clade = Scalar("clade", identityBlock),
                Status = Scalar("status", identityBlock),
                Born = Scalar("born", identityBlock),
                Lang = Scalar("lang", manifestBlock),
                Parents = StringList("parents", lineageBlock),
                Reproduction = Scalar("reproduction", lineageBlock),
                GeneratedBy = Scalar("generated_by", lineageBlock),
                ProtoStatus = Scalar("proto_status", identityBlock),
                Aliases = StringList("aliases", identityBlock),
            },
            Kind = Scalar("kind", manifestBlock),
            BuildRunner = Scalar("runner", buildBlock),
            BuildMain = Scalar("main", buildBlock),
            ArtifactBinary = Scalar("binary", artifactsBlock),
            ArtifactPrimary = Scalar("primary", artifactsBlock),
            SourcePath = Path.GetFullPath(path),
        };
    }

    public static string? FindHolonProto(string root)
    {
        var resolved = Path.GetFullPath(root);
        if (File.Exists(resolved))
            return string.Equals(Path.GetFileName(resolved), "holon.proto", StringComparison.Ordinal) ? resolved : null;
        if (!Directory.Exists(resolved))
            return null;

        var direct = Path.Combine(resolved, "holon.proto");
        if (File.Exists(direct))
            return direct;

        var apiV1 = Path.Combine(resolved, "api", "v1", "holon.proto");
        if (File.Exists(apiV1))
            return apiV1;

        return Directory.EnumerateFiles(resolved, "holon.proto", SearchOption.AllDirectories)
            .OrderBy(path => path, StringComparer.Ordinal)
            .FirstOrDefault();
    }

    public static string ResolveManifestPath(string root)
    {
        var resolved = Path.GetFullPath(root);
        var searchRoots = new List<string> { resolved };
        var parent = Path.GetDirectoryName(resolved);
        if (string.Equals(Path.GetFileName(resolved), "protos", StringComparison.Ordinal) && !string.IsNullOrWhiteSpace(parent))
        {
            searchRoots.Add(parent);
        }
        else if (!string.IsNullOrWhiteSpace(parent) && !string.Equals(parent, resolved, StringComparison.Ordinal))
        {
            searchRoots.Add(parent);
        }

        foreach (var searchRoot in searchRoots)
        {
            var candidate = FindHolonProto(searchRoot);
            if (!string.IsNullOrWhiteSpace(candidate))
                return candidate;
        }

        throw new IOException($"no holon.proto found near {resolved}");
    }

    private static string? ExtractManifestBlock(string source)
    {
        var match = ManifestPattern.Match(source);
        if (!match.Success)
            return null;
        var braceIndex = source.IndexOf('{', match.Index);
        return braceIndex >= 0 ? BalancedBlockContents(source, braceIndex) : null;
    }

    private static string? ExtractBlock(string name, string source)
    {
        var match = Regex.Match(source, $@"\b{Regex.Escape(name)}\s*:\s*\{{");
        if (!match.Success)
            return null;
        var braceIndex = source.IndexOf('{', match.Index);
        return braceIndex >= 0 ? BalancedBlockContents(source, braceIndex) : null;
    }

    private static string Scalar(string name, string source)
    {
        var quoted = Regex.Match(source, $@"\b{Regex.Escape(name)}\s*:\s*""((?:[^""\\]|\\.)*)""");
        if (quoted.Success)
            return UnescapeProtoString(quoted.Groups[1].Value);

        var bare = Regex.Match(source, $@"\b{Regex.Escape(name)}\s*:\s*([^\s,\]\}}]+)");
        return bare.Success ? bare.Groups[1].Value : "";
    }

    private static List<string> StringList(string name, string source)
    {
        var body = Regex.Match(source, $@"(?s)\b{Regex.Escape(name)}\s*:\s*\[(.*?)\]");
        if (!body.Success)
            return new List<string>();

        return Regex.Matches(body.Groups[1].Value, @"""((?:[^""\\]|\\.)*)""|([^\s,\]]+)")
            .Select(match => match.Groups[1].Success
                ? UnescapeProtoString(match.Groups[1].Value)
                : match.Groups[2].Value)
            .Where(value => !string.IsNullOrEmpty(value))
            .ToList();
    }

    private static string? BalancedBlockContents(string source, int openingBrace)
    {
        var depth = 0;
        var insideString = false;
        var escaped = false;
        var contentStart = openingBrace + 1;

        for (var index = openingBrace; index < source.Length; index++)
        {
            var ch = source[index];
            if (insideString)
            {
                if (escaped)
                {
                    escaped = false;
                }
                else if (ch == '\\')
                {
                    escaped = true;
                }
                else if (ch == '"')
                {
                    insideString = false;
                }
                continue;
            }

            if (ch == '"')
            {
                insideString = true;
            }
            else if (ch == '{')
            {
                depth += 1;
            }
            else if (ch == '}')
            {
                depth -= 1;
                if (depth == 0)
                    return source[contentStart..index];
            }
        }

        return null;
    }

    private static string UnescapeProtoString(string value)
    {
        return value.Replace("\\\"", "\"", StringComparison.Ordinal)
            .Replace("\\\\", "\\", StringComparison.Ordinal);
    }
}
