# Canon stable ids and `canon.lock`

Status: **normative** companion of [SPEC.md](../SPEC.md) v0.1 (§12). Defines what is locked, the
`canon.lock` file, the rules that protect stable ids, when the lock is written, and what
`canon lock check` does.

Owned AUDIT items: LCK-01..05, NFR-03 (lock versioning).

Related documents: [WIRE.md](WIRE.md) (retired entries on the wire, output newlines), TYPES.md
(`E3101`, `E3102`, `E3502` and whether a live value may name a retired enum member), EVALUATION.md
(build phases), API.md (the edit operations `Add`, `Remove`, `Retire`), CLI.md (`canon build`,
`canon lock check`).

---

## 1. What is locked (LCK-02)

Three kinds of identifiers are a permanent contract. A package that declares any of them has a
`canon.lock` (§2).

| Kind | Declared by | A locked value | Its holder |
|---|---|---|---|
| `table` | `let t: stable table T = …` (public or `local`) | an entry key | (the key itself) |
| `enum` | `enum E @codes(…) { … }` (public or `local`) | a code | the member name |
| `field` | a field `f: … @stable` of the element record of a stable table | a value of `f` | the entry key |

Placement rules:

- `stable table` may appear only as the whole declared type of a top-level `let` (TYPES.md §9.3).
- `@stable` is allowed only on a field of a record that is the element type of at least one stable
  table. Its values are locked separately for each stable table of that element type.
- A `@stable` field's type must be an integer type or `String` (aliases and refinements allowed), and
  not optional.

Breaking any of these three placement rules is `E6003`, at the type or the field.

A `@stable` field's values must be unique among all entries of the table, retired entries included
(`E3102`, TYPES.md).

Nothing else is locked: variant cases, plain enums, keyed lists and plain tables may change freely.
`load.defines` tables are not stable.

---

## 2. The file

### 2.1 Location and encoding

- The lock of package `a.b` is `a/b/canon.lock` under the project root: the directory that mirrors
  the package path (`teamboard/canon.lock`, `resource/vocab/canon.lock`,
  `balance/parity/canon.lock`). No other `canon.lock` file is read.
- It is committed. It is written by `canon build` and by the edit API (§5), never by hand except as
  described in §4.6.
- UTF-8, LF line endings, one final LF (WIRE.md §11).

### 2.2 Grammar (LCK-01)

```
lockFile  = header LF { fact LF } ;
header    = "# canon.lock v1" ;
fact      = tableFact | enumFact | fieldFact ;
tableFact = "table" SEP name SEP key [ SEP "retired" ] ;
enumFact  = "enum " SEP name SEP integer SEP member [ SEP "retired" ] ;
fieldFact = "field" SEP name "." fieldName SEP value SEP key ;
SEP       = "  " ;                                   (* exactly two spaces when written *)
name      = package "." identifier ;                 (* the table's let, or the enum *)
value     = integer | jsonString ;
key       = identifier ;  member = identifier ;  fieldName = identifier ;
integer   = [ "-" ] ( "0" | nonZeroDigit { digit } ) ;
```

- The kind is left-aligned and padded with spaces to 5 characters (`enum` becomes `enum `), then
  followed by two spaces. Every other separator is two spaces.
- `package` is the lock's own package, fully qualified. For a `field` fact the name is the
  **table's** qualified `let` name followed by the field's Canon name
  (`resource.vocab.eventTypes.code`), because the same record may be the element of several
  stable tables.
- Integer values are canonical decimal. String values of `@stable String` fields are always
  JSON-quoted (WIRE.md §7.3), so `"12"` and `12` never look alike.
- A retired table entry or enum member carries the word `retired` at the end of its line. A `field`
  fact has no retirement of its own: the entry's `table` fact carries it.

### 2.3 Canonical order and writing

The writer emits the header, then one line per locked value, sorted by:

1. kind, as bytes (`enum` < `field` < `table`);
2. name, as bytes;
3. value: integers by numeric value (`2` < `10`), strings and keys by their UTF-8 bytes;
4. holder (member or key), as bytes.

A value appears on exactly one line; retirement is folded into it. The file is rewritten in full,
and only when its bytes change.

### 2.4 Reading

Reading is tolerant, so that merges (§6.3) never lose information:

- CR LF is accepted and read as LF. Blank lines are ignored. Separators may be any run of spaces.
- The first non-blank line must be exactly `# canon.lock v1` [`E6005`]; another version is
  refused with `E6005` (`canon.lock: version 2 is newer than this canon`).
