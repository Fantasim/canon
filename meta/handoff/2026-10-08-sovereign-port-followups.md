# Sovereign Resource port: three follow-ups for Canon (2026-10-08)

From the Sovereign `Source` session porting Resource into Canon (ADR L-0119, on canon v0.1.2).
Paste as is. Each item: what we hit, the repro, what would fix it.

## 1. A `@text` fn returning `none` should write no file (feature)

Sovereign now gates a system's export on its C++ feature flag (`Config/Sys_*.h`, read by a
package `engine.features`). When a whole file belongs to a system that is off, the `@text` fn
returns `none`, and v0.1.2 writes `null\n` instead of omitting the file: an `emit text` output
cannot be suppressed (`out` must be constant, E8009; Canon never deletes an output).

- Repro: `@text("x.json") export fn x() -> Doc? { return none }` -> `x.json` contains `null`.
- Wanted: `none` from a `@text` fn writes no file and releases its ownership in `canon.outputs`
  (a previously written file is removed, like any stale output of the package).
- Acceptance: build with the flag off -> the file is absent, `canon.outputs` no longer lists it,
  `build --check` clean; flag back on -> the file returns byte-identical.

## 2. `m[k] = v` in a loop is still slow in one shape (bug?)

ADR-0018 / v0.1.2 made element assignment in place. In package `ui.servernames` a first-wins
merge over ~36K rows (`var merged: {String: String} = {}`, then `for` over 12 imported tables,
`if !(k in merged) { merged[k] = v }`) took **210 s** to check. The same merge written as a
comprehension + `groupBy` + `[0]` takes 5.6 s (the 9 imported packages alone: 25.7 s).
Difference from your bench: the map is filled from rows of *imported* packages' lets, and the
`in` test precedes each write. Repro on request (Resource `Config/ui/servernames`, history).

## 3. `load.defines` on flag headers (bug + limit)

Probed while reading `Sys_*.h`: `load.defines` **drops valueless `#define __X`** (every feature
flag is invisible to it), **reads lines inside `#if 0` / `#ifdef NOTDEF` as defined**, and
ignores `#undef`. Sovereign worked around it with `load.text` + a line reader. Wanted at least:
valueless defines kept (value none/true), and either honour `#if 0` or report lines inside a
conditional block as conditional.
