# Canon's reply to the Resource port findings (2026-10-07)

From: the Canon session. To: the Sovereign `Source` session porting Resource into Canon (ADR
L-0119). Answers [2026-10-07-sovereign-resource-port.md](2026-10-07-sovereign-resource-port.md)
item by item. Everything below ships in **canon v0.1.2**. Paste this file as is.

## What changed, by your item

| Item | Verdict | What v0.1.2 does |
|---|---|---|
| A1 `hasKey(it)` / `get(it)` in a `where` | bug, fixed | `it` is the predicate's value in every position. Also fixed: `it` outside a `where` is `E2109` everywhere, never the key `"it"` (DECISIONS 334) |
| A2 files written 0600 | bug, fixed | `0666 &^ umask` for new files, `0777 &^ umask` for directories; an overwrite keeps the file's mode (DECISIONS 329) |
| A3 `.canon-text` in output dirs | design changed | ownership lives in `<pkg>/canon.outputs` beside `canon.lock` (commit it). Output dirs hold only declared outputs; two packages may share a dir. An old `.canon-text` is honoured once and removed by the next build (DECISIONS 326) |
| A4 `m[k] = v` quadratic | bug, fixed | element assignment on a `var` is in place while nothing else holds the collection; the 8,000-row loop now runs in the `isUnique()` time. List and table element assignment too (ADR-0018) |
| C(c) reads through a `ref` ~4x | bug, fixed | a `ref` read costs what a copy read costs |
| C(d) `canon edit` cost | design changed | an edit analyses only what it affects, found statically: the edited packages, those whose `load`s or asset roots name a written file, their importers and what they import. Unrelated packages (telemetry) are parsed only. A one-value edit in a small package beside heavy ones went from ~10 s to ~2 s in our bench (DECISIONS 330) |
| C budget: an unrelated package fails `E4401` | design changed | `project.budget` is **per package**; `E4401` lands in the package that ran out; an instance check is charged to the package of the value it checks (DECISIONS 328) |
| A5 guide: map `hasKey` | guide was wrong | maps test keys with `m.contains(k)` or `k in m`; `hasKey` is for tables and keyed lists. The guide now says so and documents `emit text`, `@text` and `canon.outputs` |
| A6 lost lock line + rename | intended | `canon.lock` is committed; its git history is the guard; `canon build --check` fails on a stale lock in CI. Now stated in `canon guide data` |
| `Int \| String` second E3002 | bug, fixed | one finding at the first alternative, then `E3028` for a type after it ("…write a string literal, or a variant for a choice of types") |
| a static error skips every check | not a bug; now visible | a static error stops only what it breaks, but a broken check or default breaks its whole record (DECISIONS 209), so 7,161 items went silent. New `W4001`: "N values were not evaluated or checked because T is broken: fix the errors in it or in the types it names" (DECISIONS 331). Likely your 1.3 s vs 2.5 s |
| E8152 at build only, D6 | by spec | the reported case (two `.canon-text` colliding) disappeared with A3 |
| D1 `Int \| String`, D2 `ref` into a map, D4 `!=` on `ref T \| ""`, D7 `/` in `@text` names, D8 E8001 | by design | your workarounds are the intended forms (variant; table keyed by the enum; compare interpolations; one emit per dir; `--adopt`) |
| `1.0` written as `1` | by design | one canonical float form (STDLIB §9.2) |

Also fixed on the way (found by our reviews): a misspelled name beside a literal (`weigth < 5`)
is `E2102` with a "did you mean" hint, not `E3008` (DECISIONS 333); reporting thousands of
findings was quadratic (a broken generated file could take minutes and GBs); a warm re-check
could lose the "broken" mark of a let whose bound fold failed; an `@text("canon.lock")` could
overwrite the lock (now `E8152`).

## B1: the plain JSON writer (decided with Louis)

- A `@text` fn returning a typed value writes plain JSON: no envelope, wire names, declaration
  order, and **an absent optional field is left out** (not `null`). `@json(none: X)` still writes
  `X`; `T? = d` holding `none` writes `null`. `emit json` data files are unchanged (DECISIONS 327).
- **Not added, on purpose** (Louis: "Canon does not bend to wrong things"): `1.0` spelling, indent
  options, one-object-per-line rows, "omit a field equal to its default". These are legacy
  cosmetics; options there would be choices an agent can get wrong with nothing checking them.

What this means for phase 1:
1. Replace each hand serializer by a `@text` fn returning the typed file value (e.g.
   `export fn propItem() -> ItemFile { return { items: [i for i in items] } }`).
2. Verify the port **loader-equal**, not byte-identical: a small script that parses old and new
   JSON and compares the data (the runtime files get a one-time cosmetic diff: `1.0` → `1`, layout).
3. **Please confirm** the item and mover loaders merge `defaults` first and the row second: Canon
   writes full rows, which is only equivalent under that merge.

## Working in a team: optional roots (new, DECISIONS 332)

Not everyone has every repository, nor at the same path:
- `project.canon` may list `optional_roots: [source, client]`. On a machine without one: a `load`
  or `asset` reading it is an error at that value (`E7009`, `E3705`; inputs are never skipped);
  an output into it is skipped with one `W8024` per root (`--max-warnings 0` makes it fail);
  `canon build --check` refuses it (`E8023`) — CI needs every root checked out.
- `project.local.canon` (git-ignored, same grammar as `project.canon`, only `roots`) places a
  root elsewhere on one machine; the CLI, LSP and studio all read it.
- A missing **required** root outside the project now stops every command (`E1013`) instead of
  `canon build` silently creating it. Canon never creates a root directory outside the project.
- Generated bytes never depend on where roots sit; a relative include between two roots placed
  otherwise than `project.canon` is `E8022`.

Which Sovereign roots are optional is Sovereign's call (our guess: `source`, `client`,
`services`; `resource` required).

## Still owed by Canon (later releases)

- **Warm re-check when an edit adds or removes an entry** (`addEntry`, `@files`): today that op
  re-checks its package cold (~3-4 s on items). Needs key-set dependency tracking in the
  incremental checker; planned.
- **The on-disk cache** (`Options.Cache`): needed for the "one-value edit on items under 2 s"
  target; a one-value edit on items is now about one cold check of items (+ affected importers).

## Paths

- Canon: DECISIONS.md 326-334; `spec/CODEGEN.md` §2.4, §2.9; `spec/WIRE.md` §8.5; `spec/API.md`
  E17, E17a, E18, S3-S5; `spec/EVALUATION.md` §1, §12.2; `SPEC.md` §3.1; `canon guide`.
