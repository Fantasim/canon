# Sovereign Resource port: what it needs from Canon (2026-10-07)

From: the Sovereign `Source` session porting Resource into Canon (Sovereign ADR L-0119). To: a
Canon session. Paste this file as is. It lists the bugs, gaps and costs the port hit, each with a
minimal repro run on **canon 0.1.1 (`956197b`)**, the expected behaviour, why the port needs it,
and an acceptance. Measurements ran on the NFR-01 reference machine (24 cores), every command
under `nice -n 15` and a 6 GB memory cap, at load 0.5 to 4 unless noted.

## E. Context

Sovereign is moving all of `Resource/Server` data (about 7,300 files, 7,161 of them item files)
and the 85 `Resource/Client` files into one Canon project, in two phases. **Phase 1:** convert
every family (about 40: items, movers, skills, quests, drops, ...) 1:1, and have Canon write
today's runtime files **byte-identical**, so no engine loader changes. **Phase 2:** reshape the
data once Canon owns it. Three throwaway prototypes (items, movers, skills) proved phase 1 is
feasible: every runtime file byte-identical or loader-equal, every broken value refused. They live
in the Source repo on branch `louis/telemetry-v3-on-test`, under
`meta/resource-canon/prototypes/{item,mover,skill}/` (README there; the item data regenerates
with `VOCAB=1 python3 convert.py`). Telemetry is already a Canon project inside Resource
(`Config/telemetry`, four packages) and is part of every measurement below that says "with
telemetry".

**Ranked: what would unblock most**

1. **A plain-shape JSON writer (B1).** It removes about 300 lines of hand serializer per family
   (x ~40) and most of the evaluation cost: the item serializer is 45.5M of the package's 46M
   steps, and a `@text` value is encoded "at no step cost" (CODEGEN.md §2.9).
2. **A `canon edit` that does not pay two cold analyses of the whole project (C, d).** One value
   in a 7,161-entry package takes 13-16 s; one skill value takes 10.7 s, of which 8.8 s is
   telemetry, a package the edit does not touch.
3. **The `.canon-text` manifest out of the output directory (A3).** Resource's runtime folders are
   packed into the game archive, and the manifest is also what makes two packages collide in one
   folder (E8152, D6).
4. **Map index-assign in a loop is quadratic (A4)**, and **reads through a `ref` cost ~4x a copy
   (C, c).** Both are silent traps any family author falls into.
5. **Files written mode 0600 (A2)** and **`hasKey`/`get` in a `where` (A1).**

## A. Bugs

### A1. `tbl.hasKey(it)` and `tbl.get(it)` are false inside a refinement

`project.canon` is `project r1 { canon: "0.1" }`; `p/p.canon`:

```canon
package p
/// A row.
record Row {
  /// A value.
  v: Int
}
/// A literal table.
let rows: table Row = {
  A { v: 1 }
  B { v: 2 }
}
/// Refused for every key.
type T1 = String where rows.hasKey(it)
/// Refused for every key.
type T2 = String where rows.get(it) != none
/// The same test through a function.
fn has(s: String) -> Bool {
  return rows.hasKey(s)
}
/// Accepted (correct).
type T6 = String where has(it)
/// Should pass.
let a1: T1 = "A"
/// Should pass.
let a2: T2 = "A"
/// Passes.
let a6: T6 = "A"
/// true.
let viaVar: Bool = rows.hasKey("A")
```

`canon check`:

```
error[E3206]  p/p.canon:..:14
  a1: A fails where rows.hasKey(it)
error[E3206]  p/p.canon:..:14
  a2: A fails where rows.get(it) != none
```

`a6` passes and `canon explain p:viaVar` prints `true`. Same result with a `load.defines` table.
**Expected:** `a1` and `a2` pass, `"Z"` fails, as through `has(it)`. **Why:** every interim
registry of phase 1 (`IK1_`, `JOB_`, `DST_`, `MI_`, ... from the C headers) is typed
`String where <defines table>.hasKey(it)`; the prototypes route it through a `{String: Bool}` map
built for the purpose. **Acceptance:** the snippet checks with 0 errors and `let z: T1 = "Z"` is
E3206, in a `where` on a type alias and on a field.

### A2. Every file Canon writes is mode 0600

`emit text`, `emit json` and `canon.lock`, with `umask` 002:

```
-rw------- out/data/.canon-text
-rw------- out/data/crlf.h
-rw------- out/json/one.json
-rw------- p/canon.lock
```

**Expected:** the mode a plain file create gets, `0666 &^ umask` (here `-rw-rw-r--`); an
overwrite keeps the existing file's mode. **Why:** Resource is deployed to servers where the
game runs as another user than the one who built; every other file in the repo is 0644/0664.
**Acceptance:** under `umask 022` a new output and a new `canon.lock` are 0644; an existing
output chmod'ed 0664 stays 0664 after a rebuild that changes it.

