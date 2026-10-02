# CLI

`canon <command> [arguments] [flags]`: flags may follow arguments; `canon <command> -h` lists
them (exit 2). The project is found by searching `project.canon` upward. `--format json` prints
JSON Lines; `edit` and `rename` always do.

## Commands

| Command | Does |
|---|---|
| `canon init` | write `project.canon` and a `.gitignore` entry for `.canon/` |
| `canon new <package>` | create `<dir>/<last>.canon` with the `package` line |
| `canon check [packages]` | everything but writing; findings, then the summary |
| `canon build [packages]` | check, then write outputs and `canon.lock` |
| `canon test [packages]` | run `test` blocks |
| `canon fmt [paths]` | rewrite in canonical layout |
| `canon explain <path>` | value, type, origin of every part |
| `canon refs <path>` | every reference to an entry or enum member |
| `canon edit [request.json]` | apply one edit request (stdin without a file) |
| `canon rename <name> <new-name>` | rename a Canon name everywhere |
| `canon lsp` | language server for editors (read-only) |
| `canon version` | compiler, language and format versions |
| `canon guide [topic]` | this guide |

Packages: none (all), `shop.items`, `shop...` (and below), `./shop/items`, `shop/items/x.canon`.

## Value paths

```text
items.axe.price                     table entry by key (or items[axe]), then a field
ranks[captain].title                keyed list: [k] is always a key
ranks[#0]                           keyed list or table by position (never printed)
potions[0].heal                     plain list: [n] is a position
weights[FIRE]                       enum-keyed map, member name; ["fire"] is its wire value
labels["a b"]                       map key that is not an identifier
shop:items.axe                      package-qualified: needed when the name is ambiguous
Element.FIRE                        an enum member
```

Paths start at a public `let` or `const` (`pkg:` also reaches `local` ones). Quote them in a
shell when they contain `[`: `canon explain 'potions[0]'`.

## check: findings

One object per finding, then the summary (text form: `canon guide errors`):

```json
{"severity":"error","code":"E3501","file":"shop/data/deals.json","line":2,"col":14,"endLine":2,"endCol":19,"pointer":"/0/itemId","package":"shop","path":"deals[0].item","message":"unknown key bow in items","related":[{"file":"shop/shop.canon","line":12,"col":3,"endLine":12,"endCol":18,"note":"item: ref items"}]}
{"severity":"error","code":"E4102","file":"shop/shop.canon","line":19,"col":33,"endLine":19,"endCol":39,"package":"shop","path":"h","message":"division by zero","stack":[{"fn":"half","file":"shop/shop.canon","line":21,"col":14,"endLine":21,"endCol":21}]}
{"severity":"error","code":"E3002","file":"shop/shop.canon","line":30,"col":17,"endLine":30,"endCol":20,"package":"shop","message":"expected Int, found String"}
{"summary":{"errors":3,"warnings":0,"packages":1,"ms":4}}
```

Keys, in order: `severity code file line col endLine endCol pointer package path message check
layer related stack moreFrames reads`; empty ones omitted. Columns are 1-based UTF-8 bytes,
`endCol` exclusive. `pointer`: JSON pointer in a loaded file; `path`: the value (absent on a
static finding); `related`: the declaration or check it was held to; `check`: a named check;
`layer`: the layer that set it; `stack`: Canon stack, innermost first; `reads`: fields a record
check read. Past 1,000 findings in a package the summary gains `"truncated":[{"package":"p",
"errors":1,"warnings":0}]`.

## explain

```text
config.server.port = 9000  Int(1024..=65535)
  set by  app/staging.layer.canon:6  layer staging
  default  app/app.canon:9  8080
```

One origin per line, newest first: `set by` (a layer), `literal`, `default`, `loaded`
(`#/pointer`, `row n`), `spread`, `computed` (with `in <fn>` frames). Parts follow, every level
unless `--depth n`. JSON is one line, no summary:

