# State — Canon compiler

Updated: 2026-09-24 (night, autonomous session: M1 in flight)

## Current focus

**M1 — check, build, CLI** ([plan.md](plan.md) § M1). M0 accepted (DECISIONS 138,
[reviews/M0-review.md](reviews/M0-review.md)). Committed M1 pieces: parser (`syntax`), `wire`
encoder, fingerprint, `gen/json`; `project` loading + `build` skeleton + `cli`/`cmd/canon`
(`version`, `init`, `new`, `check`; DECISIONS 139–145, ADR-0002); `verify` (stage B), `lock`
verification/update, `rules` (stages C/D) on hand-built values (DECISIONS 146–149).

**TYP done, uncommitted:** `internal/check` (resolver + checker, DECISIONS 150–161) and
`types.Identical/Assignable/Join`; teamboard/ui/roles and every example check with 0 findings;
130+ txtars, Info goldens. Not yet typed: views, translation templates; `W1003` (needs selection).
**Uncommitted, blocked:** `internal/gen/go` (baked Go generator; `rt.go.txt` names E3201,
E4101–E4104, E4108, which need txtar cases in `check`/`eval`/`eval/std` first, DECISIONS 125).

**NEXT:** commit `check` after review; then EVL (`internal/eval` + stdlib subset, E41xx txtars,
`eval.NewFolder`, the `Where`/`Run` surfaces of DECISIONS 148); wire `build.Checker` and a
`Host` adapter (blocked on Louis-call 7's `Host.Verify` signature); `canon build`; goldens.

## What exists

- Spec: SPEC.md, CLI.md, DECISIONS.md (1–149; 30+ autonomous, to review), `spec/`,
  `meta/spec-phase/` (audits, review, git-ignored mockups).
- Contracts (IMPLEMENTATION-PLAN §4): `source`, `syntax`, `types`, `value`, `ir`, `diag`
  (generated from spec/ERRORS.md by `diaggen`), `edit`, `lock`, `project`, `check/info.go`,
  `eval/host.go`.
- `api/` (FindProject, Open, Packages, Check, Revision, Version; panics → `*InternalError`).
- `internal/testkit` (dependency rule, golden txtar harness, `fixturegen`); `tools/audit`
  (own module), `.sovaudit/`; `examples/` (own module); CI skeleton `.github/workflows/check.yml`.

## Open Louis-calls

1. **Enable the hooks.** `.claude/hooks/` scripts are deliberately NOT wired; the block to paste
   is in [../.claude/README.md](../.claude/README.md) § Hooks.
2. **Review DECISIONS 30–149** (autonomous). With 68: rename `api/canon.go` to `project.go`
   when API.md §1, CLI.md §5 and README.md are next edited.
3. **IMPLEMENTATION-PLAN §12.5 is stale** (it says there is no `meta/`).
4. **Absolute-path denies** in `.claude/settings.local.json` (ADR-0001): a new sibling service
   must be added to both lists.
5. **`make check-real`** (DECISIONS 29) lands with the real-data job (M3).
6. **M1 LOD/API (DECISIONS 139–144):** cobra or `flag` (`go.mod`); O3's missing root has no
   code (unchecked); §15 types no `ErrNoProject` (we: `*ProjectError`+`E1003`, whose "or its
   parents" misfits `--project`); I/O errors, and `*ProjectError` from Check/Packages, are
   outside API.md R3 and CLI.md §2.5 (we: wrapped, exit 2); `E7003` with no roots ends
   `roots:` (JSON: `roots: `).
7. **M1 VER (DECISIONS 146–149).** Conversion-time `E32xx` owned by `verify` but met in `eval`;
   `E3201`'s eval form lands with EVL/check; `E3505` owned by `eval`, met in stage B
   (`Result.Unbound`); `Where` cost (none) and hard error (poison). §4.8's
   `Host.Verify(…) bool` cannot carry `Result` (Poisoned, Unbound, ErrNoBag): its return must
   change (verify's `Result`, or eval gains `Poison(root)` + an E3505 path) under §4's review
   rule before `build` wires `verify`. `types.Predicate{Expr,Text}` has no package/scope (add
   `Pkg string`, or `Where(ctx, owner, p, it)`); `Where`/`Run` missing from §4.8. Also: "live
   table entry" (EVAL §5) vs "live entry" (TYPES §10.3); rename pairing count (LOCK §4.1);
   `E6002` conflict `value:Name`; no related location for Basic types; `E3701`–`E3703` order;
   `W6006` location (lock line 1).
8. **Real data:** 123 of 6,944 items violate the pairs rules (E7117) and icon file names drift
   in letter case: both are data-fix scripts for later, not compiler work.
9. **Audit tighten bypass:** the M0.5 agent ran `go run . baseline --tighten` directly because
   `make audit-tighten` needed an approval nobody could give at night (it only removes lines).
10. **M1 TYP (DECISIONS 150–161):** `Check` has no selection (`W1003`); `Keys`/`Symbols` cannot
    hold a `name:` map key (an `Ident`); no `E3015` kind for load arguments; `E3204`/`E3205`
    static but owned by `verify`; `values` on code emits (CODEGEN §2.1 table vs bullet).
11. **SYN jsonsrc (DECISIONS 162–165):** E7104 "first at line:col" (WIRE §3.2) vs `Loc` = path:line
    (ERRORS §1.3); E7105 `{offset}` raw vs normalized; E7109 byte vs character, detail `""` at EOF;
    `\ud800\udcGG` E7105 over E7109; BOM counted in line-1 columns; E7104 key rendered verbatim.

## Verify queue

- `go test -race ./...` in CI (green locally). The hooks, once enabled.

## What could not be verified

`fixturegen` on real data (tested on synthetic trees and the committed fixtures). The CI
workflow never ran on a runner (actionlint only). Whether the relative `..` permission
patterns match. `verify`/`rules` ran only on hand-built values and a scripted evaluator.