- Line order does not matter; the file is read as a **set of facts**. Identical facts are merged. A
  fact with `retired` and the same fact without it mean "retired". A merged fact keeps its earliest
  line, which the findings about it cite.
- A line that does not parse, a kind that is not `table`, `enum` or `field`, a name outside the
  lock's package, or a git conflict marker (`<<<<<<<`, `=======`, `>>>>>>>`) is `E6005`, at that
  line.

A lock that differs from its canonical form (unsorted, CR LF, duplicates) is valid; the next
`canon build` rewrites it canonically, and `canon build --check` reports that it would change.

---

## 3. Facts in the sources

During verification (EVALUATION.md §1: stage B, phase 4), the compiler computes the **current
facts** of each package:

- for each stable table `t`: one `table` fact per entry (live or retired), with `retired` when the
  entry is written `retired`;
- for each `@codes` enum `E`: one `enum` fact per member, with its code and `retired` flag;
- for each `@stable` field `f` of the element of stable table `t`: one `field` fact per entry of `t`
  (retired included), with the entry's value.

Entries count whatever their origin: the table literal, `entry` files, `load`, `load.dir`, or a
computed expression.

---

## 4. Rules

The **lock facts** L are compared with the **current facts** S. Every rule below is checked, and
every violation reported (the build never stops at the first).

### 4.1 Nothing is ever removed [`E6001`]

| Situation | Finding |
|---|---|
| L has `table t k`, S has no entry `k` in `t` | `E6001` at the lock line: entry removed or renamed; retire it instead |
| L has `enum E c m`, S has no member `m` in `E` | `E6001`: member removed or renamed |
| L names a table, enum or field that S no longer has at all (deleted, renamed, or no longer `stable` / `@codes` / `@stable`) | one `E6001` for the whole collection, at its first lock line (not one per value) |
| an edit-API `Remove` on a stable entry | refused with `E6001` |

When the locked value is now held by a new holder (a table with exactly one new key; a code or a
`@stable` value now held by a member or entry that is not in L), the message says so (variants
`renamed` and `held` of `E6001`, ERRORS.md): `teamboard.statuses.wont_do was removed; rejected is
new: renaming a stable id is not allowed.` This is a rename (SPEC §12), and it is reported only as
this `E6001` (4.2 does not report it again).

### 4.2 Nothing is ever reused, moved or changed [`E6002`]

| Situation | Finding |
|---|---|
| L has `table t k retired`, S has `k` live | `E6002` at the entry: a retired id cannot come back |
| L has `enum E c m retired`, S has `m` live | `E6002` at the member |
| L has `enum E c m`, S has `m` with code `c2 ≠ c` | `E6002` at the member: renumbered |
| L has `enum E c m`, S has another member `m2` with code `c`, and `m` still exists in S | `E6002` at `m2`: code `c` belongs to `m` (retired or not) |
| L has `field t.f v k`, S has entry `k` with `f = v2 ≠ v` | `E6002` at the value: a stable value never changes |
| L has `field t.f v k`, S has another entry `k2` with `f = v`, and `k` still exists in S | `E6002` at `k2`'s value: `v` belongs to `k` |
| L itself holds two facts for one value (`enum E c m1` and `enum E c m2`; `field t.f v k1` and `field t.f v k2`), or two values for one holder (`enum E c1 m` and `enum E c2 m`; `field t.f v1 k` and `field t.f v2 k`) | `E6002` at the second lock line: conflicting lock, usually after a merge (§6.3) |

### 4.3 Retiring

Retiring is the only way to take an id out of use:

- a table entry: `retired open { … }` in a literal, `retired entry statuses.open { … }` in an entry
  file, `"$retired": true` in a loaded source (WIRE.md §5.7), or the edit API's `Retire`;
- an enum member: `retired FIRE = 1`.

A retired entry or member stays in the sources forever, keeps its locked values, and is checked
like any other (its refinements, refs and checks still apply). Retirement is one-way: removing
`retired` from an entry or member whose lock line says `retired` is `E6002` (§4.2). The only way
back is the reviewed hand edit of §4.6; the edit API's `Unretire` is always refused
(`ErrStableKey`, API.md E4). A reference from a live entry to a
retired one is `E3502` (TYPES.md); from a retired entry to anything, it is allowed. Using a retired
enum member or variant case in a value, in Canon source or loaded data, is `E3506` (TYPES.md);
the decoder still recognises its wire value (it is not `E7111`), so the finding names it.

### 4.4 Adding

