# Canon spec audit: pre-implementation review

Scope: SPEC.md (v0.1 draft), CLI.md, DECISIONS.md, README.md, and every file under `examples/`
(including `examples/pipeline/expected/*`). The question for each component was: could two
engineers build it from the text alone and end up with compatible results?

## Verdict

**The spec cannot be implemented as-is by parallel agents.** The language design is coherent and the
examples are unusually good test material, but the documents fix intent, not behaviour. The
grammar in Appendix A cannot parse several examples: `x { … }` is ambiguous after `if`/`match`/`when`,
`ref T?` binds the wrong way, an annotation on its own line is not allowed, `project.canon` has no
`package` line, and keywords (`check`, `package`) are used as enum members. The v0 acceptance
example (`taxonomy.canon`) is itself an `E2101` error under §6.2. Every byte-level artifact is left
undefined: canonical JSON, the fingerprint input, the formatter layout, the lock format, codegen
naming, conformance vectors and the view-model schema. The hand-written goldens in
`examples/pipeline/expected/` contradict the spec in at least a dozen places, including the int
width, file names, the Go store, the view-model `$schema`, the Go header and the conformance vectors,
so they cannot be used as tests yet. The type system leaves open three points that every example
relies on: optional narrowing or unwrapping, the entry/`ref`/record relationship, and sized-integer
arithmetic. There is also no error catalogue, no module/interface plan, and no fixture data for the
`@resource/...` files that the examples load. I count **82 blockers, 117 guesses and 7 nits**
(206 findings; the first count said 81, 114 and 202, corrected by AUDIT-2.md A2-35).
About half of the blockers can be closed in a day by accepting the proposed answers below. The rest
need the companion documents listed at the end: grammar v2, type rules, wire format, codegen
naming/API, formatter, view-model schema, error catalogue and module plan.

Severity key: **blocker**: two engineers would build incompatible things, or work cannot start.
**guess**: someone must invent a reasonable rule. **nit**: cosmetic or low risk.

Every finding gives a *proposed answer*: a concrete default the author can accept or change.

---

## 0. Cross-document contradictions and the goldens (GEN)

