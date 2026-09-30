# ADR-0011 — The incremental memo: one store, epochs, lineages

Date: 2026-09-30. Status: accepted (M4 units U8, U10–U13, P3, P12, B1).

## Context

NFR-02 asks that re-checking after an edit costs about the change, and IMPLEMENTATION-PLAN §7.6
that an incremental result equal a cold one in everything observable: findings and their order,
positions, budget charges, forcing order, read sets and outputs. The spec says nothing about how.
Every layer of the pipeline (parse, check, load, evaluation, verification, rules) can reuse work,
and each reuse is a place where incremental and cold can drift apart.

## Decision

- **One cache per project, owned by `workspace`**; `build.Cache` holds a persistent file set
  (parse reuse), the latest `check.Session` of up to four **lineages** (one per selection), the
  load memo and one `eval.Memo`. Compaction of the file set starts a new generation.
- **Epochs.** A full check opens a new epoch; a `Recheck` (entry-only files changed) continues its
  lineage's epoch. Every stored result is keyed on its epoch, so nothing is replayed across two
  type checks. An epoch dropped by compaction, by `retire` (a check landing in a dead generation) or
  by a lineage advancing is **forgotten**, and a forgotten epoch is never recreated.
- **One store.** Stage A (evaluation) results are `eval.Memo` entries; the load memo lives beside
  them; stage B (verify) and C (rules) results are slots on the same entries (`Attach`), counted in
  their bytes and evicted, pruned and forgotten with them. One byte bound (≤ 512 MB across stores,
  least recent emptied first) covers everything.
- **Replay re-asks, never assumes.** A replay re-reads every ref target, asset and load in its
  recorded order, charges the recorded steps against the live budget, and refuses (runs cold)
  whenever a read changed, a read is not yet forced, the budget cannot fit, or the entry shares a
  node with a value it read. E4401 always comes from a real run.
- **Kept check-time folds replay on the program's own `Info`**, so the folder's evaluator, its
  counter and const cache continue as in a cold run (B1).
- **Shared parse prefix (amended by P15).** A changed clean file shares the old parse's declarations
  lying wholly before its first changed byte (entries excepted), and Recheck accepts it when every
  let keeps its signature (row keys, modifiers and annotations included), renewing every object of
  the file. Renewals nothing reads stale today (`listKeys` file ids, `typeObjects`, bound files)
  are kept and commented "by construction".
- **Parts (amended by P16).** A `load.dir` element whose decoding was pure is a part: an entry of
  the same store keyed by (load, parsed-tree node), counted, pruned and forgotten like any entry,
  served at its cold charge or decoded for real. List elements replay stages B and C like table
  entries, keyed with their path segment, and a token replays only through the top-level value
  whose evaluation made it (the owner rule, beside stage C's `alone()`). A replay marks nothing
  seen: an owned entry of an alone value is reached by no other value traversed, and occurs once in
  that value.
- **Generation memos (amended by PA2).** A memo of data derived only from a `*source.File`'s
  bytes (P14's JSON trees, PA2's defines classifications) lives in the file set's generation,
  keyed by name, holds one entry per name the generation read, and is dropped at compaction. It
  sits outside the byte bound, like the file set whose bytes it mirrors. A generation may also
  keep a *hint* derived from evaluated facts (PB2's per-table lock order), one entry per table, if
  it is reused only after the new facts compare equal to the ones it was made from, so it can
  save work but never change a result.
- **Derived syntax memos (amended by P17).** A memo of data derived only from one immutable
  `*syntax.File` (log-2026-09-29 P13c-r) may be process-wide, outside the project cache and its
  byte bound, if it is keyed by a weak pointer, dropped when its file is collected, never ranged
  over and never holds analysis state (`Info`, types, values). Its size then follows the live
  files; precedent `gen/view`'s field cache.
- **Gate.** Every incremental test compares the full dump (findings, JSON, view model, budget
  charges) with a cold analysis, over the corpora and over every budget from 1 to the cold need.

## Consequences

The warm `[items]` re-check of the 7,000-entry benchmark went from 522 to 275 ms p95 (this
machine); the rest lies outside the memo (asset listing, lock merge, i18n, the Recheck clone). The
invariants the replay rests on are rulings, not spec text (log-2026-09-29 M4 U10, P3-r, P12-r, B1):
read edges for stage C's `alone()` (a value holds part of another only if its evaluation or its
stage-B run read it, directly or through untraversed values; P18, which replaced completion
order), the token rule, the retag log for the lazy path index.
A change to evaluation order or identity tagging must re-examine them; the ≡-cold gates are what
catches a miss.
