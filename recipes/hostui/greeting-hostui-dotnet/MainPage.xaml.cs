using Greeting.Godotnet.Services;
using Microsoft.Maui.Controls;

namespace Greeting.Godotnet;

public partial class MainPage : ContentPage
{
    private readonly DaemonProcess _daemon = new();
    private readonly CancellationTokenSource _lifetime = new();
    private GreetingClient? _client;
    private bool _initialized;
    private bool _shutdownStarted;

    public MainPage()
    {
        InitializeComponent();
    }

    protected override async void OnAppearing()
    {
        base.OnAppearing();

        if (_initialized)
        {
            return;
        }

        _initialized = true;
        try
        {
            await _daemon.StartAsync(_lifetime.Token);
            _client = _daemon.Client;

            var languages = await _client.ListLanguagesAsync(_lifetime.Token);
            var items = languages.ToList();
            LanguagePicker.ItemsSource = items;
            LanguagePicker.ItemDisplayBinding = new Binding(nameof(GreetingLanguageItem.Label));
            LanguagePicker.SelectedItem = items.FirstOrDefault();
            StatusLabel.Text = $"Connected via {_daemon.HolonSlug}";
        }
        catch (Exception ex)
        {
            StatusLabel.Text = ex.Message;
        }
    }

    private async void OnSayHelloClicked(object sender, EventArgs e)
    {
        if (_client is null)
        {
            GreetingLabel.Text = "The gRPC client is not ready.";
            return;
        }

        if (LanguagePicker.SelectedItem is not GreetingLanguageItem language)
        {
            GreetingLabel.Text = "Select a language first.";
            return;
        }

        try
        {
            var result = await _client.SayHelloAsync(
                string.IsNullOrWhiteSpace(NameEntry.Text) ? "Gudule" : NameEntry.Text.Trim(),
                language.Code,
                _lifetime.Token);
            GreetingLabel.Text = result.Greeting;
            StatusLabel.Text = $"{result.Language} ({result.LanguageCode})";
        }
        catch (Exception ex)
        {
            GreetingLabel.Text = ex.Message;
        }
    }

    public async Task ShutdownAsync()
    {
        if (_shutdownStarted)
        {
            return;
        }

        _shutdownStarted = true;
        _client = null;

        if (!_lifetime.IsCancellationRequested)
        {
            _lifetime.Cancel();
        }

        await _daemon.DisposeAsync();
        _lifetime.Dispose();
    }
}
