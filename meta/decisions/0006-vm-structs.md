# ADR-0006 — `api/vm`: view-model structs generated from the schema

- Date: 2026-09-28
- Status: accepted (orchestrator; landed cda1581; `api/vm` is not a frozen contract of
  IMPLEMENTATION-PLAN §4)

## Context

API.md R10 wants the view-model Go structs "generated" from `spec/viewmodel.schema.json`;
`views` builds a `*vm.ViewModel`, `gen/view` writes its bytes (VIEWMODEL J1–J3, J10, WIRE §7),
and `ViewModel.Decode` reads one back with `encoding/json`. The schema is kind-tagged `oneOf`s of
closed objects, with numbers that may exceed 2^53 (J10) and members whose zero value is
meaningful.

## Decision

- A generator, `api/vm/internal/vmgen` (a `go run` command, like `diaggen`), writes
  `api/vm/vm.gen.go`; `make vm-check` diffs it against a fresh run (IMPLEMENTATION-PLAN §12.2).
- Each `oneOf` of objects is **one flattened struct** carrying every branch's members; a definition
  that is a branch maps to its union's struct. Member order is a topological merge of the branches'
  orders (ties by first appearance), so each branch's own J2 order is kept; a conflict refuses.
- Presence: a member not always required is `omitzero`; it is a **pointer** when the schema also
  admits its zero value (`select: ""`, `signed: false`, `missing: 0`, …). A nil slice or map is
  absent, an empty non-nil one present; builders keep required slices and maps non-nil.
- Numbers: JSON numbers and J10's integer-or-decimal-string become `Number{Text, Quoted}` (exact
  text); string-or-integer is `Scalar`; `textRef` is `TextRef`; an unconstrained member is
  `json.RawMessage`.
- The generator refuses, writing nothing: an unknown keyword anywhere, an object that is not
  closed (`type: object`, `additionalProperties: false`), an alias definition, a `$ref` cycle
  without an object between, a non-integer numeric `const`/`enum`, a type conflict between
  branches. `api/vm/schema_test.go` checks the structs against the schema with its own reader.

## Consequences

`encoding/json` alone is not the J1 writer (it writes nil as `null`, compactly, and escapes
differently), so `gen/view` owns serialization. `encoding/json` also matches member names
case-insensitively and accepts `null` for pointers, slices and maps: `api.ViewModel.Decode` adds
the strict check. A schema change that the generator does not model fails `vm-check` loudly.