### A3. `.canon-text` is written into the output directory

Any `emit text { out: "@out/data/" }` writes `out/data/.canon-text` beside the files
(CODEGEN.md §2.9, "Ownership"). **Why it blocks:** Resource's runtime directories
(`Server/Item/`, `Client/`, ...) are packed as they are into the game's data archive; a tool file
there ships to players, and the packer would need a Canon-specific exclusion. It is also the only
reason two packages cannot write different files into one directory (D6). **Expected:** ownership
recorded outside the output directory: for example one project-level manifest (beside
`canon.lock`, or under a `.canon/` directory at the project root) listing every text output by
its `@root/...` path. **Acceptance:** after `canon build`, no file other than the declared outputs
exists in any `emit text` directory; `--adopt`, `E8001` on a foreign file and `build --check`
behave as today; two packages may write different file names into one directory.

### A4. `m[k] = v` in a loop is quadratic

Synthetic package, one table of N rows of 20 `Int` fields, then:

```canon
check {
  var seen: {Int: String} = {}
  for r in rows {
    if seen.get(r.f0) != none { fail(r, "dup") }
    seen[r.f0] = r.id
  }
}
```

| N | no check | `[r.f0 for r in rows].isUnique()` | the loop above |
|---|---|---|---|
| 2,000 | 0.11 s | 0.11 s | 0.39 s |
| 4,000 | 0.20 s | 0.20 s | 1.35 s |
| 8,000 | 0.41 s | 0.45 s | 4.84 s |

Doubling N multiplies the loop's own cost by ~3.6, so each assignment copies the map. On the item
package, this exact pattern (an id-uniqueness check) costs **3.2 s of the 7.2 s check**; rewritten
with `isUnique()` it costs 0.14 s. **Expected:** amortized O(1) assignment to a `var` the loop
owns (the value semantics stay: copy on write only when the map is shared). **Acceptance:** the
loop at N=8,000 within 2x of the `isUnique` column.

### A5. Two guide/implementation mismatches (v0.1.1)

- `canon guide logic` lists `hasKey(k)` for maps; `{String: Bool}.hasKey(k)` is
  `E3003 {String: Bool} has no method hasKey`. One of the two is wrong.
- `canon guide` never mentions `emit text` or `@text`, which are the only way to write a legacy
  runtime file today. Agents of the port learn Canon from the guide only.

**Acceptance:** guide and checker agree on map `hasKey`; `canon guide emit` documents `emit text`,
`@text` (String and typed results, §2.9 / WIRE §8.5) and `.canon-text`.

### A6. `W6006` and the lock: intended?

Probed: add a stable entry by hand, then `canon check` (0 findings, exit 0) and `canon lock
check` (`W6006 1 stable value is not in canon.lock yet: run canon build`, exit 0). That warning is
as documented. The risk is elsewhere: delete a lock line (or `canon.lock`) and rename `a` to `z`
in the source; `check` and `lock check` exit 0, `build` re-locks `z` silently, and the rename
that E6001 exists to refuse is lost. `build --check` exits 1 ("stale canon.lock"), but it cannot
tell an append from a lost line. **Question for Canon:** is this intended (git history of
`canon.lock` is the guard)? If yes, say so in `canon guide data`; if not, a lock-integrity check
(for example `canon lock check --since <git rev>`: refuse a line present at the rev and absent
now) would let Sovereign's CI catch it.

### Also found

- `E8152` (D6) and `E4401` (C) are reported only by `build` / at an unrelated position; see there.
- `let u: Int | String = 3` reports `E3002 expected String, found Int` and a second
  `E3002 expected String, found String` at the `String` token: the second message is wrong.
- A static error (`E5004` from a `test` naming a removed check) stops evaluation of every value:
  `canon check` returned in 1.3 s instead of 2.5 s, with no hint that checks were skipped.

## B. Limitations that cost real work

### B1. No plain-shape JSON writer

`emit json` writes `{"$schema": ..., "value"|"rows": ...}`, absent optionals as `null`, and
`1.0` as `1`. Today's loaders need the plain shape, and **absent is not `null`** for some of them
(a missing PvP field falls back to the PvE value; `null` does not). So each family carries a
hand `emit text` serializer: items 304 lines, movers 277, skills 369 (with its shared JSON
spelling helpers); ~40 families.

Typed `@text` results (WIRE §8.5) already get half way. Probe (doc comments left out):

