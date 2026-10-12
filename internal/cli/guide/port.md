# Port a legacy file

A legacy file keeps its shape; the Canon model does not. Clean field names in Canon, the legacy
names on the wire (DECISIONS 3, 21). Never copy a legacy prefix, a 0/1 integer, a bitmask, a
numbered column or a flattened group into the model: each has a wire form that keeps it in the
file and out of the types (`canon guide wire`). The commands are in `canon guide recipes`.

| The file has | The model has | Wire form |
|---|---|---|
| `szName`, `dwCost` (Hungarian prefixes) | `name`, `price` | `@json("szName")` |
| `0` / `1` for a flag | `Bool` | `@json("bTwoHanded", int)` |
| seconds or milliseconds | `Duration` | `@json("dwCooldownSec", unit: s)` |
| one integer OR-ing flags | `[Flag]` | `@json("dwFlag", bits)` |
| `dwDestParam0..2` with `nAdjParamVal0..2` | `[Bonus](..=3)` | `@json(pairs: [...])` |
| a type tag with its fields in the parent | a `variant` | `@json(inline)` |
| a file name | an asset type | `asset("icons", ext: [svg])` |
| an id saved elsewhere | a stable table | `stable table`, `@stable` |
| a wrapper, a count, anything irregular | a `local record` | a `@text` function |

## 1. Read the file first

```json guide/legacyport/data/weapons.json
{
  "bow": {
    "dwID": 1,
    "szName": "Bow",
    "szIcon": "bow.svg",
    "dwCost": 150,
    "bTwoHanded": 1,
    "dwFlag": 0,
    "dwDestParam0": "DST_DEX",
    "nAdjParamVal0": 3,
    "dwDestParam1": "DST_STA",
    "nAdjParamVal1": 1,
    "dwKind": "bow",
    "dwRange": 8
  },
  "axe": {
    "dwID": 2,
    "szName": "Axe",
    "szIcon": "axe.svg",
    "dwCost": 120,
    "bTwoHanded": 1,
    "dwCooldownSec": 8,
    "dwFlag": 5,
    "dwDestParam0": "DST_STR",
    "nAdjParamVal0": 5,
    "dwKind": "sword",
    "dwReach": 2
  }
}
```

Read every key of one row and every value of the odd ones. `dwFlag` 5 is flags 1 and 4;
`dwDestParam0` and `nAdjParamVal0` are the first of up to three pairs; `dwKind` is a tag whose
cases add `dwReach` or `dwRange`. The keys below `"axe"` and `"bow"` become table keys.

## 2. The vocabulary

```canon guide/legacyport/vocab.canon
/// A legacy weapon file, ported: clean names in the model, the file's own names on the wire.
/// `canon guide port` walks through it.
package guide.legacyport

/// The stats a weapon can raise. The file holds the legacy define of each.
enum Stat {
  strength = "DST_STR"
  dexterity = "DST_DEX"
  stamina = "DST_STA"
}

/// Item flags. The file holds one integer: the OR of the codes of the flags set.
enum Flag @codes(UInt32) {
  binds = 1
  undestroyable = 2
  equipBind = 4
}

/// An icon file under icons/. A missing file or a wrong letter case is `E3701`.
type Icon = asset("icons", ext: [svg])
```

- A plain enum member takes its wire spelling after `=`: `strength` in Canon, `"DST_STR"` in
  the file.
- A `@codes` enum has a permanent code per member: `Flag` is what `bits` ORs (`canon guide wire`).
- `asset("icons", ext: [svg])` is a `String` that names an existing file (`canon guide assets`).

```text guide/legacyport/icons/axe.svg
<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>
```

```text guide/legacyport/icons/bow.svg
<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>
```

## 3. The parts: pairs and an inline variant

```canon guide/legacyport/kind.canon
package guide.legacyport

/// A stat a weapon raises, and by how much.
record Bonus {
  /// The stat raised.
  stat: Stat
  /// How much.
  amount: Int
}

/// What a weapon is. The tag and the fields of the case sit in the weapon's own object.
variant Kind @json(tag: "dwKind") {
  /// A blade.
  sword {
    /// Reach in tiles.
    reach: Int(1..) = 1 @json("dwReach")
  }
  /// Shoots arrows.
  bow {
    /// Range in tiles.
    range: Int(1..) @json("dwRange")
  }
}
```

- `Bonus` has exactly two fields: the first goes under the first key template, the second under
  the second. Its own field names never reach the file.
- `@json(tag: "dwKind")` names the tag key; each case field keeps its legacy name.

## 4. The record

