# Canon's next versions: what the Resource port taught us

2026-10-12, canon 0.1.5. To: Louis (the roadmap), then the Canon sessions that build it.

**Status: proposal.**
- Every item was checked against DECISIONS, ACCEPTED-CHOICES and the spec.
- Built items become DECISIONS items, and the spec follows (DECISIONS 207). Technical choices are the orchestrator's.
- One ordering call is Louis's (§6).
- Items that would reverse a decision are not proposed. §5 lists them, with the decision that refuses each.

Companion, to Sovereign: [2026-10-12-canon-to-sovereign-resource-structure.md](2026-10-12-canon-to-sovereign-resource-structure.md).

## 1. Why

Resource is Canon's first large real use: 88 packages, 8,187 files and 715k lines. A read-only review (2026-10-12) sorted what it found by who decided it:

1. **Shapes the port's 1:1 brief imposed** (Source ADR L-0119 phase 1). These are deliberate: they avoid a Source refactor.
2. **Structure the porting agents chose freely.** Sovereign's to fix, in the companion handoff.
3. **Limits of Canon itself.** This file.

The features that keep legacy shapes out of a model went unused:
- `@json(pairs:)`, `@json(bits)`, assets, `@stable` and `@json(inline)` have 0 uses each.
- No clean field name maps to a legacy wire name, among 296 Hungarian-named fields. DECISIONS 3 says "clean names in Canon, legacy names on the wire".

There are two causes:
- Resource's agents may learn Canon only from `canon guide`, which never mentions `pairs`.
- M6 legacy C++ is not built, and it is the step that would let a clean model feed the existing C++ structs.

### 1.1 The generality rule: Resource is the evidence, not the scope

Canon is a general language. CLAUDE.md forbids Sovereign-specific behaviour in the compiler. Sovereign's needs may decide the **order** of work, never the **design**.

An item is in this plan only if all three hold:
1. **It is specified without naming Sovereign or its formats.**
2. **Another plausible user needs it.** For example, an existing C++ codebase, any project with persisted ids, any editor on the API, or any project edited by several agents.
3. **It adds no knob that only one consumer uses** (DECISIONS 319, 327).

Each item below states why it is general. What failed the test is in §5.

## 2. v0.1.6: known bugs, owed parity, robustness, small additions

The release is additive: no language form changes meaning. Sizes: **S** is a unit of hours, **M** a unit of about a day.

**Bugs** (the standing rule: fix every known bug):

| # | Item | Size | Source |
|---|---|---|---|
| F1 | `canon lsp` never gives memory back: 13 MB grows to 6 GB over a working day. Sovereign's gate then refuses updates for lack of memory. Profile, bound it, and state the bound in the release notes | M | Scripts `meta/handoffs/2026-10-12-canon-lsp-memory-leak.md` (not yet in this mailbox) |
| F2 | An Edit whose wire value is an enum member named like a reserved word is an internal error ("format: the new text does not parse in its place"), and qualified forms (`Word.if`) are refused | S | config-studio `meta/handoff/2026-10-09-canon-findings.md` §1 |
| F7 | `E3305` reported twice on an untyped map literal nested in an if, a list or an in-list | S | log-2026-10-08, `E3305_3` |
| F8 | Decoder parity: | S | log-2026-10-08 |
| | - Go and C++ decoders need the tag of an `@json(inline)` field that has a default, and WIRE §5.4 is silent on it; | | |
| | - generated decoders take a duplicate JSON key last, while `load` refuses it (`E7104`) | | |
| F9 | Showcase #1 (`ref in keyedList`) was fixed in the evaluator only. Verify generated code, and fix it there too | S | log-2026-10-05 |
| F10 | C++ parity items owed by U3: `no entry` on stored-fn cells that hold a resolved ref, and the keyed-list check using the owner's accessor name (`@cpp(name:)`). Verify them, and close any that are still open | S | log-2026-10-06 |
| F11 | A case-only rename of a `@text` output on a case-insensitive file system is `E8001` | S | DECISIONS 326, owed |
| F12 | The LSP scope snapshot tokenises a call inside an interpolation coarsely | S | log-2026-10-05 |
| F13 | API.md S11 against §3.4 (`SetOverlay`/`ClearOverlay`). The frozen `api/canon.go` is involved, so IMPLEMENTATION-PLAN §4's review rule applies | S | log-2026-10-03 |