```json
{"explain":{"path":"shop:limits","type":"shop.Limits","text":"Limits{players: 100, queue: 10}","value":{"nPlayers":100,"nQueue":10},"origin":{"kind":"json","file":"shop/data/limits.json","line":1,"col":1,"endLine":1,"endCol":18,"text":"Limits{players: 100, queue: 10}"},"parts":[{"path":"shop:limits.queue","type":"Int","text":"10","value":10,"origin":{"kind":"default","file":"shop/shop.canon","line":34,"col":16,"endLine":34,"endCol":18,"via":{"kind":"json","file":"shop/data/limits.json","line":1,"col":1,"endLine":1,"endCol":18},"text":"10"}}]}}
```

`text` is Canon text, `value` the wire JSON. `origin.kind`: `literal json csv defines text
default spread computed layer`; also `pointer`, `layer`, `via` (the literal or object a default
or spread came through), `stack`, `replaced` (the origin a layer replaced).

## refs

```json
{"ref":{"kind":"value","package":"shop","path":"deals[0]","file":"shop/shop.canon","line":27,"col":27}}
{"ref":{"kind":"code","package":"shop","file":"shop/shop.canon","line":40,"col":9}}
{"summary":{"target":"shop:items.axe","count":2}}
```

`kind`: `value` (a ref holding the key), `key` (a map key), `code`, `view`, `check`, `layer`;
`path` only for `value` and `key`. Target: a table entry, keyed-list element or enum member.

## edit

```json
{
  "base": "r1:5f0c...",
  "dryRun": false,
  "allowErrors": false,
  "normalize": false,
  "ops": [
    {"op": "set", "path": "items.axe.price", "value": 130},
    {"op": "addEntry", "path": "items", "key": "mace", "source": "{ name: \"Mace\", kind: weapon }"},
    {"op": "move", "path": "ranks[major]", "index": 1},
    {"op": "setCase", "path": "quests.hunt.reward", "case": "gold", "source": "{ amount: 5 }"},
    {"op": "set", "path": "limits.motd", "value": null}
  ]
}
```

| Op | Members | On |
|---|---|---|
| `set` | `path`, `value` or `source` | any editable value; a value equal to the default removes the field |
| `reset` | `path` | a field with a default: removed from its literal or JSON object |
| `add` | `path`, `value`/`source` | append to a list or keyed list |
| `insert` | `path`, `index`, `value`/`source` | insert at a position |
| `addEntry` | `path`, `key`, `value`/`source` | table or map; a table with entry files gets a new file (`@files`) |
| `remove` | `path` | list element, map entry, entry of a non-stable table |
| `move` | `path`, `index` | reorder |
| `rename` | `path`, `key` | key of a non-stable table entry, keyed-list element or map entry; refs follow |
| `retire` | `path` | stable table entry or `@codes` member; writes the lock line |
| `unretire` | `path` | always refused (`ErrStableKey`) |
| `setCase` | `path`, `case`, `value`/`source` optional | variant case; same-name same-type fields kept, the rest in `dropped` |
| `renameName` | `path` (the name), `name` | as `canon rename` |

- `value` is the wire form (JSON as in data files, `@json` names, `null` is `none`); `source` is
  Canon literal text (Canon names, bare members). Never both.
- `base`: the `revision` of the last output; files changed since refuse the edit (`ErrStale`).
  `{"ops":[]}` writes nothing and prints the current `revision`. Omit `base` to skip the check.
- Ops apply in order, all or none. The result is checked: an error refuses it unless
  `allowErrors`. `dryRun` writes nothing. A file not in canonical layout is refused unless
  `normalize` (or run `canon fmt`, `canon fmt --json-sources` first).
- `"editLayer": "<layer>"` (or `--edit-layer`) writes `set`, `reset`, `addEntry` as amendments in
  that layer's file (`canon guide layers`).

Three output shapes:

1. Done (exit 0): the `edit` object, then the findings and the summary. `applied` false with
   exit 0 means `dryRun` or nothing to change.
