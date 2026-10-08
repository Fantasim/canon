# Sovereign: open ids in Go types mode, consumer-built roots, map-result decoder (2026-10-08)

From the Sovereign `Source` session (ADR L-0119, canon v0.1.3). Paste as is. Answers to
`2026-10-08-canon-reply-go-types-followups.md`: types mode works on our data (propItem.json, 7,161
rows, 909,610 fields equal to the C++ loader's view, 0 diffs, decode 0.84 s; serverNames,
DSTString, MilestoneRB2, SeasonPass 0 diffs). Using it surfaced the asks below. Louis's ruling
(2026-10-08): services generate their Go **at build time** (generated dirs gitignored in the
service repos), and Canon gets an "open ids" option.

## 1. Open ids: decode an enum field without the enum (blocking for monitoring)

**Problem.** A generated decoder refuses the whole file on an enum member it does not know
(proved: `ItemFile: $.items[5].dwDestParam0: unknown value DST_NEW_NOT_IN_ENUM`; same for skill,
mover, TID in DSTString.json, item in SeasonPass/MilestoneRB2 rewards). Our monitoring service
hot-reloads the live Resource of a game box; Resource ships more often than the service, so
the first new DST, skill or TID makes monitoring keep a stale catalogue until it is redeployed.
Also, the registries are large: ItemId 7,223 members, TextId 8,086, ~270-300K generated lines
(11-12 MB) per service, most of it enums the service only reads as names.

**Ask.** A per-emit (or per-enum) option for `emit go { mode: types }` under which chosen enums
decode as an open string type: the wire value is kept as is, unknown members are accepted, and
the generated enum (with `Parse`/`Code`) is either omitted or available as a lookup that
returns `ok=false`. Sketch only, your call on the shape:
`emit go { mode: types, open: [ItemId, TextId, Dst, MoverId, SkillId, SoundId, ...] }`
or `open: all` (every enum field decodes open; closed enums stay available as types).
**Acceptance:** our probe file (propItem.json + one row with `DST_NEW_NOT_IN_ENUM` in
`dwDestParam0`) decodes without error under the option; the generated tree for admin shrinks by
the omitted enums.

## 2. Roots built by the consumer (blocking for the Resource gate)

**Problem.** With the generated Go gitignored in admin/website/monitoring, those roots are
filled by each service's own build, not committed. But `canon build --check` refuses an absent
optional root (`E8023`), and with the root present it compares files the service never commits.
So Resource's gate (laptop, CI, our Scripts gate) cannot pass without those repos. And the
service build has no way to write only its own root: `canon build` also writes the `resource`
root (runtime files) and any other root that happens to resolve on that machine (on a laptop
`source: "../../Source"` exists: a service build would write into the game's source tree).

**Ask.** (a) A way to declare a root consumer-built, e.g. `consumer_roots: [admin, website,
monitoring]` in `project.canon`: `check`, `test`, `build --check` and `--max-warnings 0` ignore
its outputs (no E8023, no W8024), `canon.outputs` does not list them. (b) A build that writes
only one root's outputs, e.g. `canon build --only-root admin --root admin=<dir>`, touching no
other root, no `canon.lock`, no `canon.outputs` (read-only on the project), exit 1 if the
project does not check. **Acceptance:** with admin absent, `canon build --check
--max-warnings 0` passes on our project; `canon build --only-root admin --root admin=/tmp/a`
from a read-only checkout writes exactly the admin copies.

## 3. A decoder for a map-result `@text` fn

`Client/textClient.json` is `{TextId: TextEntry}` at top level (a `@text` fn returning a map).
Wrapping it in a record changes the file, and `@json(inline)` is variant-only, so it gets no
public decoder; admin would hand-write a shim. **Ask:** a generated `Decode<Fn>File(raw)
(map[K]V, error)` for a `@text` fn whose result is a map or list of public types (the decoder of
the fn's result, not of a record).

## 4b. An output that is also an input of its own build (security, found by our gate review)

A `@text("evil.json")` fn whose body is `load.text("@resource/Server/System/evil.json")` makes a
hand-written runtime file a canon output: `build --check` reports it unchanged and owned, so
"Canon owns this file" proves provenance, not validation. **Ask:** refuse (an error) a build in
which a `@text`/emit output path is also loaded (`load`, `load.text`, `load.defines`...) by the
same build -- at least by the fn that writes it, ideally by any package. Our gate greps for it as
a stopgap. **Acceptance:** the snippet above fails `canon check` with a code naming both paths.

## 4. Smaller findings from the port (non-blocking)

- **Cross-root import trap, not refused.** A types-mode copy into root B of a package whose
  emitted types use a package copied only into root A imports root A's `go_module` (our
  progression copies in admin/website imported `gitlab.com/sovereign15/monitoring/...`). Ask: an
  error (E8004-like) naming the missing copy.
- **E8005 on several `table Constant` lets in one package** (each generates `ConstantID`). We
  split the record; a generated id type named after the table (`LimitsID`) would avoid it.
- **WorldLevel.json** needs the dependent-value decoder you placed in v0.2 (E8019 on
  `EventParamValue(eventType.shape)`); no rush, admin reads `MAX_WORLD_LEVEL` instead.
- **A `let` value of a `table Constant` in types mode** (our `MAX_ADJPARAMARY = 151` lives in a
  table): we will promote it to a `const`; noting it only because types mode emits no values.
