# Logic

## Functions and methods

```canon calc/calc.canon
/// Functions, methods, checks and tests.
package calc

/// A level band.
enum Band ordered { low, mid, high }

/// A healing potion.
record Potion {
  /// Hit points restored.
  heal: Int(1..)
  /// Time before the next drink.
  cooldown: Duration = 1s

  // A method: `self` first, fields in scope by name.
  fn perSecond(self) -> Float { return Float(heal) / (cooldown / 1s) }

  check capped: heal <= 1_000 else "heals {heal}, above 1000"
  // Reported at the field `cooldown` instead of the record.
  warn fast: cooldown >= 500ms at cooldown else "cooldown {cooldown} is very short"
}

/// Parameters and return type are required; a parameter may have a constant default.
fn band(level: Int, highFrom: Int = 60) -> Band {
  if level < 20 { return low }
  return if level < highFrom { mid } else { high }
}

/// A function parameter takes a lambda or a function name.
fn countWhere(xs: [Int], pred: fn(Int) -> Bool) -> Int { return xs.count(pred) }

/// Statements: let, var, if, for, while, break, continue, return, match.
fn digits(n: Int) -> Int {
  var left = n
  var count = 1
  while left >= 10 {
    left /= 10
    count += 1
  }
  return count
}

let potions: [Potion] = [{ heal: 50 }, { heal: 400, cooldown: 3s }]

/// Comprehensions: for, if and let clauses in any order after the first for.
let strong: [Int] = [p.heal for p in potions if p.heal > 100]

let bands: {Int: Band} = { lv: band(lv) for lv in [5, 30, 90] }

/// A package-level block check runs once; fail (error) and warn (warning) report at any value.
check {
  for p in potions {
    if p.perSecond() > 200.0 { fail(p, "{p.heal} HP every {p.cooldown} is too much") }
  }
}

test "band boundaries" {
  expect band(19) == low
  expect band(20) == mid
  expect band(60) == high
  expect band(60, highFrom: 70) == mid
  expect countWhere([1, 5, 9], x => x > 3) == 2
  expect digits(1_000) == 4
}

test "checks fire" {
  expect Potion { heal: 10, cooldown: 100ms } warns fast
  expect Potion { heal: 2_000 } fails capped
  expect Potion { heal: 2_000 } fails "above 1000"
  expect Potion { heal: 2_000 } fails E5001
  expect { ...potions[0], cooldown: 2s } passes
}
```

- Named arguments follow positional ones: `band(5, highFrom: 40)`. A method call needs `()`.
- Lambdas: `x => e`, `(a, b) => e`, shorthand `.field` / `.method()` for `x => x.field`
  (postfix steps only: `.heal > 5` is `(x => x.heal) > 5`, an error; write `p => p.heal > 5`).
  A lambda needs an expected function type (a stdlib or user parameter).
- `var` only can be assigned: `x = e`, `+= -= *= /=`, `xs[i] = e`; `p.f = e` is `E3307`.
  Lists and maps are values: copying then mutating never changes the copy. Recursion and
  `while` are allowed: the step budget of the package bounds them (`E4401`).

## Expressions

Precedence, low to high: `x => e`; `??`; `or`; `and`; `not`; `==` `!=` `<` `<=` `>` `>=` `in`
`is` (no chaining: `a < b and b < c`, `E1128`); `..` `..=`; `+` `-`; `*` `/` `%`; unary `-`;
postfix `.f` `?.f` `[i]` `(args)` `!`. `and`/`or`/`not` take `Bool` only and short-circuit.

- Int `/` truncates, `%` has the left sign. Overflow `E4101`, division by zero `E4102`, a
  NaN or infinite float `E4104`.
- `+` joins strings and lists; prefer interpolation. `Duration ± Duration`, `Duration * Int`,
  `Duration / Int`, `Duration / Duration` (a `Float`).
- `==` is deep on records, lists and maps; refs and entries compare by identity. `<` works on
  numbers, durations, strings and `ordered` enums only (`E3310`).
- `if c { a } else { b }` is an expression when each branch is one expression and `else` exists.
- `x in xs`: an element of a list, table, keyed list or range; for a map, a key (`k in m`, same as
  `m.contains(k)`). A key of a table or keyed list is not an element (`E3026`): test it with
  `xs.hasKey(k)` (or `xs.get(k) != none`). `hasKey` does not exist on a map (`E3003`).
  `v is case` tests a variant case.

