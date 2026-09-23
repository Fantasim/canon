# Canon spec audit 2: final pre-lock review of v0.1

Scope: DECISIONS.md (1–27, binding), meta/spec-phase/review/ACCEPTED-CHOICES.md (binding), SPEC.md, CLI.md,
README.md, every file in `spec/` (including `viewmodel.schema.json` and ERRORS.md),
`api/canon.go`, and every file under `examples/` (sources, `_fixtures`, `features`, `game`,
every `expected/`). AUDIT.md is the checklist. The question is the same as for AUDIT.md: could
parallel agents build each module from the text alone and end up with compatible results?

## Verdict

**v0.1 can be locked once four blockers are fixed. Each fix is small and mechanical, about a
day of work, and none of them reopens the language design.** The companion documents close
every one of AUDIT.md's blockers. The error catalogue is internally exact: 298 codes, each
defined in exactly one diagnostics table and owned by the document ERRORS.md names. No section
cross-reference is dangling. The byte-level artifacts reproduce from the spec by hand:

- the pipeline fingerprint `pipeline.Potion@f750790e` recomputes with `sha256sum`;
- all ten FINGERPRINT vectors, the four WIRE samples and both LOCK samples hash as stated;
- `potions.json` and `teamboard/expected/canon.lock` match their spec samples byte for byte;
- `potion.view.json` validates against `viewmodel.schema.json` (Draft 2020-12, 0 errors);
- the C++ goldens compile warning-free with g++ 15 and clang 21, under
  `-std=c++17 -Wall -Wextra -Wpedantic -Werror`, and the conformance entry point returns 0;
- the Go goldens vet, build and test under `-race`;
- `GOTOOLCHAIN=local make check` passes.

What still blocks parallel work:

1. **Diagnostics.** DECISIONS 27 makes `spec/ERRORS.md` the source that `internal/diag/codes.go`
   is generated from. IMPLEMENTATION-PLAN §4.4 and M0 say the opposite, and ERRORS.md holds no
   message templates or argument lists. The templates that do exist are prose with
   alternatives, so the generator and the `diag.E3501.At(span, args…)` call convention cannot
   be built compatibly.
2. **Fingerprint vectors 4 and 10** contradict FINGERPRINT §4.4's own rule and
   `resource/vocab/vocab.canon`: wrong arm order, and Canon names where the rule says wire
   values. Computed by the rule, the hashes are `39fd8523` and `6dc7944d`.
3. **W1640.** Its rule (VIEWMODEL N4) applies to every package. The findings goldens assume it
   fires only for packages with `emit view`. Read literally, `canon check teamboard`, the v0
   (M1) acceptance test, prints 10 warnings where its golden has 0.
4. **Missing frozen contracts.** No frozen contract fixes what the type checker hands the
   evaluator. None fixes how the evaluator reaches `load` and `verify`, which the plan's own
   dependency rule forbids it to import. The TYP, EVL, LOD and VER agents would each invent
   that glue in M1.

**Counts: 4 blockers, 15 guesses, 16 nits (35 new findings).** AUDIT.md's blockers: all 82
are RESOLVED. The text of AUDIT.md says 81, but it holds 82 blocker entries (A2-35). Four of them
(EMT-01, GEN-07, DIAG-01, ORG-01) are resolved, but a new finding reopens a detail of each.

Severity key as in AUDIT.md. **blocker**: two engineers would build incompatible things, or
cannot proceed. **guess**: the text leaves a choice, and a default is proposed. **nit**:
cosmetic or low risk.

---

## 1. AUDIT.md blockers

"Where" names the section that now settles the point. `*` marks a blocker that is resolved but
has a detail reopened by a new finding of §2.

