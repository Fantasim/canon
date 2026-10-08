# ADR-0019 — Edit memory: no analysis kept per op, layout verdicts carried across ops

Date: 2026-10-08. Status: accepted (API.md E1, M5, M9; DECISIONS 258, 330; handoff
2026-10-08-sovereign-port-followups item 4).

## Context

Sovereign's 36-op `canon edit` on a package holding a 167k-line data file went past a 4 GB cap
and swapped for 12+ minutes. API.md E1 re-analyses the package after every op, and peak memory
grew with the op count: 130 MB for 1 op, 637 MB for 36 ops at 20K lines; 2.7 GB at 80K.

Two causes. `applier.named` kept a `rootRef` per op for the Undo comparison, and a `rootRef`
holds the op's `*check.Package`, so every intermediate analysis stayed alive. And each op's
analysis parses the edited file into a new tree, so `rewriteCanon` judged the whole file's
layout again on every op (M9), rendering the whole file (~115 bytes live per source byte).

## Decision

- **Per-op data holds paths, not analyses.** `named` stores `Path{Package, Root}`
  (`rootRef.at()`), the only fields `comparedRoots` reads. Peak memory is about two analyses,
  whatever the op count.
- **A fixed point's verdict carries to the next tree.** `fileState.fixed` is set only when
  `rewriteCanon`'s input tree was judged canonical, since `format.Rewrite` guarantees that its
  output from a fixed point is one (API.md M5). Every other write (`wrote()`) clears it. The next
  `rewriteCanon` adopts the verdict (`format.Adopt`) on the new tree, and only after the bytes are
  checked equal to `cur`. That keeps DECISIONS 258: a positive verdict comes only from a tree
  holding exactly those bytes. Nothing enters the `Verdicts` memo.

## Consequences

- 170K lines / 36 ops: from > 3 GB to ~1.5 GB RSS; 80K: from 2.7 GB to 0.6 GB. Output is
  byte-identical (all edit goldens). Reviewed by instrumenting every adoption across the
  edit, api, lsp, workspace and cli suites: 2,976 adoptions, 0 violations.
- Still owed. Time is ops × a full analysis (E1 by design): batching provably independent ops, or
  incremental analysis, is a v0.2 question. FMT's first whole-file judgement and each
  `Rewrite`'s reparse hold the remaining peak; per-item judgement or the on-disk cache would take
  170K lines under 1 GB.
- A smaller live heap means more GC cycles under evaluation that allocates heavily. `cmd/canon`
  may later set a soft memory limit.
