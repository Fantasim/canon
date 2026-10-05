# Design review — bans and gaps that push users into poor code (2026-10-05)

Question (Louis): which Canon features, like the retired-member ban (ADR-0015), make users code
poorly? Two read-only reviews: workarounds in real programs (`examples/`, telemetry first) and
every prohibition in `spec/ERRORS.md` checked against its owning section. The orchestrator
re-read the top claims; "seen" marks evidence read directly, the rest is the reviewers' word.

**The pattern.** A ban is sound when it has a documented reason **and** an escape hatch. A ban
without an escape hatch makes users move data out of Canon, retype it as `String`, or copy it,
which is worse than what the ban prevents. Every item below is a ban or gap of that kind.

## Tier 1 — fix (hit in real use, or certain to be hit; the workaround is unsafe or copies data)

| # | Issue | Where | Workaround forced today | Direction |
|---|---|---|---|---|
| 1 | A ref from a live entry to a retired one (`E3502`) has the same shape as ADR-0015 | TYPES §10.3, LOCK §4.3 (seen) | history rows store keys as `String` (no existence check) or are themselves retired (wrong meaning) | extend ADR-0015 to `past ref T`; also fix the gap of which holders `E3502` covers ("live entry" vs a plain list) |
| 2 | An enum type is not a value: there is no way to reflect over "the enum this column uses" | telemetry `vocab.canon:134-201` (seen) | 3 parallel lists of 7, `.members.map(...)` copied 7 times, enum names typed as strings, coverage compared as `[String]` | let `Member(v).members` and the enum's name be read through a value; logged 2026-10-04 |
| 3 | A dependent type's argument cannot go through a ref | telemetry `telemetry.canon:199-206` (seen) | `column:` and `of:` written together 11 times, plus a check by hand that they agree | `kinds: [Member(column.values)]`; logged 2026-10-04 |
| 4 | Structured text (JSON, SQL) has no encoder: `emit json` cannot write a computed shape | telemetry `catalog.canon`, `telemetry.canon:105` (seen), `sql.canon:239` | a hand-written JSON builder that escapes only `\` and `"`, so all text must be printable ASCII; SQL strings are interpolated without quoting | an encoder for `@text` bodies (JSON first, SQL literal quoting); logged 2026-10-04 |
| 5 | An input cannot have a default (`E1907`) | EVALUATION §11.1 (seen) | `input Int?`, then `?? 8080` in Go, C++ and TS separately: the default is copied 3 times and never checked | `input Int(1..65535) = 8080 from env "PORT"`, checked at build time and emitted in `LoadInputs` |
| 6 | Each record with inputs can be used only once (`E1903`), never in a list or variant | EVALUATION §11.1 (seen) | `PrimaryDb`/`ReplicaDb` as copy-pasted record types; the choice of variant moves out of Canon | bind env names per use (`primary: Db from env prefix "PRIMARY_"`) |

## Tier 2 — likely worth it (medium confidence or medium impact)

| # | Issue | Workaround today | Direction |
|---|---|---|---|
| 7 | Layers cannot append to a list, remove a map entry, or read the base value (`E4301`, EVALUATION §9.2-9.3) | a dev layer retypes the whole list; for loaded data the whole collection | `base` on the layer's right-hand side, or append/remove amend ops |
| 8 | One spread, of the same type only (`E3303`/`E3323`); no map `+` or map spread (`E3007`/`E3320`) | every field spelled out; one merge fn per map type | several spreads, right side wins; map merge |
| 9 | No map that must cover every member of an enum | a package `check` "a live X with no Y", then `?.v ?? 0` at every lookup (telemetry, teamboard) | a total map type `{E: T}!` checked for coverage |
| 10 | Retirement and lock exist only for enums and stable tables, not nested keyed lists | telemetry hand-rolls `since`/`retiredIn` and about 40 lines of checks, within one build only | retirement and lock on keyed lists (logged as "a language change") |
| 11 | A field default cannot call a user fn (`E3010`, TYPES §15) | the formula is pasted into each default, or becomes a method that cannot be overridden | allow pure user fns that read no `let` |

## Tier 3 — minor, cheap, or a stated non-goal

- Reserved words as enum member names (`none_`, `nothing = "none"`): cheap fix (allow them
  after `.` and in member position).
- No set type or uniqueness refinement: `isUnique()` checks written by hand 5 times.
- Copied record types for loaded rows (propItem copied 3 times, with copied defaults); tuples or
  positional pairs (`Layout = String(/^[1-9]:[1-9]$/)`); no iteration over record fields;
  variants cannot share fields.
- A layer cannot add a stable-table entry (`E6004`); a package with inputs cannot emit TypeScript
  (`E8104`).
- User generics (`Weighted<T>`): SPEC §22 makes them a non-goal. Reopen only on evidence. Items 2
  and 8 are partly the same need.

Judged sound (no action): import cycles, field assignment, function types only in parameters,
recursive types, call-depth and budget limits, un-retiring a locked id, the translated-fn subset,
stale translation keys, the portable regex subset, reserved `id`/`retired`/`kind`/`members`
(`@json` renames cover them), one view per type.

## Recommendation

Do item 1 together with ADR-0015 (same rule, same unit). Items 2-4 have real-use evidence and are
already logged: they come next. Items 5-6 matter only to service configs, so they wait for the
first real user who has inputs. A standing rule would catch future cases: **every ban in
ERRORS.md has a documented reason and an escape hatch, or a note saying why none is needed**.
That fits DOCTRINE as a spec-review check.