```canon guide/legacyport/weapon.canon
package guide.legacyport

/// A weapon: clean names here, the legacy name of each key in `@json`.
record Weapon {
  /// The number stored in saves. It never changes.
  code: Int(1..) @stable @json("dwID")
  /// Shown name.
  name: String(1..) @json("szName")
  /// Icon file.
  icon: Icon @json("szIcon")
  /// Price in gold.
  price: Int(0..) = 0 @json("dwCost")
  /// Needs both hands. 0 or 1 on the wire.
  twoHanded: Bool = false @json("bTwoHanded", int)
  /// Wait between two uses. Whole seconds on the wire.
  cooldown: Duration = 0s @json("dwCooldownSec", unit: s)
  /// Property flags. One bitmask on the wire.
  flags: [Flag] where it.isUnique() = [] @json("dwFlag", bits)
  /// Up to three stat bonuses. Numbered parallel keys on the wire.
  bonuses: [Bonus](..=3) = [] @json(pairs: ["dwDestParam{i}", "nAdjParamVal{i}"])
  /// What it is. The tag and the case fields are flat in the weapon's object.
  kind: Kind @json(inline)
}

/// Every weapon, read from the legacy file as it is. The keys are permanent.
let weapons: stable table Weapon = load("data/weapons.json")
```

- Read the file as it is first (`load`): the types are right when `canon check` is clean. The
  command that turns loaded rows into `.canon` entries is `convert` (CLI.md §3.10, §6.4); `canon`
  does not offer it yet, so keep the `load` (entries can still be added with `canon edit`
  `addEntry`, `canon guide data`).
- Fields are Canon names (`twoHanded`, `cooldown`); the keys of the file are in `@json` only.
  `canon rename` changes a Canon name and leaves the wire name alone (`canon guide recipes`).
- `code` is the `@stable` field: the number saves hold. A table row has no `id` field of its
  own: the file's keys are the entry keys, and the element may not declare `id` (`E2105`).

## 5. What stays irregular: a `@text` projection

The reader wants `nVersion` and `nCount` around the rows. They are not part of the model, so a
`local record` and a `@text` function write them:

```canon guide/legacyport/legacy.canon
package guide.legacyport

/// What the legacy reader wants around the rows: a version and a count it allocates from.
local record LegacyFile {
  /// Format version the reader checks.
  version: Int @json("nVersion")
  /// How many rows follow.
  count: Int @json("nCount")
  /// The rows, spelled as the legacy file spells them.
  weapons: table Weapon
}

/// The legacy file, written from the clean model.
@text("weapons.legacy.json")
export fn legacyFile() -> LegacyFile {
  return { version: 3, count: weapons.len(), weapons: weapons }
}

emit json { out: "out/weapons.json", values: [weapons] }
emit text { out: "out" }
```

`canon build` writes the legacy file from the clean model. Every key has its legacy name and the
rows keep the shape of step 1; a key the model defaults is written (`dwCooldownSec` of the bow):

```text out/guide/legacyport/out/weapons.legacy.json
{
  "nVersion": 3,
  "nCount": 2,
  "weapons": {
    "bow": {
      "dwID": 1,
      "szName": "Bow",
      "szIcon": "bow.svg",
      "dwCost": 150,
      "bTwoHanded": 1,
      "dwCooldownSec": 0,
      "dwFlag": 0,
      "dwDestParam0": "DST_DEX",
      "nAdjParamVal0": 3,
      "dwDestParam1": "DST_STA",
      "nAdjParamVal1": 1,
      "dwKind": "bow",
      "dwRange": 8
    },
    "axe": {
      "dwID": 2,
      "szName": "Axe",
      "szIcon": "axe.svg",
      "dwCost": 120,
      "bTwoHanded": 1,
      "dwCooldownSec": 8,
      "dwFlag": 5,
      "dwDestParam0": "DST_STR",
      "nAdjParamVal0": 5,
      "dwKind": "sword",
      "dwReach": 2
    }
  }
}
```

- `emit json` writes Canon's own data wire (`$schema`, `rows`) with the same field names: for
  Canon runtimes. The `@text` file is the one a legacy reader opens.
- A `@text` function is for the shape of a file, not for data: a count it derives, a wrapper, a
  sentinel row. If the same irregularity appears in every file, the type is wrong: look for a
  `wire` form first.

The lock records the stable keys, the flag codes and the `code` values; commit it:

```text out/guide/legacyport/canon.lock
# canon.lock v1
enum   guide.legacyport.Flag  1  binds
enum   guide.legacyport.Flag  2  undestroyable
enum   guide.legacyport.Flag  4  equipBind
field  guide.legacyport.weapons.code  1  bow
field  guide.legacyport.weapons.code  2  axe
table  guide.legacyport.weapons  axe
table  guide.legacyport.weapons  bow
```

## Mistakes

| Mistake | Result |
|---|---|
| a legacy name as the Canon name (`dwCost: Int`) | passes `check`, every reader sees the prefix: name it `price` and keep `@json("dwCost")` |
| `Int(0..=1)` for a flag | `Bool` with `@json(int)` |
| `[Bonus]` with no upper bound under `pairs:` | `E3316`: write `(..=3)` |
| `kind: Kind? @json(inline)` | `E3316`: inline needs a non-optional variant |
| a bit that is not a `Flag` code in `dwFlag` | `E7111`: the data is wrong or a flag is missing |
| `bTwoHanded: true` in the file | `E7110`: the wire holds 0 or 1 |
| a gap in the pairs (`dwDestParam2` with no `dwDestParam1`) | `E7117`: fix the file once |
| `"Axe.svg"` for a file named `axe.svg` | `E3701`: exact case |
