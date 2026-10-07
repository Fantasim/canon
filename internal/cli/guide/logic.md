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
| lists, tables, keyed lists | `len` `isEmpty` `first()` `last()` `first(pred)` `get` `contains` `indexOf` `map` `filter` `flatMap` `flatten` `any` `all` `count` `sum` `min` `max` `minBy` `maxBy` `sortBy` `reverse` `groupBy` `unique` `isUnique` `enumerate` `pairs` `zip` `join(sep)` `intersect` `union` `diff` `toMap(kf, vf)` |
| tables, keyed lists | `t[k]` `t.k` `get(k)` `hasKey(k)` `find(k)` `at(i)` `keys()` `values()`; tables: `active()` (not retired) |
| maps | `m[k]` `len` `isEmpty` `keys()` `values()` `get(k)` `contains(k)` (a key, same as `k in m`) `map` `filter` `all` `any` `count` (predicates take `(k, v)`) |
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
a compile error) nor package-level checks. A translated `export fn` method must be called by a
test of its package (`E9008`).
