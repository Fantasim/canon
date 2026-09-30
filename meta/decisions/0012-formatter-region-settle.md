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
  without measuring. Such a section, like a top-level declaration between hard lines, therefore lies
  between BREAK-mode line breaks that no `fits` look-ahead crosses, and a newline resets the
  column; a section a joined comment follows is excluded. Items the focus skips only remove flags,
  which is conservative. The formatter never aligns columns (DECISIONS 18).
- **Run-time guards.** Each settle turn checks that the old section renders to exactly its own
  bytes and that tokens, dropped commas (DECISIONS 211) and comments are equal outside the item.
  Any doubt falls back to the whole-file step for that turn. By the argument above, the
  f-section, alignment and line checks never reject on reachable input (Rewrite judges only a
  canonical file): they are a safety net, pinned by tests on non-canonical files at the `judge`
  level and by `TestSectionsPrintAlone` over every section of the examples, corpus and table.
- **Differential tests.** `checkRewrite`, `FuzzRewrite` and `FuzzRewriteAround` compare the region
  path with `RewriteWhole` byte for byte, and mutants of every guard fail a committed test.

## Consequences

Monster apply went from 47 ms / 38.6 MB to 19 ms / 15.6 MB with a fresh tree, and to 5 ms once the
verdict is known. The fresh parse each Edit's snapshot makes (14 ms) is removed by PB4: each
workspace Project keeps a least-recently-used memo (256 entries) of fixed-point verdicts keyed by
(content SHA-256, project kind), holding positives only, and only from a tree `Canonical` judged
fixed. On a hit for a tree of the same bytes, `format.Adopt` records that tree's layout, which is
then exactly what a fresh judgement gives. A change to the
printer's measuring (`fits`, groups) or to §6.1's single-line rule must re-examine the section
argument; the differential tests are what catch a miss.
