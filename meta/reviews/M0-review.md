# M0 review — Canon compiler contracts

Reviewer: spec-reviewer (read-only). Date: 2026-09-23 (night). Scope: `git diff 96a21a9..HEAD`
(commits 33fea96..bdf3788): `tools/audit`, `cmd/canon`, `internal/*`, `api/*`, Makefile,
`.github/`. Judged against DECISIONS 1–90 (incl. the autonomous 30–90 as binding),
IMPLEMENTATION-PLAN §3, §4, §5–§7, §12 and the Addendum, and the owning spec documents.

## Verdict

**ACCEPTED WITH FIXES. M0 is not done yet.** The work that exists is careful and gated. It compiles,
`make check` and the race tests are green, the audit is at zero, diaggen round-trips and every
M0 golden is reproducible. Three things keep M0 from being closed:

1. Its own acceptance is not met. Two of the eight §4 contracts (`check/info.go`,
   `eval/host.go`) and three declarations of a third (`edit.Step`, `Resolved`, `Resolve`) do not
   exist.
2. No consumer approval is recorded.
3. Several contract gaps would force a frozen-contract change in M1. The must-fix list below
   closes them. Most are small.

## Gates run

| Command | Result |
|---|---|
| `GOTOOLCHAIN=local make check` (clean HEAD, before any working-tree edits) | **green**: fmt, vet, tests, goldens-vet, goldens-check, diag-check, audit-self (`0 new`), audit-check (`0 new, 0 grew`) |
| `GOTOOLCHAIN=local go test -race -count=1 ./...` | **green**, on the working tree and on a clean `git archive HEAD` copy |
| Goldens regenerated with `-update` in a clean copy (`internal/diag`, `internal/lock`, `internal/testkit/golden`) | **zero diff**: `messages.txtar`, `render/*.txtar` and `E6005_*.txtar` are tool-written |
| Fuzz, 20 s each: `edit.FuzzParse` (~15.8M execs), `lock.FuzzParse` (~8.4M execs) | pass, no corpus written |
| Ad-hoc map-range scan (go/types, all packages, tests included) | no `range` over a map in non-test code; in tests: `api/example_build_test.go:35` (prints, see S-API-5), plus harmless ones in catalog, diaggen, fixturegen and grammar tests |

**Working tree not clean during the review.** Uncommitted edits appeared while the review ran, in:
`api/*`, `internal/source`, `internal/testkit/{deps_test.go,golden/*}`, `internal/lock/findings_test.go`,
the new `internal/diag/fileset_test.go` and `api/errors_test.go`, and **`spec/API.md`, `spec/ERRORS.md`,
`spec/IMPLEMENTATION-PLAN.md`, `spec/TYPES.md`**. Findings are against HEAD; where the work in progress
already addresses one, it is noted. See P1.

---

## Must-fix before M1

