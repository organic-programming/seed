# v0.6 Plan :
A very serious release

- [] [ORIGIN_FLAG.md](ORIGIN_FLAG.md)
- [] [OP RUN ISSUES](../holons/grace-op/OP_RUN.md)
- [] As any holon op can be run. e.g `op op <command>`    
- [] [DISCOVERY FIX](DISCOVERY.md)
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


