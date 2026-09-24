# Louis-calls of 2026-09-24 (autonomous day session)

Each item names what needs you. DECISIONS 192–200 outrank the spec text until you update it.

## Decisions to confirm
- **192**: my reading of your "1–2 line header" instruction (Go: marker + `// Package …`; conformance
  files one line; C++/TS marker only; package doc and T1 dropped).
- **193**: sovcommon conventions adopted in baked Go (`StatusID`, map-indexed `FindBy`, bare case
  override) and kept from the spec (getters + unexported fields, `Get` prefix, `self`, explicit enum
  values, `_` storage names).
- **194–199**: stage E, evaluator bounds, build behaviour, fieldless-case docs, per-step bounded work.
  197/199 amend STDLIB costs: `+`, string built-ins and equality are charged per byte/pair.

## Spec text to update (DECISIONS already override it)
- CODEGEN §2.5, §6.3 (headers, rt doc), §3.3 (`StatusId` → `StatusID`).
- STDLIB §4.2, §7, §9.1 (costs), §10 (`r.len()` = `max(end − start, 0)`).
- DECISIONS 186 wording (`Builder.Build` reports into a capture bag, 197).
- CODEGEN §3.4: TypeScript reserved words are not enumerated (E8011 for TS checks syntax only).

## Open gaps needing a rule
- Values nested deeper than the host stack / exponential shared trees under the *free* verify
  and instance-check walks (internal/verify, internal/rules): need a code in ERRORS.md or a
  charge rule (EVALUATION §12.1).
- TYPES §7.5: equality is not transitive (entry vs record by fields, entry vs entry by identity).
- E3501 is reported twice for one bad ref (stage-A dereference + stage-B verify): legal as
  written; say if you want one.
- IMPLEMENTATION-PLAN §3: ir's Consumes column omits `project` (needed by §4.5).
- WIRE §8.1 lets a data value go to any file name; E8153 and DECISIONS 126 require `<value>.json`.
- ERRORS.md templates: E8153 says "emitted in data mode" for @reload values no data emit
  selects; E8015 renders "a Int".
- `--adopt` entries: display path or file-system path (M6).
- CLI.md gives no sample of `canon build`'s own report; the txtars froze a format (see
  internal/cli/testdata/commands/build_*.txtar).

## Repository and tooling
- `pkg-size` (observe only): eval, eval/std, ir, check, format exceed 15 files / 4000 lines. The
  layout is IMPLEMENTATION-PLAN §12.4's; say if you want splits.
- The audit's `comment-adr-narration` forbids wrapping any comment that cites a rule, so such
  comments stay on one long line.
- README.md links `meta/spec-phase/mockups/` (studio.html), which is not committed: a dead link in
  any fresh checkout. Commit it or drop the link.
- golangci-lint's cache is shared by content hash across copies of tools/audit; a stale entry
  broke audit-self once (`cd tools/audit/toolchain && go tool golangci-lint cache clean`).
- Not verifiable here: a Go 1.23 toolchain (only 1.25/1.26 installed), GCC 9, Clang 10, MSVC.
- M1 acceptance item 6 (sovcommon teamboard integration) is postponed (DECISIONS 189).
