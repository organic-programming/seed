# csharp-hello-world

A minimal holon implementing HelloService.Greet in C#.

## Test

```bash
dotnet test
```

## Invoke via stdio (zero config)

```bash
dotnet publish -c Release -o out
op grpc+stdio://"dotnet out/HelloWorld.dll" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
