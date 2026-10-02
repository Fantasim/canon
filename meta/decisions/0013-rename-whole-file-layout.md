# ADR-0013 — A name rename splices tokens and lays the whole file out

Date: 2026-10-02. Status: accepted (M5 unit R2, DECISIONS 275).

## Context

`RenameName` (API.md §8.9) replaces every occurrence of a Canon name: in declarations, expressions,
import lists, views, layer paths, translation keys and `@files` templates. `format.Rewrite`
(FORMATTER §13) changes items by Replace, and it cannot address a token in an import list
("no item holds it"). A rename can also add `@json("<old wire name>")` and K4 kind words, and it can
make a line too long, so the item holding the change must be laid out again (M5).

## Decision

- **Splice, then lay out the whole file.** Each occurrence (from `check.Program.Occurrences`) is
  spliced into the raw text, and the file is printed with `format.Source`. This is safe because M9
  makes the base canonical: re-laying a canonical file changes only the items whose text changed.
  Import name lists come out sorted, as the formatter always sorts them.
- **M6 still judges the write.** Each whole-file write records a `regionNode` region for every
  top-level item (import, declaration, amend block, translation entry) that holds a splice. The M6
  check (`keptOutside`) fails on any byte changed outside them (`TestRenameNameRegions`).
- **E35 by identifier ordinals.** Preservation compares every identifier node of each file, in
  source order, resolved or not, with equal counts. Import lists are compared as a counted set,
  because the formatter re-sorts them. The identifiers the rename itself adds or removes (an
  inserted or dropped `@json` name, a K4 kind word) are skipped by ordinal (`Plan.NameEdits`). The
  formatter never adds or removes identifiers, and imports come first in a file, so the ordinals
  computed before layout still hold after it. A position Undo is found the same way: the declaring
  identifier's ordinal is looked up in a parse of the written file.

## Consequences

- One code path serves every occurrence kind. A file that M5 re-breaks keeps its other items'
  bytes. The Undo restores values, and bytes except where E37 names an exception.
- The cost is one `format.Source` per touched file plus one parse. The E35 walk covers the
  re-checked packages and their imports. Neither has been measured on the 7,000-entry benchmark;
  renames are not part of NFR-01.
- Rejected: extending `format.Rewrite` to address tokens inside import lists. That would add a
  second settle path for one caller.
