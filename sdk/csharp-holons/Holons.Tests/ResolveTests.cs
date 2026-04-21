namespace Holons.Tests;

public class ResolveTests
{
    [Fact]
    public void KnownSlug()
    {
        using var env = new RuntimeEnvironment();
        var packageDir = Path.Combine(env.Root, "known.holon");
        DiscoveryTestSupport.WritePackageHolon(
            packageDir,
            new PackageSeed("known", "uuid-known", "Known", "Slug"));

        var result = Discovery.Resolve(Discovery.LOCAL, "known", env.Root, Discovery.CWD, Discovery.NO_TIMEOUT);

        Assert.Null(result.Error);
        Assert.NotNull(result.Ref);
        Assert.Equal("known", result.Ref!.Info?.Slug);
        Assert.Equal(DiscoveryTestSupport.FileUrl(packageDir), result.Ref.Url);
    }

    [Fact]
    public void MissingTarget()
    {
        using var env = new RuntimeEnvironment();

        var result = Discovery.Resolve(Discovery.LOCAL, "missing", env.Root, Discovery.CWD, Discovery.NO_TIMEOUT);

        Assert.NotNull(result.Error);
        Assert.Null(result.Ref);
    }

    [Fact]
    public void InvalidSpecifiers()
    {
        using var env = new RuntimeEnvironment();

        var result = Discovery.Resolve(Discovery.LOCAL, "missing", env.Root, 0xFF, Discovery.NO_TIMEOUT);

        Assert.NotNull(result.Error);
        Assert.Null(result.Ref);
    }
}
