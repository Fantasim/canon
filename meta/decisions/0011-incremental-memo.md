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
- **Gate.** Every incremental test compares the full dump (findings, JSON, view model, budget
  charges) with a cold analysis, over the corpora and over every budget from 1 to the cold need.

## Consequences

The warm `[items]` re-check of the 7,000-entry benchmark went from 522 to 275 ms p95 (this
machine); the rest lies outside the memo (asset listing, lock merge, i18n, the Recheck clone). The
invariants the replay rests on are rulings, not spec text (log-2026-09-29 M4 U10, P3-r, P12-r, B1):
completion order for stage C's `alone()`, the token rule, the retag log for the lazy path index.
A change to evaluation order or identity tagging must re-examine them; the ≡-cold gates are what
catches a miss.
