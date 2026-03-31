using System.Text.Json;

namespace Holons.Tests;

public class DiscoverTests
{
    [Fact]
    public void DiscoverAllLayers()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "cwd-alpha.holon"),
            new PackageSeed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"));
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, ".op", "build", "built-beta.holon"),
            new PackageSeed("built-beta", "uuid-built-beta", "Built", "Beta"));
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpBin, "installed-gamma.holon"),
            new PackageSeed("installed-gamma", "uuid-installed-gamma", "Installed", "Gamma"));
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpHome, "cache", "deps", "cached-delta.holon"),
            new PackageSeed("cached-delta", "uuid-cached-delta", "Cached", "Delta"));

        var executable = Path.Combine(env.Root, "TestApp.app", "Contents", "MacOS", "TestApp");
        Directory.CreateDirectory(Path.GetDirectoryName(executable)!);
        File.WriteAllText(executable, "#!/bin/sh\n");
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "TestApp.app", "Contents", "Resources", "Holons", "bundle.holon"),
            new PackageSeed("bundle", "uuid-bundle", "Bundle", "Holon"));
        env.SetExecutablePath(executable);

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.ALL, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(
            new[] { "built-beta", "bundle", "cached-delta", "cwd-alpha", "installed-gamma" },
            DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void FilterBySpecifiers()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "cwd-alpha.holon"),
            new PackageSeed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"));
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, ".op", "build", "built-beta.holon"),
            new PackageSeed("built-beta", "uuid-built-beta", "Built", "Beta"));
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpBin, "installed-gamma.holon"),
            new PackageSeed("installed-gamma", "uuid-installed-gamma", "Installed", "Gamma"));

        var result = Discovery.Discover(
            Discovery.LOCAL,
            null,
            env.Root,
            Discovery.BUILT | Discovery.INSTALLED,
            Discovery.NO_LIMIT,
            Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "built-beta", "installed-gamma" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void MatchBySlug()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "alpha.holon"),
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "beta.holon"),
            new PackageSeed("beta", "uuid-beta", "Beta", "Two"));

        var result = Discovery.Discover(Discovery.LOCAL, "beta", env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "beta" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void MatchByAlias()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "alpha.holon"),
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One", Aliases: new[] { "first" }));

        var result = Discovery.Discover(Discovery.LOCAL, "first", env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "alpha" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void MatchByUuidPrefix()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "alpha.holon"),
            new PackageSeed("alpha", "12345678-aaaa", "Alpha", "One"));

        var result = Discovery.Discover(Discovery.LOCAL, "12345678", env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "alpha" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void MatchByPath()
    {
        using var env = new RuntimeEnvironment();
        var packageDir = Path.Combine(env.Root, "nested", "alpha.holon");
        DiscoveryTestSupport.WritePackageHolon(
            packageDir,
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));

        var result = Discovery.Discover(Discovery.LOCAL, "nested/alpha.holon", env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        var found = Assert.Single(result.Found);
        Assert.Equal(DiscoveryTestSupport.FileUrl(packageDir), found.Url);
    }

    [Fact]
    public void LimitOne()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "alpha.holon"), new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "beta.holon"), new PackageSeed("beta", "uuid-beta", "Beta", "Two"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, 1, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Single(result.Found);
    }

    [Fact]
    public void LimitZeroMeansUnlimited()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "alpha.holon"), new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "beta.holon"), new PackageSeed("beta", "uuid-beta", "Beta", "Two"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, 0, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(2, result.Found.Count);
    }

    [Fact]
    public void NegativeLimitReturnsEmpty()
    {
        using var env = new RuntimeEnvironment();

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, -1, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Empty(result.Found);
    }

    [Fact]
    public void InvalidSpecifiers()
    {
        using var env = new RuntimeEnvironment();

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, 0xFF, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.NotNull(result.Error);
        Assert.Empty(result.Found);
    }

    [Fact]
    public void SpecifiersZeroTreatedAsAll()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "cwd-alpha.holon"), new PackageSeed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"));
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, ".op", "build", "built-beta.holon"), new PackageSeed("built-beta", "uuid-built-beta", "Built", "Beta"));
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.OpBin, "installed-gamma.holon"), new PackageSeed("installed-gamma", "uuid-installed-gamma", "Installed", "Gamma"));
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.OpHome, "cache", "cached-delta.holon"), new PackageSeed("cached-delta", "uuid-cached-delta", "Cached", "Delta"));

        var allResult = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.ALL, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);
        var zeroResult = Discovery.Discover(Discovery.LOCAL, null, env.Root, 0, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(allResult.Error);
        Assert.Null(zeroResult.Error);
        Assert.Equal(DiscoveryTestSupport.SortedSlugs(allResult), DiscoveryTestSupport.SortedSlugs(zeroResult));
    }

    [Fact]
    public void NullExpressionReturnsAll()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "alpha.holon"), new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "beta.holon"), new PackageSeed("beta", "uuid-beta", "Beta", "Two"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(2, result.Found.Count);
    }

    [Fact]
    public void MissingExpressionReturnsEmpty()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(Path.Combine(env.Root, "alpha.holon"), new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));

        var result = Discovery.Discover(Discovery.LOCAL, "missing", env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Empty(result.Found);
    }

    [Fact]
    public void ExcludedDirsSkipped()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "kept", "alpha.holon"),
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));

        foreach (var skipped in new[]
        {
            Path.Combine(env.Root, ".git", "hidden", "ignored.holon"),
            Path.Combine(env.Root, ".op", "hidden", "ignored.holon"),
            Path.Combine(env.Root, "node_modules", "hidden", "ignored.holon"),
            Path.Combine(env.Root, "vendor", "hidden", "ignored.holon"),
            Path.Combine(env.Root, "build", "hidden", "ignored.holon"),
            Path.Combine(env.Root, "testdata", "hidden", "ignored.holon"),
            Path.Combine(env.Root, ".cache", "hidden", "ignored.holon"),
        })
        {
            DiscoveryTestSupport.WritePackageHolon(
                skipped,
                new PackageSeed($"ignored-{Path.GetFileName(Path.GetDirectoryName(skipped)!)}", Guid.NewGuid().ToString("N"), "Ignored", "Holon"));
        }

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "alpha" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void DeduplicateByUuid()
    {
        using var env = new RuntimeEnvironment();
        var cwdPath = Path.Combine(env.Root, "alpha.holon");
        var builtPath = Path.Combine(env.Root, ".op", "build", "alpha-built.holon");
        DiscoveryTestSupport.WritePackageHolon(cwdPath, new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));
        DiscoveryTestSupport.WritePackageHolon(builtPath, new PackageSeed("alpha-built", "uuid-alpha", "Alpha", "One"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.ALL, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        var found = Assert.Single(result.Found);
        Assert.Equal(DiscoveryTestSupport.FileUrl(cwdPath), found.Url);
    }

    [Fact]
    public void HolonJsonFastPath()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "alpha.holon"),
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"),
            withHolonJson: true,
            withBinary: false);

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        var found = Assert.Single(result.Found);
        Assert.Equal("alpha", found.Info?.Slug);
    }

    [Fact]
    public void DescribeFallbackWhenHolonJsonMissing()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "alpha.holon"),
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"),
            withHolonJson: false,
            withBinary: true);

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CWD, Discovery.NO_LIMIT, 5000);

        Assert.Null(result.Error);
        var found = Assert.Single(result.Found);
        Assert.NotNull(found.Info);
        Assert.Equal("static-fixture", found.Info!.Slug);
    }

    [Fact]
    public void SiblingsLayer()
    {
        using var env = new RuntimeEnvironment();
        var executable = Path.Combine(env.Root, "TestApp.app", "Contents", "MacOS", "TestApp");
        Directory.CreateDirectory(Path.GetDirectoryName(executable)!);
        File.WriteAllText(executable, "#!/bin/sh\n");
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "TestApp.app", "Contents", "Resources", "Holons", "bundle.holon"),
            new PackageSeed("bundle", "uuid-bundle", "Bundle", "Holon"));
        env.SetExecutablePath(executable);

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.SIBLINGS, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "bundle" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void SourceLayerOffloadsToLocalOp()
    {
        using var env = new RuntimeEnvironment();
        var sourceDir = Path.Combine(env.Root, "source-holon");
        Directory.CreateDirectory(sourceDir);
        var toolsDir = Path.Combine(env.Root, "tools");
        var cwdFile = Path.Combine(env.Root, "op.cwd");
        var argsFile = Path.Combine(env.Root, "op.args");
        var payload = JsonSerializer.Serialize(new
        {
            entries = new[]
            {
                new
                {
                    identity = new
                    {
                        uuid = "uuid-source-alpha",
                        givenName = "Source",
                        familyName = "Alpha",
                        status = "draft",
                        lang = "csharp",
                    },
                    relativePath = "source-holon",
                },
            },
        });
        DiscoveryTestSupport.WriteFakeOpScript(toolsDir, cwdFile, argsFile, payload);
        env.PrependPath(toolsDir);

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.SOURCE, Discovery.NO_LIMIT, 5000);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "source-alpha" }, DiscoveryTestSupport.SortedSlugs(result));
        Assert.Equal(
            Path.GetFileName(Path.GetFullPath(env.Root)),
            Path.GetFileName(File.ReadAllText(cwdFile).Trim()));
        Assert.Equal(new[] { "discover", "--json" }, File.ReadAllLines(argsFile).Where(line => line.Length > 0).ToArray());
    }

    [Fact]
    public void BuiltLayer()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, ".op", "build", "built.holon"),
            new PackageSeed("built", "uuid-built", "Built", "Holon"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.BUILT, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "built" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void InstalledLayer()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpBin, "installed.holon"),
            new PackageSeed("installed", "uuid-installed", "Installed", "Holon"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.INSTALLED, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "installed" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void CachedLayer()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpHome, "cache", "deep", "cached.holon"),
            new PackageSeed("cached", "uuid-cached", "Cached", "Holon"));

        var result = Discovery.Discover(Discovery.LOCAL, null, env.Root, Discovery.CACHED, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "cached" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void NilRootDefaultsToCwd()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.Root, "alpha.holon"),
            new PackageSeed("alpha", "uuid-alpha", "Alpha", "One"));
        env.SetCurrentDirectory(env.Root);

        var result = Discovery.Discover(Discovery.LOCAL, null, null, Discovery.CWD, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.Equal(new[] { "alpha" }, DiscoveryTestSupport.SortedSlugs(result));
    }

    [Fact]
    public void EmptyRootReturnsError()
    {
        var result = Discovery.Discover(Discovery.LOCAL, null, string.Empty, Discovery.ALL, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.NotNull(result.Error);
        Assert.Empty(result.Found);
    }

    [Fact]
    public void UnsupportedScopeReturnsError()
    {
        using var env = new RuntimeEnvironment();

        var proxyResult = Discovery.Discover(Discovery.PROXY, null, env.Root, Discovery.ALL, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);
        var delegatedResult = Discovery.Discover(Discovery.DELEGATED, null, env.Root, Discovery.ALL, Discovery.NO_LIMIT, Discovery.NO_TIMEOUT);

        Assert.NotNull(proxyResult.Error);
        Assert.NotNull(delegatedResult.Error);
    }
}