```canon
record Item {
  dwID: String
  dwNum: Int?
  dwCost: Int? @json(none: "=")
  fFrame: Float = 1.0
}
record File {
  version: Int = 1
  items: [Item]
}
@text("prop.json")
export fn prop() -> File {
  return { items: [{ dwID: "II_A" }, { dwID: "II_B", dwNum: 2, dwCost: 5, fFrame: 2.5 }] }
}
emit text { out: "@out/data/" }
```

Writes no envelope, wire names applied, declaration order, `"="` for `none`. What it cannot do:

| Needed | Written today | Example from Resource |
|---|---|---|
| absent optional omitted | `"dwNum": null` | most PvP/PvE pairs; `propMoverEx.json` |
| a field equal to a default omitted ("defaults + deltas" files) | always written | `Server/Item/propItem.json`, `Server/Mover/propMover.json`: a `defaults` object, then rows holding only what differs |
| `Float` spelled as the source spells it (`1.0`) | `1` (also `"{1.0}"` interpolates `1`) | `skills.json` `"pvpPowerMul": 1.0`; `propMover.json` `"fFrame": 1.0` |
| layout: indent width, one compact object per line for row lists | always 2-space, one key per line | `propItem.json` rows: `{"dwID": "II_...", "nVer": 6, ...}` one per line |

**Request:** options on the typed `@text` JSON writer (or `@json` annotations), for example
`@json(omit: none)` / `@json(omit: default)` per field or record, a Float spelling that keeps
`1.0`, and a per-`@text` layout (`indent`, `rows: compact`). Trailing newline, encoding and key
order are already right. **Acceptance:** the three Resource files named above reproduce
byte-identical from typed values with no hand string building, and the value is encoded at no
step cost.

### B2. Text encoding: covered

`"\r\n"` in a `@text` String writes `0d 0a` as is; CRLF headers need nothing from Canon. (The
single UTF-16 file in Resource is converted to UTF-8 during the port; no request.)

## C. Performance: "how can it take this much? it was not like that in Canon's benchmark"

**Canon's own numbers** (spec/IMPLEMENTATION-PLAN.md §7.6, handoff 2026-10-01-m4-complete.md
row 4, ADR-0011): the 7,000-entry benchmark (about 50 fields set per entry, refs, 3 record
checks, 2 package checks) cold-checks in **2.49 s at 1.22 GB RSS**; the NFR-01 `Edit` p95 is
**0.267 s**, measured on a warm in-process `api` workspace.

**Prototype numbers, re-run today** (`perf.sh`, `prof.sh`, `prof2.sh` of the item prototype):
`check items` 7.2-8.1 s / 1.15 GB; `build` 7.7-8.3 s; whole project with telemetry 9.2 s;
`canon edit` of one value 15.7-16.7 s / 1.9 GB; add an item plus a layout row 24.4 s / 2.5 GB
(the 66 s recorded earlier was with the serializer reading through refs, see c).

### Isolation on the item package (`check items`, telemetry removed, best of 2)

| Variant | Time | RSS |
|---|---|---|
| floor: 7,161 entry files + 20,683-row text table, refs resolved, no checks, no serializer | 1.41 s | 685 MB |
| same data in 149 files (one per category) / in one file | 1.24 / 1.25 s | 629 / 648 MB |
| + 4 record checks (three text-table lookups per item) | 2.47 s | 1.08 GB |
| as prototyped (record checks + id-uniqueness loop + serializer) | 7.22 s | 1.10 GB |
| without the serializer | 5.49 s | 1.15 GB |
| without the uniqueness loop | 3.98 s | 1.15 GB |
| uniqueness loop rewritten with `isUnique()` (A4) | 4.12 s | 1.09 GB |
| that, in 149 files | 3.97 s | 0.99 GB |
| serializer with String `+=` instead of list + `join` | 7.13 s | 1.12 GB |
| layout list (7,161 refs) dropped, serializer walks the table | 6.98 s | 1.10 GB |
| serializer reads fields through the `ref` (`let it = row.item`) | **24.10 s** | 1.16 GB |

So for (a) to (e):

- **(a) parsing/checking 7,161 files:** 1.4 s, and the file count is not the cost (one file per
  category saves 0.17 s). With record checks it is 2.47 s / 1.08 GB: **exactly Canon's benchmark**.
- **(b) the serializer:** +1.7 s and 45.5M steps (of 46M for the package), for a value Canon
  could write at no step cost (B1). String `+=` versus `join`: no difference.
- **(c) ref reads:** +16.9 s when the serializer reads ~126 fields through a `ref`. Synthetic:
  7,000 rows x 20 field reads, `let x = r` (a ref) 2.44 s versus `let x: Row = r` (one copy)
  0.61 s, about **13 µs per field read through a ref**.
