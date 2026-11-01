# v0.2 CLEANUP: the owed list (ADR-0023 §11.5)

Collected from [log-2026-10-31](../decisions/log-2026-10-31.md), where each entry says why it is owed.
CLEANUP may not change behaviour: a golden diff, or a changed message byte, is a bug. Each item
names the packages it touches. One shared helper replaces each duplicate, and the change must not
add an import cycle.

## Duplicate helpers

1. **T19 bound wording.** `boundSubject`, `sideText` and `stepText` in gen/cpp, gen/go and gen/ts
   (gen/ts spells its constants differently for the audit's `const-dup`) become one `internal/ir`
   helper. The bytes must stay identical.
2. **The C++ and TS doc block form.** gen/cpp `docBlock` and gen/ts `docComment` share the star-line
   loop and the `*/` escape. Share the loop only if a neutral home exists; otherwise add a parity
   test.
3. **`fencedCpp` and `fenced`.** `internal/gen/cpp/runtime/spec_test.go` repeats
   `internal/gen/ts/spec_test.go`'s fenced-block reader (test code). Use a testkit helper.
4. **Branch-walk mirrors.**
   - `internal/edit/applysettype.go` mirrors the checker's `branchWalk`, `depFunc`, `dependentList`
     and `braceBranch` tests (about 25 lines).
   - EDIT's `firstBranch` mirrors `branchWalk`.
   - Give each one shared helper, or a parity test where sharing would cross a layering rule.
5. **`linkOf` and `resolvedAs`.** build's `linkOf` repeats load's `resolvedAs`. Make one helper in
   `internal/project`.
6. **`jsonHooksOrigin`.** gen/cpp repeats ir's `tablesOwnToWire() || hasPositionTypes()`. Export one
   predicate from the plan.
7. **`floatBound` and `floatLit`.** gen/go's `floatBound` (rangecheck.go) is `literal.go`'s
   `floatLit(f, 64)` except for `-0`. Share them, keeping the `-0` case; document it.
8. **The out resolution in check.** `refine.go` still cuts `@root/` itself at two sites, next to
   `resolveOut` (emitpath.go). Use one path. `project.cutRoot` stays unexported unless a second
   package needs it.

## Dead code and tidying

9. `cppPlanned`'s and `goPlanned`'s unused `_ *ir.Emit` parameters.
10. gen/cpp `idEnums` writes the deleted nlohmann pair inline instead of through `deletedJSON`. The
    `enum`/`branchEnums` doc comments omit the pair.
11. gen/go `operators.go` holds more than operators: rename it `constants_tables.go`, as
    `internal/ir` does.
12. gen/go `rangecheck.go` uses two failure-text escaping styles (`errAt` unescaped, `rowsRange`
    escaped). Pick one, with the bytes unchanged.
13. Drop "(pending)" from the "DECISIONS 423 (pending)" citations in gen/cpp (423 is written).
14. `internal/build/recheckbudget_test.go:18-19`: the `priced` comment ("defaults after, as stage E
    does") predates the phase-2 default folds. Reword it.
15. `foldedDefault` (internal/check) does not break its record when its fold fails silently by
    reading an already-failed const. Document why, since the output equals HEAD. Do not change the
    behaviour.
16. `scopeOrigin` (internal/ir `cppforeign.go`) decides "already qualified" by a name lookup. Taking
    the package from the item is sturdier. The message bytes must not change on any test.
17. `internal/gen/ts/range_cases_test.go` empty-key comment: check that it is true now (EMPTY-KEY
    landed).
18. Every `make audit` `dup-in-repo` finding the run's units added (compare against the v0.1.13
    baseline).

## Not CLEANUP's

- Spec wording owed: the release spec pass, by the orchestrator.
- Behaviour changes, which go to a unit.
