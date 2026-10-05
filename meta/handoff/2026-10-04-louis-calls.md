# Louis calls — 2026-10-04

Technical calls made today are in [log-2026-10-04](../decisions/log-2026-10-04.md) (DECISIONS
293-301). One question is philosophy, so it is yours.

## Q1. What does "retired" forbid? — answered 2026-10-05

**Answer:** (b), placed on the type: a slot typed `past E` may hold retired members; plain `E`
still refuses them. See [ADR-0015](../decisions/0015-past-types-name-retired-members.md).

Today a retired enum member (or variant case) may not appear in any stored value (`E3506`): it
exists only so its code is never reused. Telemetry shows a case where history is the data itself:

- GrantKind 3 (`LEVEL_UP_GIFT`) and 48 (`GUILD_DISBAND_BANK`) are retired, but the lake still holds
  rows with those codes. The ledger map should keep classifying those historical rows (which
  currency, mint or sink), and the lake's enum view should still label them.
- With DECISIONS 299, the label is solved: `GrantKind.members` sees retired members, so the enum
  view can list them. The ledger is not: a ledger role saying `kinds: [LEVEL_UP_GIFT]` is a stored
  value holding a retired member, so `E3506`.

The options, as philosophy:

- **(a) Retired means "never in new data" (today).** Historical classification lives outside
  Canon, or the member stays live with a `@deprecated` note instead of being retired. Simple rule;
  history loses its typing.
- **(b) Retired means "no new code, still describable".** A retired member may be named where the
  data describes the past. That needs a marker: a field or type that says "this is about history",
  e.g. `kinds: [GrantKind] @history`, and only there `E3506` is lifted. It is more expressive and
  one more concept.
- **(c) Retired members are always nameable; only reuse of the code is forbidden** (the lock already
  enforces that). This is the most permissive. A retired member could then creep back into live
  data unnoticed.

My lean is (b): it keeps "retired" meaningful for live data and lets telemetry's ledger stay
fully typed. Until you answer, the telemetry example keeps today's rule (a), and the gap is listed in
the Source handoff.