| AUDIT id | Status | Where it is settled |
|---|---|---|
| GEN-01 | RESOLVED | SPEC preamble and IMPLEMENTATION-PLAN §7.1 (illustrative, semantic compare until M2, then byte for byte); goldens regenerated around `pipeline.Potion@f750790e` (FINGERPRINT vector 1, recomputed); `potions.json` = WIRE §8.3 byte for byte. Residual drift in the code goldens: A2-10 |
| GEN-02 | RESOLVED | SPEC §5.2, CODEGEN §4.1; goldens use `int64`/`int64_t` |
| GEN-03 | RESOLVED | TYPES §3.1 (`E2006`); files moved to `balance/parity/` and `service/resourcestudio/` |
| GEN-04 | RESOLVED | WIRE §2.1–§2.3 (lexical `..`, containment, `E7001`); `examples/project.canon` roots `sovcommon`, `web`, `parity`, `generated`, `pipeline_go`, `features` |
| GEN-05 | RESOLVED | CODEGEN §2.4, WIRE §8.4, `--adopt` (CLI §3.4, §3.10; API B2) |
| GEN-07 | RESOLVED* | GRAMMAR §2.2/§9.1 (package doc), I18N §5 (W1701 once per package and language, `emit view` packages only); findings goldens list W1701. W1640 scope: A2-03 |
| LEX-01 | RESOLVED | GRAMMAR §2.7 |
| LEX-02 | RESOLVED | GRAMMAR §2.6, §2.9 |
| LEX-08 | RESOLVED | GRAMMAR §4.3 (nameable words) |
| GRM-01 | RESOLVED | GRAMMAR §6.1 (header mode) |
| GRM-02 | RESOLVED | GRAMMAR §3.1 rule 1 |
| GRM-03 | RESOLVED | GRAMMAR §3.1 rule 4 (see A2-26) |
| GRM-04 | RESOLVED | GRAMMAR §5.9 `refType = "ref" qualifiedIdent` |
| GRM-05 | RESOLVED | GRAMMAR §6.6, TYPES §7.4 |
| GRM-07 | RESOLVED | GRAMMAR §5.2, §7 |
| GRM-10 | RESOLVED | TYPES §5.2 |
| GRM-15 | RESOLVED | GRAMMAR §5.9 `fnType`, TYPES §12.3 |
| RES-01 | RESOLVED | TYPES §1 (one bidirectional pass, symbolic keys) |
| RES-02 | RESOLVED | TYPES §4.2; taxonomy `icon: columns` is the member |
| RES-03 | RESOLVED | TYPES §10.2, EVALUATION §3.4 |
| RES-05 | RESOLVED | TYPES §3.5 |
| TYP-01 | RESOLVED | DECISIONS 17, TYPES §6.5–§6.6 (examples fixed). Assignability gap: A2-05 |
| TYP-02 | RESOLVED | TYPES §6.2–§6.3 |
| TYP-03 | RESOLVED | TYPES §7.2 |
| TYP-04 | RESOLVED | TYPES §6.2 (storage points), §7.4; EVALUATION §4.3 |
| TYP-06 | RESOLVED | TYPES §5.1, WIRE §6.1 (`E7002`); vocab and sweep_plan annotated |
| TYP-07 | RESOLVED | STDLIB §1, TYPES §12.2 |
| DEP-01 | RESOLVED | TYPES §11.4 |
| DEP-02 | RESOLVED | TYPES §11.6, EVALUATION §5 |
| DEP-03 | RESOLVED | CODEGEN §5.6 (`<Alias>Branch`) |
| DEP-04 | RESOLVED | TYPES §11.5, CODEGEN §4.2, VIEWMODEL J11–J14 |
| EVL-01 | RESOLVED | EVALUATION §2.1 |
| EVL-02 | RESOLVED | EVALUATION §4.2, §8.1 (per instance) |
| EVL-03 | RESOLVED | EVALUATION §12, STDLIB §1.3 and per-function costs |
| EVL-04 | RESOLVED | EVALUATION §4.1 |
| CHK-01 | RESOLVED | EVALUATION §8.3 |
| STD-01 | RESOLVED | STDLIB §4–§6 |
| STD-03 | RESOLVED | STDLIB §8, SPEC §5.2 (search semantics) |
| STD-06 | RESOLVED | STDLIB §9 |
| LCK-02 | RESOLVED | LOCK §1 |
| LOD-01 | RESOLVED | WIRE §6.8 |
| LOD-02 | RESOLVED | WIRE §3, §5.1, §5.4 |
| LOD-04 | RESOLVED | WIRE §5.7, §6.5 |
| WIR-01 | RESOLVED | WIRE §7, §8.2 (samples hash as stated) |
| WIR-02 | RESOLVED | WIRE §5.4 |
| WIR-03 | RESOLVED | WIRE §5.1 |
| WIR-04 | RESOLVED | WIRE §5.3 |
| WIR-05 | RESOLVED | WIRE §5.8 |
| WIR-06 | RESOLVED | WIRE §5.7, §8.2 |
| WIR-07 | RESOLVED | WIRE §4.2, §5.6 |
| EMT-01 | RESOLVED* | FINGERPRINT §3–§4 (vectors 1–3 and 5–9 reproduce). Wrong vectors 4 and 10: A2-02 |
| EMT-03 | RESOLVED | CODEGEN §2.1, WIRE §8.1 |
| EMT-04 | RESOLVED | CODEGEN §5.3 |
| EMT-05 | RESOLVED | CODEGEN §5.13 |
| EMT-06 | RESOLVED | CODEGEN §2.8, GRAMMAR §7.1 `go_module` |
| CG-01 | RESOLVED | CODEGEN §3.1–§3.3 |
| CG-02 | RESOLVED | CODEGEN §3.4–§3.5. When the codes fire: A2-08 |
| CG-03 | RESOLVED | CODEGEN §5.8 |
| CG-04 | RESOLVED | CODEGEN §5.9 |
| GO-01 | RESOLVED | CODEGEN §5.11; golden has `PipelineSnapshot`/`PipelineStore`/`var Store` |
| CPP-01 | RESOLVED | CODEGEN §7.8 (fixture gap: A2-14) |
| TS-01 | RESOLVED | CODEGEN §4, §8 |
| CNF-01 | RESOLVED | CONFORMANCE §6; the 11 pipeline vectors match §6.6 |
| CNF-02 | RESOLVED | CONFORMANCE §2–§4, DECISIONS 19 |
| RLD-01 | RESOLVED | CODEGEN §5.11, WIRE §8.1 (`E8153`) |
| VIEW-01 | RESOLVED | API §11 (`Evaluate`) |
| VM-01 | RESOLVED | VIEWMODEL §12 + `viewmodel.schema.json`; golden validates |
| VM-02 | RESOLVED | VIEWMODEL §12.3 |
| VM-03 | RESOLVED | VIEWMODEL J7 |
| I18N-01 | RESOLVED | I18N §3 (farm catalogue of 88 keys recounted by hand) |
| LAY-02 | RESOLVED | EVALUATION §9.2–§9.3 |
| LAY-04 | RESOLVED | EVALUATION §11, CODEGEN §5.12 |
| DIAG-01 | RESOLVED* | ERRORS.md: complete and consistent. Registry mechanism contradicts DECISIONS 27: A2-01 |
| FMT-01 | RESOLVED | FORMATTER.md (DECISIONS 18) |
| FMT-02 | RESOLVED | FORMATTER §14 |
| API-01 | RESOLVED | API.md, `api/canon.go` |
| API-02 | RESOLVED | API §6 |
| API-03 | RESOLVED | API §9, FORMATTER §13 |
| CLI-02 | RESOLVED | IMPLEMENTATION-PLAN §8.3 |
| NFR-01 | RESOLVED | IMPLEMENTATION-PLAN §7.6 |
| ORG-01 | RESOLVED* | IMPLEMENTATION-PLAN §3–§5. Two missing contracts: A2-04 |
| ORG-02 | RESOLVED | `examples/_fixtures/` + README; `expected/findings.txt` for every example |

---

## 2. New findings

### 2.1 Blockers

**A2-01 · blocker · Where diagnostic codes, messages and arguments come from**
- Where:
  - DECISIONS 27: "`spec/ERRORS.md` is the single source; `internal/diag/codes.go` is generated
    from it and diff-checked by `make check`. Code reports `diag.E3501.At(span, args…)`; no
    diagnostic message text appears anywhere else".
  - IMPLEMENTATION-PLAN §4.4 says the reverse: "In M0, `codes.go` is written from it; from then on
    `go run ./internal/diag/cmd/errorsdoc > spec/ERRORS.md` regenerates it from the registry and
    CI fails if the committed file differs". So does M0's acceptance: "`spec/ERRORS.md`
    regenerated from the registry is unchanged". The addendum repeats DECISIONS 27.
  - ERRORS.md itself: "The message templates are in the owning documents". Its rows hold only
    code, severity, owning *document* and a meaning. `diag.Def` needs `Template` and `Owner`
    (a *package*, "e.g. `load`").
  - §4.4 declares codes as string constants (`type Code string`, "`diag.E3501`"). DECISIONS 27
    calls a method on them (`diag.E3501.At(…)`).
- Problem: four things are undefined.
  - Which file is the source.
  - Where the generator finds a template.
  - How `args…` map to placeholders. Templates reuse names (`{name}` twice in W1604) and mix
    literal braces (`{{index}}` in E1623).
  - What the message of a code with several templates is. At least 27 templates are prose
    alternatives, for example:
    - TYPES E2103: `` `ref {T}`: {no collection of `{T}` / several collections: `{list}`}; … ``;
    - GRAMMAR E1008: two templates separated by ` / `;
    - LOCK E6002: six variants;
    - VIEWMODEL E1606: `{columns \| filters}`.

  The QA agent writes the generator and every other agent calls `At`: they cannot agree on a
  convention from this text. LOCK.md is an example: its E6001 template
  (`{collection}.{holder} was removed{rename}; retire it instead`, :492) does not produce its
  own examples (:144, :357–359, :468).
