# java-holons

Java SDK for holons.

## serve

```java
import gen.describe_generated;
import java.util.List;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.holons.Serve;

public final class Main {
    public static void main(String[] args) throws Exception {
        Serve.ParsedFlags parsed = Serve.parseOptions(args);
        Describe.useStaticResponse(describe_generated.StaticDescribeResponse());
        Serve.runWithOptions(
                parsed.listenUri(),
                List.of(new GreetingServer()),
                new Serve.Options().withReflect(parsed.reflect()));
    }
}
```

## transport

For server listeners, pass `--listen tcp://127.0.0.1:9090`, `--listen unix:///tmp/gabriel.sock`, or `--listen stdio://`.

For client-side Holon-RPC transports, use `HolonRPCClient` with `ws://` / `wss://` endpoints and `HolonRPCHttpClient` with `http://host/api/v1/rpc` or `https://host/api/v1/rpc`.

## identity / describe

Wire the generated Incode Description with one line:

```java
Describe.useStaticResponse(describe_generated.StaticDescribeResponse());
```

`op build` generates `gen/describe_generated.java`; runtime startup fails with `no Incode Description registered — run op build` until that static response is wired.

## discover

```java
var entry = Discover.findBySlug("gabriel-greeting-java");
```

## connect

```java
var channel = Connect.connect("gabriel-greeting-java");
```
