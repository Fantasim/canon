# Emberfall showcase: workarounds that can now be removed (2026-10-05)

For whoever maintains /home/louis/dev/canon-showcase (read-only for the compiler). Each item
names the COMPILER-ISSUES.md entry, the compiler change (branch `feat/past-and-ergonomics`, in
`main` once Louis merges and pushes) and what to do in the showcase. Rulings:
[decisions/log-2026-10-05.md](../decisions/log-2026-10-05.md), DECISIONS 309-316.

## Can be removed

| Issue | Change | In the showcase |
|---|---|---|
| #1 `ref in keyedList` false | DECISIONS 314: the operand's static type decides, exact key type first | `world.canon` Quest check: `z.spawns.any(s => s.monster == k.monster)` can be `k.monster in z.spawns` |
| #2 TS conformance order | DECISIONS 311: each parameter checked in turn, in the evaluator too | `game.core.mitigated`'s TS conformance test passes after a rebuild; nothing to change |
| #3 Rename of a keyed-list key | DECISIONS 310, 316: Rename cascades through keyed-list keys, layer paths included | Renaming `monsters.forest_wolf` works (API, `canon edit`, editor); the demo can go back to it |
| `@text` beside a data emit (E8013) | Bug fixed in ir: a `@text` fn is a file, not API (CODEGEN §2.9); verified through Go emits, the rule is shared by C++ and TS | `game.wiki` can move back into its package; rebuild all three targets to confirm |
| `MapField` (E8019) | DECISIONS 312: Go and C++ `data` loaders read map fields | Armour resistances can be `{Element: Int}` again instead of `[Resistance] keyed by element` |
| Project name and doc read from `project.canon` text | DECISIONS 313: `Project.Info(ctx)` | Use `Info`; stop parsing `project.canon` |
| One `Project` per language for `Check` | DECISIONS 313: `CheckWith(ctx, CheckRequest{Packages, Lang})` | One `Project` per layer is enough; pass the language per call |
| View model cached per revision by the editor | DECISIONS 313: kept per snapshot by the API | The editor cache can go |

## Stays (by decision)

- **#4 incomplete entry poisons its table** (DECISIONS 309): new values are sent complete. The
  editor's "ask the required fields first" flow is now the specified one (VIEWMODEL D3, T3). What
  changed: a poisoned table can be repaired through the API (Remove, Set of the broken entry's
  field, Undo), and the Undo of an incomplete `AddEntry` into a stable table is a `Remove`.
- **`ForeignDataRecord` / `CrossPackageBakedValue`** (E8019): lifted before v0.1, in a dedicated
  generator step (DECISIONS 312). `LevelRange` stays in `game.world` until then.
- **Intended, no change:** E8015 (wrap a computed list in a record), `Watch` reporting another
  `Project`'s writes as `external`, TS `.js` import specifiers, no generated `go.mod`, a view's
  `preview` asset must exist, `canon version` printing `unknown` for a build from a dirty tree.
