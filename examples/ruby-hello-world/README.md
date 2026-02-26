---
# Cartouche v1
title: "ruby-hello-world — Hello World Holon in Ruby"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
# ruby-hello-world

A minimal holon implementing HelloService.Greet in Ruby.

## Test

```bash
ruby test_hello.rb
```

## Invoke via stdio (zero config)

```bash
op grpc+stdio://"ruby hello.rb" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
