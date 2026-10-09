# Sovereign: studio edit latency on a 76-package project (2026-10-09)

From the Sovereign `Source` session. Paste as is. **Informational, no deadline**: Louis says Canon's
design will not change soon, so Sovereign's studio (`services/config-studio`, a copy of the
Emberfall editor) works around it with batched saves. This is the measured case for later.

## Setup

Sovereign `Resource/Config` (Resource branch `louis/config-studio` from `test` 5c2e1dc9), 76
packages, ~7,900 files; `items` is 7,253 single-entry files. canon 0.1.4 through the Go API
(`canon.Open`), editor under `systemd-run MemoryMax=8G nice -n 15`, default `GOGC`, no `GOMEMLIMIT`.

## Numbers (HTTP API, same calls as the editor frontend)

| Action | Wall | Peak RSS |
|---|---:|---:|
| Open project until the API serves | 17-19 s | 3.0-3.3 GB |
| View model per package (cold) | ~2 s | |
| One record value, cold (lazy package load) / warm | 14.1 s / 0.05 s | |
| Evaluate a drafted record | 4 ms - 0.11 s | |
| `Edit` of one Int field (`dwCost`), applied | 24-26 s | 5.8 GB |
| Same `Edit`, `dryRun` | 11-14 s | |
| `canon edit` CLI dry run, same edit | 15.8 s | 3.5 GB |
| External file change until the SSE reload event | 13 s | 5.2 GB |
| `value` of `items:items`, depth 3 | did not finish in 6 min | 6.1 GB |

One field edit in `items` re-checks the 30 packages that import `items`, the whole dependent
closure. An abandoned `value` request keeps running after the client disconnects (the server's
timeout does not cancel it).

## What Sovereign does meanwhile

The studio stages edits client-side, gives per-record feedback with `Evaluate` (fast), and sends
one batched `Edit` on Save: one re-check per save instead of per field. Large tables load
paged/projected rows from the studio's own server instead of one deep `value`.

## Ideas, ranked by what would help the studio most

1. Re-check only what an edit can affect: the edited entry's own checks, then only the
   cross-entry checks and dependents that read the changed field (per-entry or per-field
   dependency tracking), instead of every package importing the edited one.
2. A paged/projected value read in the Go API (`offset`, `limit`, selected fields) for big tables.
3. Context cancellation through `Value`/`Edit` so an abandoned request stops.
4. `GOMEMLIMIT` guidance for long-running API hosts: the 10-09 memory brief measured
   `canon check` at 1.99 GB with `GOMEMLIMIT=2GiB` vs 3.29 GB default.
