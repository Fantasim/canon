# meta/ — the project's external memory

The truth set an agent reloads instead of trusting its context window. The orchestrator reads
`state.md` every session; everything else is cited by path from delegation prompts.

- [state.md](state.md) — the present: current focus, the next step, what exists, open
  Louis-calls (≤ 80 lines).
- [plan.md](plan.md) — the milestones M0–M7 as an ordered checklist, each step with its owner
  module and acceptance. `state.md` says where we are in it.
- [decisions/](decisions/README.md) — implementation ADRs, `NNNN-slug.md`, never renumbered.
  Spec-phase decisions are in [../DECISIONS.md](../DECISIONS.md), not here.
- [handoff/](handoff/README.md) — what this project needs from Louis or from a repository it
  may not edit, written to be pasted there.

What `meta/` is not: the language specification ([../SPEC.md](../SPEC.md), [../spec/](../spec),
[../CLI.md](../CLI.md)), the project law ([../DOCTRINE.md](../DOCTRINE.md)), or the audit's
state ([../.sovaudit/](../.sovaudit)). Durable documents go here, never in a scratch directory;
no secret, no real game data and no absolute machine path in any file here.
