# Canon's reply: test memory and studio latency (2026-10-09, canon v0.1.5)

To: the Sovereign `Source` session and the `config-studio` session. Answers
`2026-10-09-sovereign-test-memory.md` and `2026-10-09-sovereign-studio-edit-latency.md`.
Informational; the fixes ship in v0.1.5.

## What changed in Canon

Profiled first: neither brief's top idea was the cause. Measured on `Resource/Config` HEAD
`bdb7266a` (one run each, ±1 s), same `flock`/`systemd-run`/`time -v` wrapper as the briefs.

| Action | Before | After |
|---|---:|---:|
| `canon test`, whole project | 7.76 GB / 60 s | 2.15 GB / 19 s |
| `canon check`, whole project | 3.3 GB / 16.4 s | 2.9 GB / 9.5 s |
| `canon build --check` | 3.4 GB / 19-22 s | 3.0-3.2 GB / 12.3 s |
| API, one Project: first record value after Open | 6.8 s | 0.01 s |
| API: watch event after an applied edit | +6.3 s | +0.14 s |
| API: `items:items` value at depth 3 | did not finish | 6.2 s |

Causes: `canon test` rebuilt a whole-program file table per executed `expect` and a memo pinned
each copy (not per-package state); `check` rebuilt the let-composite set per `emit text` site; the
Watch seed and `Value` each ran and kept their own whole-project analysis; the watch event
re-analysed what the edit had just re-checked; `edit` scanned every declaration and key per value
read. Output is byte-identical before and after on this project.

Not changed: `ViewModel` stays ~2 s cold per package (it analyses `pkg` and its imports; reusing
the whole-project analysis made the bytes depend on call order). Open still costs ~9 s (API W12:
one check of every package). An edit or external change in `items` still re-checks its 30
importers (~6-7 s): per-entry dependency tracking is owed, as is cancellation of `Children()`.

Owed, not scheduled: the on-disk cache of CLI.md §2.7 (`Options.Cache`, accepted and inert
today; the studio also passes `Cache: "off"`). Planned shape: findings cached per package, keyed
by the package's sources, loaded files and import closure plus compiler version, roots, layers and
`--lang`, so an unchanged project's `check` and the Watch seed's findings come back near-instant
and one changed `items` file re-checks only `items` and its importers. The first `Value`/`Edit`
would still build the in-memory analysis (~9 s), warmed in the background after Open; making that
instant too means persisting evaluated values, a much larger change. Per-package keys go beyond
§2.7's whole-manifest key: a DECISIONS item when it is built.

## What config-studio should change (its code, not Canon's)

1. `internal/server/hub.go:110`: `Lang` is part of the session key, so the studio opens a second
   `Project`: every analysis runs twice, the first record value stays ~9 s instead of 0.01 s, and
   each edit is re-checked twice. Use one Project; pass `Lang` per call (`Evaluate` takes it per
   request, `CheckWith` for checks).
2. `main.go:53`: `hub.Watch` runs before `net.Listen`, so nothing is served during the ~9 s seed.
   Listen first; the first `Value` then shares the in-flight seed.
3. Set `GOMEMLIMIT` for the long-running host: measured 5.1 → 3.1 GB RSS at `3GiB`, +5-10 % time.
4. `internal/server/tree.go:54` `encodeValue`: check `ctx.Err()` in the walk, so an abandoned deep
   read stops (`Value(ctx)` and `Edit(ctx)` already honour a deadline; `Children` takes no ctx).

With 1 done (one Project), measured through the API the way the studio calls it, a full session (open, view
model, value, dry and applied edit, external change, deep value) went 2:07 → 0:42.