A value in S that is not in L is new. It is not an error: `canon build` appends it (§5). New values
must still respect 4.2 (a new member may not take a locked code, a new entry may not take a
locked `@stable` value).

### 4.5 Order of checks

Lock rules are checked during verification (EVALUATION.md stage B), together with keys (`E3101`,
`E3102`) and refs. A lock file with `E6005` is not compared at all (one finding per bad line,
nothing else from that lock).

### 4.6 Hand edits

The lock is append-only for tools. People may edit it in a reviewed commit for two reasons only:

- **renaming a collection or a package**: rewrite the qualified names of its lines (a stable table
  `let` renamed from `statuses` to `postStatuses`, or package `teamboard` moved), in the same commit
  as the rename;
- **deliberately forgetting**: deleting lines to release ids or codes, or deleting the word
  `retired` from a line to bring an id back (with the `retired` prefix removed from the source in
  the same commit), a decision the diff makes visible. The edit API's `Unretire` is therefore
  refused (API.md: `ErrStableKey`).

`canon` cannot tell a hand edit from a tool edit; the review of `canon.lock` diffs is the safeguard.

---

## 5. When the lock is written

| Operation | Lock |
|---|---|
| `canon check`, `canon test`, `canon lock check` | read only |
| `canon build` with no error in any loaded package, selected or imported (any `--target` selection) | new facts appended and retirements recorded, for each **selected** package; written only if the bytes change, atomically with the other outputs |
| `canon build` with an error in any loaded package | not written (a failed build leaves every output untouched) |
| `canon build --check` | not written; exit 1 if any selected lock would change (CLI.md §3.4) |
| `canon build --layer …` | never written (§6.1); the lock rules are checked as in a plain build |
| edit API `Add` on a stable table | the new `table` (and `field`) facts are written in the same atomic edit as the source |
| edit API `Retire` | the fact gets `retired` in the same atomic edit |
| edit API `Remove` on a stable entry | refused (`E6001`) |
| edit API `Set` of a `@stable` field of an existing entry | refused (`E6002`) |
| edit API `Set` or `Reset` on a root stable-table entry the lock does not hold yet | the entry's whole facts are written in the same atomic edit |
| any other edit | never; an edit writes only the lines its own ops require (API.md E20) |
| `canon fmt`, `canon convert` | never; `convert` moves entries without changing ids |

A package whose lock would be empty has no lock file. A lock is never deleted by `canon`.

---

## 6. Layers, edits and merges (LCK-04)

### 6.1 Layers

A layer changes values, never ids. An amendment that adds an entry to a stable table, or changes a
`@stable` field, is `E6004` at the amendment. Layers never write the lock, and a layered build
checks the same lock rules as a plain one.

### 6.2 Edits

The edit API keeps the lock in step with the sources inside one atomic edit (§5), so a studio user
never has to run `canon build` to lock a new entry. A `Retire` of an entry referenced by live
entries fails with `E3502` like any other edit that breaks a check (unless `AllowErrors`).

An edit with `AllowErrors` still locks: an id its ops add or retire gets its lines whenever its
own entry evaluates, whatever errors the rest of the package holds. Its lines are written only
whole, and only when none of its facts conflicts with the lock as read (§4.2, §4.4; §4.6's
append-only rule) or duplicates another fact the post-edit sources would lock (a value locked for
a duplicated holder would stay wrong for good); otherwise the id is skipped and its `E6002` or
`E3102` finding stays. Ids one edit adds that conflict among themselves are all skipped. Facts the
lock already holds are not tested again: a `Retire` of a locked id records `retired` even when an
unlocked holder duplicates its value (§4.3: retirement is one-way). API.md E20 has the details.

### 6.3 Merges

- Two branches that each add different entries add different lines: git merges them unless they
  are adjacent, and a textual conflict inside `canon.lock` is `E6005` until resolved.
- Recommended: `canon.lock merge=union` in `.gitattributes`. A union merge keeps every line of both
  sides; since the reader treats the file as a set (§2.4), this is always safe, and the next build
  rewrites it canonically.
- Two branches that give the **same new value** to different holders (two new event types both
  with `code: 9000`; two new members both `= 7`) produce two lock facts for one value after the
  merge: `E6002` (4.2, last row). The fix is to renumber the newer one in the source and delete its
  line from the lock.
- Two branches that add the **same new key** to a stable table with different contents merge into
  one lock line but two source entries: `E3101` (duplicate key) in the sources.

---

## 7. Retired entries in outputs (LCK-05)

