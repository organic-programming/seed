# kotlin-hello-world

A minimal holon implementing HelloService.Greet in Kotlin.

## Test

```bash
JAVA_HOME=/opt/homebrew/opt/openjdk@21 gradle test -Dorg.gradle.java.home=/opt/homebrew/opt/openjdk@21
```

## Invoke via stdio (zero config)

```bash
JAVA_HOME=/opt/homebrew/opt/openjdk@21 gradle jar
op grpc+stdio://"java -jar build/libs/kotlin-hello-world-0.1.0.jar" Greet '{"name":"Alice"}'
# → { "message": "Hello, Alice!" }
```
