# csharp-hello-world

A minimal holon implementing `HelloService.Greet` in C# with
`csharp-holons` serve parsing and `ConnectTarget()`.

## Run

```bash
dotnet run --project HelloWorld
dotnet run --project HelloWorld -- Alice
dotnet run --project HelloWorld -- serve --listen tcp://127.0.0.1:9090
```

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

## Connect example

```bash
dotnet run --project HelloWorld -- connect-example
# → {"message":"hello-from-csharp","sdk":"csharp-holons","version":"0.1.0"}
```
