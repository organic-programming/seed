# kotlin-hello-world

A minimal holon implementing `HelloService.Greet` in Kotlin with
`kotlin-holons` serve parsing and `Connect.connect()`.

## Build & Test

```bash
gradle test
```

## Run

```bash
gradle jar
java -cp build/classes/kotlin/main:build/resources/main org.organicprogramming.hello.HelloKt Bob
java -cp build/classes/kotlin/main:build/resources/main org.organicprogramming.hello.HelloKt serve --listen tcp://127.0.0.1:9090
```

## Invoke via stdio (zero config)

```bash
gradle jar
op grpc+stdio://"java -jar build/libs/kotlin-hello-world-0.1.0.jar" Greet '{"name":"Bob"}'
# → { "message": "Hello, Bob!" }
```

## Connect example

```bash
gradle runConnectExample
# → {"message":"hello-from-kotlin","sdk":"kotlin-holons","version":"0.1.0"}
```
