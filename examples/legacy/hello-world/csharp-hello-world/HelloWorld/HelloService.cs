namespace HelloWorld;

/// <summary>Pure deterministic HelloService — no gRPC deps needed for test.</summary>
public static class HelloService
{
    /// <summary>Greet returns a greeting for the given name.</summary>
    public static string Greet(string name)
    {
        var n = string.IsNullOrEmpty(name) ? "World" : name;
        return $"Hello, {n}!";
    }
}