Retired entries and members are part of every output; only `.active()` leaves them out.

| Output | Retired entries |
|---|---|
| JSON data files | written, with `"$retired": true` (WIRE.md §5.7, §8.3) |
| generated code, `baked` and `embedded` | in the table, in its iteration, found by `Find`, flagged retired (CODEGEN.md names the getter); still members of the id enum, so stored data decodes |
| generated code, `data` mode | loaded from the data file like live ones, flagged retired |
| generated enums (`@codes`) | retired members stay, with their code |
| lookup tables of finite `export fn`s | retired members and entries are part of the domain (CG-08) |
| view model | flagged `retired`; search rows flagged; never offered in pickers (MOCKUP-GAPS 21) |
| Canon expressions | `for`, `len()`, `keys()`, `first()`, `t.k` see them; `.active()` does not; `.retired` is `true` |

---

## 8. `canon lock check` (LCK-03)

```
canon lock check [packages…]
```

1. Parse and type-check the selected packages and their imports (phases 1 and 2).
2. Evaluate only what the current facts need: each stable table's entry keys and retired flags,
   each `@stable` field value of their entries, and whatever those depend on, `load`s included.
   `@codes` enums need no evaluation. A stable table is forced whole (every field of its entries:
   evaluation cannot compute part of a table). No other `let` or table is forced, no `check` runs,
   and nothing is verified beyond the facts themselves: this step wins over §4.5, which is about
   `canon build`.
3. Read each selected package's lock and apply §2.4 and §4.
4. Report `W6006` per package when S has values that L lacks: "3 values are not locked yet: run
   `canon build`". Only `lock check` reports it (so a pre-commit hook catches a forgotten build).
   For a package that has no `canon.lock` yet, `W6006` has no location.

Findings: `E6001`, `E6002`, `E6003`, `E6004` (layers given with `--layer`), `E6005`, `W6006`, and
any parse, type or evaluation error met while computing the facts. Exit codes as CLI.md §2.5.

It is fast when stable tables are literals; a stable table loaded from many files costs that load.
CLI.md §6.3's "neither evaluates values" becomes "evaluates only stable collections".

---

## 9. Worked example: `examples/teamboard/taxonomy.canon`

Package `teamboard` has seven stable tables and no `@codes` enum or `@stable` field.

### 9.1 First build

`canon build teamboard` creates `teamboard/canon.lock`, exactly these 1 108 bytes (SHA-256
`583148b7566db8242c749304d6d385102b35ae94956ca79d4992034ef1c192e3`):

```
# canon.lock v1
table  teamboard.areaGroups  in_game
table  teamboard.areaGroups  internal
table  teamboard.areaGroups  out_game
table  teamboard.areas  admin_panel
table  teamboard.areas  analytics
table  teamboard.areas  balance
table  teamboard.areas  client
table  teamboard.areas  game
table  teamboard.areas  game_server
table  teamboard.areas  internal
table  teamboard.areas  resource
table  teamboard.areas  resource_studio
table  teamboard.areas  sovcommon
table  teamboard.areas  team_board
table  teamboard.areas  website
table  teamboard.columns  done
table  teamboard.columns  fixed
table  teamboard.columns  taken
table  teamboard.columns  unclaimed
table  teamboard.flags  blocking
table  teamboard.flags  sensitive
table  teamboard.intents  idea
table  teamboard.intents  issue
table  teamboard.severities  critical
table  teamboard.severities  low
table  teamboard.severities  normal
table  teamboard.statuses  duplicate
table  teamboard.statuses  fixed
table  teamboard.statuses  open
table  teamboard.statuses  taken
table  teamboard.statuses  verified
table  teamboard.statuses  wont_do
```

`areaGroups` sorts before `areas` (`G` is 0x47, `s` is 0x73); keys sort by bytes, not by entry
order (`game` < `game_server`).

### 9.2 Adding a status

Add `blocked { tone: warning, label: "Blocked", terminal: false, next: [open] }` to `statuses`
(and to a column, or the package check reports it unclaimed). `canon lock check` reports
`W6006` (1 value not locked yet). `canon build` inserts one line:

```diff
 table  teamboard.severities  normal
+table  teamboard.statuses  blocked
 table  teamboard.statuses  duplicate
```

### 9.3 Renaming a status

Renaming `wont_do` to `rejected` in the table (and in every `next`):

```
error[E6001]  teamboard/canon.lock:33:1
  teamboard.statuses.wont_do was removed; rejected is new: renaming a stable id is not allowed.
  Keep wont_do and retire it: retired wont_do { … }
```

