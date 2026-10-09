# Handoffs

What this project needs from someone or somewhere it may not change itself. A handoff is a file
here, `<PREFIX>-<n>-slug.md`, written so it can be pasted into a session of the receiver as is:
what is needed, why, the exact acceptance, and the paths involved. It is deleted once applied
(git keeps it). No handoff needing action is open.

`2026-09-24-GEN-01-pipeline-diff.md` is on record here as **informational** (DECISIONS 190: the
M2 pipeline-regeneration diff Louis may want to read); the orchestrator already reviewed it and
continued M2. No action is needed from Louis; it is not deleted so the diff stays easy to find.

`2026-09-29-m3-complete.md` is the M3-acceptance report to Louis (outcome, what landed, the
decisions worth knowing, what was not verified, what M4 starts with). Informational, like the
GEN-01 diff above; kept so the record of the milestone stays easy to find.

`2026-10-01-m4-complete.md` is the M4-acceptance report to Louis (the acceptance table with proofs, the
NFR-01 journey, three calls, the post-M4 spec sync owed, later units, what was not verified).
Informational; the three calls are also in [../state.md](../state.md).

`2026-10-01-next-start-prompt.md` is the prompt that starts the session after M4 (M4.1 first, then
the spec sync, multi-destination emits, M5–M7), with M4's lessons. `2026-10-01-m41-inputs/` holds
M4.1's two unlanded patches and the reviewer's probes (log-2026-09-29 "U-E22-r").

`2026-10-05-telemetry-source.md`, `2026-10-05-showcase-workarounds.md` and
`2026-10-06-design-audit.md` are informational for the receivers (Source, the Emberfall showcase,
Louis); the last is the audit behind DECISIONS 317-324 and v0.1.0.

`2026-10-07-sovereign-resource-port.md` (what Sovereign's Resource port, ADR L-0119, needed from
Canon) is **answered** by `2026-10-07-canon-reply-resource-port.md`, shipped in v0.1.2. The reply is
informational for the Source session: what changed per item, the plain JSON writer, optional roots,
and what Canon still owes (warm entry add/remove re-check, the on-disk cache).

`2026-10-08-sovereign-go-types-mode.md` and `2026-10-08-sovereign-port-followups.md` are
**answered** by `2026-10-08-canon-reply-go-types-followups.md`, shipped in v0.1.3 (DECISIONS
335-338, ADR-0019): Go types mode, maybe-files, the map-write fix and map `union`, edit memory,
literal spellings; `load.defines` unchanged, with the flag-header recipe instead.

`2026-10-08-sovereign-go-open-ids-consumer-roots.md` is **answered** by
`2026-10-09-canon-reply-open-ids-consumer-roots.md`, shipped in v0.1.4 (DECISIONS 339-343): open
enums, `Decode<Fn>File`, consumer roots and `--only-root`, E8026, E8027; item 4's cross-root trap
and ConstantID clash are owed.

## To Louis (`L-*`)

For anything on CLAUDE.md's "Forbidden without asking Louis" list: a spec change (DECISIONS.md,
SPEC.md, CLI.md, `spec/`), a new dependency, a frozen-contract change, a golden diff he must
review (M2's pipeline regeneration), an integration he owns. Short calls go in
[../state.md](../state.md) "Open Louis-calls"; a handoff file is for one that needs a brief.

## To another repository (`R-*`)

Resource, Source and every sibling service are **read-only** from here: agents of this project
never edit them, never run their generators, never commit there, and never open a branch there.
A need is written as a handoff named after the receiver (`R-1-sovcommon-teamboard.md`) and Louis
carries it to a session of that repository, under that repository's own CLAUDE.md. The
integrations the plan already foresees are handoffs of this kind: M1's generated `teamboard`
package in sovcommon, M4's studio spike in resourcestudio.

A handoff never carries real game data, a secret or an absolute machine path; it names files by
their path from the receiving repository's root.
