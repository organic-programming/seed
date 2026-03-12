using Grpc.Net.Client;
using Greeting.V1;

namespace Greeting.Godotnet.Services;

public sealed record GreetingLanguageItem(string Code, string Name, string Native)
{
    public string Label => string.IsNullOrWhiteSpace(Native) ? Name : $"{Native} ({Name})";
}

public sealed record GreetingResultItem(string Greeting, string Language, string LanguageCode);

public sealed class GreetingClient
{
    private readonly GreetingService.GreetingServiceClient _client;

    public GreetingClient(GrpcChannel channel)
    {
        _client = new GreetingService.GreetingServiceClient(channel);
    }

    public async Task<IReadOnlyList<GreetingLanguageItem>> ListLanguagesAsync(CancellationToken cancellationToken = default)
    {
        var response = await _client.ListLanguagesAsync(
            new ListLanguagesRequest(),
            cancellationToken: cancellationToken).ResponseAsync;
        return response.Languages
            .Select(language => new GreetingLanguageItem(language.Code, language.Name, language.Native))
            .ToList();
    }

    public async Task<GreetingResultItem> SayHelloAsync(string name, string languageCode, CancellationToken cancellationToken = default)
    {
        var response = await _client.SayHelloAsync(
            new SayHelloRequest
            {
                Name = name,
                LangCode = languageCode,
            },
            cancellationToken: cancellationToken).ResponseAsync;

        return new GreetingResultItem(response.Greeting, response.Language, response.LangCode);
    }
}