- Proposed:
  1. ERRORS.md is the source (DECISIONS 27 wins). Delete the reverse-generation sentence and
     M0's "regenerated … unchanged" criterion from IMPLEMENTATION-PLAN §4.4 and §6. Rewrite
     ERRORS.md's own preamble, which says "From M0 the registry regenerates this file".
  2. Give each ERRORS.md row four more columns:
     - `Package`: the owning Go package;
     - `Template`: exactly one English template;
     - `Args`: the ordered, comma-separated placeholder names;
     - `Variant`, when needed.

     A code with several messages gets one row per variant (`E2103/none`, `E2103/several`), and
     `At` takes the variant as its first argument.
  3. Placeholders are `{name}`. `{{` and `}}` are literal braces. `args` are positional, in the
     order of the `Args` column, and formatted with the canonical text form of STDLIB §9.
  4. Owning documents keep the trigger column only, or mark their templates "informative, see
     ERRORS.md".
  5. Freeze `func (d Def) At(span source.Span, args ...any) *diag.Builder`, with
     `.Related(span, note)`, `.Path(p)`, `.Check(name)`, `.Layer(l)`, `.Stack(frames)` and
     `.Report(bag)`. Make `diag.E3501` a `Def` value, not a `Code` string.

**A2-02 · blocker · Fingerprint vectors 4 and 10 break FINGERPRINT's own rule**
- Where:
  - FINGERPRINT §4.4: "one arm per member of the discriminant's enum in declaration order
    (retired included), keyed by the member's wire value".
  - `resource/vocab/vocab.canon:124-134` declares `none_, monster, item, dungeon, element, stat,
    upgrade_type = "upgradeType", game_mode = "gameMode", quest_style = "questStyle"`.
  - Vector 4 (FINGERPRINT.md:418) and vector 10 (:634-635) read
    `"none_"=never,"monster"=…,"item"=…,"dungeon"=…,"upgrade_type"=ref(String),"game_mode"=String,"quest_style"=@2,"element"=@3,"stat"=never`.
- Problem: that order is neither declaration order nor arm order. The keys are Canon names, not
  wire values. In vector 4 the pre-order numbering is also wrong: Element must be `@2` and
  QuestStyle `@3`. M2 requires "FINGERPRINT.md's test vectors pass". An agent must choose
  between the rule and the vector, so two agents can emit different `$schema` values for every
  dependent type.
- Proposed: keep the rule and replace the two vectors. Computed with the rule and checked with
  `sha256sum`:
  - vector 4: arms `…"dungeon"=ref(String),"element"=@2,"stat"=never,"upgradeType"=ref(String),"gameMode"=String,"questStyle"=@3`, with the Element and QuestStyle blocks swapped. 1491 bytes, SHA-256 `39fd85236e45fc388bc91810ac7921dea1f207503734605e690e97563a90334b`, `$schema` `resource.heistia.HeistiaConfig@39fd8523`.
  - vector 10: the same arms, with `@3` for QuestStyle and `@11` for Element; numbering
    otherwise unchanged. 3719 bytes, SHA-256 `6dc7944d6240802b4af81487a4d053bfc59cf9412c7c67e81ed7b33ef9620924`, `$schema` `resource.adventurequest.AdventureQuestConfig@6dc7944d`.

**A2-03 · blocker · W1640's scope contradicts the findings goldens, including the v0 test**
- Where:
  - VIEWMODEL N4 (:843-845): "Public values without a menu appear in a final 'No menu' section
    … An editable one (`editable` is not `none`) outside the studio package gets `W1640`."
  - ERRORS.md:127: "an editable public value has no menu".
  - `teamboard/expected/findings.txt`: `0 errors, 0 warnings in 1 package (…)`.
  - IMPLEMENTATION-PLAN M1 item 2: "`canon check teamboard` … prints exactly
    `examples/teamboard/expected/findings.txt`".