**Owed to consumers:**

| # | Item | Size | Source |
|---|---|---|---|
| F3 | Owed to Sovereign: | M | meta/state.md; Resource ADR 0007; website H-14 |
| | - the cross-root import trap; | | |
| | - `E8005` on several `table Constant` lets in one package, plus an `E8005` message that names its way out (a distinct `package` option, DECISIONS 305); | | |
| | - a types-mode table constant that emits no value; | | |
| | - the dependent-value decoder for a `WorldLevel`-style file. | | |
| | **Blocking now:** website H-14 #4, where `content` cannot emit Go because `world` cannot (`E8005`, `E8019`) | | |
| F14 | Parity with Go (DECISIONS 339, 340): C++ and TS open enums, and C++ and TS decoders for a map/list `@text` result. Also `ownerHooks` exact for data-mode owners | M | meta/state.md |

**Robustness.** This was N6. It is general because it serves any project that several processes or agents write at once.

| # | Item | Size |
|---|---|---|
| F15 | - unique staging names per write (two builds of one package race on `.<base>.canon-tmp`; amends DECISIONS 230); | M |
| | - an advisory per-package writer lock in `build`, `edit` and `fmt`; | |
| | - a file with no parseable `package` line counts against its own directory, not the project bag that fails every scoped command (amends DECISIONS 140); | |
| | - an exact toolchain pin in project.canon; | |
| | - `E8152` (two packages claim one output) reported by `canon check` too (extends DECISIONS 37 and 320); | |
| | - `build --check` reports a file that carries the generated marker in an emit directory that no emit writes (Sovereign security-gate H8) | |

**Editor API.** General because it serves any editor built on the API, not only config-studio.

| # | Item | Size | Source |
|---|---|---|---|
| F16 | A per-call `Lang` on `Value` and `Edit`, as `CheckRequest.Lang` already has. Today an editor that shows several languages opens one full project per language: about 2 GB and 10 s each on a project of Resource's size | S | config-studio findings §4 |
| F17 | Placed root directories in `ProjectInfo`, or `Project.AssetFile(root, name)`, so an editor can preview the assets of a relocated root | S | config-studio findings §2 |

F16 and F17 change the frozen `api/canon.go`, so IMPLEMENTATION-PLAN §4's review rule applies.

**Stdlib.** This was part of N10. General because any text emit needs these.

| # | Item | Size |
|---|---|---|
| F18 | Hex, width and padding in format specs (STDLIB §9.5), and a `duplicates()` builtin (no generics, SPEC §22) | S |

**Guide and spec:**

| # | Item | Size | Source |
|---|---|---|---|
| F4 | `@json(pairs:)` in `canon guide data`, and a recipe "port a legacy file": clean names with `@json("legacyName")`, `Bool @json(int)`, `bits`, `Duration` units, assets, `pairs`, inline variants, stable tables, and an export fn projection for the rest | S | review |
| F19 | The guide gives stable tables, `@stable`, assets and `bits` more than one line each | S | review |
| F6 | The `cards`, `tags`, `enumRow` and `positional` view controls in the guide, so their JSON becomes a contract | S | config-studio findings §3 |
| F5 | Spec hygiene: | S | |
| | (a) sync EVALUATION §10.3 to the log-2026-10-06 ruling (a name matching only a package check is `E5004`); | | |
| | (b) mark DECISIONS 7 "[superseded by 188]"; | | |
| | (c) answer the 2026-10-07 port items D3 and D9 | | |

**Stretch, if the wave has room:**

| # | Item | Size | Why general |
|---|---|---|---|
| F20 | `@json(codes)` per field, not only per enum (port item D3). It unblocks Sovereign's `E1118` cases (ND H-18) | M | Any wire where one field stores an enum as its code and another as its name |
| F21 | Warm re-check for an entry add or remove (meta/plan.md) | M | Any large table edited live |
| F22 | A token-streaming JSON reader for the Go data and types loaders (about 110 ms/MB today) | M | Any large data file |