The label can change freely (`label: "Rejected"`): only the key is the contract.

### 9.4 Retiring a status

Write `retired duplicate { … }` and remove `duplicate` from the live entries that reference it:
`open.next`, `taken.next` and column `done.statuses` (otherwise each is `E3502`). `canon build`
rewrites its line:

```diff
-table  teamboard.statuses  duplicate
+table  teamboard.statuses  duplicate  retired
```

Generated Go still has `StatusDuplicate` in the id enum; `statuses.active()` no longer yields it;
the package check `for s in statuses.active()` no longer requires a column for it.

### 9.5 Deleting or reviving a retired status

- Deleting the retired `duplicate` entry later: `E6001` (a retired entry stays forever).
- Removing the `retired` prefix to bring it back: `E6002` at the entry (a retired id cannot come
  back; add a new status instead).

### 9.6 Codes and stable fields: `examples/resource/vocab/vocab.canon`

`resource/vocab/canon.lock` after the first build (3 995 bytes, SHA-256
`f71a7d091f356e308652875b83be336cc95ca8cd247cff949c7a6a30bfaea98a`). Note the numeric order of
codes (`5` < `10` < `1000`) and the field fact named after the table `eventTypes`:

```
# canon.lock v1
enum   resource.vocab.Element  1  FIRE
enum   resource.vocab.Element  2  WATER
enum   resource.vocab.Element  3  ELECTRICITY
enum   resource.vocab.Element  4  WIND
enum   resource.vocab.Element  5  EARTH
enum   resource.vocab.QuestStyle  1  daily
enum   resource.vocab.QuestStyle  2  weekly
enum   resource.vocab.QuestStyle  3  forever
enum   resource.vocab.QuestStyle  4  season_pass
enum   resource.vocab.QuestStyle  5  pvp
enum   resource.vocab.QuestStyle  6  azure
enum   resource.vocab.RewardType  1  activity_points
enum   resource.vocab.RewardType  2  season_pass_exp
enum   resource.vocab.Stage  100  Stage_1
enum   resource.vocab.Stage  101  Stage_2
enum   resource.vocab.Stage  102  Stage_3
field  resource.vocab.eventTypes.code  0  COMBAT_KILL_MONSTER
field  resource.vocab.eventTypes.code  1  COMBAT_KILL_GIANT
field  resource.vocab.eventTypes.code  2  COMBAT_KILL_BOSS
field  resource.vocab.eventTypes.code  3  COMBAT_KILL_WORLD_BOSS
field  resource.vocab.eventTypes.code  5  COMBAT_KILL_BY_ELEMENT
field  resource.vocab.eventTypes.code  10  COMBAT_DAMAGE_PVE
field  resource.vocab.eventTypes.code  12  COMBAT_DAMAGE_PVP_EVENT
field  resource.vocab.eventTypes.code  30  COMBAT_KILL_FFA
field  resource.vocab.eventTypes.code  31  COMBAT_KILL_GUILD_SIEGE
field  resource.vocab.eventTypes.code  32  COMBAT_KILL_TDM
field  resource.vocab.eventTypes.code  50  COMBAT_GAME_MODE_START
field  resource.vocab.eventTypes.code  1000  ECONOMY_DROP_PENYAS
field  resource.vocab.eventTypes.code  1001  ECONOMY_DROP_ITEM
field  resource.vocab.eventTypes.code  1031  ECONOMY_SPEND_VOTE_POINTS
field  resource.vocab.eventTypes.code  1050  ECONOMY_CONSUME_ITEM
field  resource.vocab.eventTypes.code  2030  PROGRESSION_COLLECT_MONSTER
field  resource.vocab.eventTypes.code  2031  PROGRESSION_COLLECT_ITEM
field  resource.vocab.eventTypes.code  2090  PROGRESSION_CARD_DRAWN
field  resource.vocab.eventTypes.code  4000  CONTENT_COMPLETE_DUNGEON
field  resource.vocab.eventTypes.code  4010  CONTENT_COMPLETE_ADVENTURE_QUEST
field  resource.vocab.eventTypes.code  4050  CONTENT_WHEEL_OF_FORTUNE
field  resource.vocab.eventTypes.code  4080  CONTENT_LOTTERY_BUY_TICKETS
field  resource.vocab.eventTypes.code  5000  ITEMS_CRAFTED
field  resource.vocab.eventTypes.code  5001  ITEMS_RECYCLED
field  resource.vocab.eventTypes.code  5010  ITEMS_PERFORM_UPGRADE
field  resource.vocab.eventTypes.code  8000  MISC_ACTIVE_PLAYTIME
field  resource.vocab.eventTypes.code  8100  MISC_STATS_SNAPSHOT
table  resource.vocab.eventTypes  COMBAT_DAMAGE_PVE
table  resource.vocab.eventTypes  COMBAT_DAMAGE_PVP_EVENT
table  resource.vocab.eventTypes  COMBAT_GAME_MODE_START
table  resource.vocab.eventTypes  COMBAT_KILL_BOSS
table  resource.vocab.eventTypes  COMBAT_KILL_BY_ELEMENT
table  resource.vocab.eventTypes  COMBAT_KILL_FFA
table  resource.vocab.eventTypes  COMBAT_KILL_GIANT
table  resource.vocab.eventTypes  COMBAT_KILL_GUILD_SIEGE
table  resource.vocab.eventTypes  COMBAT_KILL_MONSTER
table  resource.vocab.eventTypes  COMBAT_KILL_TDM
table  resource.vocab.eventTypes  COMBAT_KILL_WORLD_BOSS
table  resource.vocab.eventTypes  CONTENT_COMPLETE_ADVENTURE_QUEST
table  resource.vocab.eventTypes  CONTENT_COMPLETE_DUNGEON
table  resource.vocab.eventTypes  CONTENT_LOTTERY_BUY_TICKETS
table  resource.vocab.eventTypes  CONTENT_WHEEL_OF_FORTUNE
table  resource.vocab.eventTypes  ECONOMY_CONSUME_ITEM
table  resource.vocab.eventTypes  ECONOMY_DROP_ITEM
table  resource.vocab.eventTypes  ECONOMY_DROP_PENYAS
table  resource.vocab.eventTypes  ECONOMY_SPEND_VOTE_POINTS
table  resource.vocab.eventTypes  ITEMS_CRAFTED
table  resource.vocab.eventTypes  ITEMS_PERFORM_UPGRADE
table  resource.vocab.eventTypes  ITEMS_RECYCLED
table  resource.vocab.eventTypes  MISC_ACTIVE_PLAYTIME
table  resource.vocab.eventTypes  MISC_STATS_SNAPSHOT
table  resource.vocab.eventTypes  PROGRESSION_CARD_DRAWN
table  resource.vocab.eventTypes  PROGRESSION_COLLECT_ITEM
table  resource.vocab.eventTypes  PROGRESSION_COLLECT_MONSTER
```

