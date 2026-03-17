namespace Greeting.Godotnet;

public partial class App : Application
{
    public App()
    {
        InitializeComponent();
    }

    protected override Window CreateWindow(IActivationState? activationState)
    {
        var page = new MainPage();
        var window = new Window(page);
        window.Destroying += async (_, _) => await page.ShutdownAsync();
        return window;
    }
}
