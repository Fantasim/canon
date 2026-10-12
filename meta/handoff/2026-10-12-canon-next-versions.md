# Canon's next versions: what the Resource port taught us

2026-10-12, canon 0.1.5. To: Louis (the roadmap), then the Canon sessions that build it.

**Status: proposal.**
- Every item has been checked against DECISIONS, ACCEPTED-CHOICES and the spec.
- When an item is built it becomes a DECISIONS item, and the spec follows (DECISIONS 207). The technical choices are the orchestrator's.
- One ordering call is Louis's (§6).
- Items that would reverse a decision are not proposed. §5 lists them, with the decision that refuses each.

Companion, to Sovereign: [2026-10-12-canon-to-sovereign-resource-structure.md](2026-10-12-canon-to-sovereign-resource-structure.md).

## 1. Why

Resource is Canon's first large real use: 88 packages, 8,187 files and 715k lines. A read-only review (2026-10-12) sorted what it found by who decided it:

1. **Shapes imposed by the port's 1:1 brief** (Source ADR L-0119 phase 1). These are deliberate: they avoid a Source refactor.
2. **Structure the porting agents chose freely.** Sovereign's to fix; see the companion handoff.
3. **Limits of Canon itself.** This file.

The features that exist to keep legacy shapes out of a model went unused:
- `@json(pairs:)`, `@json(bits)`, assets, `@stable` and `@json(inline)` have 0 uses each.
- No clean field name maps to a legacy wire name, among 296 Hungarian-named fields. DECISIONS 3 already says "clean names in Canon, legacy names on the wire".

There are two causes. Resource's agents may learn Canon only from `canon guide`, whose `@json` table lists every form except `pairs`. And the step that would let a clean model feed the C++ structs (M6 legacy C++) is not built, so a clean model would have meant a Source refactor.

## 2. v0.1.6: owed fixes and the guide (no language change)

| # | Item | Source |
|---|---|---|
| F1 | `canon lsp` never gives memory back: 13 MB growing to 6 GB over a working day. Sovereign's gate then refuses updates for lack of memory. Find what is retained per request or per edit, bound it, and state the bound in the release notes | Scripts `meta/handoffs/2026-10-12-canon-lsp-memory-leak.md` (not yet in this mailbox) |
| F2 | Internal compiler error: an Edit whose wire value is an enum member named like a reserved word fails with "format: the new text does not parse in its place". The qualified forms (`Word.if`) are refused too | config-studio `meta/handoff/2026-10-09-canon-findings.md` §1 (marked "for Louis to send or not") |
| F3 | Owed to Sovereign: the cross-root import trap; `E8005` on several `table Constant` lets in one package; a types-mode table constant emitting no value; the dependent-value decoder for a `WorldLevel`-style file. **Blocking now:** website handoff H-14 #4, where `content` cannot emit Go because `world` cannot (`E8005`, `E8019`) | meta/state.md "Owed from 2026-10-08 asks"; Resource ADR 0007 |
| F4 | Guide: `@json(pairs:)` in `canon guide data`, and a "port a legacy file" recipe (§3, N3) | review |
| F5 | Spec hygiene: | |
| | (a) sync EVALUATION §10.3 to the log-2026-10-06 ruling (a name matching only a package check is `E5004`) under a DECISIONS item | log-2026-10-06 |
| | (b) mark DECISIONS 7 "[superseded by 188]" (DECISIONS 322 hygiene) | |
| | (c) answer the 2026-10-07 port items D3 (per-field codes) and D9 (rename and file moves), which have no recorded answer | |
| F6 | Guide: the `cards`, `tags`, `enumRow` and `positional` view controls, so their JSON becomes a contract | config-studio findings §3 |

## 3. v0.2: the language and toolchain items

### N1. Legacy C++ structs (M6): what makes the Resource cleanup Source-safe