2. Refused for its findings (`ErrRejected`, exit 1): `applied` false, empty `changes` and
   `dropped`, no `undo`, then the findings. A poisoned value or a broken declaration on the way:
   no `edit` object, only its findings and the summary, exit 1.
3. Any other refusal: no stdout, one stderr line `canon: [op <n>: <path>: ]<reason>`
   (`canon guide errors`).

```json
{"edit":{"applied":true,"revision":"r1:b3ab...","changes":[{"kind":"created","path":"shop/items/weapon/mace.canon"},{"kind":"modified","path":"shop/shop.canon"}],"dropped":[],"undo":{"base":"r1:b3ab...","ops":[{"op":"remove","path":"shop:items.mace"},{"op":"set","path":"shop:items.axe.price","source":"120"}]}}}
{"summary":{"errors":0,"warnings":0,"packages":1,"ms":13}}
```

`changes[].kind`: `modified`, `created`, `deleted`, `renamed` (with `oldPath`). `dropped`:
values removed by cascades, `{"path", "value"}`. `undo` is a complete request (with `editLayer`
when the edit had one): save it, feed it to `canon edit` to revert the values. Exception: an
entry added to a stable table is permanent, so that undo (a `remove`) is refused.

## rename

`<name>`: `pkg:Type`, `pkg:Type.field`, `pkg:Variant.case.field`, `pkg:Type.method`, `pkg:fn`,
`pkg:fn.param`, `pkg:fn.local`, `pkg:let`, `CONST`, or `file:line:col` of an identifier; `pkg:`
may be dropped when the first name is public and unique. Every use follows: qualified uses,
imports, literal field names in data and entry files, named arguments, views, translation keys,
layer paths, `emit values:`, `@files` variables. A loaded or emitted field gains
`@json("<old wire name>")`, so data never changes. Comments, strings, file names, data files and
`canon.lock` are untouched. Refused: enum members and variant cases (no op renames them in this
version), entry keys (edit op `rename`; `ErrStableKey` when stable); a stable table's `let`, a
`@codes` enum, a `@stable` field (`ErrStableKey`); a package with a broken declaration (reason
`broken`); a collision (findings); a capture of another name (`ErrNameClash`). Output: a
`{"rename":{...}}` object with the keys of `edit`; its `undo` is a `renameName` request.

## build and test JSON

```json
{"output":{"path":"@gen/data/products.json","target":"json","package":"store","status":"written"}}
{"lock":{"package":"shop","file":"shop/canon.lock","line":"table  shop.items  bow"}}
{"summary":{"errors":0,"warnings":0,"packages":1,"ms":14,"written":2,"stale":0}}
{"test":{"package":"calc","name":"band boundaries","file":"calc/calc.canon","line":56,"status":"fail","failures":[{"file":"calc/calc.canon","line":57,"col":3,"expect":"expect band(19) == mid","expected":"mid","got":"low","findings":[]}]}}
{"summary":{"passed":1,"failed":1,"ms":3}}
```

Output `status`: `written`, `unchanged`, `stale`, `adopted`. `fmt` prints
`{"file":...,"formatted":false}` per file, then `{"summary":{"files":n,"unformatted":k}}`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | an error finding; a failing test; `--check` found something out of date; an edit refused (`ErrRejected`, `ErrStale`, `ErrNotEditable`, `ErrStableKey`, `ErrKeyExists`, `ErrNotCanonical`, `ErrNameClash`) or a value that could not be computed (`ErrNoValue`) |
| 2 | usage: bad flag or argument, unknown package or layer, no `project.canon`, a bad path or op (`ErrBadPath`, `ErrNoPath`, `ErrAmbiguousPath`, `ErrBadOp`, `ErrBadValue`), a target with no generator |
| 3 | internal compiler error (`ErrInternal`): report it |
| 4 | more warnings than `--max-warnings <n>` |
| 130 | interrupted; writes finished or rolled back |