Violations against this lock:

| Change in vocab.canon | Finding |
|---|---|
| `FIRE = 1` renamed `BLAZE = 1` | `E6001` at `enum   resource.vocab.Element  1  FIRE`: `resource.vocab.Element.FIRE was removed; its code 1 is now held by BLAZE: renaming is not allowed` |
| `WATER = 2` changed to `WATER = 7` | `E6002` at `WATER`: renumbered (locked code 2) |
| `retired WIND = 4` then a new `STORM = 4` | `E6002` at `STORM`: code 4 belongs to `WIND` |
| `COMBAT_KILL_GIANT { code: 1, … }` changed to `code: 4` | `E6002` at the value: a stable value never changes |
| new `COMBAT_KILL_RAID { code: 3, … }` | `E6002` at the value: 3 belongs to `COMBAT_KILL_WORLD_BOSS` (and `E3102`, the value is not unique) |
| after a merge, the lock holds `field  resource.vocab.eventTypes.code  9000  A` and `…  9000  B` | `E6002` at the second line |
| a layer `amend eventTypes { COMBAT_KILL_RAID: { code: 9001, … } }` | `E6004` |
| `code: UInt16? @stable` | `E6003` (optional) |

---

## 10. Versioning (NFR-03)

- The header line carries the format version: `# canon.lock v1`. A compiler reads the versions it
  knows and refuses newer ones (`E6005`). It always writes the newest version it knows; upgrading
  the format rewrites the header and lines in the same canonical way, with no loss of facts.
- The lock format is independent of the language version and of the fingerprint version.

---

## 11. Diagnostics

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E6001 | error | §4.1, §5 (edit `Remove`) |
| E6002 | error | §4.2, §5 (edit `Set`), §6.3 |
| E6003 | error | §1 |
| E6004 | error | §6.1 |
| E6005 | error | §2.4 |
| W6006 | warning | §8, `canon lock check` only |

The single catalogue of every code is [ERRORS.md](ERRORS.md).
