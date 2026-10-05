# Design audit: what agents decided while you delegated (2026-10-06)

For Louis. Three read-only reviews covered: the decisions agents made alone (DECISIONS 30-204,
"Decided without Louis"), the recent decisions (250-316), and every gap where Canon accepts a
program but cannot deliver it. One reviewer ran probes; the other findings rest on the spec, the
code and the logs.

## Verdict

**Mostly sound, with four real problems.** About 150 of 168 old decisions and most recent ones are
fine: implementation detail, contracts, the cost model, strictness of the kind you asked for. The
problems:

1. **Canon accepts more than it can generate.** The biggest one. About ten kinds of valid Canon
   pass `check` and are then refused by the Go/C++/TS generators (`E8019`), and some modes pass
   `check` and fail only at `build`. This is what made Emberfall copy shared types and telemetry
   repeat its `column:`/`of:` field. It was made legal early (DECISIONS 124, 180) as a temporary
   escape hatch and never closed.
2. **One silent wrong answer remains** (correctness bug, probed): a name in scope of the wrong
   type still becomes a key. `fn f(pick: String) -> ref rows { return pick }` returns the entry
   *named* "pick", with no finding. Decision 303 fixed only the optional case.
3. **Two ways to say the same thing**, in the language itself: see the table below.
4. **Workarounds that became permanent**: a second edit engine (309), one collection with two
   names reconciled at comparison time (315), a growing list of formatter comma exceptions.

## 1. Accepted but not generated (ranked by how often a real user hits it)

| # | What you cannot do | Where | Workaround it forces | Hit |
|---|---|---|---|---|
| A1 | Use another package's record inside a value, in Go (`data` and `baked`) | E8019 ForeignDataRecord, CrossPackageBakedValue | copy the type into every package | high |
| A2 | Same in C++ `baked`, and as a root value in C++ `data` | E8019 | copy the type | high |
| A5 | A dependent type decided through a ref or optional (`[Member(column.values)]`) in baked emits **and in data loaders** | E8019 DependentType | repeat the deciding field (`of:`) | high |
| A7 | Go `embedded`, Go `types`, C++ `embedded` modes: pass `check`, fail `build`; `E8013`'s message even recommends them | gen/go, gen/cpp | none | medium |
| A3/A4 | TS tables of another package's records; lookups taking another package's ref; a few more foreign shapes | E8019 | move code into the owner | medium |
| A8 | Legacy C++ structs (`@cpp(struct:)`): examples check clean, build fails | gen/cpp (M6) | none yet | medium |
| A9-A13 | ~12 rarer shapes (C++ recursive variants, `[T?]`, `{K: V?}`, variant methods, …) | E8019 | reshape the data | low |

The plan only promised to lift part of A1/A2/A5 "before v0.1"; A5's loader half, A3/A4 and A7 are
in no plan.

## 2. Two ways to do one thing

| Item | The two ways | Direction |
|---|---|---|
| Optional fields (154) | `x: T?` and `x: T? = none` mean the same; examples mix them (~51 vs bare) | keep one; `canon fmt` rewrites the other |
| "Every value" (194) | omit `values:`, or write `values: []` (empty means all) | refuse `values: []` |
| `in` on keyed lists (314) | one operator ranks three readings (key, element, converted key); a type can be both | `in` = element only; `hasKey(k)` (or `get(k) != none`) for keys |
| JSON output (308) | `emit json` and a `@text` fn returning a value; which to use is only in the log | state the rule in CODEGEN §2.9 |
| `Check` / `CheckWith` (313) | language set three ways (`Options`, `EvalRequest`, `CheckRequest`) | mild; one request form before 1.0 |
| TS `parse` / `decode` (278) | `decode` is the lossy twin | keep `parse` as the default, mark `decode` lossy |

## 3. Over-built or workaround-entrenched

- **309, poisoned tables:** instead of isolating a bad entry, a second edit engine (~500 lines)
  re-implements 7 edit ops on text, with partial coverage. Real fix: entry-level isolation
  (~250-500 lines, "E3302 becomes soft"), then delete the second engine.
- **315, one collection with two names:** kept and reconciled where they meet; it has leaked
  three times already. Fix properly when the frozen value contract is next opened.
- **304, `past` per type position:** your choice and it works, but its cost (grammar exceptions,
  a flag on a frozen type, extension to type functions, two review CRITICALs) is high for one real
  field. Freeze its scope; do not extend it further.
- **Formatter separators (179, 211, 216, 260, 302):** each fuzz finding added one more exception
  to the newline grammar. One formatter rule (always write separators in broken lists) ends it.
- **257/273, Undo:** inverse ops plus a "restore the whole item" fallback, two mechanisms (low
  confidence).

## 4. Bans still without a way out (against DECISIONS 305)

Input fields cannot have a default, and a record with inputs can be used once (B1, B2: blocking
for service configs); layers cannot append to a list, remove an entry or read the base (B3); one
spread only, no map merge (B4); an enum member or variant case can never be renamed (E28); the
`E8019` message names no way out. 305 applies to new bans only: no sweep of old ones yet.

## 5. Hygiene

DECISIONS.md items that later items replaced carry no "superseded" mark, so the top-ranked document
holds dead rules (83, 143, 162, 170, 180, 185, 197). `W1003` (naming) and several codes are never
reported. Public API fields (`Options.Layers`, …) have no doc comments (trimmed for an audit
ratio). `canon -h` and `canon help` exit 2 as errors. `api/canon.go` was never renamed.

## Recommendation

1. **Now:** fix the silent wrong answer (§ Verdict 2). It is a correctness bug.
2. **A "v0.1 gate" milestone before releasing v0.1.0:** "every construct `check` accepts emits in
   every target" (A1, A2, A5 baked and loaders, A7), with a conformance test that enforces it, and
   remove the duplicate ways of §2 (optional fields, `values: []`). Then release.
3. **Hygiene pass:** superseded marks in DECISIONS, never-reported codes, API doc comments,
   `canon help`.
4. **Your calls** (philosophy, so yours):
   - Should `in` keep its three readings, or split into `in` (elements) and `hasKey` (keys)?
   - Entry-level isolation for poisoned tables in v0.1, or later?
   - Freeze `past` as it is?
   - Input defaults and reusable input records (B1, B2): v0.1, or wait for the first service user?
