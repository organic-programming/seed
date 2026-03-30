# v0.6 Plan :
A serious release

## Todo 

- Why is examples/hello-world/gabriel-greeting-swift/gen/describe_generated.swift using base64 encoded  ?  That's not the case of any other language examples/hello-world/gabriel-greeting-c/gen/describe_generated.c ? 

- OP RUN Issues : 
    1. 🐞  Default `op run gabriel-greeting-go` should be equal `op run gabriel-greeting-go --listen tcp://127.0.0.1:0` Currently : `op run gabriel-greeting-go` == `op run gabriel-greeting-go --listen stdio://`
    2. 📣 `op run gabriel-greeting-go --listen stdio://` stdio in this run context only should be rejected as it is absurd.
    3. 🐞  `cd ~/Desktop/ && op list --root ~/Desktop/templates` is able to find an holon in `~/Desktop/isolated`
    4. 🐞 IMPORTANT BUG : `op run gabriel-greeting-go --listen tcp://127.0.0.1:0 --root ~/Desktop/templates`->  `~/Desktop/isolated/ 00:00:00 ✗ run failed op run: no holon.proto found in /Users/bpds/Desktop/isolated/gabriel-greeting-go.holon`
    5. 🐞 `op run gabriel-greeting-go --listen tcp://127.0.0.1:0 --bin` (fails with --bin) -> `op: holon "run" not found`  while `op gabriel-greeting-go SayHello {} --bin` works normally
    6. 🐞 `op run op` should work

    ## REGRESSION TEST : 

    From the seed you can use alternative paths (tmp it is better)
    1. 📣 `op run gabriel-greeting-dart --listen stdio://` should be rejected.
    2. 📣 `op run gabriel-greeting-dart` should use tcp.
    3. 📣 `op build gabriel-greeting-dart`, `mkdir -p ~/Desktop/isolated && cp examples/hello-world/gabriel-greeting-dart/.op/build/gabriel-greeting-dart.holon  ~/Desktop/isolated` `cd ~/Desktop/ && op list --root ~/Desktop/templates` should not be able to find the dart holon `~/Desktop/isolated` AND  `op list` should be able to find it
    4. 📣 `op run op` should work as any holon.
    5. BENCHMARK : op list --root ~/ should remain fast ( current response on my mac os == `op list --root ~/  2.06s user 9.55s system 41% cpu 28.253 total`)

    ⚠️ GENERAL QUESTION how can we automate integration test using op and helloworld samples
    WHAT IS THE NAME FOR SUCH TESTS ? 

- [] [ORIGIN_FLAG.md](ORIGIN_FLAG.md)
- [] [OP RUN ISSUES](../holons/grace-op/OP_RUN.md)
- [] As any holon op can be run. e.g `op op <command>`    
- [] DISCOVERY FIX
-  `op proxy` [PROXY.md](../holons/grace-op/PROXY.md) TO BE PLANIFIED

- Create real rules for OP
    - Always guarantee the ["Surface symmetry — the golden rule"](../CONSTITUTION.md#surface-symmetry--the-golden-rule)
    - The golden rule implementation should push the code api to the front ( the other part should be mechanical and conventionnaly documented by clear example) 
    - Never leave a test failing. 
    - Use SDK first


# Gabriel app swiftUI

# SDK
- auto-TLS via [CertMagic](https://github.com/caddyserver/certmagic) for `https://` and `wss://` listeners (replaces manual `?cert=&key=` params) TODO -> create detailled specs with tls config or cert magic .

# OP
- op install should support git urls (binaries, source, url), relation to op get ?
- we need to review all the subcommand and provide a `op help <command>`
- Man of op should be integrated in the proto (and injected in the holon help) question à approfondir ... 


