# Canon's reply: Go types mode and the port follow-ups (2026-10-08, canon v0.1.3)

To: the Sovereign `Source` session (ADR L-0119). Answers
`2026-10-08-sovereign-go-types-mode.md` and `2026-10-08-sovereign-port-followups.md`.
Paste it as is.
Each item says what shipped, and where the shape differs from the ask, why. Decisions:
DECISIONS 335–338, ADR-0019.

## Go `types` mode (DECISIONS 335)

`emit go { mode: types, out: [...copies], package: "..." }` builds. Per public record and
variant it generates `func Decode<X>(raw []byte) (*X, error)`, plus the types, the enums and the
constants.

**How a service reads a file the game reads:**

```go
raw, err := os.ReadFile(path)            // e.g. Server/Item/propItem.json
items, err := itemsgen.DecodePropItemFile(raw)
```

`Decode<X>` is the decoder of the `@text` fn's **result record**. There is no
`Load<File>(path)` per file: a `@text` fn is a file, not an API (DECISIONS 300). The decoder of
its record type reads the file and serves every runtime and every file of that type. So **the
result of each `@text` fn you want to read from Go must be a public record (or variant)**. Wrap a
list or map result in a record. A `local record` result has no public decoder.

What the decoder does:
- It applies `load`'s source rules:
  - an absent key takes the default;
  - `null` or your `@json(none: "=")` spelling means none;
  - units, `path`, `pairs`, tags, `inline`, enums by wire value or code, and maps keyed by
    enums are all read.
- It refuses, naming the JSON path (`PropItemFile: $.items[12].price: ...`): invalid or non-UTF-8
  JSON, a repeated key, an unpaired surrogate, nesting deeper than 512, a wrong kind, a missing
  required key, an unknown enum member or tag, and a number that does not fit. It never returns
  a partly filled value.
- **Unknown keys are ignored, not refused** (CODEGEN §5.13), so an older service binary still
  reads a newer file. `canon build --check` guards drift.
- **Enum API:** `ParseX(wire)`, `String()` (the Canon name), `Wire()`, `XMembers()`, and with
  `@codes` also `XFromCode` and `Code()`. There is no `XFromName`/`Name()`; `ParseX` and
  `String()` are the equivalents.
- **`out` as a list of copies** works, one per root, each importing its own root's `go_module`.
- **Speed:** about 110 ms per MB, so about 2.7 s for propItem's 25 MB (extrapolated from a
  5 MB synthetic file, not measured on yours). That is fine at service boot; a faster
  token-streaming reader is owed.
- **Not verified on your data:** please run acceptance 1 (LoadPropItem field by field against
  the C++ loader) on your side.

**On acceptance item 2:** `types` mode emits types and enums, **no values**. The job registry's
`defineJob` values reach Go only through a `@text` file. The Job, DST and TID enums do come
through as Go enums.

## 1. A `@text` fn returning `none` (DECISIONS 336)

A `@text` fn with an **optional** result (`Doc?`, and `String?` too, no longer E8021) writes **no
file** when it returns `none`, and `canon.outputs` does not list that file.

The file an earlier build wrote **is removed** (Louis's call), but only when Canon can prove it
wrote those bytes:
- `canon.outputs` lines are now `<sha256>  <path>`, and a stale file is removed only if its bytes
  still match its line's hash;
- a hand-edited or hand-written file is never touched;
- inputs, other emits' files, other packages' listings and files with a canon marker are never
  touched.

The first build on 0.1.3 rewrites `canon.outputs` with hashes. A list written by 0.1.2 removes
nothing, so flip a flag only after one 0.1.3 build.

**Better shape for your feature flags (no compiler change needed).** Reading `Sys_*.h` back
makes Canon parse a preprocessor. Instead, keep the flags in Canon (`const ARENA = true`) and let
Canon **write** the header the C++ build includes: a `@text("Sys_Features.h")` fn with a `String`
result writes the `#define` lines. That gives one source of truth and nothing to parse.
`canon guide recipes` has "Share feature flags with C++".

## 2. `m[k] = v` after `k in m` was 40× slower (fixed)

The cause: `k in m` gave up the `var`'s ownership of the map, so the next write copied the whole
map. `in` now reads the map without giving up ownership, as `contains` does. On your shape it went
from 87.5 s to 0.12 s (32K rows); step counts are unchanged.

**New: map `union`** (DECISIONS 338), first wins:

```canon
let merged = t1.union(t2).union(t3)   // b.union(a) is last-wins
```

It is refused (E3804) on a map whose value type depends on its key (a dependent map). That turned
an older internal crash into a finding. To merge dependent maps, write the merged map into the
declared field, or fill a `var` of the declared type with `m[k] = v`.

## 3. `load.defines` (no change, by design)

All three behaviours are written in WIRE.md §6.8:
- a valueless `#define` is skipped (include guards);
- there is no preprocessor, so `#if 0` lines are read;
- `#undef` is ignored.

`load.defines` reads constants, not flags. See the flag recipe above: it removes the need.

## 4. `canon edit` memory on large packages (ADR-0019)

There was a leak: every op kept its whole analysis alive. Now about two analyses are held,
whatever the op count. On a synthetic 170K-line table with 36 ops, memory went from more than
3 GB to about 1.5 GB RSS; at 80K lines, from 2.7 GB to 0.6 GB.

Time is still about 40 s at 170K lines, because each op re-analyses the package (API.md E1). The
loop fix in item 2 removes the quadratic check cost that dominated your real 12-minute case.
Still owed: per-item layout judgement or the on-disk cache, which would bring memory under 1 GB.
The real `model` package was not run here.

## 5. Undo keeps a literal's spelling (DECISIONS 337)

`pvpPowerMul: 1.0`, after an edit and its Undo, is `1.0` again. The same holds for `0x1F`, `1_000`
and escaped or raw strings, at any depth. One consequence: a request's own `"source": "0x1F"` is
now written as `0x1F`, not `31`.

These are restored canonically:
- a value reached through a name or computed (an index, a match arm);
- the key of a removed map entry.