- **(d) `canon edit`:** a cold process that analyses the project before and after the edit, so
  it costs ~1.8x a cold `check` for one op and ~3.1x for two ops (`addEntry` + `add`):

  | Project | 1 `set` | `addEntry` + layout `add` |
  |---|---|---|
  | as prototyped, with telemetry | 15.7 s / 1.88 GB | 24.4 s / 2.51 GB |
  | items only | 13.1 s / 1.35 GB | 21.2 s / 1.84 GB |
  | items only, `isUnique` + 149 files | 6.8 s / 1.24 GB | 11.9 s / 1.84 GB |
  | items only, floor | 2.6 s / 1.04 GB | 4.4 s / 1.60 GB |

  It also analyses packages the edit does not reach: the four telemetry packages alone check in
  4.4 s, and a one-value skill edit takes **10.7 s with telemetry, 1.9 s without** (movers:
  1.6 s versus 5.5-6.5 s). API.md E18 says "the affected packages are re-checked"; the CLI pays
  for all of them, twice. The warm path that NFR-01 measures is never reached from the CLI
  (the on-disk cache, `Options.Cache`, is still inert, IMPLEMENTATION-PLAN §7.6).
- **(e) the layout list:** ~0.25 s. Not a cost.

**Budget.** Steps needed (bisected): `items` 46M (45.5M of it the serializer), `items.client`
30M, the four telemetry packages 39M, the whole prototype project 165-173M against the 100M
default. The E4401 was reported at `telemetry/telemetry.canon:577` (`heaviest:
items.propItemJson`): one project-wide budget makes an unrelated package fail when a family
grows. A per-package budget, or the finding placed at the heaviest value, would make it local.

**Conclusion.** Of the 7.2 s item check, **2.5 s is Canon's base cost** and matches its benchmark
(RSS ~150 KB per entry is Canon's too). **3.1 s was our modelling** (the uniqueness loop; fixed
on our side, though only because A4 is a trap), and **1.7 s is the hand serializer**, which exists
only because of B1. The edit numbers are **Canon cost by construction**: two cold analyses of
the whole project per CLI call, at least one more per extra op, plus every unrelated package.
The benchmark never measured that path. Most helpful Canon changes, in order: B1 (removes 45M
steps and ~300 lines per family; check items would drop to ~2.6 s), a warm or scoped
`canon edit` (on-disk cache or a resident process; re-check only the edited package and its
importers; one re-check per request, not per op), A4, then faster ref reads. **Acceptance for
the port:** a one-value `canon edit` on the item package under 2 s, in a project that also holds
telemetry.

## D. Nice to have, not blocking (a workaround exists for each)

1. No `Int | String` union (`E3002`): split fields or a variant work.
2. `ref` into an enum-keyed map: `ref byKind` is `E3504 byKind is not a collection or a record
   type`; a table keyed by the enum works.
3. `@json(none: ...)` on an optional `ref` (`E1118`) and `@json(codes)` per field: interim
   strings work.
4. `!=` between two `ref T | ""` values is `E3007`; comparing their interpolations works.
5. Maps have no `hasKey` (`E3003`, see A5); `get(k) != none` works.
6. One emitting package per output directory: two packages writing `x.txt` and `y.txt` into one
   directory pass `check` and fail `build` with `E8152 outputs collide: @out/d/.canon-text and
   @out/d/.canon-text`. Only the manifests collide (A3). Workaround: one "runtime" package per
   output directory. Also a check/build divergence (DECISIONS 320).
7. `@text` file names cannot hold `/` (`E8021`): one emit per directory works.
8. A runtime file Canon did not create is `E8001`: `--adopt` or `git rm` on the first build.
9. `@files` cannot place an entry by a field's value in a folder name the way Resource names it
   today; the converter writes the paths.
10. A static error stops every check evaluation (see "Also found"); fixing statics first works.

## Paths

- Canon: `spec/CODEGEN.md` §2.9, `spec/WIRE.md` §8.5 and §11, `spec/API.md` E18,
  `spec/IMPLEMENTATION-PLAN.md` §7.6, `meta/decisions/0011-incremental-memo.md`,
  `meta/handoff/2026-10-01-m4-complete.md`.
- Sovereign Source (branch `louis/telemetry-v3-on-test`): `meta/decisions/L-0119-resource-into-canon.md`
  (d.8), `meta/resource-canon/prototypes/` (item: `perf.sh`, `prof.sh`, `prof2.sh`,
  `render.v1.canon`/`render.v2.canon`, `edit.json`, `add.json`; skill: `edit.json`, `undo.json`).
- Sovereign Resource: `Server/Item/propItem.json`, `Server/Mover/propMover.json`,
  `Server/Mover/propMoverEx.json`, `Server/Skill/skills.json`, `Config/telemetry/`.
