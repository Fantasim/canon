# Handoffs

What this project needs from someone or somewhere it may not change itself. A handoff is a file
here, `<PREFIX>-<n>-slug.md`, written so it can be pasted into a session of the receiver as is:
what is needed, why, the exact acceptance, and the paths involved. It is deleted once applied
(git keeps it). None open.

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