**MF-1. Two §4 contracts are missing; M0's scope and acceptance are unmet.**
- **Where:** `internal/check/` and `internal/eval/` hold only `doc.go` and `example_test.go`.
- **Spec:** IMPLEMENTATION-PLAN §4.7 and §4.8; §6 M0 ("the eight contracts of §4 as compiling
  Go"); §5.2 M0 row (TYP `check/info.go`, EVL `eval/host.go`).
- **What differs:** `meta/plan.md:24` says "six frozen contracts" and leaves both out. The plan
  file contradicts the implementation plan without a DECISION.
- **Fix (code):** write `check/info.go` (Check, Bags, Folder, Program, Package, Info, Object,
  Selection, Conversion, Callee, MatchInfo) and `eval/host.go` (Host, Root, NewFolder, New,
  Evaluator, Force, BeginVerification, Call) as compiling Go with their constants and Examples.
  Correct `meta/plan.md` M0.4 to "eight". If Louis prefers to defer them, record that as a DECISION.
- **Design point to fix when writing it (from the syntax review):** §4.7's
  `Uses map[*syntax.IdentExpr]Object` cannot record uses that are `*syntax.Ident`. Those are type
  names, patterns, the `is` target, `TypedLit`/`RefType`/`TableType` names, view field and column
  names, amend segments and `entry t.` tables. API.md E11 (Rename) needs them. Key `Uses` by
  `syntax.Node`, or add a map for `*syntax.Ident`.

**MF-2. `edit.Step`, `Resolved` and `Resolve` are missing, and `Snapshot` has no home.**
- **Where:** `internal/edit/path.go`.
- **Spec:** IMPLEMENTATION-PLAN §4.6; DECISIONS 87 defers them to M4.
- **What differs:** DECISIONS 87 says they "need `types.Type`, `value.Value` and a snapshot". The
  first two exist and sit above `edit` in §3, so `Step` and `Resolved` compile today. Only
  `Snapshot` is blocked. `workspace` sits below `edit`, so a `Snapshot` interface that `workspace`
  implements would be a third injection seam, which §3 ("exactly two such seams") forbids. Adding
  the declarations after the freeze is a §4 change.
- **Fix (code + DECISION):** declare `Step` and `Resolved` now. Decide in DECISIONS/§4.6 what
  `Snapshot` is: an `edit`-owned concrete type built from `build` results, or a §3 amendment for a
  third seam. `Resolve`'s body can then wait for M4.

**MF-3. `*source.FileSet` does not implement `diag.Files` at HEAD.**
- **Where:** `internal/source/source.go` (HEAD has only `Add`, `File`, `Locate`) against
  `internal/diag/diag.go:35-39`.
- **Spec:** DECISIONS 81 ("`*source.FileSet` satisfies it with three one-line methods").
- **What differs:** no production caller can hand a `FileSet` to `NewBag` or `Render`. Every
  current caller uses a test-local `MemFiles` or `oneFile`.
- **Fix (code):** commit the in-progress `Path`, `Position` and `Content` methods with the
  compile-time assertion (`internal/diag/fileset_test.go`) and their tests.

**MF-4. Findings output depends on the order of `Report` calls, so it is not deterministic under concurrency.**
- **Where:** `internal/diag/bag.go:62-68` and `:109-124`; `internal/diag/render.go` (sorts, does not
  deduplicate).
- **Spec:** EVALUATION.md §14 ("the first one produced"); DECISIONS 83; NFR-05 and
  IMPLEMENTATION-PLAN §7.5 (Workers=1 and Workers=8 must give identical findings).
- **What differs:** findings that tie on the F2 key are exactly the duplicates.
  `SortStableFunc` plus "keep the first" keeps whichever goroutine added first, and that finding's
  Path, Pointer, Check, Layer, Related, Stack and Reads survive with it. The sub-reviewer proved it
  with an overlay test: `.Path("x")` then `.Path("y")` renders `x: m`; the reverse order renders
  `y: m`. `Render` over findings from several packages keeps the caller's concatenation order on
  ties.
- **Fix (code):** add a total, deterministic tiebreak after the F2 key: Package, Path, Pointer,
  Check, Layer, then Related, Stack and Reads compared element by element. Keep the smallest as
  "first produced". Alternatively, have producers supply a deterministic sequence number. Add a
  test that reports duplicates in both orders and asserts identical output.

**MF-5. No legal way to render API findings (`cli` consumes `api`), and `MoreFrames` is lost at the boundary.**
- **Where:** `api/findings.go:30-43` against `internal/diag/render.go`, `text.go` and `json.go`.
- **Spec:** API.md §4.1, F5 and F13; ERRORS.md §1.5 (stack and summary lines are "written by
  `diag`'s renderer"); DECISIONS 54 (no hand-built `diag.Finding` outside `diag`) and 85 (one JSON
  codec); IMPLEMENTATION-PLAN §3 (`cli` consumes `api`); M1 acceptance 2 (`canon check teamboard`
  prints `findings.txt` byte for byte).
- **What differs:** `diag.Render` takes `[]diag.Finding` plus `diag.Files`. `cli` gets
  `[]canon.Finding`, which has resolved locations, and it may not convert them back.
  `canon.Finding` has no `MoreFrames`, so the `(<n> more frames)` line cannot be produced.
  `canon.Finding.MarshalJSON` would be a second F5 writer.
- **Fix (spec + code, one DECISION):** add a dropped-frame count to `canon.Finding`, JSON-hidden or
  specified in F5. Then give `diag` text and JSON writers over a resolved form (for example
  `diag.Located`) that both `diag.Render` and `api` use, or have `api` export
  `WriteFindings(w, findings, summary, format)` built on `diag`. Both confirming reviews (api,
  diag) flagged this independently.

**MF-6. `ir.Emit` does not carry what `gen/go` and `gen/cpp` need to write imports.**
- **Where:** `internal/ir/ir.go:160-177` (`Emit{Target, Out, Mode, Values, GoPackage,
  Namespace}`; `Generator func(p *Package, e *Emit)`); `PackageRef.Emits` (`:31-35`).
- **Spec:** CODEGEN.md §2.8. A Go output imports `<import path of D>/rt`: the pipeline golden
  imports `example.com/potions/rt`, and M1's baked teamboard does the same. The import path comes
  from `project.canon`'s `go_module` and the closest mapped root. C++ includes an imported header
  by a path relative to the including file's directory. `E8007` is a stage-E check.
- **What differs:** the generator is a pure function of `(ir.Package, ir.Emit)` and receives no
  project. `Out` is the written form (`"@sovcommon/teamboard"`), so neither the Go import path nor
  a relative C++ include can be computed from the IR.
- **Fix (code):** add resolved fields to `ir.Emit`, which `build`/`ir` fill in stage E:
  - `Dir`: the output directory as a project-relative or root display path, `/`-separated.
  - `GoImport`: the Go import path of `Dir`, empty for other targets.

  Imported packages' emits carry the same. State in §4.5 that generators never resolve roots.

**MF-7. `syntax.FloatLit` cannot represent `-0.0`.**
- **Where:** `internal/syntax/ast_expr.go:29-33` (`Coef *big.Int; Exp int`), with unary minus
  folded (DECISIONS 76).
- **Spec:** GRAMMAR §2.4 (exact values); EVALUATION.md §6.2 (IEEE binary64); CONFORMANCE.md §5
  (bit comparison); STDLIB `min(0.0, -0.0)`.
- **What differs:** a `big.Int` zero has no sign. The literal `-0.0` folds to +0, while `-(0.0)`
  evaluates to -0.0.
- **Fix (code):** add `Neg bool` with `Coef ≥ 0`. While in there, make `Exp` an `int64`, since
  `int` is 32-bit on 386/arm. GRAMMAR has no bound on the exponent, so add a spec rule for an
  out-of-range one (`E1110`, or saturate).

**MF-8. The tree shape after a syntax error is undefined.**
- **Where:** the whole `syntax` node set. There is no `Bad*` kind and no documented nil rule.
- **Spec:** GRAMMAR §10 (recovery; "continues with the items that parsed"); IMPLEMENTATION-PLAN
  §4.1 (the language server gets a tree for broken files).
- **What differs:** `check`, `format` and `lsp` must know whether `LetDecl.Value` can be nil after
  `let x =`. Adding `Bad*` kinds later would change the frozen node set.
- **Fix:** decide now. Either add `BadExpr`, `BadType`, `BadStmt` and `BadDecl` (each spanning the
  skipped tokens), or document in `ast.go` and a DECISION: "a failed item is dropped; in a kept
  item, fields the grammar requires are never nil".

**MF-9. `lock.Add` accepts facts that `Format` writes and `Parse` then refuses.**
- **Where:** `internal/lock/lock.go:58-69` (`Add`), `:72-99` (`Format`).
- **Spec:** LOCK.md §2.2 and §2.3 ("A value appears on exactly one line").
- **What differs (verified by add → format → parse):**
  - an `enum` fact with a string value;
  - a `field` fact with `Retired`;
  - two `table` facts with the same holder and different `Value.Int`, which print two identical
    lines;
  - a name outside `File.Package`, or a holder that is not an identifier.

  M1 `build` is the first writer, and one bad fact makes the next build refuse its own lock.
- **Fix (code):** normalize and validate in `Add`, returning an error or documenting a panic as a
  programming error. Add a property test that feeds `Add` output back through `Format` and `Parse`.

**MF-10. The `maprange` analyzer does not exist.**
- **Where:** `internal/testkit/analyzers/maprange` is absent. `tools/audit/rules.md:347-348` says
  "which the maprange analyzer checks".
- **Spec:** IMPLEMENTATION-PLAN §6 (every milestone requires "the `maprange` analyzer clean"),
  §7.5; DOCTRINE §5.
- **What differs:** `//canon:unordered` is exempt from `ignore-count` (DECISIONS 56) on the premise
  that the analyzer checks it, and nothing does. M1 adds most of the output packages.
- **Fix (code, QA):** add the analyzer (go/analysis over `api`, `build`, `diag`, `edit`, `format`,
  `gen/...`, `i18n`, `ir`, `jsonsrc`, `lock`, `views`, `wire`) and wire it into `make check` through
  `go vet -vettool`.

**P1. Process: agents edited `spec/` without Louis, and consumer approval is unrecorded.**
- **Where:** the working tree holds uncommitted edits to `spec/API.md` (§1.1, §1.2, §2.1 `Roots`,
  `Len`, §8.8, V4a, X1), `spec/ERRORS.md` (§2.2 `Template`), `spec/IMPLEMENTATION-PLAN.md` (§4.2,
  §4.4, §4.5 sketches) and `spec/TYPES.md` (§2, `Refined` row removed).
- **Spec:** CLAUDE.md "Forbidden without asking Louis: any edit to … `spec/`"; DOCTRINE §8.
- **What differs:**
  - The edits are errata that match DECISIONS 57, 68, 77, 80 and 81 and several findings here.
    Some are behavioural sentences, though, such as the new X1 error-text grammar and `Roots`
    relative to the project root.
  - M0 acceptance also says "each [contract] approved by its consumers". No approval is recorded
    anywhere; `meta/state.md` says "awaiting review".
- **Fix:** leave the `spec/` edits uncommitted, or commit them only as a proposal marked for
  Louis, listed in `meta/state.md` Open Louis-calls. Record consumer sign-off per contract (this
  review can stand for QA's) before ticking M0.4. Add DECISIONS 72–90 to the "Review" Louis-call
  (state.md lists only 53–71).

---

## Should-fix

### Syntax (`internal/syntax`)

**S-SYN-1. A typed-nil child crashes the walker.**
- **Where:** `walk.go:46-53`. `visit` checks `n != none`, which only catches nil interfaces.
- **What happens:** a `(*BinaryExpr)(nil)` stored in an `Expr` field makes `Walk`, `Inspect`,
  `Children` and `File.Span` panic. Confirmed by a scratch test.
- **Fix:** add an unexported `isNil()` to `Node`, or document the invariant and test it.

**S-SYN-2. The zero `Tok` is a valid token.**
- **Where:** `constants.go:4` (`NoTok = -1`). Optional token fields left unset mean "present at
  token 0": `Modifiers.*`, `FnDecl.Self`, `EnumDecl.Ordered`, `FieldDecl.Input`,
  `TableType.Stable`, `ViewGroup.Advanced`, `ProjectEntry.Colon`, `Delims`. The tests set `NoTok`
  by hand (`nodes_test.go:231, 265, 297`).
- **Fix:** make `Tok` 1-based with `Tokens[0]` as a BOF sentinel and `NoTok = 0`. Only cheap
  before the freeze.

**S-SYN-3. `File.Trailing(n)` misses a comment after a separator comma.**
- **Where:** `ast.go:162`. In `a: 1, // c` the comment is the comma's trailing trivia.
- **Spec:** FORMATTER §8.1 and §10; API.md §9.1 (an item owns a trailing comment on its last line).
- **Fix:** include the next `,` token's `Trailing`, or document the rule.

**S-SYN-4. Token boundaries and the Leading/Trailing split are underspecified.**
- **Where:** `token.go:23-24, 38`; DECISIONS 73.
- **Spec:** GRAMMAR §2.6 and §3.2; FORMATTER §8.1 and §8.2.
- **What is not stated:**
  - which bytes `STRING_HEAD`, `STRING_MID` and `STRING_TAIL` cover;
  - whether `FORMAT_SPEC` includes the `:`;
  - the position of the zero-width `NL` token;
  - which tokens `Interp`'s bounds cover;
  - which side a multi-line block comment lands on;
  - that "line breaks = line numbers" means P's end line against N's start line.
- **Fix (doc):** a comment in `token.go` and a line in DECISIONS 73.

**S-SYN-5. The comprehension key and the project doc each lack a defined home.**
- **Where:** `ast_expr_lit.go:55, 70` (the key of `{ name: v for … }` is a `FieldItem` with an
  `*Ident`); `ast.go:127` against `ast_file.go:61` (`File.Doc` against `ProjectDecl.Doc`).
- **Spec:** TYPES §5.2 (keys of a comprehension are expressions).
- **Fix (doc):** "with `Clauses`, items are `MapItem`s keyed by an `*IdentExpr`"; "in
  project.canon the doc is `ProjectDecl.Doc`, `File.Doc` is nil".

**S-SYN-6. Exports M1 needs are missing (additive).**
- **What is missing:**
  - a frozen `Parse(src *source.File, kind FileKind, bag *diag.Bag) *File` signature. E1011
    depends on whether the file is `project.canon`.
  - a `LookupWord(string) TokenKind` / `IsNameable` table, for `format` and `edit` to print a
    reserved word used as a key (GRAMMAR §4.3).
- **Fix (code):** add both.

**S-SYN-7. Some syntax tests are circular.**
- **Where:**
  - `nodes_test.go`: the `kids` counts are hand-typed.
  - `file_test.go` `TestTokensAreLossless`: runs over a hand-built fixture with no `NL`, BOM,
    string parts or block comment.
  - `grammar_test.go:92-144`: the production→kind mapping is hand-typed. The production names
    themselves are read from GRAMMAR.md, which is good.
- **Fix:**
  - replace the `kids` test with a reflection test that fills every Node, interface and slice
    field. The sub-reviewer's version passes today: all 119 kinds yield every child.
  - require a lexer round trip over `examples/` plus fuzzing in M1.

### Diagnostics (`internal/diag`)

**S-DIAG-1. The message golden cannot catch a placeholder bound to the wrong argument of the same type.**
- **Where:** `builder_test.go:19-35`; `testdata/messages.txtar`. Every argument of a type gets the
  same sample, and `TestEveryMessageRenders` (`render_test.go:178-202`) checks `Contains` through
  the same renderer.
- **Fix:** position-distinct samples in the generated table, then regenerate the golden.

**S-DIAG-2. `Builder.Stack` called twice double-counts `MoreFrames`.**
- **Where:** `builder.go:76-81`. `Stack(20 frames)` twice gives 8, not 4.
- **Fix:** keep the cut count in its own field, overwritten by `Stack`.

**S-DIAG-3. A `Finding` cannot be added to a `Bag`, and bags cannot be merged.**
- **What breaks:** `workspace` caches cannot re-inject stored findings without a hand-built
  `Finding` (DECISIONS 54). Per-stage bags would double-count `Packages` and `Truncated`.
- **Fix:** add `(*Bag).Merge` or `Add(...Finding)` inside `diag`, or state that there is one bag
  per package across all stages.

**S-DIAG-4. The runtime check in `make diag-check` checks nothing today.**
- **Where:** `Makefile:53` and `catalog/runtime.go:12-31`. The `-runtime internal/gen` walk
  matches zero files and passes silently.
- **Fix:** fail when zero helper files are found, or state that the gate is empty until `gen`
  lands.

**S-DIAG-5. Five ERRORS.md §2.1 refusals are implemented but untested.**
- **Where:** `catalog/catalog_test.go:46-84`. Missing cases:
  - a duplicate variant name;
  - a duplicate or non-lowerCamel argument name;
  - a repeated or wordless Kind row;
  - message rows out of the codes table's order;
  - a malformed separator row.
- **Fix:** add the five rows.

**S-DIAG-6. `Findings()` shares memory with the bag.**
- **What happens:** it clones only the outer slice, so `Related`, `Stack` and `Reads` are shared
  (`bag.go:59`, `builder.go:113-115`).
- **Fix:** deep-copy them, or document findings as read-only.

**S-DIAG-7. Text arguments are rendered raw.**
- **What happens:** a `lock` E6005 `kind` argument carries a terminal escape or a tab straight into
  text output (`lock/parse.go:114-115`). The same holds for `\r` in any `Text`.
- **Fix:** sanitize `Text` arguments in `diag` (protects every code), or quote them in `lock`.

**S-DIAG-8. `checkWord` is both a JSON key and a message template.**
- **Where:** `json.go:37`; the summary nouns are shared the same way.
- **Fix:** add separate key constants.

### API (`api/`)

**S-API-1. `Version().Commit` reports the embedding program's commit.**
- **Where:** `api/build.go:115-126`. `vcs.revision` belongs to the main module; `vcs.modified` is
  ignored; it is empty for `go install …@v` builds. Verified with a scratch binary.
- **Spec:** API.md §14.
- **Fix:** use it only when `info.Main.Path` is the canonlang module, else use the `Deps` entry's
  version.

**S-API-2. `ErrUnknownLayer` cannot carry `E1901` or the layer name.**
- **Spec:** API.md O4 and R3; CLI.md §2.3 and §2.5.
- **Fix (spec first):** give it an error type or a `*ProjectError` with findings.

**S-API-3. O3 contradicts §15 on `E1001`.**
- **What differs:** O3 says a project.canon error wraps `ErrProject`, but §15 maps `E1001` to
  `ErrUnsupportedVersion`. `errors.Is(pe, ErrProject)` is false for it (verified).
- **Fix (spec):** say O3 wraps `ErrProject` or `ErrUnsupportedVersion`.

**S-API-4. The JSON forms fall short of what API.md says.**
- **Where:** `Edit` does not implement `json.Marshaler`/`Unmarshaler` as §8.8 says (the working
  tree rewords §8.8). `EvalResult{}` marshals `null` slices and maps where V4a and DECISIONS 45
  require `[]`/`{}`; `Heading.Cells` too.
- **Fix:** add `MarshalJSON` (an API addition, needs a DECISION), or require producers never to
  leave them nil.

**S-API-5. The Examples will stop working in M1.**
- **Where:** `api/example_test.go:16-40` opens `examples/` with the roots not redirected, so the
  output will depend on the machine once `Open` is real (`_fixtures/README.md`).
  `example_build_test.go:35` prints in map order. Several Examples call the panicking
  `Revision()` stub and will break at M1, not M4 as DECISIONS 67 and `api/doc.go` say.
- **Fix:**
  - redirect every root;
  - sort the map keys;
  - implement `Revision` with `Open`;
  - correct DECISIONS 67's milestone.

### Edit and lock

**S-EDIT-1. `edit` has no typed error.**
- **Where:** `edit/errors.go:6-12`, `path.go:114-116`. It exposes only a formatted string.
- **Spec:** API.md §15 (`*PathError{…, Detail}`).
- **What happens:** `api` would have to cut strings to fill `Detail`.
- **Fix:** export `type SyntaxError struct{ Offset int; Reason error }` unwrapping to `ErrBadPath`.
  Cheap now, a §4 change later.

**S-EDIT-2. The §4.6 sketch is stale.**
- **What differs:** it still says "errors wrap canon.ErrBadPath" and lacks `KeyLit.Raw`
  (DECISIONS 87).
- **Fix:** Louis updates §4.6.

**S-LOCK-1. The lock tolerates two things LOCK.md does not list.**
- **Where:** `lock/parse.go:51`, `constants.go:21`.
  - A lone trailing `\r` on the last line is accepted. LOCK.md §2.4 lists only CR LF (DECISIONS 88:
    "exactly").
  - The diff3/zdiff3 marker `|||||||` is reported as `kind`, not `merge`.
- **Fix:** strip `\r` only when LF follows; add `|||||||` to LOCK.md §2.4 and to `mergeMarkers`.

**S-LOCK-2. Per-code findings tests need a private `-update` writer.**
- **Where:** `lock/findings_test.go`. `golden.Run` writes `want`, while §7.2 names the section
  `findings.txt`, so every package with per-code cases would copy this writer.
- **Fix:** make the harness write `findings.txt`. The working tree is doing this: commit it.

### Types, value, IR

**S-VAL-1. `value.Ref` panics when its type is not a ref.**
- **Where:** `value/composite.go` (`Ref.identity`: `r, _ := v.T.Base().(*types.RefType)` then
  `r.Target`).
- **What happens:** it panics if `T` is not a ref type after `Base()`, for example `ref X?`.
  `Table.CanonText` panics likewise on an entry without `Ident`.
- **Fix:** document the invariant (a `Ref`'s `T.Base()` is `*types.RefType`) on the type and assert
  it in tests, or guard it.

**S-VAL-2. Map equality is quadratic.**
- **Where:** `Map.equal`, through `Map.Get`, which is linear.
- **Why it matters:** the NFR-01 benchmark has large maps.
- **Fix:** acceptable at v0; note for M4 memoization.

**S-TYP-1. Some types texts are odd, and `DefineType` is mutable.**
- **What happens:** `Refined{Of: Optional(Int), Range}` prints `Int?(0..)`, which is not writable
  syntax. `types.DefineType` is an exported mutable pointer global.
- **Fix:** have the checker never build the former (document it), and unexport `DefineType` or
  document it read-only.

**S-SRC-1. `TestAddNormalizes` does not test what DECISIONS 72 claims.**
- **Where:** `source/source_test.go:37`. It passes `dir\a.canon` but never asserts `Path`.
  `filepath.ToSlash` does not touch `\` on Linux, so the "both paths `/`-separated" claim of
  DECISIONS 72 holds only on Windows.
- **Fix:** assert the platform behaviour, or state it in DECISIONS 72.

### Audit and strictness

**S-AUD-1. `TestEveryCodeIsTested` does not exist.**
- **Spec:** §4.4 and §7.2 name it; DECISIONS 55 moved the judgment to the audit rule
  `diag-code-untested`.
- **Fix:** have Louis amend §4.4 and §7.2 to point to the rule, or add the test.

**S-AUD-2. The baseline and threshold guard compare against `HEAD`.**
- **Where:** `repo.DefaultBase`; the Makefile passes no `--base`.
- **What happens:** the guard catches uncommitted loosening, which suffices for the local,
  commit-after-`make check` workflow (DECISIONS 28). The CI workflow's "full clone for the
  baseline guard" rationale does not hold: in CI the working tree equals `HEAD`. CI never runs
  anyway (no remote).
- **Fix:** document this, or pass `--base HEAD~1` / merge-base in CI.

---

## Notes (verified correct, or low impact)

**Audit (DECISIONS 25–27, 53–56).**
- `diag-message-inline` (enforce): 5 detections, type-checked, tests included.
- `diag-code-untested` (ratchet from zero) and `diag-code-unreported` (observe).
- `ignore-count`: counts every pinned suppression syntax; `//canon:unordered` is exempt.
- `thresholds.tsv`: 20 keys, strictly parsed, raise guarded; stock linters rendered from it.
- Repository baseline is empty (271 → 0 entries in M0.5; shrink only).
- Compiler code has zero suppressions. `tools/audit`'s own baseline (ignore-count 4, 13 legacy
  `pkg-example`) did not grow.
- No code or message literal appears outside `internal/diag`, in `internal/`, `api/` or `cmd/`.

**Diaggen.**
- Deterministic.
- `make diag-check` diffs `codes.go` and `codes_test.go`; `TestCommittedFilesAreCurrent` does the
  same.
- The round-trip test rebuilds the ERRORS.md message rows and code/severity/package from the
  registry. It does not compare the §1.3 or §1.6 tables, nor Owner or Meaning.
- All 438 messages render: 435 through constructors, 3 runtime codes from samples. Pinned by
  `messages.txtar`, which is reproducible with `-update`.

**diag behaviour.**
- The F2 key, the EVALUATION §14 dedup key, F7 truncation and `Summary.Merge` match the spec.
- The text form is byte-exact on the heistia and farm `findings.txt`.
- The JSON key order and omissions follow F5.
- WIRE §7.3 escaping is correct (`<`, `>`, `&`, U+2028 raw; invalid UTF-8 becomes U+FFFD).
- `UnquoteJSON` is strict.

**Lock.**
- Every tolerance of LOCK.md §2.4 checked line by line.
- Canonical order and bytes are correct.
- The §9 sample test is real: it reads LOCK.md at test time and checks sizes and SHA-256.
- The teamboard lock matches byte for byte.
- Missing for M1 (VER): the comparison of the lock with the program's facts
  (E6001/E6002/W6006). `lock` is not a §4 contract, so adding it is not a contract change.
- Parsing unsorted input is O(n²): 3.6 s for 50k reversed lines. Sort once instead.

**Edit path.**
- The grammar matches API.md §6.1 in every case probed.
- `Parse(s).String() == s` is fuzzed.
- `String()` is only guaranteed for paths from `Parse`.

**`api` split.** "No API change" is verified with a go/types dump of both surfaces. The only
additions are `ErrSyntax` (DECISIONS 48) and `(*SyntaxError).Unwrap`. The latter is implied by
§15 but not named in §12.4 or a DECISION; record it. Enums and JSON tags match API.md.

**`cmd/canon`.** One stderr line and exit 2 (DECISIONS 62). A usage text, not a finding, so it is
legitimately outside `diag`.

**Dependency test** (`internal/testkit/deps_test.go`).
- Reads §3 and §11 at test time.
- Has a synthetic-violation test.
- Stats sources to defeat the test cache.
- All imports conform: `lock → diag, source`; `edit → diag`; `value → types, source, diag`;
  `ir → types, value, diag`.

**`syntax` coverage.**
- All 114 GRAMMAR ebnf productions have a node or field.
- `children()` is exhaustive for all 119 kinds (reflection check).
- The sealed interfaces allow type switches.
- Unary-minus folding keeps tokens lossless.
- `E1133` is owned by `syntax` (ERRORS.md Package column), so nodes without `Mods` are fine if the
  parser reports it. DECISIONS 75's "Modifiers … for E1133" wording should say so.

**Types, value, IR.**
- Match TYPES §2, STD-06 §9 (text forms, float layout, duration decomposition, nested-string
  escapes) and TYPES §7.5 equality.
- They carry every FINGERPRINT §3 input.
- They follow DECISIONS 77–80.
- `Identical`, `Assignable` and `Join` are deferred to M1 as additive functions (DECISIONS 77).

**Tautological tests** (harmless, but they prove nothing):
- `value.TestProv` (field assignment);
- `ir.TestEnums` (distinct iota values);
- `ExampleSet`, `ExampleSetCase` and `ExampleValue_String` (echo their inputs);
- the trivial golden (by design).

**Harness.** One trivial golden runs (M0 acceptance). `fixturegen` is deterministic (sorted
manifest, budget checked before writing) but untested on real data (`testdata-real/` is empty).

**CI.** `.github/workflows/check.yml` is correct on paper, but the repository has no remote
(DECISIONS 28), so it never runs (DECISIONS 65 acknowledges this).

## Spec gaps for Louis

- GRAMMAR has no bound on a float exponent (MF-7).
- The tree shape after a parse error (MF-8).
- Where `Snapshot` lives relative to the two-seam rule (MF-2).
- How API findings are rendered and the missing `MoreFrames` (MF-5).
- What `ErrUnknownLayer` carries (S-API-2).
- The O3/§15 contradiction (S-API-3).
- `EvalResult`'s nil collections, and which `Summary` V4a means: the CLI summary has `ms`
  (S-API-4).
- The diff3 marker in LOCK.md (S-LOCK-1).
- The BOM before the `canon.lock` header (LOCK.md §2.4 refuses it with a misleading `missing
  header`).
- Dedup across packages (EVALUATION §14 is silent).
- Stale text:
  - GRAMMAR §2.1 and §10 against DECISIONS 73: `breaks`, leading trivia;
  - §4.4 and §7.2 `TestEveryCodeIsTested` (S-AUD-1);
  - §12.4's "first `cmd/canon` path that reaches the API";
  - §12.5 on `meta/`.
