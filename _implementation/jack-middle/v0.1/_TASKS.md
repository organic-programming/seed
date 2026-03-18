# Jack-Middle v0.1 Design Tasks — Transparent Proxy

## Tasks

| # | File | Summary | Depends on | Status |
|---|---|---|---|---|
| 01 | [TASK01](./jack-middle_v0.1_TASK01_proxy_core.md) | Transparent gRPC proxy core (`UnknownServiceHandler`) | — | — |
| 02 | [TASK02](./jack-middle_v0.1_TASK02_middleware_chain.md) | Middleware chain interface and execution | TASK01 | — |
| 03 | [TASK03](./jack-middle_v0.1_TASK03_logger_metrics.md) | Built-in middleware: logger + metrics | TASK02 | — |
| 04 | [TASK04](./jack-middle_v0.1_TASK04_recorder_fault.md) | Built-in middleware: recorder + fault injection | TASK02 | — |
| 05 | [TASK05](./jack-middle_v0.1_TASK05_cli_connect.md) | CLI entrypoint, backend connect, port hijacking | TASK01, TASK02 | — |
| 06 | [TASK06](./jack-middle_v0.1_TASK06_control_service.md) | Control proto (`middle.v1.MiddleService`) | TASK03, TASK04, TASK05 | — |
| 07 | [TASK07](./jack-middle_v0.1_TASK07_manifest_reflection.md) | `holon.yaml` + gRPC reflection | TASK06 | — |
| 08 | [TASK08](./jack-middle_v0.1_TASK08_plugin_wiring.md) | Plugin holon wiring (`middleware.v1.PluginService`) | TASK01, TASK02 | — |

Design document: [DESIGN_transparent_proxy.md](./DESIGN_transparent_proxy.md)
