---
id: decision:generic-input-accessors
type: decision
title: Generic Set and Get for State Machine Inputs
---

Expose state machine value inputs through one generic method pair rather than three typed pairs (api:state-machine-runtime).

```yaml
api:
  before: "SetBool / SetNumber / SetString + BoolInput / NumberInput / StringInput"
  after: "Set[T InputValue](name, v) + Get[T InputValue](name) (T, bool)"
constraint:
  admits: bool, string, every int and uint width, float32, float64, and named types over them
  why_wide: >
    a narrow bool|float64|string constraint rejects an untyped constant,
    because Set("speed", 5) infers int; it also forces games to convert
    their own int or float32 fields
cost:
  - module minimum rises to Go 1.27, which added generic methods
  - Get needs an explicit type argument; there is nothing to infer it from
```

Passing an unsupported type is a compile error, unlike an `any` parameter. Type mismatch at the value level stays a runtime concern: Get reports false, and a guard on a wrongly typed input simply never passes.
