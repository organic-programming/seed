# v0.6 Plan :
A very serious release minimalist

--

- Discovery - must be universal, for any sdk, any language, any platform.
- The spec should be unified
- The discovery code should be factored for any usage (autocompletion, list, run ... <slug> etc ...)
- cd ~/Desktop/isolated && op gabriel-greeting-go SayHello '{"name":"","lang_code":"fr"}'
- When we call op <slug> <method> <json> e.g op gabriel-greeting-go SayHello '{"name":"","lang_code":"fr"}' could we have a flag to output the bin path ?
- could we support `op <binary url> <method> <json>` with no discovery
- Auto completion on "op gabriel-greeting-s" should propose  op gabriel-greeting-swift  and the method list ( Call Describe ) 



- Balance pro and cons : After testing if possible add support for ws, rest+sse to any lang that can support it easily?
- Test if describe is now accessible from any isolated built holon. 
-  `op proxy` [PROXY.md](../holons/grace-op/PROXY.md) TO BE PLANIFIED


- Create real rules for OP
    - Always guarantee the ["Surface symmetry — the golden rule"](../AGENT.md#surface-symmetry--the-golden-rule)
    - Never leave a test failing. 
    - Use SDK first


# Gabriel app swiftUI

# SDK
- auto-TLS via [CertMagic](https://github.com/caddyserver/certmagic) for `https://` and `wss://` listeners (replaces manual `?cert=&key=` params) TODO -> create detailled specs with tls config or cert magic .

# OP
- op install should support git urls (binaries, source, url), relation to op get ?
- we need to review all the subcommand and provide a `op help <command>`
- Man of op should be integrated in the proto (and injected in the holon help) question à approfondir ... 


