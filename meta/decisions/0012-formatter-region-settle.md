# ADR-0012 — Rewrite settles one section; the layout verdict is kept per tree

Date: 2026-09-30. Status: accepted (M4 unit P19).

## Context

NFR-01 times an `Edit` of one field on the 7,000-entry benchmark. An edit of one value in
`monster.canon`, a 500-row inline table, re-formatted the whole file twice: API.md M9 proved the
file was a fixed point (`Source(raw) == raw`), then `format.Rewrite` (FORMATTER §13) re-parsed,
rebuilt and settled it again. That cost about 47 ms and 38 MB per Edit. The spec fixes only the
bytes: a Rewrite equals what the whole-file path prints.

## Decision

- **A layout verdict per tree.** `format.Canonical(f)` works out once per `*syntax.File` whether
  the file is usable and whether its render equals its content. The memo is weak-keyed on the
  immutable tree, as ADR-0011's derived syntax memos allow. M9 uses it only when the tree holds
  exactly the raw bytes; any error or mismatch runs `format.Source` as before.
- **A region settle.** When every change is a Replace in a canonical, non-project file, Rewrite
  builds only the items that meet the changes and settles one section: the smallest item holding
  the changed bytes that a list or the file lays out after a line break.
- **Why one section suffices.** FORMATTER §6.1's single-line bit comes from source newlines, so in
  canonical text every broken brace list is forced. Forced and hard propagate up through `cat`,
  `indent`, `group`, `rhs` and `ifBreak`, so each ancestor of a broken list's item prints broken
  without measuring. Such a section therefore lies between BREAK-mode line breaks that no `fits`
  look-ahead crosses, and a newline resets the column. Items the focus skips only remove flags,
  which is conservative. The formatter never aligns columns (DECISIONS 18).
- **Run-time guards.** Each settle turn checks that the old section renders to exactly its own
  bytes and that tokens, dropped commas (DECISIONS 211) and comments are equal outside the item.
  Any doubt falls back to the whole-file step for that turn.
- **Differential tests.** `checkRewrite`, `FuzzRewrite` and `FuzzRewriteAround` compare the region
  path with `RewriteWhole` byte for byte, and mutants of every guard fail a committed test.

## Consequences

Monster apply went from 47 ms / 38.6 MB to 19 ms / 15.6 MB with a fresh tree, and to 5 ms once the
verdict is known. The last 14 ms is the fresh parse each Edit's snapshot makes: log-2026-09-29 P19
rules a bounded content-hash → verdict memo held by the workspace for later. A change to the
printer's measuring (`fits`, groups) or to §6.1's single-line rule must re-examine the section
argument; the differential tests are what catch a miss.