```canon fragment
match reward {
  gold(g) => g.amount           // binds the case's value
  item(i) => i.count
  nothing => 0
}
match season {
  summer, winter => true        // several patterns
  _ => false
}
```

`match` covers every member or case, retired ones included, or has `_` (`E3601`); scrutinee is an
enum, a variant, a `Bool`, or an optional of one (`none` pattern). No literal patterns.

## Standard library

| On | Functions |
|---|---|
| numbers | `Int(f)` `Float(i)` `String(x)` `abs` `min(a, b, ...)` `max` `clamp(x, lo, hi)` `floor` `ceil` `round` `sqrt` `pow` |
| strings | `len()` (bytes) `isEmpty()` `contains` `startsWith` `endsWith` `split(sep)` `trim()` `lower()` `upper()` `replace(a, b)` `matches(/re/)` `find(s)` `s[a..b]` |
| lists, tables, keyed lists | `len` `isEmpty` `first()` `last()` `first(pred)` `get` `contains` `indexOf` `map` `filter` `flatMap` `flatten` `any` `all` `count` `sum` `min` `max` `minBy` `maxBy` `sortBy` `reverse` `groupBy` `unique` `duplicates` (each repeated element once, first-seen order) `isUnique` `enumerate` `pairs` `zip` `join(sep)` `intersect` `union` `diff` `toMap(kf, vf)` |
| tables, keyed lists | `t[k]` `t.k` `get(k)` `hasKey(k)` `find(k)` `at(i)` `keys()` `values()`; tables: `active()` (not retired) |
| maps | `m[k]` `len` `isEmpty` `keys()` `values()` `get(k)` `contains(k)` (a key, same as `k in m`) `map` `filter` `all` `any` `count` (predicates take `(k, v)`) `union(b)` (`a`'s entries, then `b`'s new keys: first wins; `b.union(a)` is last-wins; on a kept dependent map `b` and the result have the receiver's type, on another map whose type reads a key binder it is `E3804`; `canon guide types`) |
| ranges | `r.start` `r.end` `r.len()` `r.isEmpty()` `r.contains(x)` |
| graphs | `reachable(from: x, next: f)` `cycles(xs, next: f)` `topoSort(xs, next: f)` |
| optional results | `first` `last` `get` `find` `min` `max` `minBy` `maxBy` return `T?` |

## Cost

Evaluation runs under a step budget of `project.budget`, counted per package: each step is
charged to the package that declares the value, check or function evaluated, whoever forced it
(`E4401` names the heaviest value; a package never fails for another's work).

- Fine in a loop (v0.1.2): `xs.isUnique()` (linear in `xs`, not quadratic), `xs[i] = e` and
  `m[k] = e` (not proportional to the length), and reading a field through a `ref`
  (`p.item.price`).
- Linear, so not inside a loop over the same data: `xs.contains(x)`, `xs.indexOf(x)`, `filter`,
  `map`. To test membership many times, build a map once (`xs.toMap(...)`) and use `k in m` (cost 1).

## Checks

| Form | Finding |
|---|---|
| `check cond else "msg {x}"` | error `E5001` |
| `warn cond else "msg"` | warning `W5001` |
| `check name: cond else "msg"` | named: test target, translation key, finding's `check` |
| `check cond at field else "msg"` | in a record: reported at that field (`E1633` if unknown) |
| `check { ... fail(v, "msg") ... warn(v, "msg") }` | `E5002` / `W5002` at value `v`'s source |

- In a record, variant or case: runs once per instance, fields in scope. At package level: once.
- Checks run after types, refinements and refs are verified; a check reading a failed value is
  skipped. Every check runs; nothing stops at the first error.
- `fail`/`warn` only lexically inside `check { }` (`E1105`).

## Tests

`expect <Bool>` (a comparison prints expected and got); `expect v passes` (no error building
`v`); `expect v fails "text"`, `fails checkName`, `fails E5001` (an error whose message contains
the text, from that check, or with that code); `warns` likewise for warnings. `canon test
[packages] [--run regex]` runs them; a failing `expect` does not stop its test. `fails` sees
evaluation findings of `v` and its record checks, never static ones (a literal out of range is
a compile error) nor package-level checks: `expect package` runs those (below). A translated
`export fn` method must be called by a test of its package (`E9008`).

## Testing package checks: `expect package`

A package-level check reads top-level values, which a test cannot change, and `fails checkName`
cannot name it for a value. `expect package` builds the test's own package as `canon check` does,
so its checks run; `{ name: value, … }` makes each named top-level `let` hold `value` while it is
built.

```canon crafting/crafting.canon
/// Crafting levels. `expect package` (EVALUATION.md §10.5) builds this package over values a test
/// substitutes, so its package checks have tests.
package crafting

/// One crafting level.
record Level {
  /// Level index.
  index: Int(0..=24)
  /// Experience needed to reach it.
  exp: Int(0..)
}

/// The shipped levels.
let levels: [Level] = [{ index: 0, exp: 0 }, { index: 1, exp: 100 }, { index: 2, exp: 250 }]

/// The highest level a player can reach.
let maxLevel: Int(0..=24) = 2

/// How many levels ship; follows levels.
let levelCount: Int = levels.len()

check enough: levelCount >= 2 else "at least two levels ship"

check uniqueIndex: [l.index for l in levels].isUnique() else "a level index is listed twice"

check {
  for l in levels {
    if l.index > maxLevel {
      fail(l, "level {l.index} is above maxLevel {maxLevel}")
    }
  }
}

test "the shipped levels pass, and still pass without the last one" {
  expect package passes
  expect package { levels: levels[0..2] } passes
}

test "a level listed twice, or above the cap, is refused" {
  expect package { levels: levels + [levels[1]] } fails uniqueIndex
  expect package { maxLevel: 1 } fails "level 2 is above maxLevel 1"
  expect package { levels: levels[0..1] } fails enough
}
```

- The outcomes are those of a value subject: `passes`, `fails`/`warns` with a text, a check name or
  a code. `{` is on the `package` line and an outcome is required; `package {}` is `E1116` (omit
  the braces). After `expect`, `package` is reserved: write a data symbol of that name qualified
  (`Icon.package`).
- A name is a top-level `let` of the test's own package, public or `local`. A `const`, a fn, a type,
  a built-in, a test local, a let of another package or a let given twice is `E5006`:
  `<name> is not a let: expect package substitutes top-level lets of its own package`,
  `<name> is a let of another package: substitute it in a test of that package`,
  `<name> is substituted twice (first at <loc>)`. An unknown name is `E2102`.
- Each value is checked against the let's type: a literal outside a refinement or of the wrong type
  (`E3002`, `E3204`) is a compile error, printed first, and the test fails without running. A
  value computed out of its range (`zero() - 1`) is not static: the `expect` captures the finding.
  The values are computed in the test, left to right, before anything is built, from the test's
  values (with the active layers, the substituted lets included) and its locals.
- The world: every top-level `const` and `let` of the package is evaluated afresh. A substituted
  let takes its value, with no initializer, `entry` declaration or layer, so values derived from
  it follow (`levelCount` follows `levels`). Values of other packages are the test's. Values are
  verified, instance checks and then package checks run; every finding is captured. Not run:
  emits, conformance vectors, the lock comparison, view checks. The world is discarded after its
  `expect`; the test's values do not change.
- `fails name` takes a package check of this package, or a check reachable from the static type of
  a top-level value. Otherwise `E5004`: `no check named <name> runs for package <pkg>`. For a
  value subject, naming a package check says `… is a package check, which only expect package
  runs`.
- A `table` let takes table values only: `{ gem { price: 500 } }`. A typed list value is `E3002`
  (`expected table shop.Item, found [shop.Item]`); an untyped list literal such as
  `{ items: [{ price: 1 }] }` is `E3305` (`cannot tell what this { … } is; give it a type`); a
  spread is `E3320` in a table literal (`spread items are not allowed in table literals`). To add
  to a table, write all its entries.
- An entry of a record field's collection (a `table` or keyed list inside a record) that a value
  takes from the test keeps the test's instance as its owner, so a `ref` made from it in the world
  can be `E3503`. Substitute the record that owns the collection, whole, instead of one entry.
- Cost: each `expect package` costs what stages A to D of the package cost, on the package's
  budget, and nothing is shared or cached between worlds. A loop of them multiplies it: a package
  whose build takes tens of millions of steps allows a few worlds. The way out of `E4401` is
  `project.budget`; `E4401` in a world stops the test and is never captured.
- `--layer` applies in the test and in every world; a substituted value gets no layer.
- `canon test` reports, besides its own results, the run-level findings no substitution can change:
  `E3102` (a repeated `@codes` code) and `E6004` (a layer on a stable table, reported when a test
  forces the amended table), and exits 1 as `canon check` does. `W7101` is not reported by
  `canon test`. A world that reads a table poisoned by a refused stable amendment aborts silently
  and captures nothing: `E6004` is the run's, so `expect package passes` can pass under that layer
  and the run still fails.
