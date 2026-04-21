# matt-calculator-go

A Go gRPC holon exposing a **stateful in-memory accumulator** via five arithmetic operations.
Purpose: demonstrate `op invoke` multi-payload JSON Lines output.

## Accumulator scope

> [!IMPORTANT]
> The accumulator is **per-process**. `op invoke` launches a fresh process for each command invocation,
> so the accumulator always starts at **0.0**. Two separate `op invoke` calls do **not** share state.

## Build

```bash
op build matt-calculator-go
```

## Demo — sequential multi-payload invocation

```bash
op invoke matt-calculator-go \
  Set      '{"value":20.0}' \
  Add      '{"value":1.0}'  \
  Subtract '{"value":4.0}'  \
  Divide   '{"by":5.0}'     \
  Multiply '{"by":3.0}'
```

Expected JSON Lines output (one object per line):

```jsonl
{"result":20,"expression":"set → 20"}
{"result":21,"expression":"20 + 1 = 21"}
{"result":17,"expression":"21 - 4 = 17"}
{"result":3.4,"expression":"17 / 5 = 3.4"}
{"result":10.2,"expression":"3.4 × 3 = 10.2"}
```

## Single-call usage

```bash
op invoke matt-calculator-go Set '{"value":42}'
# {"result":42,"expression":"set → 42"}
```

## Error handling

`Divide` with `by: 0` returns `codes.InvalidArgument`:

```bash
op invoke matt-calculator-go Divide '{"by":0}'
# exit code 1, stderr: "op invoke: ... division by zero"
```

## Service contract

See [`api/v1/holon.proto`](api/v1/holon.proto) and the shared contract in
[`examples/_protos/v1/calculator.proto`](../../_protos/v1/calculator.proto).

## Operations

| RPC       | Request field | Effect                          |
|-----------|---------------|---------------------------------|
| `Set`     | `value`       | `acc = value`                   |
| `Add`     | `value`       | `acc += value`                  |
| `Subtract`| `value`       | `acc -= value`                  |
| `Multiply`| `by`          | `acc *= by`                     |
| `Divide`  | `by`          | `acc /= by` (error if `by == 0`)|