## 3. v0.2: language and toolchain

### N1. Legacy C++ structs (M6)

**Why general.**
- This is DECISIONS 5, from the first day (2026-09-23). Any existing C++ codebase can adopt Canon without rewriting its data structs.
- The user names their own struct, header, members and types: `@cpp(struct: "…", field: "…", type: "…")`. Canon generates the code that fills the struct, and `static_assert`s that check the user's declarations.
- `@cpp(access: fields | both | getters)` lets a codebase move from public fields to getters, one struct at a time.
- Nothing in the compiler names Sovereign. `ItemProp` appears only in the spec's worked example.

**What it gives Sovereign.** A clean Canon model fills the existing `ItemProp`. Source swaps one hand-written loader per family, and its call sites stay as they are.

**State.** Specified (CODEGEN legacy structs). Planned as M6, not started ("waits, Louis").

**Acceptance.**
- The CODEGEN worked example generates and compiles.
- A synthetic fixture shaped like a legacy struct round-trips loader-equal against a hand-written reference loader.
- Goldens green, `-race` clean.

### N2. One declaration per id, by extending the form that exists

**Why general.** Any project whose ids are stored (database, wire) and also carry data. Without this, every such entity is declared twice and kept in step by checks.

**Resource evidence.**
- `ItemId` has 7,263 members (7,203 live), for 7,203 entries; `TextId` has 9,566 members, for 9,391 rows.
- 9 persisted-id enums carry binding checks, 21 side maps carry completeness checks, and there are 55 "has no row" checks.

**What exists.**
- A stable table already locks an integer per key through a `@stable` integer field (LOCK §1), with a generated `FindByCode`.
- But a table's id type is `string` in `data`/`types` modes (CODEGEN §5.3), and `open:` and `@cpp(defines:)` take enums only.

**Proposal.** No second spelling (DECISIONS 319).
- A stable table whose element has exactly one `@stable` integer field generates its id type with those codes, as a `@codes` enum would: openable, renderable to a define header, and a `ref` to it can be written as its code.
- The lock is unchanged.
- Total maps `{E: T}!` require every live member to have a value. They are also the form for per-member data on enums.

**Acceptance.**
- An example where one declaration gives the id, its code, its row and its generated id type in Go, C++ and TS.
- The change touches the frozen `types.go`, so §4's review rule applies.

### N3. `canon lint`, advisory, for legacy smells

**Why general.** It enforces DECISIONS 3 and 21 for every user. The guide part ships in v0.1.6 (F4, F19).

**Design.**
- Agent-facing JSON, outside `check`, so that it never breaks a `--max-warnings 0` gate. There is precedent for warnings scoped to one command (DECISIONS 322).
- **It flags:**
  - a legacy-prefixed field name with no `@json` rename;
  - an integer field whose values are all 0 and 1;
  - numbered sibling fields;
  - `String where it in <enum>.members` names;
  - the `{k: true for …}` idiom;
  - a field whose wire name starts with `_` and is written to an output;
  - a hand-built output imitating the generated marker;
  - an entry file off its `@files` path;
  - a hand-maintained file over a size limit.
- `--fix` emits `canon edit` requests (DECISIONS 274), never whole packages (DECISIONS 188).

**Acceptance.** One txtar case per lint.

### N4. Wire forms that keep legacy shapes out of the model

**Why general.** Any legacy JSON with numbered columns or flattened groups. DECISIONS 21 exists for exactly this. The forms are additive, so FINGERPRINT stays v1.

**Proposal.**
- **`@json(pairs:)` with N columns, a start index, or a single column.** WIRE §5.14 today allows exactly two fields, with `{i}` from 0.
- **`@json(inline)` on a record-typed field,** so a group stays flat on the wire. WIRE §4.1 allows it on non-optional variants only.

Per-field `codes` moved to v0.1.6 (F20).

