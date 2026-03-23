using HelloWorld;

namespace HelloWorld.Tests;

public class HelloServiceTest
{
    [Fact]
    public void GreetWithName()
    {
        Assert.Equal("Hello, Bob!", HelloService.Greet("Bob"));
    }

    [Fact]
    public void GreetDefault()
    {
        Assert.Equal("Hello, World!", HelloService.Greet(""));
    }

    [Fact]
    public void GreetNull()
    {
        Assert.Equal("Hello, World!", HelloService.Greet(null!));
    }
}
