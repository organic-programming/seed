using Greeting.V1;

namespace GabrielGreeting.Csharp.Api;

public static class PublicApi
{
    public static ListLanguagesResponse ListLanguages(ListLanguagesRequest request)
    {
        _ = request;
        var response = new ListLanguagesResponse();
        response.Languages.AddRange(_Internal.Greetings.All.Select(entry => new Language
        {
            Code = entry.Code,
            Name = entry.Name,
            Native = entry.Native,
        }));
        return response;
    }

    public static SayHelloResponse SayHello(SayHelloRequest request)
    {
        var entry = _Internal.Greetings.Lookup(request.LangCode);
        var name = string.IsNullOrWhiteSpace(request.Name) ? entry.DefaultName : request.Name.Trim();
        return new SayHelloResponse
        {
            Greeting = entry.Template.Replace("%s", name, StringComparison.Ordinal),
            Language = entry.Name,
            LangCode = entry.Code,
        };
    }
}