- Problem: teamboard has 10 editable public lets (`intents` … `areas`, taxonomy.canon:152-236),
  no view and no menu. Under N4 as written that is 10 × W1640. The same holds for several other
  goldens that expect 0 warnings:
  - `service/resourcestudio` (`config`);
  - `balance/parity` (`settings`, `plan`);
  - `features/codes`, `embedded`, `legacycpp`, `match`, `retired`.

  Every goldened package that has an `emit view` also gives its public values a menu. So the
  goldens were written for a rule scoped like W1701 (I18N W1: "only for packages that emit a
  view"). One agent following VIEWMODEL fails M1; one following the goldens contradicts the
  normative text.
- Proposed: scope it like W1701. Add to N4 and to ERRORS.md: "reported only for a selected
  package that has an `emit view`".

**A2-04 · blocker · Two contracts missing for M1's parallel work**
- Where:
  - IMPLEMENTATION-PLAN §4 freezes six contracts: AST, `types`, `value`, `diag`, `ir`,
    `edit.Path`.
  - §3: `eval` "Consumes: check, value". `load` and `verify` are listed below `eval`, and "an
    arrow may only point up this table" (CI-checked by `deps_test.go`).
  - EVALUATION §1 and §3.1 require the evaluator to force `load` expressions. It must also
    verify, at once, "a value forced for the first time after stage B has started".
  - §5.2: in M1, TYP (checker), EVL (eval), VER (verify) and LOD (`wire`) run in parallel.
- Problem: two interfaces are missing.
  - **What `check` hands on.** Nothing fixes what `eval`, `views`, `ir` and `conform` receive
    from `check`: resolved objects per identifier, the type of every expression, and the
    implicit conversions TYPES §6.2 applies (deref, entry→ref, wrap, int literal→Float). Also
    the symbolic keys, the resolved `ref` targets (`types.Collection`), the chosen stdlib
    overloads, and which `match` arms are exhaustive.
  - **How `eval` calls down.** It cannot import `load` or `verify`, and no host interface is
    specified.

  Each of the four agents would invent its own glue in M1, and the pieces would not fit.
  (Also: `cli`'s row consumes `infer` and `convert`, which sit below it in the table, so
  `deps_test.go` fails as written.)
- Proposed: add two frozen contracts to §4 and finish them in M0.
  - **§4.7 `internal/check/info.go`.** On the model of `go/types.Info`:
    - `type Info struct { Types map[syntax.Expr]types.Type; Uses map[*syntax.Ident]Object; Conv map[syntax.Expr]Conversion; Symbolic map[syntax.Expr]*types.Collection; Calls map[*syntax.CallExpr]Builtin }`;
    - `type Object interface{ … }`, with kinds Const, Let, Fn, Param, Local, Field, Member, Case, Entry, Builtin;
    - `func Check(files …) (*Program, []diag.Finding)`.
  - **§4.8 `internal/eval/host.go`.**
    - `type Host interface { Load(ctx, *syntax.LoadExpr, types.Type) (value.Value, []diag.Finding); Verify(root string, v value.Value) []diag.Finding; Assets(root string) (Listing, error) }`.
    - `build` implements it with `load` and `verify`.
  - Reorder the §3 table so that `infer` and `convert` come before `cli`.

### 2.2 Guesses

**A2-05 · guess · No optional-to-optional assignability, which taxonomy and rules need**
- Where: TYPES §6.2 lists `T ≤ T?` ("and `S ≤ T` implies `S ≤ T?`"), `T? ≤ T` never, and
  `[S] ≤ [T]` and maps component-wise, then says "Anything else is `E3002`". There is no row
  for `S? ≤ T?`.
- Problem: two v0 examples need that row.
  - taxonomy.canon:397-398: `export fn columnOf(s: ref Status) -> ref Column?` returns
    `columns.active().first(…)`, which is a `Column?`.
  - rules.canon:36: in `cycles(nodes, next: .parent)`, the shorthand returns `ref TalentNode?`
    where `fn(T) -> T?` with `T = TalentNode` is expected.

  A literal reading rejects both. M1 would still pass once someone adds the rule, but two
  checkers could disagree on edge cases (joins, `??`).
- Proposed: add the row "`S? ≤ T?` if `S ≤ T`: the conversion applies to a present value;
  `none` stays `none`".

**A2-06 · guess · `?.` after a segment that is not optional**
- Where:
  - TYPES §6.5: "`!`, `?.` or `??` on a non-optional left side … are `W3401`", and "each `?.`
    unwraps its receiver for the suffixes after it".
  - sweep_plan.canon:260-261: `weapon: w?.item?.id ?? ""`, `weaponId: w?.item?.value ?? 0`,
    where `item: ref itemH` is not optional.
  - `balance/parity/expected/findings.txt`: 0 warnings.
- Problem: is the left side of the second `?.` the chain prefix (optional as an expression) or
  the unwrapped segment `item` (not optional)? The two readings give different findings for
  the golden. The chain type is ambiguous too: `etcJobs.first(…)?.jobBase` (sweep_plan:148)
  ends on a `String?` field, and "made optional once at the end" would give `String??`.
- Proposed:
  - `W3401` fires on a `?.` whose receiver segment, after the earlier `?.` unwraps, is not
    optional.
  - A chain whose last segment is already optional is not wrapped again.
  - Rewrite sweep_plan as `w?.item.id ?? ""` and `w?.item.value ?? 0`, as TYPES §6.5's own
    example does.

**A2-07 · guess · Keyed-list `entry` bodies: SPEC and TYPES disagree**
- Where:
  - SPEC §4.3: "For a keyed list … the key is the entry's field `f`: it may be omitted from the
    body, and if written it must equal the key". Also: the collection's "literal must be a table
    literal or a list literal (possibly empty); on anything else it is `E3103`".
  - TYPES §9.3: "for a keyed list, `k` is the value of the key field, which the body must then
    omit (… giving it is `E3321`)". TYPES puts no condition on the initializer.
- Proposed: TYPES wins (companion owns typing): giving the key field is `E3321`; `entry` needs
  the `let`'s initializer to be a table or list literal (possibly empty), else `E3103`. Fix
  SPEC §4.3's first sentence and add the initializer rule to TYPES §9.3.

**A2-08 · guess · When emit-validation codes are reported**
- Where:
  - CODEGEN §12 owns `E8002`–`E8018`, `E8101`, `E8103`–`E8109`, `E8201`/`E8202` and `W8006`,
    but never says in which phase they are reported. WIRE says it for `E8102` only ("reported
    during `check`").
  - CLI §3.3: "`check` and `build` report the same findings".
  - Concrete case: `game/items/item.canon:84`, `enum ItemElement { _NONE, _FIRE, … }`. The
    package has `emit cpp`, and CODEGEN §3.4 says "A name starting with `_` followed by an
    uppercase letter … is `E8005`". Yet `game/items/expected/findings.txt` has 0 errors.
- Problem: if these codes are found only at emit, `check` and `build` disagree and the golden
  hides a failing build. If they are found in `check`, the game.items golden is wrong.
- Proposed:
  - Every emit-validation code is computed in `canon check` (stage E+, on the IR, writing
    nothing); only `E8001` and `E8152` need the file system.
  - Add `@cpp(name: "None_")` and so on to `ItemElement`'s members, or rename them.

**A2-09 · guess · DECISIONS 26 ("no exemptions") against the embedded runtime sources**
- Where: DECISIONS 26: "the strictest technically sound option; there are no exemptions".
  IMPLEMENTATION-PLAN §12.4: "the runtime helper texts (`rt.go`, …) are embedded files
  (`//go:embed`) under `runtime/`, output rather than code, excluded from the audit".
- Problem: an `rt.go` inside the compiler module is a Go package that `go vet ./...` and
  `go build ./...` compile. It also breaks the doctrine: bare numbers (`86_400_000`), literals
  used twice, nesting over 3. Either DECISIONS 26 is broken or `rt.go` cannot be written as
  specified.
- Proposed: store runtime texts as non-Go data (`runtime/rt.go.txt`, `canon_runtime.h.txt`),
  embedded and written out under their real names. They are data files, so neither Go tooling
  nor the auditor judges them, and no exemption is needed. Say so in §12.4.

**A2-10 · guess · Generated Go/C++ text beyond what CODEGEN fixes**
- Where:
  - CODEGEN §2.6: "The doc of a field goes on its getter … a record's or enum's doc on the type".
  - §2.5: "No other prose is generated". §2.7: fixed section order.
  - `pipeline.gen.h:46,54,55` and `go/potions.gen.go:31,49,51` drop the docs of `Potion`,
    `cooldown` and `stack`, which potion.canon:11, 19 and 21 do have.
  - The goldens contain prose that is not in the T-table (`pipeline.gen.h:86,115-118`,
    `potions.gen.go:75-85,109-120`).
  - C++ needs declare-before-use: `detail::Decode` and `Potion_healFor` come before
    `class Potion`, at `pipeline.gen.h:35-44`. That contradicts §2.7's order.
  - The include set and order of `.gen.cpp` are unstated.
- Problem: from M2 on the goldens are compared byte for byte, and the CODEGEN text does not fix
  these bytes. Comments are ignored until M2 (IMPLEMENTATION-PLAN §7.1), so no work is blocked
  today.
- Proposed:
  - Add the three missing docs to the goldens.
  - Declare every fixed comment that appears in the goldens normative. Extend the T-table,
    per target.
  - State the real C++ order: forward declarations, then `detail` decoders and pure functions,
    then classes.
  - Add an include table: standard headers from the std names used, sorted, then
    `canon_runtime*.h`.

**A2-11 · guess · Text formats CLI calls "golden" are not fully specified**
- Where: CLI preamble: "The sample outputs … (findings, `test`, `explain`, `refs`, summaries)
  are the golden formats: an implementation prints exactly these layouts".
  - §3.5 shows only a `fails` failure. It uses `…` elisions inside the expect text, and its
    `event.canon:170` is stale (the test is at :176).
  - §3.7 (`explain`) and §3.8 (`refs`) align columns with unstated widths.
- Problem: M2 item 4 requires that breaking `healFor` "reports the failing `expect` in the
  CLI.md §3.5 format". That is a comparison failure, and no sample shows one.
- Proposed:
  - `test` prints `FAIL  <file>:<line>  <name>`.
  - Then, per failing expect:
    - `  <expect source text, whitespace runs collapsed to one space>`;
    - `  expected: <text>` and `  got: <text>` for comparisons (the STD-06 text form);
    - `  got: no finding`, or one line per finding, for `fails`/`warns`.
  - `explain` and `refs` use two spaces between columns and no padding (DECISIONS 18 in
    spirit).
  - Fix the stale line.

**A2-12 · guess · The JSON form of `Evaluate` is not specified**
- Where: `api/canon.go:691-739`: `EvalRequest` and `EvalResult` carry no JSON tags, and API §11
  gives no JSON form. VIEWMODEL §13 shows a lowerCamel exchange
  (`{"path": …, "lang": "fr", "draft": […]}`, `"title": {"value", "ok", "fallback"}`), "as the
  studio's web client receives them".
- Problem: `encoding/json` would write `Revision`, `Title`, `OK`. The studio's web client and
  the API team would then disagree at the M4 integration spike.
- Proposed: add JSON tags matching VIEWMODEL §13 (lowerCamel, `ok` for `OK`, maps with keys in
  byte order as API §1.4 says) to every result type, and state in API §11 that this is the wire
  form.

**A2-13 · guess · Frozen `types.Kind` set incomplete**
- Where:
  - IMPLEMENTATION-PLAN §4.2:
    `const ( Bool Kind = iota; Int; Float; String; Duration; Enum; Record; Variant; Case; List; Map; DepMap; Table; Ref; Optional; Union; Range; Asset; Never; Any; Func; Applied )`.
  - TYPES §2 also defines `KeyedList`, `Kind` (the type of `v.kind`), `Pair`, `DepUnion`
    distinct from `TypeApp`, `Define`, `None` and `Error`, and calls the literal union
    `LitUnion`.
- Problem: consumers of the frozen contract (EVL, IR, VM) cannot represent `Pair`, `Kind(V)`
  or the error type.
- Proposed: add the missing kinds to the M0 contract (`KeyedList` may stay
  `List`+`KeyedBy`, but say so).

**A2-14 · guess · The legacycpp fixture header is missing**
- Where:
  - IMPLEMENTATION-PLAN M6: "the `examples/features/legacycpp` `ItemProp` example compiles in
    all three modes against its hand-written header fixture".
  - `legacycpp.canon:28` names `header: "ProjectCmn.h"`.
  - No such file exists under `examples/` or `examples/_fixtures/`.
- Proposed: add `examples/features/legacycpp/ProjectCmn.h`, a minimal `struct ItemProp` with the
  five mapped members and one unmapped member, and list it in `_fixtures/README.md`.

**A2-15 · guess · Finite parameters: is `ref` into a keyed list one?**
- Where:
  - SPEC §9.4: "only finite types: `Bool`, enums, `ref` into a table".
  - CODEGEN §5.10: "every parameter finite: `Bool`, enum, `ref` into a table".
  - CONFORMANCE §2.1: `ref` parameters are `E9006` for translated functions.
- Problem: an `export fn` with a parameter `ref potions` (a keyed list) is neither lookup nor
  translatable, and no code says so.
- Proposed: keyed-list refs are finite too; the domain is the entries in entry order, as for
  tables. Or: state that such a parameter is `E9006`.

**A2-16 · guess · M1's build acceptance and TS emits**
- Where: M1 item 3: "`canon build teamboard sovcommon...` writes outputs equal to their goldens:
  Go … and JSON". `taxonomy.canon`, `ui.canon` and `roles.canon` also have `emit ts`, but
  `gen/ts` is complete only in M6.
- Proposed: M1 item 3 runs with `--target go --target json`.

**A2-17 · guess · rules.canon's findings golden misses 20 × E3301**
- Where:
  - `resource/rules/rules.canon:45`:
    `local let guildTalentTree: GuildTalentTree = load("@resource/Server/System/GuildTalentTree.json")`.
    It has no `partial: true`; the file's three other loads do.
  - `TalentNode` declares only `id` and `parent`. Each of the 4 fixture nodes also has
    `buffId`, `price`, `minGuildLevel`, `x` and `y`.
  - WIRE §5.5.1: "Keys that no field claims are `E3301`, one finding per key, unless the `load`
    has `partial: true`".
  - `resource/rules/expected/findings.txt`: 0 errors.
- Problem: a correct compiler prints 20 × E3301 and poisons the value, so M3 item 1 fails. The
  spec is clear; the example is wrong.
- Proposed: add `partial: true` at rules.canon:45.

**A2-18 · guess · Are imports per file or per package?**
- Where:
  - TYPES §3.1: "Two imports binding the same name, or an import binding a name that the package
    also declares, is `E2005`".
  - §3.2: "All top-level declarations of a package … share one namespace".
  - `game/items/item.canon:17` and `item.view.canon:8` both write `import studio`. It is the
    only package in `examples/` that imports one package in two files.
- Problem: if imports are package-wide, game.items gets `E2005`. Nothing says which reading holds.
- Proposed: imports are per file, as in Go. An imported name is visible only in its file. E2005
  applies within one file, and between one file's imports and the package's declarations.

**A2-19 · guess · How `emit` options are typed**
- Where:
  - GRAMMAR §5.3: "the options are a brace literal whose schema belongs to CODEGEN.md / WIRE.md".
  - CODEGEN §2.1 lists the option names.
  - TYPES §5.2 would classify `{ out: …, mode: baked }` with no expected type as `E3305`, and a
    bare `baked` is `E2102`. `values: [potions]` names lets.
- Problem: TYP (checker) and IR (reader of emits) must agree on how `mode`, `values`, `package`
  and `namespace` are resolved and on which codes apply. `E8003`/`E8009` cover only part of it.
- Proposed:
  - The options of each target form a built-in schema, checked like `project.canon` (GRAMMAR §7):
    - `out`, `package` and `namespace` are constant strings;
    - `mode` is a symbol from `baked embedded data types`, never resolved in scope;
    - `values` is a list of identifiers naming public top-level lets of the package (`E8009`
      otherwise).
  - No expression is evaluated.

### 2.3 Nits

**A2-20 · nit · `teamboard/taxonomy.canon:349-350` is not a fixed point of `canon fmt`.**
- The two lines are `check severities.active().count(.default) == 1` / `  else "…"`. Joined,
  they are 94 columns.
- FORMATTER §1 keeps a line-break bit for brace lists only, so §7.2's one-line-check group
  prints flat. That contradicts FORMATTER §15 ("every `.canon` file … is a fixed point").
- Fix: join the two lines. It is the only case in `examples/`: a script checked every two-line
  check.

**A2-21 · nit · References to the examples are stale.**
- TYPES §17 claims "the examples now follow the right-hand column". They do not:
  - taxonomy uses `first()!`, not `[0]`;
  - `columnOf` returns `ref Column?` instead of appending `!`;
  - rules uses `if anyJob != none`;
  - event uses a symbolic key.
- TYPES §17 and §6.6 cite a `while`-narrowing loop "from sweep_plan.canon" (:112–114). There is
  no `while` there any more.
- GRAMMAR §9.2 still says taxonomy has `const version = 7`; it has `VERSION`.
- EVALUATION §12.2's sample finding points at sweep_plan.canon:117:12, a doc-comment line.
- CLI §3.5's sample points at `event.canon:170`; the test is at :176.
- `louis.layer.canon:5` quotes stale line numbers.

**A2-22 · nit · CODEGEN §5.10: `const Column& ColumnOf(StatusId s);`.** taxonomy's `columnOf`
returns `ref Column?`, so per §5.8 the C++ form is `const Column*` (Go `*Column` is right).

**A2-23 · nit · More stale or wrong text.**
- FINGERPRINT.md:450 names `examples/balance/sweep_plan.canon`; the file is in `balance/parity/`.
- WIRE.md:910-911 says `potions.json` "had a placeholder fingerprint".
- API §8.8 uses `heistia.tasks[0].reward`, which does not exist.
- DECISIONS 17 says narrowing works in "`?:`" branches; Canon has no `?:`, so read "`if`
  expressions".
- LOCK §11's E6001 template (:492) does not produce LOCK's own examples (:144, :357–359,
  :468). That also matters for A2-01.

**A2-24 · nit · SPEC §5.7 and §4.3 use `E3102` / `E3101` loosely for duplicate keys.** TYPES
§9.3: tables are `E3101`, keyed lists `E3102`.

**A2-25 · nit · SPEC §10.1: a record check runs "once per value … each distinct value".**
EVALUATION §4.2 and §8.1 say once per *instance*: equal values from two literals are checked
twice. Replace "distinct value" with "instance".

**A2-26 · nit · GRAMMAR §3.1 rule 4: a field's own-line annotation directly followed by a
`check`/`fn` item is read as a prefix annotation of that item.** The lookahead skips line
breaks, so the annotation becomes `E1118` on the check. No example does this. Say "the next
significant token after the annotation run, on the same line or the next".

**A2-27 · nit · Small contradictions between companion documents.** Each has an obvious winner.
- **Layer-file detection.** EVALUATION §9.1: "a file whose second line is `layer name`".
  GRAMMAR §5.2 decides by the first tokens, after comments. Both layer files have `layer` on
  line 7. GRAMMAR wins.
- **`fail` as a name.** GRAMMAR §4.2 says `fail` is an ordinary predeclared identifier. TYPES
  §3.3 says "`fail`, `warn` and `load` are syntax, not names".
- **`#`.** GRAMMAR §1 produces a `#` token in `[#n]`, but the §2.8 punctuation list and the §2.9
  lexer modes omit it. EVALUATION §9.2 writes `"[#" n "]"` as one token.
- **Productions TYPES assumes.** TYPES' table assumes `ref qualifiedName`, `is qualifiedName`
  and `lvalue = name { "[" expr "]" }`. GRAMMAR has `qualifiedIdent`/`qualifiedWord` and parses
  an assignment target as an expression.
- **TYPES §3.2** cites "§7.4" for `Int(f)` in value position; it means STDLIB §2.1.
- **`check … at f`.** EVALUATION §8.3's location table has no row for it; only VIEWMODEL G19
  states it. The farm W5001 golden depends on it.
- **`@json(unit:)` on `Duration?`.** GRAMMAR §8.3 lists "`Duration`, or list/map of
  `Duration`". WIRE §4.1 and four examples allow `T?`.
- **E3505.** Its owner is TYPES §10.2, but only EVALUATION §3.4 says when it fires.
- **E4503** is static but numbered in the E4xxx evaluation range.

**A2-28 · nit · IMPLEMENTATION-PLAN principle 3 cites "`// WIRE.md W3`".** WIRE, TYPES, GRAMMAR
and EVALUATION number sections, not rules. Say "a rule id, or a § number where the document has
none".

**A2-29 · nit · `api/` has no `doc.go` and no example test (DECISIONS 25).**
- `canon.go` holds a file-header package comment, which D25 forbids outside `doc.go`.
- `SyntaxError` wraps no sentinel, although API §15 says "Every error type wraps one sentinel".
- The M0 split can fix this; add `ErrSyntax`.

**A2-30 · nit · Golden texts disagree on small things.**
- The C++ conformance message starts with `"pipeline: "` (`pipeline_conformance.gen.cpp:55`);
  the CONFORMANCE §7.2 format has no prefix.
- The schema-mismatch text ends "binary." in C++ and TS, "binary" in Go (`rt.go`).
- The TS conformance template writes `-9223372036854775808` (CONFORMANCE.md:392, 411), but §4
  says out-of-range inputs are written as their nearest double.
- `PipelineStore` lacks the `friend struct detail::PipelineAccess;` that CODEGEN §7.2 requires
  of "every class".

**A2-31 · nit · FINGERPRINT §3's exclusion table omits checks and warns.** Only §5 says changing
a check leaves the hash alone. Also state whether the key enum of a `ref` into an enum-keyed
keyed list is walked and numbered.

**A2-32 · nit · Where the teamboard lock golden is compared.**
- IMPLEMENTATION-PLAN §7.1 compares outputs through `expected/MANIFEST`; M1 item 5 compares
  `canon.lock` separately.
- Say whether a MANIFEST lists `canon.lock` (proposed: yes, `teamboard/canon.lock canon.lock`).

**A2-33 · nit · `make check`'s golden gate is a stub.**
- `goldens-check` prints "not wired yet".
- The auditor reports "1 fixed … run `baseline --tighten`", so the ratchet baseline is not
  tight. Tighten it before locking.
- `examples/go.mod` holds no package, so `go vet` there fails with "no packages". The Makefile
  does not run it.

**A2-34 · nit · IMPLEMENTATION-PLAN §12.4 says `api/canon.go` is "one 1,070-line file".** It is
1,075 lines.

**A2-35 · nit · AUDIT.md's verdict says "81 blockers", but it contains 82 blocker entries** (and
117 guesses, not 114). The table of §1 lists all 82.

---

## 3. What was checked, and how

- **Error catalogue.** Scripts extracted every `E####`/`W####` from all documents and examples.
  - All 298 catalogued codes are defined in exactly one diagnostics table, whose document is
    the owner ERRORS.md names: 277 errors, 18 warnings, 3 run-time codes.
  - The only referenced but uncatalogued codes are the retired `E1624`, `E1625`, `E3014`,
    `E3319` and `E8016`, plus the `E1234`/`W1234` placeholders of SPEC §18.
- **Cross-references.** All 703 `DOC.md §n` and `DOC.md Xn` references, and the SPEC/CLI §
  references, resolve. The one exception is "WIRE.md W3" (A2-28). All AUDIT ids and MOCKUP-GAPS
  numbers cited exist.
- **Fingerprints.**
  - `pipeline.Potion@f750790e` recomputed from potion.canon with `printf … | sha256sum`.
  - The texts of all 10 vectors hash to their stated sizes and hashes.
  - Checked against their sources: vectors 2, 3, 5, 7 and 8 by hand, all ten by a second
    reviewer. Vectors 4 and 10 fail (A2-02).
- **WIRE and LOCK samples.** `potions.json` (334 bytes, `2a51028f…`), the deck and flow samples,
  `teamboard/expected/canon.lock` (1108 bytes, `583148b7…`) and the vocab lock excerpt all hash
  as stated. Lock ordering, separators and the header are right.
- **View model.**
  - `potion.view.json` validates with jsonschema 4.19.2 (Draft 2020-12), and the schema rejects
    12 deliberate mutations.
  - The bytes equal `JSON.stringify(vm, null, 2)` plus LF.
  - Checked against VIEWMODEL: member order (J2), omissions (J3), duration units (X12), usage
    (L7), the entry column (T6/T7), i18n (14 keys = I18N §11), and the W1701 location and span.
- **Findings goldens.**
  - The text format matches API §4.4.
  - W1701 appears exactly for the packages with `emit view`, at the W2 location.
  - Recounted counts: pipeline 14, farm 39 of 88, studio 13, sovcommon.time 20.
  - The farm and heistia W5001 findings correspond to the fixtures (`maxLevel` 3 with 2 levels;
    duplicate "Drop moonstone").
- **Code goldens.**
  - g++ 15 and clang++ 21 with `-std=c++17` and `-std=c++20`,
    `-Wall -Wextra -Wpedantic -Werror`, and `-fno-exceptions -fsanitize=address,undefined`:
    0 diagnostics; `RunPipelineConformance() == 0`; load, schema-mismatch and missing-file
    paths behave as specified.
  - Go 1.25.7: `go vet`, `go build` and `go test -race` pass; `gofmt -l` is empty.
  - `canon_runtime.h`, `canon_runtime_json.h` and `rt/rt.go` are byte-identical to CODEGEN §7.4,
    §7.5 and §6.3.
- **Examples.**
  - Read against GRAMMAR, TYPES and STDLIB, in particular: taxonomy, sweep_plan,
    adventurequest, heistia, vocab, event, farm, rules, resourcestudio, game/items (with its
    entries, view and translation) and features/*.
  - Two reviewers parsed every file independently against the EBNF. No construct outside
    GRAMMAR.md was found.
  - Every check was re-run by hand against the fixture data.
  - The type-rule gaps found are A2-05, A2-06, A2-07, A2-08 and A2-18.
  - Example defects: A2-17 (rules findings) and A2-20 (layout).
- **`GOTOOLCHAIN=local make check`** exits 0 (see A2-33).

---

## Resolution

Closed on 2026-09-23 (night, autonomous session). Every finding is fixed in the file that owns
it; the choices that went beyond the proposed answers are DECISIONS 30–51. After the changes,
`GOTOOLCHAIN=local make check` passes, the audit baseline was tightened (4 entries), the ten
FINGERPRINT vectors hash as stated, the C++ goldens compile warning-free (g++ and clang++,
`-std=c++17` and `-std=c++20`, `-Wall -Wextra -Wpedantic -Werror`) with
`RunPipelineConformance() == 0`, and the Go goldens pass `go vet` and `go test -race`.

**Counts: 35 of 35 fixed** (4 blockers, 15 guesses, 16 nits).

| ID | What changed |
|---|---|
| A2-01 | ERRORS.md is the single source (DECISIONS 30–32): preamble rewritten; §1 message tables (one row per variant), template grammar, 15 typed argument kinds, `Kind` vocabulary with its users, related notes, and the texts generated code signals (§1.6); §2 the `diaggen` contract (what it reads and refuses, what it generates: `Def`, one typed `At…` constructor per variant, `Builder`), and the rules for callers. Every code has a `Package` column and 438 messages. The owning documents keep only code, severity and trigger (GRAMMAR §12, TYPES Diagnostics, VIEWMODEL §16, I18N §12, EVALUATION Diagnostics, STDLIB Diagnostics, LOCK §11, WIRE Diagnostics, CODEGEN §12, CONFORMANCE §8; I18N W3). IMPLEMENTATION-PLAN §3 (`diag` row), §4.4 (rewritten: generated `codes.go`, one way to report, `diag-check`), §5.3, M0 (scope and acceptance), §12.2 step 5, §12.4, addendum; API F3; SPEC §21.2 and its closing paragraph; README. Samples brought in line with the templates: CLI §2.4, SPEC §21.1, API §4.2 (`E3501`), LOCK §4.1, §9.3, §9.6 and §2.4 (`E6001`, `E6005`) |
| A2-02 | FINGERPRINT vector 4 (arms in declaration order with wire values, Element `@2`, QuestStyle `@3`: 1491 bytes, `39fd8523…`) and vector 10 (arms fixed, Element `@11`: 3719 bytes, `6dc7944d…`); both recomputed with `sha256sum`, and all ten vectors re-verified. No other file cited the old hashes |
| A2-03 | VIEWMODEL N4 and ERRORS.md `W1640`: reported only for a selected package with an `emit view`, like `W1701` (DECISIONS 33). Every goldened package with an `emit view` already gives its editable public values a menu, so no `findings.txt` changes |
| A2-04 | IMPLEMENTATION-PLAN §4.7 `check/info.go` (`Check`, `Program`, `Info` with `Types`, `TypeExprs`, `Defs`, `Uses`, `Selections`, `Conv`, `Keys`, `Calls`, `Literals`, `Matches`, `Broken`; `Object`, `Conversion`, `Callee`, `MatchInfo`) and §4.8 `eval/host.go` (`Host` with `Load` and `Verify`, `Evaluator` constructor and wiring); the `check.Folder` seam for constants (DECISIONS 34); §3: `value` above `check`, `infer` and `convert` above `cli`, the seam rule, `ir` consumes `check`; §4 "eight contracts"; §5.2 M0 row |
| A2-05 | TYPES §6.2: row `S? ≤ T?` if `S ≤ T` |
| A2-06 | TYPES §6.5: `W3401` on a `?.` whose receiver segment is not optional; a chain ending on an optional is not wrapped again (DECISIONS 35). `balance/parity/sweep_plan.canon:260–261` now `w?.item.id`, `w?.item.value` |
| A2-07 | SPEC §4.3 (key field omitted, `E3321`); TYPES §9.3 (`entry` needs a table or list literal initializer, `E3103`); ERRORS.md `E3103` meaning and its `literal` variant |
| A2-08 | CODEGEN §2.1 ("When emit rules are checked") and EVALUATION §1 (stage E validates every emit; `check` = `build` but `E8001`, `E8152`) (DECISIONS 37); `game/items/item.canon` `ItemElement` members renamed `NONE = "_NONE"`… (DECISIONS 38) |
| A2-09 | IMPLEMENTATION-PLAN §12.4: runtime helper texts are `runtime/*.txt` data files, embedded and written under their real names; no exemption (DECISIONS 49) |
| A2-10 | CODEGEN §2.5: every fixed comment per target (T1–T11) is normative, with its exact line breaks; §2.7: C++ declare-before-use order and the include rule (DECISIONS 43). Goldens: docs of `Potion`, `cooldown`, `stack` added to `pipeline.gen.h` and `go/potions.gen.go`; `<cstddef>` added to `pipeline.gen.cpp` |
| A2-11 | CLI §3.5 (exact `test` layout, comparison and hard-error lines, a second sample for `healFor`, `event.canon:176`), §3.7 (exact `explain` layout, sample from the resourcestudio layer), §3.8 (exact `refs` layout, sample from the fixtures) (DECISIONS 44) |
| A2-12 | `api/canon.go`: JSON tags on `EvalRequest`, `EvalResult`, `Text`, `ShowLine`, `Heading`, `Dropped`; API §11 V4a states the wire form (DECISIONS 45) |
| A2-13 | IMPLEMENTATION-PLAN §4.2: `types.Kind` follows TYPES §2 (`VariantKind`, `LitUnion`, `Pair`, `TypeApp`, `DepUnion`, `Define`, `None`, `Error`; `KeyedList` = `List` + `KeyedBy`; asset as a refinement); §4.3 `Value.CanonText` (DECISIONS 46) |
| A2-14 | `examples/features/legacycpp/ProjectCmn.h` added (five mapped members, one unmapped; compiles standalone); `examples/_fixtures/README.md` lists it; IMPLEMENTATION-PLAN §7.9 names it |
| A2-15 | SPEC §9.4 and CODEGEN §5.10: a `ref` into a keyed list is not finite; such a function is translated and its parameter is `E9006` (DECISIONS 36) |
| A2-16 | IMPLEMENTATION-PLAN M1 item 3: `canon build --target go --target json …` |
| A2-17 | `resource/rules/rules.canon`: the GuildTalentTree `load` has `partial: true`, with a comment (DECISIONS 47) |
| A2-18 | TYPES §3.1 (imports are per file; `E2005` within a file and against the package's declarations) and §3.3 step 5; ERRORS.md `E2005` meaning (DECISIONS 39) |
| A2-19 | CODEGEN §2.1 "Typing of the options" (built-in schema, nothing evaluated or resolved; `values` names public lets), unknown target is `E8003`; GRAMMAR §5.3; ERRORS.md `E8003`/`E8009` variants |
| A2-20 | `teamboard/taxonomy.canon`: the severities check joined on one line (94 columns) |
| A2-21 | TYPES §17 rewritten against today's examples, §6.6 example attributions; GRAMMAR §9.2 example; EVALUATION §12.2 sample at `sweep_plan.canon:265:15`; CLI §3.5 line; `service/resourcestudio/louis.layer.canon` comment |
| A2-22 | CODEGEN §5.10: `const Column* ColumnOf(StatusId s);` |
| A2-23 | FINGERPRINT vector 5 path `balance/parity/`; WIRE §8.3 "potions.json holds exactly these bytes"; API §8.8 `setCase` on `eventConfig.events[moonstone_rain].kind`; DECISIONS 17 wording (DECISIONS 51); LOCK §11 via A2-01 |
| A2-24 | SPEC §4.3 and §5.7: `E3101` for tables, `E3102` for keyed lists |
| A2-25 | SPEC §10.1: once per instance |
| A2-26 | GRAMMAR §3.1 rule 4 (the next significant token after the run, and its consequence); FORMATTER §7.2 field declaration row (DECISIONS 41) |
| A2-27 | EVALUATION §9.1 (GRAMMAR §5.2 decides layer files); `fail` is predeclared: TYPES §3.3 (DECISIONS 40); `#`: GRAMMAR §2.8, §2.9 and EVALUATION §9.2; TYPES preamble table (GRAMMAR's productions); TYPES §3.2 cites STDLIB §2.1; EVALUATION §8.3 row for `check … at f`; GRAMMAR §8.3 `@json(unit:)` on `Duration?`; TYPES §10.2 says when `E3505` fires; `E4503` is owned by `check` (ERRORS.md preamble and `Package`) |
| A2-28 | IMPLEMENTATION-PLAN §1 principle 3: rule id, else a § number |
| A2-29 | `api/doc.go` holds the package comment (the file header is gone from `canon.go`); `ErrSyntax` specified in API §15 and IMPLEMENTATION-PLAN §12.4, coded in the M0 split together with the example test (DECISIONS 48) |
| A2-30 | CONFORMANCE §7.2 documents the C++ `<package>: ` prefix; the mismatch text ends without a period in C++ and TS too (CODEGEN §7.5, §8.2, `canon_runtime_json.h`); CONFORMANCE §4 and §7.2 write `-9223372036854776000`; `PipelineStore` declares `friend struct detail::PipelineAccess;` (DECISIONS 42) |
| A2-31 | FINGERPRINT §3 excludes checks and warns; §4.4 walks and numbers the enum key of a `ref` into an enum-keyed keyed list |
| A2-32 | IMPLEMENTATION-PLAN §7.1 (a written `canon.lock` is a MANIFEST line) and M1 items 3 and 5 |
| A2-33 | Makefile: `goldens-check` verifies every MANIFEST now, new `diag-check` target, note on `examples/go.mod`; IMPLEMENTATION-PLAN §12.2; baseline tightened; `AUDIT-2.md` in `.sovaudit/root-allow.txt` (DECISIONS 50) |
| A2-34 | IMPLEMENTATION-PLAN §12.4 no longer states a line count |
| A2-35 | AUDIT.md verdict: 82 blockers, 117 guesses, 7 nits (206 findings); DECISIONS triage line (DECISIONS 51) |

Nothing is left for Louis to decide before the lock; DECISIONS 30–51 are open to his review.
