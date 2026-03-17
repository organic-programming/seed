using GabrielGreeting.Csharp.Api;
using Greeting.V1;

namespace GabrielGreeting.Csharp.Tests.Api;

public class PublicApiTests
{
    [Fact]
    public void ListLanguagesIncludesEnglish()
    {
        var response = PublicApi.ListLanguages(new ListLanguagesRequest());
        var english = response.Languages.Single(language => language.Code == "en");

        Assert.Equal("English", english.Name);
        Assert.Equal("English", english.Native);
    }

    [Fact]
    public void SayHelloUsesRequestedLanguage()
    {
        var response = PublicApi.SayHello(new SayHelloRequest
        {
            Name = "Alice",
            LangCode = "fr",
        });

        Assert.Equal("Bonjour Alice", response.Greeting);
        Assert.Equal("French", response.Language);
        Assert.Equal("fr", response.LangCode);
    }

    [Fact]
    public void SayHelloUsesLocalizedDefaultName()
    {
        var response = PublicApi.SayHello(new SayHelloRequest
        {
            LangCode = "ja",
        });

        Assert.Equal("こんにちは、マリアさん", response.Greeting);
        Assert.Equal("Japanese", response.Language);
        Assert.Equal("ja", response.LangCode);
    }

    [Fact]
    public void SayHelloFallsBackToEnglish()
    {
        var response = PublicApi.SayHello(new SayHelloRequest
        {
            LangCode = "unknown",
        });

        Assert.Equal("Hello Mary", response.Greeting);
        Assert.Equal("English", response.Language);
        Assert.Equal("en", response.LangCode);
    }
}
