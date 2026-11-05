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
| `E1118` | an annotation where it is not allowed; a `@go`, `@ts` or `@cpp` annotation on a `local` declaration, or inside one: nothing generates a local, so remove the annotation, or drop `local` to emit it |
| `E1013` | a required root is not on this machine: clone it there, place it in `project.local.canon`, or list it in `optional_roots` |
| `E1014` `E1009` | `project.local.canon` holds more than a `roots` map of declared roots; `optional_roots` names a non-root |
| `E7009` `E3705` | a `load` or `asset` under an absent optional root: place the root, or check other packages |
| `W8024` `E8023` | outputs skipped under an absent optional root (build); `build --check` refuses them: CI needs the root |
| `E8022` `E8025` | a relative include or import between roots placed differently, or climbing above the project: emit a copy under the same root |
| `E1138` | a quoted word that is not one of the words that need it: write it without backticks (`` `return` `` is quoted, `` `kind` `` is `kind`) |
| `E1017` `E1018` | `tiers`: a name that is not an identifier, a path listed twice; a package no path places, or a path that places none |
| `E1019` | `out_templates` in `project.canon`: a target that is not an emit target, a placeholder other than `{dir}` and `{dotted}`, a template starting with `@`, a path with an empty, `.` or `..` segment, a character outside `A-Z a-z 0-9 . _ -` and `/`, or a `ts` template not ending in `.ts` (`canon guide emit`) |
| `E2007` | an import climbs `tiers`: import the same or an earlier tier, move the code down, or change `tiers` |
| `E1125` `E1126` | reserved word as a name: rename; keep a wire name with `@json` |
| `E2102` | unknown name: typo (also beside a literal: `weigth < 5`), missing `import`, or a key the table does not have |
| `E2103` `E2106` | write `ref <collection>`; a name declared twice |
| `E3001` | annotate the public `let`: `let n: T = ...` |
| `E3002` `E3311` | wrong type; an Int where a Float is expected: `Float(i)` |
| `E3003` | no such field or method: typo, or a case field read without `match`/`is` |
| `E3008` | type not inferable (only literals): annotate; `.f > 1` is not a lambda: `x => x.f > 1` |
| `E2109` | `it` outside a `where` predicate (never a key: write `"it"` for the key) |
| `E3301` | unknown field: remove it, fix the wire name, or `load(..., partial: true)` |
| `E3302` | missing field: give it, or add a default to the field |
| `E3101` `E3102` | keys of tables, keyed lists and `@stable` values are unique |
| `E3316` | an `@json` form on the wrong type or two fields on one wire key; `enumWire`, `caseWire`: two enum members or two cases on one wire, give one another wire (`= "..."`, `@json("...")`); `nul`: a wire name holding U+0000, write another character (`canon guide wire`) |
| `E3320` | an item not allowed in that literal: a spread in a table literal, so an `expect package` substitution writes every entry |
| `E3201` | out of a sized integer or `Duration` range |
| `E3204` `E3205` `E3206` | range or length, regex, `where`: fix the value or the refinement |
| `E3315` | `null` in a required field: make it `T?`, or fix the data |
| `E3402` `E3403` | may be `none`: `if x != none`, `??`, `?.`, `!` |
| `E3026` | a key tested with `in` on a table or keyed list: `xs.hasKey(k)` (a map: `k in m`) |
| `E3028` | a type after the first alternative of `A \| "lit"`: write a string literal, or a variant for a choice of types (`"x"?` and `["x"]` are types too) |
| `E3027` | a name in scope used as a key but not a value of the key type: write the key as a string (`"n"`), for an enum or union key type a member of it, or for an integer key type, sized or refined, a key of that type (`write a key of type Int`; where a `ref` is expected, only a literal: `write the key as a literal of type Int`) |
| `W3001` | `x: T? = none`: remove the `= none` (`canon fmt` does) |
| `W8031` | an entry position, or a number or text made from one, is written to an output: write the entry's key or a `@stable` field, or wrap it in `positional(…)` to write it on purpose |
| `W1912` | an amend path selects an entry by position (`[#n]`): select it by key |
| `W3002` | `E.members.first(m => m.name == s)` visits every member: write `E.named(s)` |
| `E8005` | two generated names collide: put `@go(name:)`, `@cpp(name:)` or `@ts(name:)` on one; `foreign…`: a C++ name that a package this one does not read writes as a `#define`, and the finding names the way out: `@cpp(name:)` on the declaration (`foreignDefine`), the Canon rename of the source of a derived name (`foreignDerived`), renaming a table id's entry (`foreignId`), or, always, a use of a type of that package in this one (`foreignFixed` has only this; `canon guide emit`) |
| `E8009` | an invalid emit option; `outTemplate`: a root name in `out` with no `out_templates` entry for this target, add one to `project.canon` or write the path as a string; `outQuoted`: a quoted `out` equal to the name of a root that has a template, write the name unquoted, or `"./<name>"` for the directory beside the file; `outChars`: a segment of the project-relative path an `out` resolves to (the root's own path, or the emitting file's directory for an unrooted `out`, included) uses a character outside `A-Z a-z 0-9 . _ -`: rename the directory (an `out` that does not resolve is `E7001` alone) |
| `E8001` | the build would overwrite a file it does not own: `canon build --adopt <path>` (text files, legacy C++ headers), or move it |
| `E8026` | a `load.dir` pattern (or a `load`) reads a file an emit of the build writes, before the output exists: narrow the pattern, or move the emit's `out` outside the load's base |
| `E8021` | a misplaced `@text` or a bad file name (`canon guide emit`); `source`: a name ending in `.canon` would be read as a source of the project, give it another extension |
| `E8019` | the emit asks for what no generator builds: the message names the way out. A `const` of an optional or a `ref` type in a go or cpp emit (`OptionalConstant`, `RefConstant`): make it a field of a record `let`; a `const` whose type holds a dependent type, in every code emit (`DependentConstant`): hold the value in a record field beside its discriminant, in a `let` of that record |
| `E3501` | ref to a missing key: fix the key or add the entry |
| `E3024` | `past` on a type that is not an enum, a variant or a `ref`: `[past E]`, not `past [E]` |
| `E3502` `E3506` | a stored value refers to a retired entry, or uses a retired member: repoint it, or type the slot `past` |
| `E4001` `E4002` | `!` on `none`; missing key or index: `get(k)` gives `T?` |
| `E4101` `E4102` | overflow, division by zero: guard the operands |
| `E4401` | budget spent (per package): the heaviest value is named; fix the loop or raise `budget` |
| `E5001` `E5002` | your check: fix the data, or the check |
| `E5004` | `expect … fails name` names no check that runs for its subject; a package check is tested with `expect package`, which the message says (`canon guide logic`) |
| `E5006` | `expect package` substitutes only a top-level `let` of the test's own package, once: a `const`, fn, type, test local or another package's let (`not a let`, `a let of another package`) or a let listed twice (`substituted twice`) is refused; an unknown name is `E2102` |
| `E6001` `E6002` | a stable id removed or reused: put it back as `retired`, pick a new key |
| `E6004` | a layer adds an entry to a stable table (`entry`) or changes a stable field (`field`): change the base source. `canon test` reports it when a test forces the amended table, and a repeated `@codes` code (`E3102`) always: a green `expect package passes` does not hide them (`canon guide logic`) |
| `E7002` | `load` needs an expected type: annotate the `let` |
| `E7008` | `load` path is not a string literal: one literal `load` per file chosen with `if`/`match`, or `load.dir` |
| `E7103` `E7110` `E7111` `E7112` | JSON value of the wrong kind, member or case: fix the file or `@json` |
| `E1702` | translation key matches nothing: renamed field or misspelled key |
| `E9001` `E9008` | translated `export fn`: portable subset only; call the method from a `test` |

A broken record or variant silences the values it types: one `W4001` at its declaration says "N
values were not evaluated or checked because T is broken: fix the errors in it or in the types it
names". Fix those first; "0 findings" on those values means "not checked".

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