**Problem.**
- Source fills `ItemProp`-style structs with hand-written loaders that read the legacy JSON.
- So a clean Canon model meant one of two bad outcomes: the legacy shape stays in Canon, or every C++ call site changes. That is why phase 1 kept the shape.
- M6 maps clean fields, inline variants and lists onto the existing struct through a generated `Fill`/`Load`, using `@cpp(struct: "ItemProp", field: "dwItemKind1", type: "DWORD")` (CODEGEN's worked example, and the 2026-09-23 `@cpp(access: …)` decision).
- With it, Source swaps one hand-written loader per family, and its call sites stay as they are.

**State.** Specified. Planned as M6 "legacy C++", not started ("waits, Louis", meta/state.md).

**Acceptance.**
- The CODEGEN worked example generates and compiles.
- A synthetic fixture shaped like ItemProp round-trips loader-equal against a hand-written reference loader.
- Goldens are green and `-race` is clean. M6's own plan entry stays authoritative.

### N2. One declaration per id, by extending the form that exists

**Problem.** An id that is stored and sent on the wire must be a `@codes` enum, while its data is a table or map keyed separately. Every entity is therefore declared twice and tied together by checks:
- `ItemId` has 7,263 members (7,203 live), for 7,203 item entries;
- `TextId` has 9,566 members, for 9,391 rows;
- `MoverId` keys a map;
- 9 persisted-id enums carry hand-written binding checks;
- 21 enum-keyed side maps carry completeness checks;
- there are 55 "X has no Y row" checks.

**What exists.**
- A stable table already locks an integer per key through a `@stable` integer field (LOCK §1, §9.6), with a generated `FindByCode` (CODEGEN §5.9).
- But a table's id type is `string` in `data`/`types` modes (CODEGEN §5.3).
- `open:` takes enums only (DECISIONS 339), and `@cpp(defines:)` renders enums only (CODEGEN §7.9).

**Proposal.** One form per meaning (DECISIONS 319): no new `@codes` spelling on tables.
- A stable table whose element has exactly one `@stable` integer field generates its id type with those codes, as a `@codes` enum would:
  - Go constants plus `Known()`, openable with `open:`;
  - a C++ `enum class`;
  - a TS union;
  - a define header through `@cpp(defines:)`;
  - a `ref` to it written as its code with `@json(codes)`.
- The lock is unchanged: the `field` facts already hold the codes.
- **Total maps** `{E: T}!`: every live member of `E` must have a value, and retired members may keep theirs. This was proposed in the 2026-10-05 design review (tier 2, item 9) and never decided. Per-member data on enums uses this form, with no attribute syntax.

**Acceptance.**
- An example project where one declaration gives the id, its code, its row and its generated id type in Go, C++ and TS.
- Spec touched: TYPES §8-§10, CODEGEN §5.2-§5.3/§7.9, WIRE §5.8-§5.9, FINGERPRINT §4.4.
- It touches the frozen `types.go`, so IMPLEMENTATION-PLAN §4's review rule applies.

### N3. Teach the clean model, then lint for legacy smells

**Guide (in v0.1.6, F4).** A recipe "port a legacy file":
- an authoring record with clean names and `@json("legacyName")` (DECISIONS 3);
- `Bool @json(int)`, `[E] @json(bits)`, `Duration` with `unit:`, assets, `@json(pairs:)` (DECISIONS 21), inline variants and stable tables;
- an export fn projection for what the wire mapping cannot express.

**`canon lint` (v0.2).**
- **Advisory and agent-facing JSON, outside `check`,** so a new warning never breaks a `--max-warnings 0` gate. There is precedent for warnings scoped to one command: W6006 only in `lock check`, W1003 only in `build` (DECISIONS 322).
- **It flags:**
  - a legacy-prefixed field name with no `@json` rename (enforces DECISIONS 3; W1003 checks casing only, so `dwID` passes);
  - an integer field whose values are all 0 and 1;
  - numbered sibling fields (DECISIONS 21);
  - `String where it in <enum>.members` names;
  - the `{k: true for …}` set idiom;
  - a field whose wire name starts with `_` and is written to an output;
  - a hand-built output whose first line imitates the generated marker;
  - an entry file not at its `@files` path;
  - a hand-maintained file over a size limit.
- **`canon lint --fix`** emits `canon edit` requests (DECISIONS 274). It never generates whole packages (DECISIONS 188).

**Acceptance.** One txtar case per lint, and the guide recipe passes `canon check` under `examples/`.

### N4. Wire forms that keep legacy shapes out of the model

These extend DECISIONS 3 and 21. They are additive, so FINGERPRINT stays v1 (§6).
- **`@json(pairs:)` with N columns, a start index, or a single column** (WIRE §5.14 today: exactly two fields, `{i}` from 0). This covers:
  - `dwReferStat/Target/Value{i}` and the five skill columns;
  - `dwReSkill1..2` and `dwItemAtkOrder1..4`;
  - `dwAtk1..4` as `[ItemId](..=4)`.
- **`@json(inline)` on a record-typed field** (today on a non-optional variant only, WIRE §4.1). A group becomes a record while staying flat on the wire: the 7 flight fields, the 13 `vfx*` fields, rectangles. Also close the open gap of log-2026-10-08: an inline field with a default needs its tag.
- **`@json(codes)` per field** (port item D3, 2026-10-07). Today one concept exists as three types (`Element`, `ElementDefine`, `ElementCode`) kept in sync by a check.

**Acceptance.**
- WIRE, GRAMMAR, FINGERPRINT and the Go/C++/TS decoders are updated together.
- Each form has a round-trip property test and an `examples/features/` case.
- Ambiguity is refused statically.

### N5. Testable package checks

**Problem.** Package checks never run for a subject, nor in `canon test` (EVALUATION §10.1-§10.2). In Resource, 0 of 97 package checks are tested. The workarounds are 69 `*Problems` functions plus wrapper and probe records.

**Proposal.** A test form that runs the package's checks over substituted top-level values and captures their findings, building on what `canon test --layer` already substitutes. Static errors stay uncatchable (CHK-05).

**Acceptance.** EVALUATION §10, a txtar case per outcome, and an example.

### N6. Robustness for parallel agents

These come from Resource `Config/README.md`, the parallel-porting section:
- **Unique staging names per write.** Two builds of one package race on the fixed `.<base>.canon-tmp` name. This amends DECISIONS 230.
- **An advisory per-package writer lock** in `build`, `edit` and `fmt`. Today edits refuse while another live journal exists, but builds have no guard.
- **A file with no parseable `package` line counts against its own directory's package,** instead of the project bag that fails every scoped command. This amends DECISIONS 140.
- **An exact toolchain pin in project.canon.** Sovereign pins a sha256 in its hook today.
- **`E8152` (two packages claim one output) reported by `canon check`** too. This extends DECISIONS 37 and 320.

**Acceptance.** A test per failure mode, under `-race`.

## 4. v0.3

- **N7. Native package moves.**
  - Sovereign's `Config/canon.lock.moves` has 680 lines. About 655 are package moves, which LOCK §4.6 leaves to reviewed hand edits.
  - Proposal: a move fact in `canon.lock v2` (LOCK §2.4 refuses unknown facts today, `E6005`), and `canon lock check --base <dir>`, which proves append-only history against an earlier checkout with no git dependency (CLI §1).
  - Member renames stay a removal plus an addition (SPEC §12).
- **N8. Import tiers and output templates.**
  - A `tiers` key in project.canon (the word `layers` is taken by amend layers) declares which packages may import which, checked by a new E2xxx. Today only acyclicity is checked.
  - Output path and Go package-name templates resume DECISIONS 229's deferral "until the repetition shows". Resource shows it: 75 of 80 Go emit paths follow one pattern, and 16 of 41 Go package names are ad hoc.
- **N9. Performance.**
  - The owed work comes first: per-entry dependencies (an `items` edit re-checks 30 importers), the stage-E memo, and the on-disk cache per package.
  - Then measure what a types-only import would save before specifying one. Unused imported values are already not evaluated (EVALUATION §2.1); phases 1-2 over the import closure remain.
- **N10. Small stdlib items.**
  - Hex, width and padding in format specs (STDLIB §9.5).
  - A `duplicates()` builtin. Without generics (SPEC §22), Resource keeps `duplicates` and `duplicateInts` side by side.
  - `jsonQuote` once wire's encoder sits below eval (log-2026-10-05).
- **N11. Entry order.**
  - `@files` only places new files, and entries load in path order. Enforcing templates (lint, then a `fmt --relocate`) would therefore change row order and baked id values.
  - Decide whether entry order should follow keys or an explicit order before any relocation tool exists.

## 5. Refused, with the decision that refuses it

The consumer adapts. In Resource's phase 2, that means the loaders accept Canon's forms.

| Asked by the evidence | Refused by |
|---|---|
| An explicit-null or always-written-key option (Resource's 105 placeholder defaults; the tutorial file's `"target": null`) | DECISIONS 327 "No option: one form" (Louis) |
| A CRLF option on `@text` | DECISIONS 308 and 327; Sovereign: "CRLF headers need nothing from Canon" (2026-10-07) |
| Untagged variants, unions of enums, numeric literal unions | TYPES §13.2 and the 2026-10-07 verdict D1. Use a variant or a dependent type, which are already untagged on the wire |
| Multi-bit masks in `@json(bits)` | ACCEPTED-CHOICES (power-of-two codes). Give each control kind its own power-of-two flag enum instead |
| Several `emit text` per package, or `/` in `@text` names (7 routing packages) | DECISIONS 229, 294 and 297, and verdict D7 |
| `expect` catching a static refinement violation | CHK-05 (DECISIONS 19) and DECISIONS 155 |
| Bounds naming another field, refinements on type-function parameters | TYPES §7.4/§15 (E3015) and DEP-05 (DECISIONS 19). Cross-field rules stay named checks |
| Named refinements with messages, rules on `@codes` | DECISIONS 319: a named check is the one form for a named rule |
| Case-insensitive assets | DECISIONS 19 and SPEC §22. Fix the data once |
| Generics, newtypes, a set type | SPEC §22, SPEC §5.10, DECISIONS 319 |
| `infer` as a package generator | DECISIONS 188 |
| Byte-identical legacy writers; file order as meaning | ruling B1; keyed collections plus an explicit order |

## 6. Louis's call

The order against the current plan, which is "telemetry first real use, then hardening".

**Recommendation:**
- **v0.1.6 now.** F1 blocks Sovereign's gate on the laptop, and F3 blocks website.
- **Then N1, N2, N3 and N6.** They make Resource's phase 2 a change on the Canon side only, and keep it from being done twice.
- **Telemetry's first use runs alongside.** Its Canon side has been ready since 2026-10-04, and the rest is Sovereign's work.