**GEN-01 · blocker · The goldens are not reproducible and disagree with the spec**
- Where: `examples/pipeline/expected/*`; SPEC §14.3 (`game.items.Item@9f3c2a71`).
- Problem: `9f3c2a71` appears as the fingerprint of `game.items.Item` (§14.3), of `pipeline.Potion`
  (potions.json, potion.gen.h, potions.go), *and* as the view-model `$schema`. It is a placeholder.
  potions.json uses hand-aligned padding (`"nHeal": 500,  "dwCooldownMs"`). potions.go rewrites the
  source prose ("Hit points restored." becomes "Heal is the number of hit points restored."; "The
  item define…" becomes "ID is the item define…"). The individual contradictions are listed below
  (GEN-02, GO-01, GO-02, CNF-01, CPP-06, VM-01, GEN-05).
- Proposed: mark `expected/` as **illustrative** until the v0 compiler regenerates it. Tests compare
  goldens *semantically* (parsed JSON, and Go/C++ ASTs modulo comments) until the byte formats
  in WIR-01, EMT-01 and CG-06 are frozen. After that, goldens are regenerated and compared byte for byte.

**GEN-02 · blocker · Refined ints become narrow C++/Go types in the goldens**
- Where: §5.2: "Generated code uses the base type: `Int(0..=100)` is emitted as `int64_t`" vs
  potion.gen.h:38 `int32_t GetHeal()`, potions.go:22 `heal int32`, potion.gen.cpp:40
  `get<int32_t>()`, and the conformance struct `int32_t heal`.
- Problem: one side must win. Narrowing on refinement bounds would change the C++/Go type when a
  bound changes, while the fingerprint (§14.4) does not cover refinements. An old binary would then
  read a wider value into a narrower type without complaint.
- Proposed: keep §5.2. `Int(…)` is always `int64`/`int64_t`. Narrow types come only from explicit
  `Int32`/`UInt16`/…. Fix the goldens.

**GEN-03 · blocker · Examples violate the package/directory rule**
- Where: §3.2: "A file may declare the package of its own directory or of an ancestor directory".
  `examples/balance/sweep_plan.canon` declares `package balance.parity` in `balance/`.
  `examples/service/resourcestudio.canon` declares `service.resourcestudio` in `service/`. Both
  declare a *descendant* package.
- Proposed: move the files to `balance/parity/` and `service/resourcestudio/`, layer files
  included. Keep the rule.

**GEN-04 · blocker · Emit paths leave their root**
- Where: §3.1: "No path may leave the project or a declared root (`E7001`)". Nine `emit`s write
  to `@services/..//…` (taxonomy.canon:292-293, ui.canon:23-24, roles.canon:18-19, event.canon:136,
  farm.canon:90, sweep_plan.canon:204, resourcestudio.canon:45), which by definition leaves `@services`.
- Proposed: add roots (`sovcommon`, `web`, `generated`, `parity`) to `examples/project.canon` and
  rewrite the paths. Also specify path normalisation: `//` is an error (`E7001`), and `..` is
  resolved lexically before the root check.

**GEN-05 · blocker · The `GENERATED` header check (E8001) cannot work for Go, JSON or `convert`**
- Where: CLI §3.4: "`canon build` refuses to overwrite a file without that header (`E8001`)"; SPEC
  §15.1 header text; potions.go:1 uses Go's `// Code generated by canon … DO NOT EDIT.`; JSON has no
  comments (potions.json has no header); CLI §3.10 step 4 makes `convert` emit JSON over the
  existing *hand-written* runtime file.
- Proposed: define the marker per format.
  - Go: `// Code generated by canon from <file>. DO NOT EDIT.` (matches Go's `^// Code generated .* DO NOT EDIT\.$`).
  - C++/TS: `// GENERATED by canon from <file>. DO NOT EDIT.`
  - JSON: a top-level `"$schema"` whose value matches `^[a-z0-9_.]+\.[A-Za-z0-9_]+@[0-9a-f]{8}$`.

  `canon convert` takes ownership of the target file explicitly: it prints "adopting <path>" and
  writes, and it is the only command allowed to do so.

**GEN-06 · nit · Wrong cross-references in examples**
- potion.canon:24 cites "SPEC §10.3" (should be §9.4). farm.view.canon:2 cites "SPEC §11.8" (should
  be §17). sweep_plan.canon:206 says "Canon only generates Go and C++" (TS is also a target). The
  sweep_plan comment says Lua reads `plan.combos`, but §14.3 wraps a record in `{"$schema","value"}`,
  so it would be `plan.value.combos`.
- Proposed: fix the comments. Decide whether `emit json` of a single record may be written
  unwrapped (`bare: true`) for non-Canon consumers.

**GEN-07 · blocker · The warnings the spec requires make the goldens and the CI recipe fail**
- Where: §2.2 (`W1001`, `W1002`); CLI §6.2 `--max-warnings 0`; potion.view.json `"findings": []`.
- Problem: potion.canon has undocumented public fields (`cooldown`, `stack`) and an undocumented
  record `Potion`, plus a `///` block before `package` that attaches to nothing. That gives `W1002`
  ×3 and `W1001`. Every example has similar gaps, and `languages: [en, fr]` adds `W1701` for every
  label. The golden says zero findings, and the CI recipe fails on every example.
- Proposed: `///` before `package` is the package doc (GRM-08). Keep `W1002`, but fix the goldens'
  `findings`. `W1701` is summarised per package (I18N-03).

---

## 1. Lexer (LEX)

**LEX-01 · blocker · Regex literal vs division**
- Where: §2.5 regex `/^II_[A-Z0-9_]+$/`; §2.6 `/` operator.
- Problem: `/` both divides and opens a regex. A context-free lexer cannot tell `a / b / c` from a
  regex.
- Proposed: `/` starts a regex only when the previous significant token is `(` or `,`, and the
  parser accepts a regex only as the sole argument of a type refinement or of `.matches(`.
  Everywhere else `/` is division.

**LEX-02 · blocker · String interpolation: nesting and the format colon**
- Where: §2.5: "interpolation `{expr}` … A format spec may follow a colon: `{heal:,}`". The examples
  nest string literals inside interpolation (`{owners.map(.id).join(", ")}` taxonomy.canon:218,
  `{missing.join(", ")}` rules.canon:72). Named arguments contain colons (`{f(x, n: 1)}`).
- Proposed: the lexer keeps a mode stack. Inside `{…}` it lexes full expressions, including nested
  strings and brackets, until the matching `}`. A format spec is a `:` at bracket depth 0 followed
  by `[+][,][.N]` and then `}`. Any other depth-0 `:` is a syntax error (`E1101`).

**LEX-03 · guess · Multiline strings**
- Where: §2.5: "common leading indentation is stripped; first and last newline removed".
- Proposed: Swift rules. The indentation of the line holding the closing `"""` is removed from every
  line. Mixed tabs and spaces in that prefix is `E1102`. Escapes and interpolation work as in
  normal strings.

**LEX-04 · guess · Duration literals and their canonical text**
- Where: §2.5: `250ms 90s 5m 1h 2d 1h30m`, "largest first when combined".
- Proposed: the token is `digits unit { digits unit }` with no spaces. Units are `d h m s ms`, each
  used at most once, in strictly decreasing order. `m` vs `ms` is longest match. Any integer per
  unit is allowed (`90m`). Canonical text, used by interpolation, `String(x)`, fmt and the view
  model: decompose into d/h/m/s/ms, largest first, omit zero parts, `0s` for zero, `-` prefix for
  negative values. `fmt` rewrites literals to canonical form.

**LEX-05 · guess · Integer literal range**
- Proposed: integer tokens are arbitrary precision until checked against their expected type.
  `-9223372036854775808` is folded as a negative literal. A literal that does not fit is `E3201`
  at compile time.

**LEX-06 · nit · `_` is both an identifier and a token**
- Where: §2.4 `letter = … | "_"`, so `_` alone is an identifier. It is also the match wildcard and
  the any-type (§5.9).
- Proposed: a lone `_` is a punctuation token. Identifiers are `letter { letter | digit | "_" }`
  and not equal to `_`.

**LEX-07 · guess · Doc comment text normalisation**
- Needed because doc text feeds generated comments, the view-model `help`, translation source text
  and `i18n stub`.
- Proposed: strip `///` plus one following space. Join consecutive lines with `\n`. Trim trailing
  blank lines. `////` is an ordinary comment.

**LEX-08 · blocker · Keywords used as names in the examples**
- Where: ui.canon:19-20 `enum Icon { … check, … package, … }`; taxonomy.canon:158 `icon: check`,
  :187 `icon: package`; farm.fr.canon:67-68 key `ModelType.check.unreachable_levels`. Appendix B
  reserves `check` and `package`.
- Proposed: any reserved word may be used (a) as an enum member, table key, variant case or
  translation-key segment where the grammar expects a name, (b) after `.`, and (c) in value
  position, but only if the word cannot start an expression there. That covers the
  declaration-only keywords `check package import emit enum record variant view widget test layer
  translation project entry export local stable table type`. Keep the list in Appendix B.

---

## 2. Grammar and parser (GRM)

**GRM-01 · blocker · `x { … }` typed literal vs a block**
- Where: App. A `primary = … | qualifiedName recordLiteralBody`. It breaks: sweep_plan.canon:102
  `if settings.includePuppeteer { ["JOB_PUPPETEER"] }`, vocab.canon:78 `match e.param {`, §16.6
  `group combat "Combat" when kind is IK1_WEAPON { attackMin, … }`, and any `for s in statuses {`
  or `while running {`.
- Proposed: Go's rule. In the header expression of `if`, `else if`, `while`, `for … in`, `match`
  (expression and type level) and view `when`, a `qualifiedName` immediately followed by `{` is
  not a typed literal. Parenthesise to get one.

**GRM-02 · blocker · Newlines inside nested brackets**
- Where: §2.3: newlines are ignored "inside `(` `)` or `[` `]`". sweep_plan.canon:178-195 puts a
  record literal with *newline-separated* fields inside `[ … ]`. Ignoring those newlines makes
  `job: job jobToken: …` unparseable.
- Proposed: the innermost open bracket decides. Inside `(`/`[` newlines are insignificant. Inside
  `{` they are separators, even when that `{` is nested in `[`/`(`.

**GRM-03 · blocker · Annotation on its own line**
- Where: farm.canon:43-44: `maxModels: Int? = none` then a new line `@deprecated("…")`. §2.3 does
  not list `@` as a continuation token, so the annotation would start a new item and fail to parse.
- Proposed: add "a line whose first token is `@`" to the continuation list, except when it is
  followed by a declaration keyword (`let`, `record`, …), which is the `@reload let` case.

**GRM-04 · blocker · `ref T?` precedence**
- Where: App. A `"ref" ( type | qualifiedName )`. `type` goes through `optType`, which consumes the
  `?`, so `ref TalentNode?` parses as `ref (TalentNode?)`. The examples mean `(ref T)?`
  (rules.canon:21, farm.canon:67).
- Proposed: `"ref" qualifiedName`. `?`, refinements and `where` then apply to the ref type. `ref`
  of anything but a name is not allowed.

**GRM-05 · blocker · Type application vs refinement**
- Where: App. A `baseType = qualifiedName [ "(" expr {"," expr} ")" ]` and `refinement = "(" (rangeExpr|regex) ")"`.
  `Int(0..)`, `Duration(1s..=1d)` and `String(/re/)` match the first rule, so `refinement` is dead
  code for named types.
- Proposed: parse `qualifiedName [ "(" args ")" ]` once and decide after resolution. If the
  name is a built-in scalar or an alias of one, the args must be exactly one range or regex, and it
  is a refinement. If the name is a parameterised record or alias, the args are values. Chained
  refinements (`Int(0..)(..=5)`) are `E1103`. Use `where` for more.

**GRM-06 · guess · Scope of `where`**
- Where: `optType = baseType [refinement] [keyed by] ["?"] ["where" expr]`.
- Proposed: `where` applies to the non-`none` value only (`Int? where it > 0` means "none, or a
  positive Int"). In a union, `where` binds to the last alternative. The predicate expression ends
  at the first token that cannot continue an expression (`=`, `@`, `from`, `,`, `}`, NL).

**GRM-07 · blocker · `project.canon` is not parseable and has no schema**
- Where: App. A `file = "package" …` (project.canon has no `package` line); `projectDecl =
  "project" identifier braceLiteral`, where `roots { … }` would parse as a *table entry*;
  `languages: [en, fr]` and `studio: studio` are bare names with no expected type.
- Proposed: add `projectFile = { docComment } "project" identifier "{" … "}"` with a built-in
  schema:
  ```
  canon: String                     (required; "MAJOR.MINOR")
  roots: {identifier: String} = {}  (`roots { k: v }` is sugar for `roots: { … }`)
  languages: [identifier](../../1..) = [en]   (BCP-47 subset: [a-z]{2,3}(_[A-Z][a-z]{3})?(_[A-Z]{2})?)
  studio: qualifiedName? = none
  budget: Int(1..) = 100_000_000
  ```
  Unknown keys are `E1002`.

**GRM-08 · guess · Doc comment before `package`**
- Where: every example starts with `///` lines before `package`. The grammar does not allow it, and
  §2.2 would give `W1001`.
- Proposed: allowed. It is the package's doc. When several files of a package have one, they are
  concatenated in file path order. It is emitted as the package doc (Go) or file header comment
  (C++/TS).

**GRM-09 · guess · Separators in brace-delimited lists**
- Where: `recordBody = "{" { recordItem SEP } "}"`, `block = "{" { statement SEP } "}"`, variant,
  view, group, amend and match arms. These require a SEP after the last item and allow no run of
  blank lines, yet examples write `record SetRow { id: Int }` and `{ return x }`.
- Proposed: every brace list is `"{" {SEP} [ item { SEP {SEP} item } {SEP} ] "}"`.

**GRM-10 · blocker · Brace literal classification (record / map / table / comprehension)**
- Where: §6.1; App. A `braceItem` and the sentence "follows from its items and its expected type".
  rules.canon:67 `{ name: job.compared()… for name, job in jobs }` uses `name` as a *variable*
  key. sweep_plan.canon:177 `{ pve: …, pvp: … }` uses enum-member keys. `{}` can be a record with
  all defaults, an empty map or an empty table.
- Proposed: the parser builds a generic `BraceLit` node and the type checker classifies it.
  - Items `ident { … }` → table.
  - `expr : expr` followed by a comprehension clause → map comprehension. The key is always an
    expression.
  - Expected type is a record/case → `ident:` is a field.
  - Expected map with enum/ref key → `ident:` resolved per §6.2 step 1, then scope.
  - Expected map with String/Int key → `ident:` is `E3304` (write `"ident":` or `(ident):`).
  - No expected type → only `"str": v` items (map `{String:_}`) or a comprehension; `{}` is `E3305`.

**GRM-11 · guess · `check {` block vs an expression starting with `{`**
- Proposed: `check {` always starts a block. `check name:` is recognised by `identifier ":"`
  lookahead.

**GRM-12 · guess · Lambda vs parenthesised expression; match patterns that look like calls**
- Proposed: after `(` look ahead to the matching `)`. If `=>` follows, parse a lambda. In a match
  arm, parse `pattern {, pattern} =>` and never an expression.

**GRM-13 · guess · Slices**
- Where: `slice = expr | [expr] (..|..=) [expr]` overlaps `range`. A range-typed variable `xs[r]` is
  not covered.
- Proposed: drop `slice`. Indexing with a `Range` value is a slice. `xs[..b]` uses the prefix range
  form.

**GRM-14 · guess · `fnDecl` parameter list**
- Where: `"(" [ "self" ] [ [ "," ] param { "," param } ] ")"` accepts `(self x: Int)` and `(, x)`.
- Proposed: `"(" [ ("self" | param) { "," param } [","] ] ")"`.

**GRM-15 · blocker · No function type syntax**
- Where: §9.1: "Functions are values and may be passed to other functions". No type syntax exists
  for a function parameter.
- Proposed: add `fn(T, U) -> R` as a `baseType`. A lambda needs an expected function type (from a
  user fn or stdlib). Function values cannot be stored in records or lets, and are not emittable
  (`E3306`).

**GRM-16 · guess · Right-hand side of `is`**
- Where: `compare = range [ … "is" range ]`, while `v is item` and `kind is IK1_WEAPON` need a
  *case name*, not an expression.
- Proposed: `compare = range "is" qualifiedName`, resolved against the variant of the left side.

**GRM-17 · guess · View vocabulary vs field names**
- Where: App. A `viewItem`. A field named `title`, `search`, `group`, `show`, `preview`, `menu`,
  `columns`, `filters`, `singular` or `subtitle` cannot be labelled, because `title "Title"` is
  read as the view title.
- Proposed: vocabulary words win. Add `field <name> ["Label"] [{props}]` as an explicit escape.

**GRM-18 · guess · Annotation syntax and catalogue**
- Where: §5.12. App. A says `annArg = [identifier ":"] expr`, but `ms`, `s`, `snake`, `fields`,
  `inline` and `UInt8` are symbols, not expressions (`s` is also a common variable name). Positions
  differ: `@reload let …` before the declaration vs `record X @json(…)` after the name.
- Proposed: write an annotation catalogue. For each annotation give the allowed positions,
  positional/named params and param kinds (string, integer, *symbol from a closed set*, type name,
  or a literal of the annotated field's type for `none:`). Symbols are never resolved in scope.
  Unknown annotations or args are `E1104`.

**GRM-19 · guess · Brace after `=>` in `match`**
- Proposed: in an expression `match`, `{` after `=>` is a brace literal. In a statement `match`,
  it is a block.

**GRM-20 · guess · Assignment targets**
- Where: `lvalue = identifier { "." identifier | "[" expr "]" }` vs §8 "assignment to a `var`, or
  to an element of a list or map".
- Proposed: `lvalue = varName { "[" expr "]" }`. Field assignment (`x.f = …`) is `E3307` at
  compile time.

**GRM-21 · nit · Statement `if` vs `if` expression**
- Proposed: at statement start, `if` is always a statement.

---

## 3. Name resolution (RES)

**RES-01 · blocker · Resolution needs types and loaded keys, but runs before both**
- Where: §11.2 lists phase 2 "Resolve names (§6.2)" before phase 3 "Type-check", yet §6.2 step 1
  resolves against the *expected type*. For `ref items` targets, the keys come from
  `load.defines`, which exists only in phase 5.
- Proposed: merge phases 2 and 3 into a bidirectional type checker. When a bare identifier's
  expected type is `ref C` and it matches no declared name, it is recorded as a *symbolic key* and
  validated in phase 6 (`E3501`). The `E2101` ambiguity test uses declared names only, never
  loaded keys.

**RES-02 · blocker · The v0 acceptance example triggers E2101**
- Where: taxonomy.canon:193 `team_board { … icon: columns … }`. `columns` is both an `Icon` member
  (step 1) and the package value `let columns` (step 2), so §6.2 makes it an ambiguity error.
  README makes taxonomy.canon the v0 "done" criterion.
- Proposed: when the expected type is an enum, variant or ref, a step-1 match wins silently. It
  is `E2101` only if a *local or parameter* (not a package or import name) of the same name has a
  type assignable to the expected type.

**RES-03 · blocker · What "in scope" means for `ref T`**
- Where: §5.8: "If exactly one collection of `T` is in scope, `ref T` targets it." rules.canon:21
  `parent: ref TalentNode?` targets the *sibling field* `nodes: [TalentNode] keyed by id` of the
  enclosing `GuildTalentTree`, which is per instance. By the text no collection is in scope, so the
  code is an error.
- Proposed: search in order and stop at the first level that has any candidate. Exactly one
  candidate at that level is required, else `E2103` (ask for `ref name`).
  1. Collection-typed fields of enclosing record types. The ref is resolved *per enclosing
     instance*.
  2. Top-level lets of the package, `local` included.
  3. Public lets of imported packages.
  4. For case 1, generated code holds the key (CG-03).

**RES-04 · guess · Where built-ins sit in the lookup order**
- Where: `LevelRange { min, max }` shadows `min()`/`max()`. Fields named `default`, `count`,
  `find` also exist.
- Proposed: locals → fields/`self` members (in record bodies) → package → imports → built-in types
  and free functions. Methods are looked up only after `.`.

**RES-05 · blocker · `t.k`: table entry or method?**
- Where: §7.5 says `x.f` reads "a field, a method, or a table entry by key". studio.canon:31
  `units` has an entry `count`, and `count(pred)` is also a method. Keys `filter`, `len`,
  `active`, `find` are all legal.
- Proposed: `t.k(…)` is always a method. `t.k` without parentheses is always the entry, or `E4002`
  if missing. A record with a field and a method of the same name is `E2104`.

**RES-06 · guess · Is `import studio` needed in view files?**
- Where: all view files `import studio`. `unit: hp`, `icon chest`, `menu events` and
  `widget: weekly_timeline` resolve against `project.studio`.
- Proposed: view props `unit`, `icon`, `tone`, `menu` and `widget` resolve against the project's
  studio package implicitly. The import is unnecessary but harmless.

**RES-07 · guess · Public types that depend on local values**
- Where: vocab.canon:78 `Param` (public) uses `ref monsters`, `ref worlds`, `ref upgradeTypes`
  (all `local`), and heistia and adventurequest import `Param`.
- Proposed: allowed. Other packages validate against the local collection. Generated code
  represents such refs by key (plus `.value` for defines; CG-03). No id enum is emitted.

**RES-08 · guess · Name clashes with `.id`, `.retired`, `.kind`**
- Where: table entries have `.id`/`.retired` (§5.7) and variants have `.kind` (§5.5). Potion has a
  field `id`, and Event and Rewards have fields `kind`.
- Proposed: a record used as a `table` element may not declare `id` or `retired` (`E2105`). A
  keyed-list element may. A variant case may not declare a field `kind`. A *record field* `kind`
  holding a variant is fine: `e.kind.kind`.

**RES-09 · guess · Refs into Int-keyed lists**
- Where: farm.canon:85 `[ModelType] keyed by typeId` (Int). How is a key written where `ref` is
  expected?
- Proposed: any literal of the key field's type is accepted as a key.

---

## 4. Type system (TYP)

**TYP-01 · blocker · Using optionals where a plain value is expected**
- Where: §5.6 only says reading a field of `none` is `E4001` at evaluation. The examples need more:
  - `let defaultSeverity: ref Severity = ….first()` (taxonomy.canon:206: `T?` into `ref T`);
  - `columnOf` returns `first(pred)` as `ref Column` (:270);
  - `itemId: items.first()` (event.canon:124);
  - `out += [at.jobBase]` after `at.jobBase != none` (sweep_plan.canon:112-113);
  - `LINK_FAMILIES.get(s.weapon)` guarded by `s.weapon == none or …` (:136);
  - `tier != none and not tier.totems` (rules.canon:138).
- Proposed: to keep every example valid, a `T?` is accepted wherever a `T` is expected, and `.`,
  `[ ]` and calls work on `T?`. The unwrap is checked at evaluation (`E4001`, located at that
  expression). No flow typing. Also add `W3401` (off by default) for implicit unwraps. If the author
  prefers static safety instead, add flow narrowing on `!= none` for locals and immutable paths
  inside `and`/`if`/`while`, plus a postfix `!`, and fix the four examples.

**TYP-02 · blocker · Entry vs `ref T` vs `T`**
- Where: §5.8, §6.3. The examples mix them:
  - taxonomy.canon:65 `next.contains(self)` (`[ref Status]` vs `Status`);
  - :234 `a.group == g` (ref vs entry);
  - :275 `[a for a in areas.active()]` returned as `[ref Area]`;
  - heistia.canon:17 `Param(eventType)` passes a `ref` to `e: EventType`.
- Proposed:
  - Static types are `T` and `ref T`.
  - An entry of a table or keyed list has type `T` plus a runtime *identity* (collection, key).
  - `T` → `ref T` is implicit. It is valid at evaluation only if the value is an entry of the
    ref's target collection (`E3503` otherwise).
  - `ref T` → `T` is implicit dereference.
  - `==` between ref and entry compares identity. Between plain records it is structural.
  - `self` inside a record body is an entry when the value is an entry.

**TYP-03 · blocker · Sized integer arithmetic**
- Where: §5.1 (`Int8…UInt64`) and §7.2 ("`Int` arithmetic is 64-bit"). Nothing says what
  `code + 1` is for `code: UInt16`, or how the evaluator holds UInt64 values above `INT64_MAX`.
- Proposed: in expressions every integer type is `Int` (64-bit signed, `E4101` on overflow). A
  sized type is `Int` plus an implicit range refinement, checked where values are stored (`E3201`).
  In v0, `UInt64` is limited to `0..=INT64_MAX`; above that is `E3201`.

**TYP-04 · blocker · When refinements are checked**
- Where: §5.2: "checked where the value is built and reported at the value's source location".
- Proposed: refinements are ignored for static typing (`Int(0..)` ≡ `Int`). A literal constant that
  is statically out of bounds is already `E3201` at compile time. At evaluation, refinements are
  checked at every conversion into a declared type: annotated `let`, record field, element of a
  typed list or map, fn argument, fn return, amend value, loaded value.

**TYP-05 · guess · Joining branch types**
- Where: `if`/`else` (sweep_plan.canon:171 `[Int]` vs `[Int(1..)]`; :102 `["…"]` vs `[]`), list
  literals, `??`, match arms.
- Proposed: the join drops refinements and keeps the base type. `none` ⊔ `T` = `T?`. An empty list
  or map takes the other branch's type. Nothing but empties and no expected type is `E3308`.

**TYP-06 · blocker · `load` without an expected type**
- Where: §13.2: "The **expected type** drives parsing". vocab.canon:14 `local let itemRows =
  load(…)` has no annotation. sweep_plan.canon:46-47 `load(…).filter(…)` makes load a method
  receiver, which has no expected type.
- Proposed: `load` needs an expected type directly from its context (annotation, field, argument,
  return), else `E7002`. Fix vocab.canon (`local let itemRows: [Item] = …`) and split sweep_plan
  (`local let allWeapons: [Weapon] = load(…)` then `.filter`).

**TYP-07 · blocker · Stdlib generics and lambda inference**
- Where: §20 gives names only. `reachable(next: f)` accepts `f` returning `T?` *or* `[T]`.
  `min(a,b,…)` is variadic. `first()`/`first(pred)` overload by arity.
- Proposed: publish full generic signatures (STDLIB.md, see §7 below). A lambda's parameter types
  come from the callee's parameter type, and its body is inferred bottom-up. The only overloads are
  arity and `next: fn(T) -> T? | [T]`.

**TYP-08 · guess · Equality edge cases**
- Proposed:
  - `==` needs the same base type up to optionality, and `none == none` is true.
  - Map equality ignores order (same key set, equal values).
  - Keyed list vs list: equal if the elements are equal.
  - Refs into different collections is a static type error (`E3309`).
  - `<` on Bool, refs or lists is `E3310`.
  - Records containing Floats compare with IEEE `==` field-wise.

**TYP-09 · guess · String-literal unions (`A | "lit"`)**
- Where: §5.9. §5.7 limits map keys to "`String`, an integer type, an enum or a `ref`", yet
  adventurequest.canon:86 uses `{SpecificKey(e): …}` where `SpecificKey = Param(e) | "default"`.
- Proposed: the base of a literal union must have a string wire form (String, enum, ref). Such
  unions are valid map keys. On decode the literal wins over a same-spelled key. Codegen uses the
  String/key type.

**TYP-10 · guess · Integer literal where a Float is expected**
- Proposed: only literal tokens, with an optional leading `-`. `const N = 7` used as a Float is
  `E3311`, and needs `Float(N)`.

**TYP-11 · guess · Float32**
- Proposed: values are rounded to nearest on storage. The only check is finiteness (`E3202`).
  JSON output is the shortest decimal that round-trips *as float32*.

**TYP-12 · guess · `Range` values**
- Where: §5.9 `.start`, `.end` ("end exclusive"). `0..=10` and open `0..` are also values.
- Proposed: a Range stores `start`, `end` (exclusive), `hasEnd`. `a..=b` stores `end = b+1`
  (Int/Duration). `.end` on an open range is `E4002`. Float ranges exist only in refinements.

**TYP-13 · guess · `Never` outside optionals**
- Proposed: a non-optional field whose type evaluates to `Never` makes the value unbuildable
  (`E3801` at the value). In codegen, the `Never` branch of a dependent type has no case.

**TYP-14 · guess · Constants of composite type**
- Where: §4.1 "computable without `load`". sweep_plan.canon:78-92 has list and map constants.
- Proposed: a constant may use literals, other constants, operators and stdlib, but no fn calls and
  no `load`. Emission: Go `var` (a slice or map behind an accessor func, since there are no const
  slices), C++ `inline const`, TS frozen `const`.

**TYP-15 · guess · What defaults may reference**
- Where: §5.4 "a constant expression or an expression over earlier fields".
- Proposed: constants, earlier fields and stdlib only; no package values, no user fns. The view
  model shows a default only when it is constant (else `"default": {"computed": true}`).

**TYP-16 · guess · Spread**
- Proposed: at most one spread, and it must come first. Its type must equal the expected type
  (`E3303`). Case spreads are allowed within the same case. Refinements and checks are re-run on
  the result.

**TYP-17 · guess · `input` fields in literals**
- Where: resourcestudio.canon:33,39 `gen: Gen = {}` where `Gen.apiKey` is an input.
- Proposed: input fields never appear in literals or loaded data (`E3312`). They are absent from
  Canon values. Reading one is a static `E3313`.

**TYP-18 · guess · Identity of parameterised record types**
- Proposed: the static type is the erased `HourlyTarget(_)`. Parameter-dependent field types are
  checked at evaluation (DEP-02).

**TYP-19 · guess · Variant literal forms**
- Proposed: a case with only defaulted fields may be written bare (`item` ≡ `item {}`).
  `Reward.item {…}` is always accepted. A case with fields cannot be written as a bare name when
  it has required fields (`E3302`).

**TYP-20 · guess · `match` scrutinee and exhaustiveness**
- Proposed: allowed scrutinees are enum, variant, Bool and their optionals (with a `none`
  pattern). Retired members and cases must also be covered. `_` is required for anything else, and
  literal patterns are not allowed in v0.

**TYP-21 · guess · Asset type details**
- Where: §16.5 `asset(root, ext: [dds, png])`.
- Proposed: `ext` items are symbols. Wire values may contain `/` subdirectories and no `..`.
  Existence is an exact, case-sensitive byte match. A case-insensitive-only match is `W3702`,
  because legacy Windows data is likely to have case drift. Checks are cached per directory listing.

**TYP-22 · guess · `@deprecated` semantics**
- Where: §5.12 "kept but has no effect".
- Proposed: still loaded, type-checked, emitted and fingerprinted. `W1003` when a literal sets it.
  Read-only in the studio.

---

## 5. Dependent types (DEP) §5.11

**DEP-01 · blocker · Static type of a dependent field in expressions**
- Where: the type of `t.filterParam` or of `ht.ratesBySpecific` keys inside checks and functions
  is undefined.
- Proposed: a synthesised union type `Param(*)`. It supports only `==`, `!= none`, interpolation,
  `String(x)`, and `match` on the discriminant. No narrowing in v0.

**DEP-02 · blocker · When dependent types are computed and checked**
- Proposed: the type-level `match` is checked for exhaustiveness statically. The type function is
  evaluated in phase 6 for each value, and counts toward the budget. A mismatch is `E3802` "value
  does not match `Param(COMBAT_KILL_FFA)` = `Never`". `none` is always valid for an optional
  dependent field.

**DEP-03 · blocker · Codegen for dependent types**
- Where: §5.11 "emitted as the union of all its branches, as a generated variant in each target".
  Missing: the name, the kind enum, the loader's branch selection (the wire is *untagged*: `""`, a
  ref key or an enum name), and the representation in C++17, Go and TS.
- Proposed:
  - Name: `<Alias>`, e.g. `Param`, with a kind enum `<Alias>Kind` whose members are the first
    pattern of each arm (`monster`, `item`, `dungeon`, `element`, `quest_style`, `upgrade_type`,
    `game_mode`). `Never` arms have no member.
  - The discriminant (`e.param`) is finite, so the compiler emits a static table from discriminant
    to kind. The loader reads the discriminating sibling (or map key) first.
  - C++17: a class with `GetKind()`, `AsMonster() -> const std::string*`, and so on.
  - Go: struct with `Kind()` and `AsX() (T, bool)`.
  - TS: discriminated union `{kind, value}`.
  - The wire is unchanged.

**DEP-04 · blocker · Dependent maps and dependent key types**
- Where: adventurequest.canon:137 `{e in eventTypes: HourlyTarget(e)}`; :86
  `{SpecificKey(e): {Stage: Int(0..)}}`.
- Proposed: codegen is a map from the ref key to the erased `HourlyTarget`. Dependent key types
  become `String`-keyed maps in code. The view model encodes the type function (VM-02).

**DEP-05 · guess · Restrictions on type functions**
- Proposed: a type-level `match` scrutinee must be an enum- or Bool-typed path rooted at a type
  parameter (or an earlier field). Parameters are records or refs. Result types have no refinement
  that depends on the parameter. These restrictions keep DEP-03's tables finite.

---

## 6. Evaluation (EVL)

**EVL-01 · blocker · Which values get evaluated**
- Where: §11.2 phase 5 "values that are emitted, checked or tested". farm.canon's `let farm` is only
  used by `emit view`, and rules.canon has no values. README says "Canon validates everything".
- Proposed: `check`/`build` evaluate *every* top-level `let` (public and local) of every selected
  package, plus whatever they need from imports, lazily. `test` evaluates on demand.

**EVL-02 · blocker · "A check runs once per value of that type"**
- Where: §10.1. Is that every construction (temporaries in fns such as `at(Wed,12,12)`,
  comprehension records, intermediate values) or reachable values only?
- Proposed: record, variant and case checks run on each distinct value reachable from (a) an
  evaluated top-level let after it is fully built, and (b) each `expect` subject. Values are
  deduplicated by identity and provenance. Temporaries inside functions are not checked, but their
  refinements are (TYP-04).

**EVL-03 · blocker · What a step is (§11.6)**
- Why it matters: whether `E4401` fires must be identical across implementations and across CLI and
  studio.
- Proposed: 1 step per evaluated AST expression node, per loop or comprehension iteration and per
  call. Stdlib collection fns cost 1 step per element visited. `sortBy` costs 1 per comparison,
  with merge sort mandated. `load` parsing costs 0. The budget is per `canon` invocation (or per
  API re-check) and covers values, checks and tests. "Heaviest value" means the top-level value with
  the most steps attributed (steps are charged to the value being forced).

**EVL-04 · blocker · Aliasing inside a call**
- Where: §8. After `var xs = [1]; let ys = xs; xs[0] = 2`, is `ys` changed? Is `xs += [x]` an
  append or a rebind? Can nested elements (`m[k][0] = v`) be written?
- Proposed: value semantics (copy-on-write). `xs += e` rebinds (`xs = xs + e`). Element assignment
  is allowed only through a `var` root, at any depth, and the copy is made by the evaluator.
  `E4201` stays as an internal safety net.

**EVL-05 · guess · How evaluation errors propagate**
- Proposed: a failed value is *poisoned*. Its dependents are not evaluated and report nothing more.
  A check that reads a poisoned value is skipped silently.

**EVL-06 · guess · Self-reference within a value**
- Where: a table entry field that reads another entry of the same table.
- Proposed: `E4301` at value granularity (refs excluded, since they are keys).

**EVL-07 · guess · Provenance details**
- Proposed:
  - A default-filled field points at the default expression in the record declaration, with a
    related location at the literal or JSON object.
  - Spread-copied fields keep their original provenance.
  - Computed values record the building expression plus a stack of at most 16 frames.
  - JSON provenance is the file, 1-based line/col (byte columns) of the value's first byte, and an
    RFC 6901 pointer.
  - Value paths use the CLI §2.6 syntax. Keyed lists are addressed by *key*: `modelTypes[3]` means
    typeId 3 (see API-02).

**EVL-08 · guess · Float errors**
- Proposed: any operation that produces NaN or ±Inf (`x / 0.0`, `sqrt(-1)`, `pow` overflow) is
  `E4104` at the expression.

**EVL-09 · guess · Order of findings**
- Proposed: findings are sorted by (file path bytes, line, col, code, message).

**EVL-10 · guess · Duration arithmetic**
- Proposed: `Int * Duration` is allowed and commutes. `Duration * Float` is not allowed. `Duration /
  Int` truncates toward zero at ms. Negative durations are allowed in expressions. Storage
  refinements decide whether they are valid in data.

---

## 7. Checks (CHK)

**CHK-01 · blocker · How E5xxx/W5xxx codes are assigned**
- Proposed: `E5001`/`W5001` for one-line `check`/`warn`, and `E5002`/`W5002` for `fail`/`warn` in a
  block. The check name, if any, goes in the finding's `check` field. Tests match `fails name` on
  that field.

**CHK-02 · guess · Location of a one-line check**
- Proposed: the record literal's opening token. For a table entry, the key token. For JSON, the
  object's `{`.

**CHK-03 · guess · Translation key for package-level named checks**
- Where: §17 only defines `Type.check.<name>`.
- Proposed: `check.<name>` for package-level checks.

**CHK-04 · guess · Where `fail`/`warn` may appear**
- Proposed: only lexically inside `check { }`. A call from a helper fn is `E1105`.

**CHK-05 · guess · What `expect v fails …` matches**
- Proposed: static type errors in a test are compile errors, never "expected". `fails`/`warns` match
  the evaluation findings (dynamic `E3xxx`, `E4xxx`, `E5xxx`) of building `v` and the values
  reachable from it.

---

## 8. Standard library (STD)

**STD-01 · blocker · Signatures and container coverage**
- Where: §20. The examples call `items.first()` on a *table* and `columns.active().first()`, which
  are not in §20.4. Unspecified: whether `active()` returns a list or a table; map `filter`/`all`
  lambda arity; `sum()` of an empty list (and its type); `join` on non-strings; `zip` with unequal
  lengths; `keys()`/`values()` of a table; `get` on tables vs `find`.
- Proposed: tables and keyed lists support every list method over their entries. `active()`
  returns `[T]` (entries keep identity). Map predicates take `(k, v)`. `sum()` of an empty list is
  `0`/`0s`, or `E3314` if the element type is unknown. `join` takes `[String]` only. `zip` of
  unequal lengths is `E4105`. For a table, `keys()` is `[ref T]` and `values()` is `[T]`. The full
  table goes in STDLIB.md.

**STD-02 · guess · String functions**
- Proposed: `split("")` is `E4106`. `trim` removes ASCII whitespace. `replace` replaces all
  occurrences. `find` returns a byte index. Slicing on a non-boundary is `E4107`.

**STD-03 · blocker · Regex semantics contradict the examples**
- Where: §5.2: "regex: the whole string must match". potion.canon:15 `name: String(/^IDS_/)` with
  data `"IDS_PROPITEM_TXT_POT_L"` fails under whole-match. Every example anchors explicitly
  (`^…$`), so the authors wrote search semantics.
- Proposed: refinements and `matches` use RE2 *search* semantics (Go `MatchString`). Fix the
  §5.2 text. The view model marks patterns as RE2 (VIEW-10).

**STD-04 · guess · Graph functions**
- Proposed: `reachable` is DFS preorder following `next` order, with `x` first. `cycles` returns
  members of SCCs of size > 1 or with a self-loop, in input order (rules.canon relies on a
  self-parent being a cycle). `topoSort` is Kahn's algorithm with input order as tie-break, and
  `E4501` on a cycle, naming it.

**STD-05 · guess · Math helpers**
- Proposed: `min`, `max`, `clamp` and `abs` need all arguments of the same base type. `clamp` with
  `lo > hi` is `E4108`. `floor`, `ceil` and `round` out of range are `E4103`.

**STD-06 · blocker · Canonical text form for interpolation, `String(x)` and messages**
- Where: §2.5 "floats in the shortest form that reads back exactly". There is no exponent rule,
  `1.0` vs `1` is open, format-spec rounding is unspecified, and the text form of lists, records,
  maps, variants and optionals is undefined.
- Proposed:
  - Floats use ECMAScript `Number::toString` (the same as JSON output WIR-01 and TS). Integral
    values print without `.0`.
  - `{x:.N}` rounds half away from zero.
  - `{x:,}` groups by 3 with `,`.
  - `{x:+}` prints `+0` for zero.
  - Lists print as `[a, b]`, maps as `{k: v}`, records as `Type{f: v}`, cases as `case{…}`,
    `none` as `none`.

---

## 9. Stable ids and the lock (LCK) §12

**LCK-01 · guess · Exact lock format**
- Where: §12 shows `table teamboard.statuses  open` and `enum  resource.vocab.Element  1  FIRE`
  (padded kind, double spaces). Sorting is "sorted" with no key, and bytewise sorting puts
  `10` < `2`.
- Proposed: fields separated by two spaces, kind left-padded to 5. Sort by (kind, qualified name,
  numeric value if integer, else bytes). Non-identifier values are JSON-quoted. Header line as
  shown. `\n` line endings.

**LCK-02 · blocker · What `@stable` locks outside stable tables**
- Where: §12 "which entry held it".
- Proposed: `@stable` is allowed only on fields of elements of `stable table`s (`E6003`
  elsewhere). Its values must be unique among all entries, retired included (`E3102`). For `@codes`
  enums the "entry" is the member name, so renaming a member keeps the code but is `E6001`.

**LCK-03 · guess · `lock check` "without evaluating"**
- Where: CLI §3.12 and §6.3 "neither evaluates values", while stable entries may come from `load`
  or computed expressions.
- Proposed: `lock check` evaluates only stable collections and their dependencies, loads included.
  Soften the pre-commit claim.

**LCK-04 · guess · Layers, studio edits and merges**
- Proposed: layers may not add entries to stable tables (`E6004`) and never write the lock. An
  edit-API `Add` on a stable table appends to the lock inside the same atomic edit. After a git
  merge, two different entries claiming the same new id is `E6002`.

**LCK-05 · guess · Retired entries in outputs**
- Proposed: retired entries appear everywhere: JSON rows (`"$retired": true`), baked values, the
  view model and search (flagged), and plain iteration. They are excluded only from `.active()`.

---

## 10. Loading files (LOD) §13

**LOD-01 · blocker · `load.defines` parsing**
- Where: §13.2, one line. Used 13 times in the examples. Unspecified: value syntax (hex, suffixes
  `UL`, negatives, parentheses, `A|B`, `A+1`, references to earlier defines, char literals, strings),
  `#if`/`#ifdef` blocks (vocab relies on `__CARD_COLLECTION`-style flags), `#undef`, duplicates,
  line continuations, comments, and file encoding (legacy headers are likely CP949/Latin-1).
- Proposed: no preprocessor. Scan `^\s*#\s*define\s+NAME\s+EXPR`, where EXPR is an integer
  literal (dec/hex/oct, suffixes ignored), unary `-`, parentheses, `| & + - << >>`, or a reference
  to an earlier define in the same file. Other defines are skipped (`W7101`, reported once per
  file). All `#if` branches are read. The same name with a different value is `E7102`. Bytes are
  decoded as Latin-1 (names are ASCII). Entries appear in file order.

**LOD-02 · blocker · Reading JSON**
- Unspecified: null vs absent, ints written as floats, precision beyond 2^53, duplicate keys,
  BOM, comments, and whether `partial` applies recursively.
- Proposed:
  - Strict RFC 8259; a BOM is tolerated.
  - Absent → default. `null` → `none` for optional fields, `E3315` for required ones.
  - `Int` accepts only integer tokens: `2.0` or `1e3` is `E7103`. Numbers are parsed exactly (big
    decimal) before conversion.
  - Duplicate keys are `E7104`.
  - `partial: true` applies recursively.
  - Non-UTF-8 input is `E7105`.

**LOD-03 · guess · The `at:` path language**
- Proposed: `.`-separated segments. `*` over an object gives a map in key order. `*` over an array
  gives a list. `[n]` is an index. `\.` escapes a dot. A missing path is `E7106`.

**LOD-04 · blocker · Keys for `table T` from files**
- Where: §13.2: "With an expected type `[T] keyed by f` or `table T`, keys come from each file's
  content". `table T` has no key field. The JSON shape of a table read from one file is also
  undefined.
- Proposed: a `table T` from one file is a JSON object keyed by id, in file order. From
  `load.dir`, the key is the file stem. A keyed list takes its key from field `f`.

**LOD-05 · guess · Globs**
- Proposed: doublestar semantics. Dotfiles are skipped. Symlinks are followed only inside roots.
  Order is the byte order of the root-relative path. Zero matches is `W7107`.

**LOD-06 · guess · `load.csv`**
- Proposed: RFC 4180, `,` separator, UTF-8. With a header, columns map to wire names. Without one,
  the result is `[[String]]`. Cells are parsed as Canon literals of the expected field type
  (`E7108`). An empty cell means default or `none`.

**LOD-07 · guess · `load.text`**
- Proposed: strict UTF-8, `\r\n` → `\n`, no trimming.

**LOD-08 · guess · Format detection**
- Proposed: `.json` → json, `.csv` → csv, `.txt` → text. `.h`/`.hpp` only through `load.defines`.
  `format:` takes `json`, `csv` or `text`.

**LOD-09 · guess · `@json(path: "a.b")`**
- Proposed: on read, a missing intermediate object means default. On write, create nested objects
  in declaration order of the first field that uses the prefix. A field whose wire name equals a
  path prefix is `E3316`.

**LOD-10 · guess · Unit conversion**
- Where: heistia `maxDuration … unit: m` holding e.g. 90s.
- Proposed: read accepts ints or finite floats, and the result must be a whole number of ms
  (`E3203`). Write outputs an integer when exact, else `E8102` at the value.

**LOD-11 · guess · Build manifest and cache**
- Proposed: the manifest contains the compiler version, language version, layer list, `--lang`,
  and a SHA-256 of every source and loaded file (the list of paths read, globs expanded). The cache
  is keyed by the manifest's hash. Cache contents are opaque, and the format is versioned.

---

## 11. Wire and JSON output (WIR)

**WIR-01 · blocker · Canonical JSON bytes**
- Where: §14.3 "fixed key order (declaration order) and fixed whitespace"; §11.3 "shortest
  round-trip". potions.json is hand-aligned.
- Proposed: UTF-8, `\n`, trailing newline. Top-level keys `$schema`, then `rows`/`value`, each on
  its own line with 2-space indent. Each element of `rows` goes on one line: `{"k": v, "k2": v2}`
  with `": "` and `", "`, no alignment, nested objects inline. Non-row values are pretty-printed
  with 2-space indent. Strings: raw UTF-8, escape only `"`, `\\` and control chars (`\n`, `\t`,
  `\u00XX`), no HTML escaping. Numbers: integers exact, floats per ECMAScript
  `Number::toString`, `-0` written as `0`.

**WIR-02 · blocker · `none` on the wire**
- Proposed: output writes `null` (every field present, keeping "every default is filled in"), or
  the `@json(none: …)` value. Input: absent means the default, `null` means `none`, and the
  `none:` value also means `none`. An optional with a non-`none` default (`Int? = 5`) is written
  with `null` for `none` and absent for the default.

**WIR-03 · blocker · Duration without `@json(unit:)`**
- Proposed: integer milliseconds.

**WIR-04 · blocker · Enum wire value, especially with `@codes`**
- Where: §5.3 `.wire`/`.code`; adventurequest `kind: RewardType @json("type")` where RewardType has
  `@codes`.
- Proposed: the wire value is the name, or the `= "…"` string. For `@codes` enums the `=` gives the
  code, so a different wire name uses `@json("…")` on the member. Codes appear only in generated
  code and the lock, unless the enum has `@json(codes)`, in which case the wire is the number.

**WIR-05 · blocker · Map encoding**
- Proposed: a JSON object with keys in insertion order. String keys are verbatim, integer keys are
  decimal, enum keys use the wire value, ref keys use the key (decimal for Int keys). Keys that
  collide after encoding are `E3317`.

**WIR-06 · blocker · Table keys in `rows`**
- Where: §14.3 `rows` for a table. `Status` has no id field. The §14.3 sample's `"dwID"` comes from
  nowhere.
- Proposed: each table row starts with `"$id": "<key>"` (and `"$retired": true` when retired).
  Keyed lists add nothing.

**WIR-07 · blocker · Variant encoding and inline collisions**
- Proposed: always an object. A case without fields is `{"kind": "nothing"}`. The tag goes first.
  With `@json(inline)`, a tag or case-field wire name equal to a parent field's wire name is
  `E3318` at declaration, and there is at most one inline variant per record. `@json(case:)` on a
  variant affects field names, not case names.

**WIR-08 · guess · The `@json(case: snake)` algorithm**
- Proposed: word boundaries fall at lower→Upper, digit→Upper, and Upper→Upper-followed-by-lower
  (`HTTPServer` → `http_server`). Digits stay with the preceding word (`stage1Rate` →
  `stage1_rate`). Everything is lowercased. Supported cases: `snake`, `camel` (identity), `kebab`,
  `upper_snake`.

**WIR-09 · guess · `$` keys for export fns**
- Where: §14.3 `"$isStrong"`.
- Proposed: `$<canonName>` whatever the case setting. A finite-input *method* is
  `"$name": {<wire of arg>: result}`, nested per extra argument. A package-level export fn in data
  mode goes into the data file under `"$fns": {"name": …}`. In baked mode it is only in code.

**WIR-10 · guess · Several values in one `emit json`**
- Proposed: if `values` has more than one element, or is omitted, `out` is a directory and each
  file is `<valueName>.json`. A map value goes under `value`.

**WIR-11 · guess · Input and deprecated fields in JSON**
- Proposed: input fields are omitted. Deprecated fields are emitted.

---

## 12. Emit and fingerprint (EMT)

**EMT-01 · blocker · Fingerprint input**
- Where: §14.4 "SHA-256 over the **shape** … in order". There is no serialization, and the list
  leaves out optionality, `none` encoding, `@json(path)`, `inline`, codes, the `$` export-fn keys
  and their types, input fields, cross-package types and recursive types.
- Proposed: a canonical text serialization (FINGERPRINT section):
  - First line `canon-fp v1`.
  - Types visited depth-first from the root, each qualified type once, then referenced as `@name`.
  - Per field: `field <wire> <typeExpr> opt=<0|1> none=<json> path=<p> inline=<0|1> unit=<u>`.
  - Enums: wire values and codes. Variants: tag and cases. `$` keys with their types.
  - Excluded: Canon names (see EMT-02), docs, refinements, defaults.
  - SHA-256 over UTF-8; first 8 hex digits, lowercase.

**EMT-02 · guess · Renaming a Canon field changes `$schema`**
- Where: §14.4 includes "field names". DECISIONS 3 says renaming never affects data, yet the
  fingerprint changes and old binaries refuse the new file.
- Proposed: hash wire names only.

**EMT-03 · blocker · `emit` options and defaults**
- Where: §14.1. Examples omit `mode` (`emit ts`, `emit json`). The README implies `types` for
  load-backed domains.
- Proposed: default modes: go `baked`, ts `baked`, cpp `baked`, json n/a. `values` defaults to all
  public values. Every target always emits all public types, consts and export fns of the package.
  One `emit` per target per package (`E8002`). `out`: go = directory, cpp = directory, ts = file,
  json = file or directory (WIR-10), view = file. Unknown options are `E8003`.

**EMT-04 · blocker · `data` mode vs id enums**
- Where: §5.7 "keys of a public table form an enum in generated code" vs §14.2 "Changing a value in
  `data` mode rebuilds only the data file". Adding an item changes `ItemId`.
- Proposed: id enums only in `baked`/`embedded`. In `data`/`types` modes ids are a distinct string
  type (`type ItemId string`, `std::string_view`).

**EMT-05 · blocker · `types` mode cannot be filled by a hand-written loader**
- Where: §14.2 "types only; the runtime keeps its own loader" vs §15.1 "Every field is private …
  Only the generated loader creates values". event.canon:135 uses `cpp types`.
- Proposed: `types` emits the read-only classes plus a generated in-memory decoder
  (`static std::optional<EventConfig> Decode(const nlohmann::json&, std::string& error)`, Go
  `Decode(raw []byte)`). There is no file I/O and no fingerprint check (the legacy file has no
  `$schema`). The runtime keeps reading its own file and calls `Decode`.

**EMT-06 · blocker · Cross-package references in generated code**
- Where: teamboard's Go needs `ui.Tone` and `roles.Role`, and heistia refs vocab. There are no Go
  import paths, C++ include paths or TS module paths. It is also unspecified what happens if the
  imported package has no emit for that target.
- Proposed: `project.canon` gets `go_module: {"<root>": "github.com/org/repo"}` so a Go import path
  is derived from the `out` directory. C++ uses `#include "<path relative to a declared include
  root>"`, and TS uses relative imports computed from the out paths. A referenced package without
  an emit for the same target is `E8004`. Types are never duplicated across outputs.

**EMT-07 · guess · Which types an emit contains**
- Proposed: all public types of the package, whether used or not. Imported types are referenced
  and never re-emitted.

---

## 13. Code generation, common (CG)

**CG-01 · blocker · Naming and case conversion per target**
- Where: §15.1 "getters follow each language's convention". The golden uses Go `ID()` (an
  initialism) but C++ `GetId()`. Enum member names from `II_WEA_AXE_ANGEL`, `gm_junior`,
  `Stage_1`, `none_`, `series_1` are undefined. The variant kind of `EventKind` would be
  `EventKindKind`. Store and table class names are undefined (golden: C++ `PotionTable`, Go
  `Table`).
- Proposed: a CODEGEN-NAMING doc.
  - Word split: on `_`, lower→Upper, letter↔digit.
  - UpperCamel = capitalise each word. Go applies an initialism list (ID, URL, API, HTTP, JSON, UI,
    DB, IP, HP, MP, TS). C++ uses no initialisms.
  - Enum members: C++ `enum class` keeps the Canon name verbatim. Go uses
    `<Enum><UpperCamel(member)>` (`ToneSeries1`). TS uses a union of *wire* strings.
  - Table ids: `<Element>Id`.
  - Containers: `<UpperCamel(valueName)>` (`Potions`).
  - Stores: `<Container>Store`.
  - Kind enums: `<Variant>Kind`, even when the name repeats.

**CG-02 · blocker · Collisions with target keywords and generated names**
- Where: Go field `default` (taxonomy.canon:73, adventurequest.canon:72: invalid Go, and teamboard is
  the v0 target), fields `type`, `range`, `func`, `map`. Method clashes with `String()`, `Kind()`,
  `Len()`, `All()`, `Find()`, `Wire()`. C++ keywords (`default`, `delete`, `new`, `and`, `not`) and
  Windows macros (`ERROR`, `DELETE`, `IN`, `OUT`, `min`, `max`). Two Canon names can map to one
  target name (`series_1` vs `series1`).
- Proposed: append `_` to keyword collisions (Go field `default_`). Getter-vs-generated method
  collisions and post-conversion duplicates are `E8005`, fixed with `@go(name:)`, `@cpp(name:)` or
  `@ts(name:)`. C++ headers `#undef`-guard nothing: document the macro risk, and flag known Windows
  macros with `W8006`.

**CG-03 · blocker · How `ref` is represented**
- Where: §15.2/§15.3 say `ref T` → `*T` / `const T&` "resolved at load". §15.5 says "References to
  values outside the snapshot are held by key". Refs to `local` or define tables and to
  sibling-field collections (RES-03) are not covered.
- Proposed: a getter returns a pointer or reference only when the target is in the *same value*
  (same data file or snapshot). Otherwise only the key getter exists (`XxxID()` /
  `GetXxxKey()`). Refs to define tables expose the key and `GetXxxValue()` (the define's integer),
  which legacy code needs as a DWORD.

**CG-04 · blocker · Shape of containers and loaders with several values**
- Where: §15.2 Go `Load(path string) (*<Value>, error)` vs golden `Load(path) (*Table, error)`.
  Also: two values of the same element type, single-record values (`farm: FarmConfig`), and baked
  accessor names (`const XTable& X()`).
- Proposed: for each value `v`:
  - Go: `Load<V>(path) (*<V>, error)`; baked `func <V>() *<V>`.
  - C++: `class <V>` (container, e.g. `Potions`) with `static Load(...)`; baked `const <V>& Get<V>()`.
  - A single record value's container *is* the record type.

**CG-05 · guess · Getters for optional containers and variants**
- Proposed: Go `(rt.List[T], bool)`, `(rt.Map[K,V], bool)`, `*Variant`. C++ `const std::vector<T>*`,
  `const FlatMap<…>*`, `const Variant*`.

**CG-06 · guess · How doc comments are copied**
- Where: golden Go rewrites prose.
- Proposed: copy verbatim (LEX-07). Go prefixes the first line with `<Name>: ` only if it does not
  already start with the name. No other rewriting.

**CG-07 · guess · Generated header and explanatory comments**
- Where: `GENERATED by canon from <file>`: which file for a multi-file package? The goldens add
  fixed prose ("Read-only by construction…").
- Proposed: `<file>` is the package directory (`pipeline/`). The explanatory comments are fixed
  templates listed in the codegen doc.

**CG-08 · guess · Lookup tables for finite-input export fns**
- Proposed: generated as dense arrays indexed by enum ordinal or table index. The domain includes
  retired members. More than 65,536 cells is `E9002`. Optional params are not allowed (`E9003`).
  Go: `func CanTransition(from, to StatusId) bool`. List results are returned as read-only views.
  Data mode: WIR-09.

**CG-09 · guess · Go `rt` helpers**
- Where: §15.2: "`rt` is a small read-only collections file generated once per output package",
  but `rt.List` syntax needs a separate Go *package* with an import path.
- Proposed: emit `<out>/rt/rt.go` (package `rt`), imported through EMT-06, and shared by all
  packages of one Go module only if they have the same version.

**CG-10 · guess · Toolchain floors**
- Proposed: Go ≥ 1.23 (`iter.Seq`, builtin `min`/`max`). C++17 on GCC ≥ 9, Clang ≥ 10 and
  MSVC 19.2x. nlohmann/json ≥ 3.9. TS ≥ 5.0, ESM, conformance tests with `node:test`.

---

## 14. Go (GO)

**GO-01 · blocker · The Go golden has no store for `@reload`**
- Where: §15.5 requires a store with `atomic.Pointer` for `@reload` values. potions.go has none.
- Proposed:
  ```go
  type PotionsStore struct{ p atomic.Pointer[Potions] }
  func (s *PotionsStore) Current() *Potions
  func (s *PotionsStore) Reload(path string) error
  ```
  Plus a package-level `var Store PotionsStore`. Fix the golden.

**GO-02 · guess · Conformance style contradicts §9.4**
- Where: §9.4 says "The translated body is emitted as a pure function that both the method and the
  test call". The golden inlines the body into the method and constructs `&Potion{heal: …}` in the
  test.
- Proposed: follow §9.4 in every target: `func potionHealFor(heal int32, missingHp int64) int64`,
  called by both the method and the test.

**GO-03 · nit · Go values can be created outside the loader**
- Where: runtime code can write `potions.Potion{}`, despite "only the generated loader creates
  values".
- Proposed: document this as a Go limitation.

**GO-04 · guess · Baked mode for large data**
- Proposed: package-level unexported `var` built by composite literals. The accessor returns a
  pointer. No `init()` side effects except input reading (LAY-04).

**GO-05 · guess · Enum API**
- Proposed: `String()` returns the Canon name, `Wire()` the wire value,
  `Parse<Enum>(wire string) (E, bool)`. Values start at 0, or equal `.code` with `@codes`.

---

## 15. C++ (CPP)

**CPP-01 · blocker · `access: fields` integration**
- Where: §15.3 "Canon generates the code that fills it". Unspecified:
  - the header of the hand-written struct;
  - who owns storage (legacy `m_aProp` arrays or a generated vector);
  - the filler's signature and who calls it;
  - how `Find` returns `const ItemProp*`;
  - what happens to unmapped members;
  - `char[64]` members (`szName`) and `DWORD` conversion checks;
  - refs to define tables (legacy wants numeric values);
  - flattening the item-kind *variant* into a flat struct;
  - the `static_assert` form.
- Proposed: `@cpp(struct: "ItemProp", header: "ItemProp.h", access: fields)` generates
  `class ItemPropTable` owning `std::vector<ItemProp>`, filled by `static void Fill(const
  nlohmann::json& row, ItemProp& out)`. `Find` returns `const ItemProp*`. Unmapped members are
  value-initialised. `@cpp(type: "char[64]")` gets a build-time length check (`E8103`). Refs to
  defines store `.value`. Inline variant fields map to members named by `@cpp(field:)` on each case
  field. For each member: `static_assert(std::is_same_v<decltype(ItemProp::m), T>)`.

**CPP-02 · guess · `access: both`**
- Proposed: the generated struct is written to `@cpp(header:)` (replacing the hand-written header,
  after an `E8001` override via `canon convert`-style adoption). Members keep the legacy names and
  types. Getters are added.

**CPP-03 · guess · `@cpp(defines: "IK1_")`**
- Proposed: the define name is the member name if it already starts with the prefix, else prefix +
  member. The value is the code with `@codes`, else the index.

**CPP-04 · guess · C++ enum details**
- Proposed: the underlying type is `uint8_t`/`uint16_t`/`uint32_t` by count, or the `@codes`
  type. Members keep Canon names; keywords get a `_` suffix. `ToWire`/`FromWire` are overloads in
  the namespace.

**CPP-05 · guess · `canon_runtime.h` API and versioning**
- Why: two output directories linked into one binary can break the ODR.
- Proposed: `namespace canon { inline namespace rt_v1 { … } }`. Specify the exact `FlatMap`,
  `Load` helpers and error types in the codegen doc.

**CPP-06 · guess · C++ file layout**
- Where: §15.6 `<package>_conformance.gen.cpp` vs golden `potion_conformance.gen.cpp`. Header per
  source file (`potion.gen.h`) or per package?
- Proposed: per package, named after the last segment: `pipeline.gen.h/.cpp`,
  `pipeline_conformance.gen.cpp`, entry point `int RunPipelineConformance()`. Go uses
  `<gopkg>_conformance_test.go`.

**CPP-07 · nit · Golden C++ bugs**
- `doc["rows"]` on a `const json` is UB when the key is missing (use `find`/`at`).
- `Size()` vs Go `Len()`: pick one name per concept.
- `PotionsStore::Current()` returns null before the first `Reload` (RLD-01).

**CPP-08 · guess · Baked C++ data**
- Proposed: function-local `static const` inside `const X& GetX()` (avoids static init order
  problems). No `constexpr` requirement.

---

## 16. TypeScript (TS)

**TS-01 · blocker · The TS target is under-specified**
- Unspecified: `data` mode runtime (browser `fetch` vs Node `fs`); enums as names or wires;
  variants (discriminant property); refs (key or object); optional (`undefined` or `null`); maps
  (`ReadonlyMap` from JSON; `Object.freeze` does not freeze a `Map`, so §15.4's "writes fail … at
  runtime" is false); the `number / bigint` rule for `UInt64`; lookup fns; file layout;
  conformance framework.
- Proposed:
  - `data` mode exports `decode(json: unknown)` only, with no I/O.
  - Enums are unions of wire strings. Variants are `{readonly kind: "<case wire>", …}`.
  - Refs are key strings plus a `resolve` helper. Optional is `T | null`.
  - Maps are `ReadonlyMap` backed by a frozen wrapper class that throws on `set`.
  - `UInt64` is `bigint` only with `@ts(bigint)`, else `number` with `E8101` beyond the safe range.
  - One `.ts` file per emit. Conformance uses `node:test`.

---

## 17. Conformance tests (CNF)

**CNF-01 · blocker · Vector selection algorithm**
- Where: §9.4 says "0, ±1, the type's limits, each refinement bound ±1". The golden has
  `{200, 9000, -5}` from tests, plus 0, 499, 500, 501, INT64_MAX and INT64_MIN. That is no ±1, and
  499/500/501 are *self-field value ±1*, which the rule does not mention. Also unspecified: which
  `self` values, how several params combine, Float/Duration/String/enum boundaries, ordering,
  dedup.
- Proposed:
  - Receivers: the distinct `self` values used by test calls of that fn, in source order. Only the
    fields the body reads are kept.
  - Per param: test values ∪ {0, 1, −1, type min, type max} ∪ {each refinement bound −1, 0, +1} ∪
    {each Int-typed self field value read by the body, −1, 0, +1}, restricted to the param's
    domain.
  - Float adds ±0.5 and ±1e300; String adds "" and each test string; enums use all members.
  - Combination: cartesian product capped at 256 per receiver, else pairwise, in a fixed order.
  - Order: test vectors first in source order, then generated ones sorted ascending, deduplicated.
  - Regenerate the golden.

**CNF-02 · blocker · The portable subset is not actually portable**
- Where: §9.4 "exactly the constructs whose behaviour is identical in Canon, Go, C++ and
  TypeScript". Counter-examples:
  - Int overflow: Canon gives `E4101`, Go wraps, C++ is UB, TS loses precision above 2^53 (and the
    golden feeds INT64_MAX).
  - Division by zero.
  - JS `Math.round` is not half-away-from-zero.
  - Float formatting in templates differs per target.
  - Go `time.Duration` is in ns.
- Proposed:
  - Every target gets generated checked helpers (`canon::Add`, …) that raise a target-native error
    (C++ throws `canon::EvalError`, Go panics with `*canon.EvalError`, TS throws), with the Canon
    code.
  - A vector where Canon errors expects that error.
  - TS vectors outside the safe integer range expect the documented `Number.isSafeInteger` throw.
  - Templates may interpolate only Int, String and enum in translated fns (no Floats).

**CNF-03 · guess · Float bit-exactness**
- Proposed: exact bit equality is required. Generated C++ uses no FMA-sensitive expressions, and
  the docs require `-ffp-contract=off` or `/fp:precise`.

---

## 18. Hot reload (RLD) §15.5

**RLD-01 · blocker · One snapshot vs one file per value**
- Where: "All `@reload` values of one `emit` are loaded together into one immutable snapshot …
  `Reload(path, error)`" vs §14.3 "`emit json` writes one file per value". The golden's `Current()`
  returns null until the first `Reload`, and there is no startup load.
- Proposed: the snapshot is `class <Package>Snapshot` with one member per `@reload` value.
  `Reload(dir, error)` reads `<dir>/<value>.json` for each. The first load is also `Reload`, and
  `Current()` returns null before it (documented). Values without `@reload` keep their plain
  `Load`.

**RLD-02 · guess · `@reload` in modes without files**
- Proposed: `@reload` in `baked`/`types` mode is `E8202`. TS has no store in v0.

---

## 19. Views (VIEW) §16

**VIEW-01 · blocker · API for `when` and `show`**
- Where: §16.10: "Conditions (`when`) and computed lines (`show`) are evaluated by the compiler
  through the edit API (CLI.md §5)". CLI §5 has no such call.
- Proposed: add `p.Evaluate(ctx, canon.EvalRequest{Path, Draft []Op}) (EvalResult, error)`
  returning `when` flags per field and group path, `show` lines, `title`/`subtitle`, and findings
  for the draft.

**VIEW-02 · guess · Scope of view templates**
- Where: adventurequest.canon:176 `title "{key}"` ("`{key}` is a map key", §16.6).
- Proposed: the scope is the fields and methods of the type, plus `id` (entries), `key` (map
  values) and `index` (list elements). A type with a field named `key`, `id` or `index` shadows the
  magic name (`W1604`).

**VIEW-03 · guess · What `menu` applies to**
- Where: `menu events icon farm` sits on a *type* (`view FarmConfig`).
- Proposed: every public value of exactly that type, in the type's package, appears under that
  menu. A value can override this with `@menu(…)`.

**VIEW-04 · guess · Views of imported types**
- Where: the grammar allows `view qualifiedName`, and §16.6 says "A type has at most one view".
- Proposed: in v0 a view must be declared in the type's own package (`E1603`). Uniqueness is then
  per package.

**VIEW-05 · guess · Who chooses the default control**
- Where: §16.2 thresholds count *active* entries, which are known only after evaluation.
- Proposed: the compiler resolves the control and writes it into the view model (`control` always
  present). The studio never re-derives it.

**VIEW-06 · guess · Groups and ordering**
- Proposed: a field in two groups is `E1605`. Ungrouped fields go to a final group with id
  `_other` (key `Type.group._other`, label "Other"), ordered by `usage` descending, then declaration
  order. "More" collects fields set by < 10% of values. `hidden` wins over group membership.

**VIEW-07 · guess · Matching widget and control types**
- Proposed: compare after stripping refinements, `keyed by` and `where`. A `T?` field matches a
  `T` parameter. `_` matches anything. Aliases are expanded.

**VIEW-08 · guess · Where `unit` applies**
- Where: adventurequest.canon:178 puts `unit: pct` on `[Int]` and on `{ref: Int}`.
- Proposed: allowed on Int, Float, Duration and on lists or map values of these. `scale` means
  shown = stored × scale.

**VIEW-09 · guess · `columns` items**
- Proposed: fields only (a method column is `E1606`; use `show`). Nested or composite fields are
  shown as their `title`. Without `columns`, the default is the first 6 scalar fields.

**VIEW-10 · guess · Regex dialect in the studio**
- Proposed: the view model flags patterns as `re2`. The studio uses an RE2-compatible JS engine, or
  calls `Evaluate`.

---

## 20. View model format (VM) §16.10

**VM-01 · blocker · No schema, and the golden contradicts the key list**
- The golden:
  - sets `$schema` to the *data* fingerprint (§16.10 says "fingerprint of the view model format");
  - nests `view` inside `types` instead of a top-level `views`;
  - inlines labels and help per language instead of `i18n`;
  - has no wire names, though §16.10 lists them;
  - lacks `usage`, `search`, `assets`, `widgets`, `i18n`;
  - encodes one type as JSON (`{"kind":"int","min":1}`) and another as a Canon string
    (`"[Potion] keyed by id"`);
  - gives `required` on some fields only;
  - makes `files` a glob rather than a file list;
  - has no `editable`.
- Proposed: publish `viewmodel.schema.json` (JSON Schema 2020-12). `$schema` becomes
  `"canon-vm/1"`. Fix the golden.

**VM-02 · blocker · Type encoding catalogue**
- Proposed: one tagged object per kind:
  - `bool`
  - `int {bits, signed, min?, max?}` (inclusive bounds)
  - `float {bits, min?, max?, minExclusive?}`
  - `string {minLen?, maxLen?, pattern?}`
  - `duration {min?, max?}` (ms integers)
  - `enum {ref: "pkg.Enum"}`
  - `record {ref}`
  - `variant {ref}`
  - `list {of, min?, max?, keyedBy?}`
  - `table {of, stable}`
  - `map {key, value}`
  - `optional {of}`
  - `ref {collection: "pkg.value[.path]"}`
  - `union {of, literals}`
  - `asset {root, ext}`
  - `never`
  - `dependent {on: "<sibling path or $key>", discriminant: "<path>", branches: {"<member>": <type>}}`
  - `predicate: true` whenever a `where` exists (validated via API).
  - Enums carry members `{name, wire, index, code?, retired, label, help, icon, tone}`.

**VM-03 · blocker · Type identity across packages**
- Where: the golden keys types by bare name. `Global` exists in both farm and adventurequest,
  `Reward` in heistia and §5.5, and `Window` comes from another package.
- Proposed: qualified names (`resource.farm.Global`) everywhere. A type from another package is
  referenced, not copied. The studio loads every package's view model.

**VM-04 · guess · Size and sharing of the search index**
- Proposed: each collection's search index is written only in its *own* package's view model,
  referenced by id. That avoids 7,000 item rows in every package that refs `items`.

**VM-05 · guess · `values` and `editable`**
- Proposed: `{type, sources: [file…] | {glob, count}, editable: "canon" | "json" | "computed" |
  "layered", path}`. A computed value is not editable. A value touched by an active layer is
  editable at base with a warning.

**VM-06 · guess · Definition of `usage`**
- Proposed: count every instance in the package's evaluated values, per type or case. A field
  counts as "set" when present in the source (literal or JSON key), not when it merely differs from
  its default.

**VM-07 · guess · Findings in the view model**
- Where: emit only runs without errors, so `findings` can only ever hold warnings.
- Proposed: `emit view` still runs when there are errors. Only code and data emits are blocked.

---

## 21. Translations (I18N) §17

**I18N-01 · blocker · Incomplete key catalogue**
- Missing keys:
  - type docs (`Type.help`);
  - enum member help (`Element.FIRE.help`, used in §16.4 but absent from §17);
  - variant cases and their fields;
  - `show` lines (no id);
  - placeholders;
  - column headers;
  - package-level checks;
  - widget docs.

  Also undefined: whether a title with no literal text (`"{typeName}"`) is a key at all.
- Proposed: keys are
  - `Type.help`
  - `Type.<field>[.help|.deprecated|.placeholder]`
  - `Variant.<case>[.help]`
  - `Variant.<case>.<field>…`
  - `Enum.<member>[.help]`
  - `Type.show.<n>` (0-based index) or `show <id> "Label" …` with an optional id
  - `Type.title|subtitle|singular`
  - `Type.group.<id>[.intro]`
  - `Type.check.<name>` and `check.<name>`

  Texts with no literal characters are not keys.

**I18N-02 · guess · Templates in translations**
- Where: farm.fr.canon:46 `Level.title "Palier {level}"`, while §17 allows templates only for check
  messages.
- Proposed: any text that is a template in the source may be a template in a translation.
  Translations are type-checked in the source item's scope (`E1703`).

**I18N-03 · guess · W1701 volume**
- Why: one warning per missing key per language means thousands, and it conflicts with CLI §6.2
  `--max-warnings 0`.
- Proposed: one `W1701` per (package, language) with a count. Details come from
  `canon i18n status`.

**I18N-04 · guess · Translation file rules**
- Proposed: keys refer to the file's own package. A language not in `project.languages` is
  `E1704`. A duplicate key is `E1705`. A translation file with a `package` line of another
  directory follows the same rules as source (§3.2).

---

## 22. Layers and inputs (LAY) §19

**LAY-01 · guess · Finding layers**
- Proposed: `--layer x` applies every `layer x` file in the loaded packages. A name that matches no
  file is `E1901` (exit 2). A package may have at most one file per layer name.

**LAY-02 · blocker · Amending computed or loaded values**
- Where: §19.1 "replace the value at each dotted path" / §11.2 "amendments replace source
  expressions". For a path inside a loaded or computed value there is no source expression. The
  syntax for adding a table entry or map key is unshown, and lists are not covered.
- Proposed: an amendment is an override applied to the value right after its base evaluation and
  before any reader. The path must exist, except the last segment may be a new map key or table key
  (the RHS is the entry literal). List elements can be replaced by index but not appended. `const`
  cannot be amended (`E1902`). The RHS is typed against the path. Provenance records "layer x".

**LAY-03 · guess · Layers and outputs**
- Where: `--layer` builds write the same `out` files as plain builds, so they flip-flop between CI
  and dev.
- Proposed: document it. Also `canon build --layer x --out-suffix` (optional, v1).

**LAY-04 · blocker · Inputs in generated code**
- Unspecified:
  - env parsing per type (Int, Bool, Duration, enum);
  - where baked mode reads the env and how it reports a missing required input (Go `init` cannot
    return an error);
  - TS (a browser has no env);
  - runtime pattern checks (Go RE2 vs C++ `std::regex` ECMAScript vs JS);
  - whether `where` runs at runtime;
  - inputs inside collections (one env var for all rows?).
- Proposed:
  - Inputs are allowed only in records reachable from a single non-collection public value
    (`E1903`).
  - Env text is parsed as the Canon literal of the type (bool: `true`/`false`, durations
    canonical, enums by wire).
  - Every target gets `LoadInputs() error` (Go) / `bool LoadInputs(std::string& error)` (C++),
    which must be called at startup. Getters of inputs panic or abort before it.
  - Patterns are restricted to the RE2 ∩ ECMAScript subset (`E1904`).
  - `where` is never checked at runtime.
  - TS: inputs are `E8104`.

---

## 23. Diagnostics (DIAG) §21

**DIAG-01 · blocker · No error catalogue**
- Where: only 37 codes are named: E1001, E1601, E1702, E2001, E2002, E2101, E3001, E3101, E3102,
  E3201, E3202, E3301-E3303, E3401, E3501, E3502, E3601, E3701, E4001, E4002, E4101-E4103, E4201,
  E4301, E4401, E6001, E6002, E7001, E8001, E8101, E8201, E9001, W1001, W1002, W1701. Parse errors,
  unknown names, type mismatch, arity, bad annotations, missing return, view errors, load I/O, JSON
  syntax, emit option errors and naming collisions have no codes. E1601 (a view error) sits in a
  range whose area list omits views. Parallel teams will allocate overlapping numbers, and tests
  assert codes.
- Proposed: ERRORS.md plus a single registry (`internal/diag/codes.go`). Owner per range:
  - `E11xx` lexer/parser
  - `E16xx` views
  - `E17xx` i18n
  - `E19xx` layers/inputs
  - `E21xx` resolution
  - `E33xx` literals
  - `E38xx` dependent types
  - `E41xx` arithmetic
  - `E45xx` stdlib
  - `E71xx` load
  - `E81xx` codegen

  Each entry: code, severity, message template, when, example. This document's proposed codes are
  a starting list.

**DIAG-02 · guess · Finding shape**
- Where: CLI §2.4 JSON has `line`/`col` but no end, and `related` has no `col`.
- Proposed: add `endLine`/`endCol`. Columns are 1-based UTF-8 bytes; the LSP converts to UTF-16.
  Add `related[].col`, `check` (name), `stack[]` (`{fn, file, line, col}`), and `layer`. Stop after
  1,000 findings per package with a summary.

**DIAG-03 · nit · Exact text layout**
- Proposed: `severity[CODE]` + two spaces + location. Each detail line is indented 2 spaces. A blank
  line between findings.

---

## 24. Formatter (FMT)

**FMT-01 · blocker · `canon fmt` layout undefined**
- Where: §2.1 "single canonical layout"; CLI §3.6. CI and pre-commit run `fmt --check`. The edit API's
  "minimal writes" re-print "in the canonical layout". The examples use *column alignment*
  (taxonomy.canon:126-197, record fields, `=` defaults). Alignment is non-local: editing one entry
  re-aligns its neighbours, which defeats minimal diffs.
- Proposed: FORMATTER.md (see §27). Key decisions:
  - **no alignment**, or alignment only within a contiguous run of single-line items, which the
    edit API then re-flows as a unit;
  - 2-space indent, width 100;
  - blank lines collapsed to at most 1;
  - comment attachment (leading, trailing, own-line);
  - single-line `{ a, b }` kept if it fits and was single-line in the input, else one item per
    line with no commas;
  - `_` separators in number literals kept as written;
  - durations canonicalised;
  - annotation order kept;
  - imports sorted;
  - long expressions broken after binary operators and before `.`;
  - every example must be a fixed point.

**FMT-02 · blocker · Canonical JSON for *sources***
- Where: CLI §3.6 `--json-sources`; DECISIONS 12.
- Proposed: 2-space indent, one key per line. The key order in existing files is *preserved*. New
  keys go after the previous declared key in declaration order. Unknown keys (from `partial`) are
  preserved in place. Numbers are re-printed canonically (WIR-01). This is not the same as the
  emitted `rows` layout.

**FMT-03 · guess · Naming-convention warnings**
- Where: §2.4 says conventions are reported by `fmt --check`. taxonomy.canon:17 `const version = 7`
  violates UPPER_SNAKE.
- Proposed: `W1003` naming convention, reported by `check` rather than `fmt`. Fix the example or
  exempt it.

---

## 25. Edit API and embedding (API) CLI §5

**API-01 · blocker · The Go API surface is prose only**
- Where: `canon.Options`, `Project`, `Check` return type, `ViewModel` ("as a Go value or JSON"),
  `Value`, `Edit`, `Op`, `Obj`, `Event`, `BuildOptions`, `ErrStale` and the revision type are
  undefined. The mapping from Go values to Canon values in ops (enum member, ref key, duration,
  variant case, none) is also undefined.
- Proposed: commit `api/canon.go` with exported types and doc comments before development starts.
  Values in ops use a small tagged union: `canon.Int(…)`, `canon.Str`, `canon.Dur(time.Duration)`,
  `canon.Member("warning")`, `canon.Key("II_…")`, `canon.Case("item", canon.Obj{…})`,
  `canon.None`, plus a `canon.FromJSON(raw)` shortcut using the wire form.

**API-02 · blocker · Edit path semantics**
- Where: CLI §2.6 `farm.modelTypes[3]`. modelTypes is keyed by `typeId` (Int), so `[3]` is
  ambiguous between index and key. `styles[daily]` vs `["daily"]`. Paths into `load.dir` values,
  computed values, defaulted fields, spread-built values and layered values are all undefined.
- Proposed:
  - `[n]` on a keyed list or table is a *key*. `[#n]` is a positional index.
  - Enum-keyed maps accept a member name or `"wire"`.
  - Editing a computed value is `ErrNotEditable` (with the path of the nearest editable source).
  - A defaulted field is inserted into the literal in declaration order.
  - A spread-built value gets an override field in the spreading literal.
  - A layered value is edited at base unless `Options.EditLayer` names the layer.

**API-03 · blocker · The "minimal writes" algorithm**
- Proposed: the unit of re-printing is the smallest enclosing *item* (field line, entry, list
  element) whose text changes. The CST keeps trivia, and comments stay attached to their item.
  Alignment interactions follow FMT-01.
- `Add` with `@files`:
  - template variables: enum → wire, ref → key, variant → case wire;
  - the path is relative to the package directory;
  - a missing directory is created;
  - a template path collision is an error.
- `Remove` on an entry file deletes the file. `Retire` inserts `retired `.

**API-04 · guess · Revision**
- Proposed: a hash of (path, content hash) for all sources, loaded files and layers of the loaded
  packages, maintained incrementally. `ErrStale` only if a file read by the edited packages changed.

**API-05 · guess · Concurrency and watching**
- Proposed: a single-writer lock. `Watch` events are coalesced over 100 ms. An external change during
  an edit makes the edit fail with `ErrStale`.

**API-06 · guess · Setting a value equal to its default**
- Proposed: the field is *removed* from the literal (or the JSON key). `none` also removes the key,
  unless `@json(none: X)` requires writing `X`.

---

## 26. Other commands (CLI)

**CLI-01 · guess · `canon infer` algorithm**
- Proposed:
  - Strip the legacy prefixes `dw n sz b f m_ by w l u i str ar`, then lowerCamel the rest.
  - Collisions get a numeric suffix.
  - Type narrowing: Bool < Int < Float < String. JSON `1.0` counts as Float.
  - A field becomes a ref when every value is a key of exactly one `load.defines` table of the
    project.
  - With `--by`, case names are the observed values (wire). A field set by at least one row of
    *every* case is hoisted.
  - Fields are ordered by first appearance in path-ordered files. Output is deterministic.

**CLI-02 · blocker · `canon convert` cannot do what it promises**
- Where: CLI §3.10 "compares every emitted output byte for byte". The view model contains source
  file paths, which change on every conversion. A domain that only emits `view` (farm) has no data
  output to compare. The per-file target (`[Potion] keyed by id` from `load.dir`, "like Items/**")
  is a *keyed list*, and `entry` only exists for tables (§4.3).
- Proposed: the proof compares (a) the evaluated values structurally and (b) every emitted output
  except provenance fields (`values.sources`, findings locations). Allow `entry` for keyed lists
  (`entry potions.II_POT_HEAL_L { … }`, where the key field is implied and may be omitted).
  Printing omits fields equal to their defaults and uses canonical literals.

**CLI-03 · guess · Output formats of `test`, `explain`, `refs` and `--watch`**
- Proposed: freeze the samples in CLI.md as golden formats. For `--format json`, one object per
  line, with a documented schema.

**CLI-04 · nit · LSP**
- Proposed: UTF-16 positions. Diagnostics in JSON files are published even if the client has not
  opened them. The TextMate grammar is hand-written, not generated.

**CLI-05 · guess · Importing doc text during migration**
- Where: DECISIONS 8 says descriptions from the existing JSON Schemas are "imported during
  migration". No tool is named.
- Proposed: `canon infer --schema <file.schema.json>` copies `description` into `///`.

---

## 27. Non-functional requirements (NFR)

**NFR-01 · blocker · No performance targets**
- Where: nothing is stated, yet the scale is 6,945 item files (DECISIONS 1), 7,000 asset checks and
  7,000-row search indexes, and the studio edits interactively.
- Proposed:
  - Cold `canon check` of the whole law repo ≤ 10 s.
  - Warm (cached) ≤ 1 s.
  - Studio `Edit` of one entry in a 7,000-entry table (re-check plus write) ≤ 300 ms p95.
  - Memory ≤ 1.5 GB.
  - View model ≤ 5 MB per package, excluding the search index.
  - A benchmark fixture with 7,000 generated entry files is part of CI.

**NFR-02 · guess · Incremental architecture**
- Proposed: v0 invalidates at package level (a package and its dependents are re-checked). Design
  the value store so that value-level invalidation can come later. The `.canon/cache` format is
  versioned and opaque.

**NFR-03 · guess · Versioning policy**
- Proposed:
  - The language version is `MAJOR.MINOR`. A compiler accepts the minors it knows within its major
    and refuses newer ones (`E1001`).
  - The fingerprint algorithm has its own version (`canon-fp v1`). Changing it is a major compiler
    release.
  - The view model has `canon-vm/N`.
  - The lock format has `# canon.lock v1`.
  - `canon_runtime.h` and `rt` versions live in their namespace or package.

**NFR-04 · guess · Platforms**
- Proposed: the CLI runs on Linux, macOS and Windows. All written paths use `/`. Outputs always use
  `\n`. Path order is byte order of `/`-separated relative paths.

**NFR-05 · guess · Determinism tests**
- Proposed: CI builds every example twice (with `GOMAXPROCS` 1 and 8) and diffs the outputs. Never
  range over Go maps in output paths.

---

## 28. Organising parallel work (ORG)

**ORG-01 · blocker · No module boundaries or interfaces**
- Proposed Go module layout, with interfaces frozen first:

  | Package | Owns | Consumes |
  |---|---|---|
  | `syntax` (lexer, parser, CST with trivia, AST) | LEX, GRM | — |
  | `project` (project.canon, packages, file set, roots, paths) | §3, GRM-07 | syntax |
  | `types` (type representation, aliases, dependent types, assignability) | TYP, DEP | — |
  | `check` (resolver + bidirectional type checker) | RES, TYP | syntax, types |
  | `value` (immutable values, identity, provenance) | EVL-07 | types |
  | `eval` (interpreter, budget, freezing, stdlib) | EVL, STD | check, value |
  | `load` (json, csv, defines, text, dir, at, partial) | LOD, WIR (decode) | value, types |
  | `verify` (refinements, refs, keys, lock) | TYP-04, LCK | eval |
  | `rules` (checks, findings) | CHK | eval |
  | `diag` (codes registry, finding, render text/json) | DIAG | — |
  | `ir` (target-neutral emit IR: types, values, export fns, lookup tables, fingerprint) | EMT | verify |
  | `gen/json`, `gen/go`, `gen/cpp`, `gen/ts`, `gen/view` | WIR, GO, CPP, TS, VM | ir |
  | `conform` (vector selection + evaluation) | CNF | eval, ir |
  | `fmt` (printer over CST) | FMT | syntax |
  | `edit` (paths, ops, minimal writes) | API | fmt, value, load |
  | `api` (public Go API), `cmd/canon`, `lsp` | CLI, API | all |

  The contracts to write first: AST node set, `types.Type`, `value.Value` (with `Prov`),
  `diag.Finding`, `ir.Package`, and `edit.Path`. The codegen teams then work only against `ir`,
  from hand-built IR fixtures.

**ORG-02 · blocker · Examples cannot run: no fixture data**
- Where: every `@resource/...`, `@client/...` and `load.defines` path points outside the repo
  (defineObj.h, propItem.json, farm_config.json, EventConfig.json and more). Only `pipeline/data` is
  provided. There is no expected-findings file for any example except pipeline.
- Proposed: add `examples/_fixtures/{Resource,Client,Source}/…` with minimal files that exercise
  each example, point `project.canon` roots at them for tests, and add an `expected/findings.txt`
  (text format) per example.

**ORG-03 · guess · Features no example exercises**
- Where: `entry`/`@files`, `retired`, value-level `match`, `load.csv`, `load.text`,
  `@cpp(struct/access)`, `@codes` wire in JSON, `embedded` mode, TS goldens, lookup-table codegen
  (teamboard has no goldens), and `expect … warns`.
- Proposed: add one small example per feature, with goldens, before those modules start.

---

## Documents that should exist before development starts

1. **GRAMMAR.md (Appendix A v2)**, normative and machine-checkable (e.g. a tree-sitter grammar
   plus a test corpus). Must contain: the lexer modes (strings, interpolation, regex rule LEX-01/02,
   durations), the newline algorithm with the bracket stack (GRM-02/03), keyword-as-name rules
   (LEX-08), fixes for GRM-01/04/05/07/09/10/15/16/17, the `project.canon` grammar and schema, and
   a corpus where every example file parses to a checked-in AST dump.
2. **TYPES.md (type rules)**, with typing judgments for: assignability (entry/ref/T, optional
   unwrap, sized ints, refinements), join rules, contextual typing propagation (every position that
   supplies an expected type, including annotations and amend), brace-literal classification,
   equality/ordering, dependent types (static view, evaluation, restrictions), when refinements are
   checked, and match exhaustiveness.
3. **STDLIB.md**: every function and method with its generic signature, receivers (list, table,
   keyed list, map, string, Range), empty-collection results, error codes, ordering guarantees,
   step cost, and canonical text/format-spec rules (STD-06).
4. **WIRE.md**: JSON decode rules (LOD-02), canonical JSON encode bytes (WIR-01), none, duration,
   enum/@codes, map, table-row, variant, inline and `$` keys, snake-case algorithm, `path`/`unit`
   semantics, `load.defines`, csv, glob and `at:` grammar, and the `emit json` file layout. Must
   include worked byte-exact samples.
5. **FINGERPRINT.md**: the canonical shape serialization, what is included and excluded, versioning,
   and test vectors (shape text → hash).
6. **CODEGEN.md** (Go, C++17, TS), per target:
   - naming algorithm, initialism list, keyword escaping, collision errors;
   - file layout and names;
   - the exact public API of every construct (type, getters, containers, id enums per mode,
     variants, dependent unions, refs by case CG-03, optional containers, lookup-table fns, stores
     and snapshots, inputs, `types`-mode decoders);
   - cross-package imports/includes and the Go module mapping;
   - runtime helper files (`rt`, `canon_runtime.h`) in full;
   - legacy C++ `fields`/`both` generation, with a worked `ItemProp` example;
   - doc-comment and header templates;
   - toolchain floors.
   Regenerated goldens for pipeline and teamboard.
7. **CONFORMANCE.md**: the vector selection algorithm, checked-arithmetic helpers and error
   signalling per target, TS integer limits, float exactness requirements, and test file templates.
8. **VIEWMODEL.md + viewmodel.schema.json**: the full JSON Schema, type-encoding catalogue (VM-02),
   qualified naming, search index format and placement, `values`/`editable`, `usage` definition,
   control resolution, dependent-type encoding, i18n layout, versioning, and a regenerated
   potion.view.json.
9. **FORMATTER.md**: the layout rules (FMT-01), comment attachment, alignment policy, canonical JSON
   for sources (FMT-02), the idempotence requirement, and every example as a fixed point.
10. **ERRORS.md**: the complete diagnostic catalogue (code, severity, message template, trigger,
    minimal example). Range ownership per module. Finding JSON schema, including spans, stack and
    `check` name.
11. **API.md + `api/canon.go` stub**: exported Go types and signatures for Open/Check/ViewModel/
    Value/Refs/Edit/Evaluate/Watch/Build, the op value encoding, path grammar and semantics
    (API-02), minimal-write rules, revision/staleness, concurrency and error values.
12. **EVALUATION.md**: which values are evaluated, when record checks run, step cost model, poisoning,
    aliasing/copy-on-write, provenance format, finding order, layer application semantics.
13. **LOCK.md**: file grammar, sort order, what is locked, retire/rename/reuse rules, interactions
    with layers, edits and merges, and what `lock check` evaluates.
14. **I18N.md**: the complete key catalogue, template rules, summarised W1701, and the `i18n stub`
    output format.
15. **IMPLEMENTATION-PLAN.md**: the module map (ORG-01), frozen interfaces, milestones (v0 =
    taxonomy with Go baked + json, then pipeline data mode with C++/Go, then load + view, then fmt
    and edit, then LSP), the owner agent per module, integration test strategy, fixture tree
    (ORG-02), missing feature examples (ORG-03), performance benchmarks and targets (NFR-01), and
    the determinism CI (NFR-05).
16. **Errata to SPEC/CLI/DECISIONS/examples**, applying the accepted answers: GEN-02..07, the fixed
    example files (package dirs, emit roots, `load` annotations, keyword members or `icon: columns`,
    `@deprecated` line, TYP-01 fixes if the author picks static safety), and regenerated `expected/`.
