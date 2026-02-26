---
# Cartouche v1
title: "java-hello-world — Hello World Holon in Java"
author:
  name: "B. ALTER"
created: 2026-02-12
access:
  humans: true
  agents: false
status: draft
---
# java-hello-world

A minimal holon implementing `HelloService.Greet` in Java.

## Build & Test

```bash
gradle test
```

## Run

```bash
gradle run
# or: java -cp build/classes/java/main org.organicprogramming.hello.HelloService Alice
```

## Invoke via stdio (zero config)

```bash
gradle jar
op grpc+stdio://"java -jar build/libs/java-hello-world-0.1.0.jar" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
