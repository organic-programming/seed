---
# Cartouche v1
title: "csharp-hello-world — Hello World Holon in C#"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
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
