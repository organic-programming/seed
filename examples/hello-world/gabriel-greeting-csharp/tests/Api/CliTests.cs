using System.Text.Json;
using GabrielGreeting.Csharp.Api;

namespace GabrielGreeting.Csharp.Tests.Api;

public class CliTests
{
    [Fact]
    public async Task RunPrintsVersion()
    {
        using var stdout = new StringWriter();
        using var stderr = new StringWriter();

        var exitCode = await Cli.RunAsync(["version"], stdout, stderr);

        Assert.Equal(0, exitCode);
        Assert.Equal(Cli.Version, stdout.ToString().Trim());
        Assert.Equal(string.Empty, stderr.ToString());
    }

    [Fact]
    public async Task RunPrintsHelp()
    {
        using var stdout = new StringWriter();
        using var stderr = new StringWriter();

        var exitCode = await Cli.RunAsync(["help"], stdout, stderr);

        Assert.Equal(0, exitCode);
        Assert.Contains("usage: gabriel-greeting-csharp", stdout.ToString());
        Assert.Contains("listLanguages", stdout.ToString());
        Assert.Equal(string.Empty, stderr.ToString());
    }

    [Fact]
    public async Task RunRendersListLanguagesJson()
    {
        using var stdout = new StringWriter();
        using var stderr = new StringWriter();

        var exitCode = await Cli.RunAsync(["listLanguages", "--format", "json"], stdout, stderr);
        using var document = JsonDocument.Parse(stdout.ToString());
        var languages = document.RootElement.GetProperty("languages");

        Assert.Equal(0, exitCode);
        Assert.Equal(56, languages.GetArrayLength());
        Assert.Equal("en", languages[0].GetProperty("code").GetString());
        Assert.Equal("English", languages[0].GetProperty("name").GetString());
        Assert.Equal(string.Empty, stderr.ToString());
    }

    [Fact]
    public async Task RunRendersSayHelloText()
    {
        using var stdout = new StringWriter();
        using var stderr = new StringWriter();

        var exitCode = await Cli.RunAsync(["sayHello", "Bob", "fr"], stdout, stderr);

        Assert.Equal(0, exitCode);
        Assert.Equal("Bonjour Bob", stdout.ToString().Trim());
        Assert.Equal(string.Empty, stderr.ToString());
    }

    [Fact]
    public async Task RunDefaultsSayHelloToEnglishJson()
    {
        using var stdout = new StringWriter();
        using var stderr = new StringWriter();

        var exitCode = await Cli.RunAsync(["sayHello", "--json"], stdout, stderr);
        using var document = JsonDocument.Parse(stdout.ToString());

        Assert.Equal(0, exitCode);
        Assert.Equal("Hello Mary", document.RootElement.GetProperty("greeting").GetString());
        Assert.Equal("English", document.RootElement.GetProperty("language").GetString());
        Assert.Equal("en", document.RootElement.GetProperty("langCode").GetString());
        Assert.Equal(string.Empty, stderr.ToString());
    }
}
