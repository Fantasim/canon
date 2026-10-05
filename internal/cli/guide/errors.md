# Errors

The loop that fixes them: `canon guide` (index).

## Reading a finding

```text
error[E3501]  shop/data/deals.json:2:14
  deals[0].item: unknown key bow in items
  expected by shop/shop.canon:12 (item: ref items)

error[E4102]  shop/shop.canon:19:33
  h: division by zero
  in half (shop/shop.canon:21)

error[E3002]  shop/shop.canon:30:17
  expected Int, found String

3 errors, 0 warnings in 1 package (3 ms)
```

- Header: `severity[code]`, then where the value is written: a `.canon` literal or a loaded file
  (JSON adds a `pointer`). A record check reports at the record (or its `at field`), a block
  check at the value given to `fail`/`warn`.
- `<value path>: <message>` is an evaluation finding about data: fix it with `canon edit` at
  `"<package>:<path>"`, or fix the rule. `expected by` is the rule (declaration or check) it was
  held to; `in <fn>` lines are the Canon stack.
- No path: a static finding (syntax, names, types, also in a literal). Fix the text by hand, then
  `canon fmt`. A declaration with a static error is not evaluated, so fixing it can reveal
  more findings.
- A failed value is poisoned: what reads it reports nothing more. Fix the first finding of a
  value, then check again.

## Common codes

| Code | Fix |
|---|---|
| `E1116` `E1117` | syntax; `E1117`: a `,` or newline between items |
| `E1125` `E1126` | reserved word as a name: rename; keep a wire name with `@json` |
| `E2102` | unknown name: typo, missing `import`, or a key the table does not have |
| `E2103` `E2106` | write `ref <collection>`; a name declared twice |
| `E3001` | annotate the public `let`: `let n: T = ...` |
| `E3002` `E3311` | wrong type; an Int where a Float is expected: `Float(i)` |
| `E3003` | no such field or method: typo, or a case field read without `match`/`is` |
| `E3008` | annotate; `.f > 1` is not a lambda: `x => x.f > 1` |
| `E3301` | unknown field: remove it, fix the wire name, or `load(..., partial: true)` |
| `E3302` | missing field: give it, or add a default to the field |
| `E3101` `E3102` | keys of tables, keyed lists and `@stable` values are unique |
| `E3201` | out of a sized integer or `Duration` range |
| `E3204` `E3205` `E3206` | range or length, regex, `where`: fix the value or the refinement |
| `E3315` | `null` in a required field: make it `T?`, or fix the data |
| `E3402` `E3403` | may be `none`: `if x != none`, `??`, `?.`, `!` |
| `E3501` | ref to a missing key: fix the key or add the entry |
| `E3024` | `past` on a type that is not an enum, a variant or a `ref`: `[past E]`, not `past [E]` |
| `E3502` `E3506` | a stored value refers to a retired entry, or uses a retired member: repoint it, or type the slot `past` |
| `E4001` `E4002` | `!` on `none`; missing key or index: `get(k)` gives `T?` |
| `E4101` `E4102` | overflow, division by zero: guard the operands |
| `E4401` | budget spent: the heaviest value is named; fix the loop or raise `budget` |
| `E5001` `E5002` | your check: fix the data, or the check |
| `E6001` `E6002` | a stable id removed or reused: put it back as `retired`, pick a new key |
| `E7002` | `load` needs an expected type: annotate the `let` |
| `E7103` `E7110` `E7111` `E7112` | JSON value of the wrong kind, member or case: fix the file or `@json` |
| `E1702` | translation key matches nothing: renamed field or misspelled key |
| `E9001` `E9008` | translated `export fn`: portable subset only; call the method from a `test` |

Warnings never block a build (`--max-warnings N` exits 4 past `N`): `W1001` stray doc comment, `W1002` missing doc comment, `W1003`
naming, `W3401` useless `?.`/`??`/`!`, `W1701` missing translations, `W5001`/`W5002` your warns.

## Edit and rename refusals

Printed on stderr as `canon: [op <n>: <path>: ]<reason>[: <detail>]`, with no JSON object.

| Reason | Exit | Fix |
|---|---|---|
| `sources changed since the base revision` | 1 | re-read the `revision` (`{"ops":[]}`), or omit `base` |
| `file is not in canonical layout` | 1 | `canon fmt`, `canon fmt --json-sources`, or `"normalize": true` |
| `value is not editable: computed` | 1 | edit what it is computed from (`canon explain` shows the origin) |
| `value is not editable: layered` | 1 | `--edit-layer <layer>`, or edit without that layer |
| `value is not editable: key` | 1 | keys change with the op `rename` |
| `value is not editable: pseudo` | 1 | `.id`, `.retired`, `.kind`: ops `rename`, `retire`, `setCase` |
| `value is not editable: format` | 1 | CSV, defines and text sources are edited by hand |
| `value is not editable: order` | 1 | order comes from file paths |
| `value is not editable: broken` | 1 | fix the package's errors before `canon rename` |
| `stable id cannot be removed, renamed or un-retired` | 1 | op `retire`; a new key for a new entry |
| `key already exists` | 1 | another key, or `set` |
| `rename would change what another name refers to` | 1 | another name |
| `value does not fit the type` | 2 | `value` is the wire form; `source` is a Canon literal |
| `no value at path` | 2 | check with `canon explain`; `addEntry` for a new key |
| `invalid path` | 2 | see the path syntax in `canon guide cli` |
| `ambiguous path` | 2 | qualify it: `pkg:path` |
| `operation not valid here` | 2 | op/container mismatch or unknown op member; members and cases cannot be renamed |
| `value could not be computed` | 1 | fix the findings printed with it |