**Acceptance.** WIRE, GRAMMAR, FINGERPRINT and the Go, C++ and TS decoders are updated together, with round-trip property tests and `examples/features/` cases. Ambiguity is refused statically.

### N5. Testable package checks

**Why general.** Any project with package-level checks. Today they never run for a subject or in `canon test` (EVALUATION §10.1-§10.2). In Resource, 0 of 97 are tested, and the workarounds are 69 `*Problems` functions plus wrapper records.

**Proposal.** A test form that runs the package checks over substituted top-level values, building on `canon test --layer`. Static errors stay uncatchable (CHK-05).

### N6. Moved

Moved to v0.1.6 as F15.

## 4. v0.3

- **N7. Native package moves.**
  - Sovereign's `Config/canon.lock.moves` has 680 lines; about 655 are package moves, which LOCK §4.6 leaves to hand edits.
  - Proposal: a move fact in `canon.lock v2` (`E6005` refuses unknown facts today), and `canon lock check --base <dir>` (no git dependency).
  - Member renames stay a removal plus an addition (SPEC §12).
  - General because any project that reorganises packages needs it.
- **N8. Import tiers and output templates.**
  - A `tiers` key in project.canon declares allowed import directions (the word `layers` is taken by amend layers).
  - Output path and Go package-name templates resume DECISIONS 229's deferral, now that the repetition shows (75 of 80 Go emit paths follow one pattern).
- **N9. Performance.** Per-entry dependencies (an `items` edit re-checks 30 importers), the stage-E memo, the on-disk cache per package. Then measure a types-only import before specifying one.
- **N10.** `jsonQuote`, once wire's encoder sits below eval (log-2026-10-05). The rest of N10 moved to v0.1.6 (F18).
- **N11. Entry order.**
  - Entries load in path order, so relocating files changes row order and baked ids.
  - Decide whether order follows keys or an explicit list before any relocation tool exists.

## 5. Refused, with the decision that refuses it

These fail the generality rule or reverse a decision. The consumer adapts; in Resource's phase 2, that means the loaders accept Canon's forms.

| Asked by the evidence | Refused by |
|---|---|
| An explicit-null or always-written-key option (Resource's placeholder defaults; the tutorial's `"target": null`) | DECISIONS 327 "No option: one form" (Louis) |
| A CRLF option on `@text` | DECISIONS 308 and 327; Sovereign: "CRLF headers need nothing from Canon" (2026-10-07) |
| Untagged variants, unions of enums, numeric literal unions | TYPES §13.2, verdict D1 (2026-10-07). Use a variant or a dependent type |
| Multi-bit masks in `@json(bits)` | ACCEPTED-CHOICES (power-of-two codes). Give each control kind its own flag enum |
| Several `emit text` per package, or `/` in `@text` names | DECISIONS 229, 294, 297, verdict D7 |
| `expect` catching a static refinement violation | CHK-05 (DECISIONS 19), DECISIONS 155 |
| Bounds naming another field; refinements on type-function parameters | TYPES §7.4/§15 (E3015), DEP-05 (DECISIONS 19). Cross-field rules stay named checks |
| Named refinements with messages; rules on `@codes` | DECISIONS 319: a named check is the one form for a named rule |
| Case-insensitive assets | DECISIONS 19, SPEC §22. Fix the data once |
| Generics, newtypes, a set type | SPEC §22, §5.10, DECISIONS 319 |
| `infer` as a package generator | DECISIONS 188 |
| Byte-identical legacy writers; file order as meaning | ruling B1; keyed collections plus an explicit order |

## 6. Louis's call

The order against the current plan, which is "telemetry first real use, then hardening".

**Recommendation:**
- **v0.1.6 now: F1 to F19, plus the stretch items if there is room.**
  - It is hardening, which is the current phase.
  - F1 blocks Sovereign's gate on the laptop, and F3 blocks website.
  - Most units are independent packages, so they run as a parallel wave in worktrees.
- **Then v0.2: N1 to N5.**
- **Telemetry's first use runs alongside.** Its Canon side has been ready since 2026-10-04.
