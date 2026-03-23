# dart-hello-world

A minimal holon implementing `HelloService.Greet` in Dart with
`dart-holons` serve parsing and `connect()`.

## Run

```bash
dart run bin/hello.dart
dart run bin/hello.dart Bob
dart run bin/hello.dart serve --listen tcp://127.0.0.1:9090
```

## Test

```bash
dart pub get && dart test
```

## Invoke via stdio (zero config)

```bash
dart compile exe bin/hello.dart -o hello
op grpc+stdio://./hello Greet '{"name":"Bob"}'
# → { "message": "Hello, Bob!" }
```

## Connect example

```bash
dart pub get
dart run tool/connect_example.dart
# → {"message":"hello-from-dart","sdk":"dart-holons","version":"0.1.0"}
```
