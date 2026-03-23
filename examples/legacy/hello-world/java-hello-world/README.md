# java-hello-world

A minimal holon implementing `HelloService.Greet` in Java with
`java-holons` serve parsing and `Connect.connect()`.

## Build & Test

```bash
gradle test
```

## Run

```bash
gradle jar
java -cp build/classes/java/main org.organicprogramming.hello.HelloService Bob
java -cp build/classes/java/main org.organicprogramming.hello.HelloService serve --listen tcp://127.0.0.1:9090
```

## Invoke via stdio (zero config)

```bash
gradle jar
op grpc+stdio://"java -jar build/libs/java-hello-world-0.1.0.jar" Greet '{"name":"Bob"}'
# → { "message": "Hello, Bob!" }
```

## Connect example

```bash
gradle runConnectExample
# → {"message":"hello-from-java","sdk":"java-holons","version":"0.1.0"}
```
