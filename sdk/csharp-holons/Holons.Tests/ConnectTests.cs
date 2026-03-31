using Grpc.Net.Client;

namespace Holons.Tests;

public class ConnectTests
{
    [Fact]
    public async Task UnresolvableTarget()
    {
        using var env = new RuntimeEnvironment();

        var result = await Connector.Connect(Discovery.LOCAL, "missing", env.Root, Discovery.INSTALLED, 1000);

        Assert.NotNull(result.Error);
        Assert.Null(result.Channel);
        Assert.Null(result.Origin);
    }

    [Fact]
    public async Task ReturnsConnectResult()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpBin, "known-slug.holon"),
            new PackageSeed("known-slug", "uuid-known", "Known", "Slug"),
            withHolonJson: true,
            withBinary: true);

        var result = await Connector.Connect(Discovery.LOCAL, "known-slug", env.Root, Discovery.INSTALLED, 5000);
        try
        {
            Assert.IsType<ConnectResult>(result);
            Assert.Null(result.Error);
            var channel = Assert.IsType<GrpcChannel>(result.Channel);
            var response = await DiscoveryTestSupport.InvokeDescribeAsync(channel);
            Assert.Equal("Static", response.Manifest.Identity.GivenName);
        }
        finally
        {
            Connector.Disconnect(result);
        }
    }

    [Fact]
    public async Task PopulatesOrigin()
    {
        using var env = new RuntimeEnvironment();
        var packageRoot = Path.Combine(env.OpBin, "origin-slug.holon");
        DiscoveryTestSupport.WritePackageHolon(
            packageRoot,
            new PackageSeed("origin-slug", "uuid-origin", "Origin", "Slug"),
            withHolonJson: true,
            withBinary: true);

        var result = await Connector.Connect(Discovery.LOCAL, "origin-slug", env.Root, Discovery.INSTALLED, 5000);
        try
        {
            Assert.Null(result.Error);
            Assert.NotNull(result.Origin);
            Assert.NotNull(result.Origin!.Info);
            Assert.Equal("origin-slug", result.Origin.Info!.Slug);
            Assert.Equal(DiscoveryTestSupport.FileUrl(packageRoot), result.Origin.Url);
        }
        finally
        {
            Connector.Disconnect(result);
        }
    }

    [Fact]
    public async Task DisconnectAcceptsConnectResult()
    {
        using var env = new RuntimeEnvironment();
        DiscoveryTestSupport.WritePackageHolon(
            Path.Combine(env.OpBin, "disconnect-slug.holon"),
            new PackageSeed("disconnect-slug", "uuid-disconnect", "Disconnect", "Slug"),
            withHolonJson: true,
            withBinary: true);

        var result = await Connector.Connect(Discovery.LOCAL, "disconnect-slug", env.Root, Discovery.INSTALLED, 5000);

        Assert.Null(result.Error);
        Assert.NotNull(result.Channel);
        Connector.Disconnect(result);
    }
}
