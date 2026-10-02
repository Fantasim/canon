# renames

The fixture of `canon rename` (DECISIONS 275, API.md §8.9, IMPLEMENTATION-PLAN §6 M5). It holds one
of every kind of occurrence a rename rewrites, in two packages: `features.renames` (the model,
its `.view`, `.fr` and `staging.layer` files, `store/` entry files, `data/` JSON, one `emit json`)
and `features.renames.shop`, which imports it and calls `xpFor` with named arguments. It checks
clean; `expected/` holds the compiler's findings, its `emit json` outputs and its `canon.lock`
(first build writes the lock, as for `teamboard`).

The acceptance goldens of M5 rename, one each:

- field on the wire: `Capacity.players`, read from `data/capacity.json` with no wire name, so the rename gains `@json("players")`
- type: `Mission`, used by the view, the translation keys, the shop and `emit json`
- function: `xpFor`, an export fn the shop calls with named arguments
- let: `missions`, read by `ref missions`, by the `entry` line in `store/rare/hunt.canon` and by the `@files` template
- local: the inner `step` of `bonus`, which shadows the outer one
- refused stable id: `ladder`, a stable table named by `canon.lock` (`ErrStableKey`)
