# Canon type rules

Normative companion to [SPEC.md](../SPEC.md) §4–§9. Version: **0.1 (draft)**.

This document says which programs are well typed, what every name means, and which checks
are static (reported before evaluation) and which are deferred to evaluation. It applies
DECISIONS 17 (strict optionals) and 19, and the accepted answers of AUDIT RES-*, TYP-* and
DEP-*.

Related documents:

| Document | Relied on for |
|---|---|
| [GRAMMAR.md](GRAMMAR.md) | the syntax. Used: postfix `!`, the type `fn(T, U) -> R`, `refType = "ref" qualifiedIdent`, `"is" qualifiedWord`, one generic brace-literal node, an assignment target parsed as an expression (§12.7 accepts only a name followed by index segments), and `none` as a match pattern |
| [EVALUATION.md](EVALUATION.md) | when deferred checks run, poisoning, provenance, the step budget, layers, runtime inputs |
| [STDLIB.md](STDLIB.md) | signatures of every built-in function and method |
| [WIRE.md](WIRE.md) | how loaded data is decoded against a type (codes E3203, E3315–E3318, E7xxx) |
| [CODEGEN.md](CODEGEN.md) | how each type is emitted |

Conventions. "Static" means reported in the resolve-and-check phase, with no evaluation. "At
evaluation" means reported by the evaluator, at the location given in EVALUATION.md. `T`, `U`,
`K`, `V` are types. `≤` is the assignability judgment of §6. Messages in the Diagnostics
table use `{…}` for inserted text and print types in Canon syntax (`[ref Status]`,
`Int(0..=100)`, `Weapon?`).

---

## 1. Checking model

Name resolution and type checking are **one phase** (phase 2 of EVALUATION.md §1, which gives
the full list): a bidirectional type checker that resolves names while it checks types (RES-01).

The checker works in this order, over every loaded package (dependencies first, see §3.1):

1. Collect every declaration of every file into its package namespace (§3.2).
2. Resolve type expressions: aliases, field types, parameters, return types, `let`
   annotations. Constant expressions used inside types (refinement bounds, `..=FARM_MAX_MODELS`)
   are folded by the evaluator at this point (§15).
3. Check every body: field defaults, `let` initializers, function bodies, checks, tests,
   `entry` declarations, layer files (all of them, active or not: EVALUATION.md §9.1), views
   and translation templates (VIEWMODEL.md and I18N.md apply these rules to their expressions).

Every expression is checked in one of two modes:

- **synthesize** `e ⇒ T`: compute the type of `e` from `e` alone;
- **check** `e ⇐ E`: `e` must have a type assignable to the expected type `E` (§6), and `E`
  may guide how `e` is read (contextual names §4, brace literals §5.2, lambdas §12.4, `none`,
  `[]`, integer literals as `Float`).

When check mode reaches an expression that has no special rule, it synthesizes `T` and requires
`T ≤ E` (else `E3002`).

A bare identifier whose expected type is `ref C` (or contains such a branch) and which is not
resolved statically is recorded as a **symbolic key**. Its existence is checked at evaluation
(`E3501`, §9.4). The ambiguity test of §4.2 uses declared names only, never loaded keys.

A declaration with a static error is **broken**. So is every declaration whose body names a
broken declaration. Broken values are never evaluated (EVALUATION.md §1), which keeps one
error from cascading. After an error the checker gives the expression the *error type*, which
is assignable to and from every type and produces no further diagnostics.

---

## 2. Type representation

Every type is one of the kinds below. Implementations represent them as a tagged union
(`types.Type`). "Static identity" is what the checker compares. Refinements never take part
in it (§6.1).

| Kind | Written | Carries | Notes |
|---|---|---|---|
| `Bool` | `Bool` | | |
| `Int` | `Int`, `Int8`…`Int32`, `UInt8`…`UInt64` | `width` (64, or the sized type) | every integer type is `Int` statically; the width is an implicit range refinement (§7.2) |
| `Float` | `Float`, `Float32` | `width` (64 or 32) | `Float32` is `Float` statically, rounded when stored (§7.3) |
| `String` | `String` | | |
| `Duration` | `Duration` | | millisecond precision; limited to ±9,223,372,036,854 ms (§7.2) |
| `Refined` | `B(lo..hi)`, `B(/re/)`, `B where p` | base type, list of refinements | never changes static identity (§6.1) |
| `Enum` | declared name | declaration | nominal |
| `Record` | declared name, `R(args)` | declaration, type arguments | nominal; arguments are erased statically (§11.3) |
| `Variant` | declared name | declaration | nominal |
| `Case` | `V.c`, or a narrowed variant | variant, case | the type of a case literal, a match binding or a narrowed path |
| `Kind` | not writable | variant | the type of `v.kind`: an enum whose members are the cases (§8.3) |
| `Optional` | `T?` | inner type, never `Optional` | `T??` is `E3401` |
| `List` | `[T]` | element type | |
| `KeyedList` | `[T] keyed by f` | element record type, key field | a list with unique keys and key lookup (§9.1) |
| `Map` | `{K: V}` | key type, value type | keys restricted (§9.2) |
| `DepMap` | `{k in c: T(k)}` | binder, collection, type function application | §11.5 |
| `Table` | `table T`, `stable table T` | element record type, `stable` | §9.3 |
| `Ref` | `ref T`, `ref name` | target collection (§10.2), element type | |
| `LitUnion` | `A \| "lit"` | base type, string literals | §13.2 |
| `Never` | `Never` | | no values |
| `Range` | `Range` | | integer ranges only (§13.3) |
| `Func` | `fn(T, U) -> R` | parameter types, result type | §12.3 |
| `Pair` | not writable | two component types | produced by `enumerate`, `zip`, `pairs` and map iteration (§12.5) |
| `TypeApp` | `Param(eventType)` | type function, argument expressions | a dependent type (§11) |
| `DepUnion` | not writable | type function | the static view of a dependent value, printed `Param(*)` (§11.4) |
| `Define` | `Define` | | the built-in element record of `load.defines` tables: `value: Int` |
| `Any` | `_` | | only in widget parameter types (SPEC §16.11) |
| `None` | not writable | | the type of the literal `none` before context gives it a type |
| `Error` | not writable | | the result of a static error; silences cascades |

Aliases (`type Penya = Int(0..)`) are expanded wherever a type is used. They are not a kind
(§13.1). `asset(root, ext: […])` is `String` with an asset refinement (§13.4).

---

## 3. Declarations and names

### 3.1 Packages and imports

- A file declares the package of its own directory or of an ancestor directory. Declaring
  any other package (for example a descendant, as in GEN-03) is `E2006`.
- A directory holding files of two packages, neither being the package of the directory, is
  `E2001`.
- `import p` needs a package `p` in the project (`E2003`). Imports are acyclic (`E2002`,
  naming the cycle).
- `import p { A, B }` binds `A` and `B`. Each must be a public declaration of `p` (`E2004`).
  `import p` binds `p` for qualified use (`p.A`); `import p as q` binds `q` instead.
- **Imports are per file**, as in Go: the names an `import` binds are visible only in the file
  that writes it. Two files of one package may import the same package (game.items's
  `item.canon` and `item.view.canon` both `import studio`).
- Two imports **of one file** binding the same name, or an import binding a name that the
  package also declares (in any file), is `E2005`, at the second binding.
- Packages are ordered for every deterministic traversal: dependencies before dependents,
  ties broken by the byte order of the qualified name.

### 3.2 One namespace per package

All top-level declarations of a package (constants, types, enums, records, variants,
functions, lets, widgets) share **one namespace**. Two declarations with the same name are
`E2106`, naming both locations. The position of a use decides which kinds are acceptable: a
type name in value position is `E2110`, except where §4.3, §5.2 and STDLIB.md §2.1 give it a
meaning (`Weekday.Mon`, `TimeOfDay { … }`, `Int(f)`).

Inside a declaration, these must also be unique (`E2106`): the fields of a record or case,
record methods, the members of an enum, the cases of a variant, the parameters of a function.

### 3.3 Lookup order

A bare identifier in **value position** is resolved in this order. The first step that finds
it wins, except for the ambiguity rule of §4.2.

1. **Contextual**, against the expected type (§4).
2. **Local scopes**, innermost first: match bindings, lambda parameters, comprehension
   variables and `let` clauses, block `let`/`var`, loop variables, function parameters.
3. **Record body**: inside a record or case body (field defaults, checks, methods), its
   fields and its user methods. A bare method call `name()` means `self.name()`. `self` is
   the record value (§8.1).
4. **Package** declarations, from every file of the package.
5. **Imports** of the current file (§3.1): names bound by `import p { … }`, then the package
   names bound by `import p` or `import p as q`.
6. **Built-ins**: the built-in types (`Int`, `Float`, `String`, `Bool`, `Duration`, `Range`,
   `Never`, `Define`, the sized types) and the free functions of STDLIB.md (`min`, `max`,
   `abs`, `clamp`, `floor`, `ceil`, `round`, `sqrt`, `pow`, `reachable`, `cycles`, `topoSort`),
   and `fail` (GRAMMAR.md §4.2: a predeclared identifier, callable only inside a `check { }`
   block, `E1105`). `warn` and `load` are reserved words (GRAMMAR.md §4.1): syntax, not names.

So a user declaration shadows a built-in: in `record LevelRange { min: Int, max: Int }`, `min`
inside the body is the field. Built-in **methods** (`len`, `count`, `filter`…) are looked up
only after `.` and never in scope.

A name found nowhere is `E2102`, with the closest spelling in scope as a hint.

In **type position** the order is: type parameters of the enclosing declaration, package
declarations, imports, built-in types. Only types, enums, records, variants and aliases are
acceptable there, plus let names after `ref` (§10.2).

### 3.4 Local names

- `let`, `var`, loop variables and parameters are block-scoped.
- Declaring a name that is already declared **in the same block** (or a parameter redeclared
  in the function's top block) is `E2107`. Shadowing a name of an outer scope is allowed.
- `self` outside a record or case body is `E2108`. `it` outside a refinement predicate
  (`where`) is `E2109`.

### 3.5 `.name`: field, entry or method (RES-05)

- `x.k(args)` is always a **method** call: a user method of the receiver's record or case, or
  a built-in method (STDLIB.md).
- `x.k` without parentheses is, depending on the receiver's static type:
  - record or case: the field `k`, else a built-in member (`id`, `retired` on entries, `kind`
    on variants, `name`/`index`/`wire`/`code` on enums, `start`/`end` on ranges);
  - `table T`, or `[T] keyed by f` with a `String` key field: the **entry** with key `k`
    (`jobH.MAX_JOB_LEVEL`, `flags.blocking`). A missing key is `E4002` at evaluation;
  - `ref T`: as for `T` (implicit dereference);
  - `Map`: never; `m.k` is `E3003` (write `m[k]`).
- A name that is none of these is `E3003`.

So the table entry `units.count` is the entry, and `units.count(pred)` is the method.

### 3.6 Name clashes

- A record or case with a field and a method of the same name: `E2104`.
- A record used as the element type of a `table` may not declare a field or method named
  `id` or `retired` (`E2105`). A keyed-list element may (`record Potion { id: … }`).
- A case may not declare a field or method named `kind` (`E2105`). A record field `kind`
  holding a variant is fine: `e.kind.kind`.
- Enum members, variant cases and table keys may be any identifier, including reserved words
  where GRAMMAR.md allows them. They never clash with declarations: they live in their type or
  collection, not in the package namespace.

---

## 4. Contextual names

### 4.1 Step 1

When a bare identifier `n` is checked against an expected type `E`, step 1 of §3.3 looks it up
in `E`, after expanding aliases and removing refinements:

| `E` | `n` resolves to |
|---|---|
| enum `Tn` | the member `Tn.n` |
| `Kind` of a variant `V` | the case `n` of `V` (as a kind value) |
| variant `V`, case type `V.c` | the case `n`, written without fields (§8.2) |
| `ref C`, where the keys of `C` are known statically | the entry of `C` with key `n` |
| `ref C`, keys not known statically | see below |
| `T?` | as for `T` |
| `A \| "lit"` | as for `A` |
| `DepUnion` (§11.4) | kept **symbolic**, resolved at evaluation against the computed branch |

The keys of `C` are **known statically** when `C` is a top-level `let` whose initializer is a
table literal (possibly `{}`); the keys are those of the literal plus every `entry C.k`
declaration. Every other collection (loaded, computed, keyed lists) has dynamic keys.

For `ref C` with dynamic keys, steps 2 to 6 are tried first. If `n` resolves there to a value
assignable to `ref C`, that value is used. Otherwise (no declaration, or one of another type)
`n` becomes a **symbolic key** of `C`, checked at evaluation (`E3501`). `II_GEN_GOLD` in a `ref
items` field is a symbolic key.

A string literal or an integer literal checked against `ref C` is a key too, if it is a
literal of the key's type: a table key is written as an identifier or a string; a keyed-list
key as a literal of the key field's type (`3` in a `ref modelTypes`, RES-09).

Expected types reach nested positions (§5), so `next: [taken, wont_do]` resolves `taken`
against `ref Status`, and `{ Stage_1: 20 }` resolves `Stage_1` against the map's key type.

### 4.2 Ambiguity (RES-02)

A step-1 match wins over steps 2 to 6 **silently**, except in one case: if a local name,
parameter or in-scope record field (steps 2 and 3) has the same name **and** a type assignable
to `E`, the use is `E2101`. Write it qualified (`Icon.columns`) or rename the local. Package
and imported names never make a step-1 match ambiguous: `icon: columns` in taxonomy.canon is
the `Icon` member, although `let columns` exists.

### 4.3 Qualified forms

`Enum.member`, `Variant.case`, `pkg.Name`, `alias.Name`, `pkg.Enum.member` and `table.key`
(value position) are always accepted and never ambiguous.

---

## 5. Expected types

### 5.1 Positions that supply an expected type

Every position below checks its expression against the stated type. Positions not listed
synthesize.

| Position | Expected type |
|---|---|
| `let x: T = e` (top-level or local), `var x: T = e` | `T` |
| assignment `x = e`, `x[i] = e` | the declared type of `x`, or of its element |
| compound assignment `x op= e` | the right operand type of `op` for `x`'s type (§7.1) |
| record or case literal field `f: e` | the declared type of `f` (for a dependent type see §11.4) |
| field default `f: T = e` | `T` |
| spread `...e` | the literal's record or case type |
| function argument, positional or named | the parameter type (after binding type parameters, §12.2) |
| parameter default | the parameter type |
| `return e` | the declared return type |
| lambda body | the result type of the expected function type, when it contains no unbound type parameter |
| list literal element | the element type of the expected list |
| map literal key / value, map comprehension key / value | `K` / `V` of the expected map |
| dependent map literal value `k: e` | `T(k)`, erased (§11.5) |
| table literal entry `k { … }` | the table's element type |
| typed literal `Name { … }` | the named record or case type |
| `if` expression branches, `match` expression arms | the expected type of the whole expression |
| `a ?? b` | `a ⇐ E?`, `b ⇐ E`; without `E`: `a` synthesizes `T?`, then `b ⇐ T` |
| `a == b`, `!=`, `<`, `<=`, `>`, `>=` | one operand is synthesized, the other is checked against its type (§7.5) |
| arithmetic `a op b` | as for comparisons; the operator table (§7.1) then gives the result |
| `x in xs` | `xs` synthesizes; `x ⇐` its element type (list, `Range`) or key type (map). On a keyed list or table, `x` is synthesized and may be an element or a key (STDLIB.md §5); a bare identifier not in scope is a key |
| `x is c` | `c` is a case of the variant type of `x` (§8.3) |
| `if` / `while` condition, `and`/`or`/`not` operands, `check`/`warn` condition, `where` predicate, comprehension `if`, `expect` comparison | `Bool` |
| index `xs[i]` | `Int` (list), `Range` (slice), the key type (keyed list, table: `String` or `ref`), `K` (map) |
| type argument `Param(eventType)`, `HourlyTarget(e)` | the declared type of the type parameter |
| refinement bound | the refined base type (`Int`, `Float`, `Duration`) |
| enum member value `= lit` | `String` (wire value) or the `@codes` type |
| `fail(at, m)`, `warn(at, m)` | `at` synthesizes (any type); `m ⇐ String` |
| amend path value | the static type at the path (EVALUATION.md §9) |
| annotation arguments | per the annotation catalogue (GRAMMAR.md §8); `@json(none: x)` per WIRE.md; an `@json` form that does not apply to the field's type is `E3316` (WIRE.md §4.1) |
| `load(…)` | the expected type of the `load` expression itself; none is `E7002` (WIRE.md) |
| unary `-e`, `(e)` | passes the expected type through |

For a comparison or arithmetic operator, the operand that is **context-dependent** is the one
checked: a bare identifier not resolvable in steps 2–6, `none`, `[]`, `{}`, or a numeric
literal. Otherwise the left operand is synthesized and the right one checked. If both operands
are context-dependent the expression is `E3008`. So `tone == warning` and `warning == tone`
both resolve `warning` against `Tone`.

### 5.2 Brace literals (GRM-10)

The parser produces one brace-literal node. The checker classifies it from its items and the
expected type `E` (aliases expanded, refinements and one level of `?` removed):

| Condition | Classification |
|---|---|
| `expr : expr` followed by a `for` clause | map comprehension. `E` must be a map (or absent). Keys are **expressions**, never contextual field names |
| `E` is a record `R` or a case `V.c` | record literal: items are `name: expr` fields and at most one leading spread |
| `E` is a `table T` | table literal: items are `key { … }` entries (with optional doc comments and `retired`) |
| `E` is a map `{K: V}` or a dependent map | map literal: items are `key: value` |
| `E` is a variant `V` | `E3002`: a variant value needs a case (`nothing`, `item { … }`) |
| `E` absent, first item `...e` | record literal of the static type of `e` (a record or case, else `E3002`) |
| `E` absent, every item `"string": v` | map `{String: V}`, `V` the join of the values (§6.4) |
| `E` absent, `{}` or any other items | `E3305` |
| `E` is any other type | `E3002` |

Keys of a map literal:

- `K` an enum, `ref`, `Kind`, `LitUnion` or `DepUnion`: `ident:` is resolved contextually
  against `K` (§4), then in scope. `{ pve: …, pvp: … }` is a `{Profile: Int}`.
- `K` is `String` or an integer type: `ident:` is `E3304`. Write `"ident":`, or `(ident):` to
  use a variable.
- Any key may be a parenthesised expression or a literal.

Item kinds that do not fit the classification (a table entry in a record literal, a field in a
map, a spread in a map or table) are `E3320`. In a record literal:

- an unknown field is `E3301`, a missing required field `E3302`, a field given twice `E3321`;
- a spread must be the first item and appear once (`E3323`). Its type must equal the literal's
  type (`E3303`). A case spread requires the same case. Later fields override;
- input fields may not be given (`E3312`, §14).

A duplicate key in a map literal is `E3322`: static when both keys are constants or
identifiers, otherwise at evaluation.

A **typed literal** `Name { … }` names its type: `Name` resolves to a record or an alias of
one, to `V.c`, or (as a bare name, when the expected type is a variant) to a case. The literal
is then checked as above and its type checked against `E`.

`event.canon`'s `expect { ...sample, schedule: […] } fails "…"` is therefore an `Event` literal,
typed by its spread.

### 5.3 Other context-dependent literals

- `none` has type `None`. Checked against `T?` it is fine; against a non-optional `T` it is
  `E3403`. Synthesized alone (`let x = none`) it is `E3008`.
- `[]` checked against `[T]` has type `[T]`. Synthesized alone it is `E3008`, unless a join
  gives it a type (§6.4). `{}` follows §5.2.
- An **integer literal** (a literal token, with an optional leading `-`) is accepted where a
  `Float` is expected, and becomes that float value (TYP-10). Any other `Int` expression where a
  `Float` is expected is `E3311` (write `Float(n)`).
- An integer literal checked against an integer type must fit its range (`E3201`, static). A
  literal checked against a refined type that it statically violates is reported at once with
  the refinement's code (§7.4).

---

## 6. Assignability, conversion and joins

### 6.1 Static identity

Two types are **the same** when, after expanding aliases:

- refinements are removed (`Int(0..)` is `Int`, TYP-04), including `where` and asset
  refinements;
- every integer type is `Int` and `Float32` is `Float`;
- records, enums and variants are compared by declaration; record arguments are ignored
  (`HourlyTarget(e)` is `HourlyTarget(_)`, TYP-18);
- refs are the same when they target the same collection (§10.2);
- lists, keyed lists, maps, optionals, pairs and function types are compared component-wise;
  a keyed list also compares its key field.

### 6.2 Assignability `S ≤ E`

`S ≤ E` means a value of static type `S` may be used where `E` is expected. Some rules
**convert** the value, and some leave a check for evaluation.

| Rule | Conversion | Deferred check |
|---|---|---|
| `S` same as `E` (§6.1) | refinements of `E` are checked | `E32xx` (§7.4) |
| `Error ≤ E`, `E ≤ Error`, `Never ≤ E` | | |
| `None ≤ T?` | | |
| `T ≤ T?` (and `S ≤ T` implies `S ≤ T?`) | wraps | |
| `S? ≤ T?` if `S ≤ T` | the conversion of `S ≤ T` applies to a present value; `none` stays `none` | the deferred check of `S ≤ T`, on a present value |
| `T? ≤ T` | **never**: `E3403` (DECISIONS 17; use narrowing, `!` or `??`, §6.5) | |
| `ref T ≤ T` | dereference | `E3501` if the key does not exist |
| `T ≤ ref T` (TYP-02) | the entry becomes a ref | `E3503` unless the value is an entry of the ref's target collection |
| `ref C ≤ ref D`, `C ≠ D` | never: `E3002` | |
| `V.c ≤ V` (case to variant) | | |
| `Kind(V)` | only to itself | |
| `[S] ≤ [T]` if `S ≤ T` | element-wise | element checks |
| `[T] keyed by f ≤ [T]`, `table T ≤ [T]` | entries keep their identity | |
| `[S] ≤ [T] keyed by f` if `S ≤ T` | element-wise | `E3102` if two keys are equal |
| `{K1: V1} ≤ {K2: V2}` if `K1 ≤ K2` and `V1 ≤ V2` | entry-wise | |
| `{K: V} ≤` dependent map over `ref C` if `K ≤ ref C` | entry-wise | `E3802` per value (§11.5) |
| `Pair(A, B) ≤ Pair(C, D)` if `A ≤ C`, `B ≤ D` | | |
| `"lit" ≤ A \| "lit"`; `S ≤ A \| "lits"` if `S ≤ A` | | |
| string literal `≤ String`-based `LitUnion` alternatives | | |
| `S ≤ DepUnion` or `TypeApp` | the value is kept | `E3802` against the computed type (§11) |
| `Func(P) -> R ≤ Func(P') -> R'` if `P` same as `P'` and `R ≤ R'` | | |
| integer literal `≤ Float` | exact value | |

Anything else is `E3002`. Notably there is no conversion between `Int` and `Float` (except
literals), `Int` and `Duration`, enums and `String`, or `[T]` and `table T`.

**Storage points** are the positions where a value is converted to a declared type and where
refinements are checked at evaluation (TYP-04): annotated `let` and `var` (each assignment
too), record and case fields (literal, default, loaded, spread override), elements, keys and
values of a list or map whose type is declared, function arguments, function results, amend
values, and loaded values. EVALUATION.md §4.3 specifies the check.

### 6.3 Identity of entries (TYP-02)

- Static types are `T` and `ref T`. There is no separate "entry type".
- At evaluation, an element of a table or keyed list carries an **identity**: its collection
  and its key. Copying, binding, passing and returning keep it. A spread (`{ ...e, … }`) or any
  other construction makes a new value without identity.
- `self` in a record body is an entry when the value is one.
- Implicit conversions between `T` and `ref T` follow §6.2. `==` uses identity when both
  sides have one (§7.5).

### 6.4 Joins (TYP-05)

When several branches must agree without an expected type (`if` expression, `match`
expression, list literal elements, `??` without context, the values of a string-keyed map
literal), their types are joined pairwise:

| Operands | Join |
|---|---|
| same type (§6.1) | that type, **without refinements** |
| `None` and `T`, or `T?` and `T` | `T?` |
| `[]` (unknown element) and `[T]`; `{}` and a map | the other operand |
| `T` and `ref T` | `T` |
| two cases of one variant, or a case and its variant | the variant |
| integer literal and `Float` | `Float` |
| `ref C` and `ref C` | `ref C` |
| lists, maps, optionals | component-wise |
| anything else | `E3308`, naming both types |

`if c { [1] } else { [] }` is `[Int]`. With an expected type, each branch is checked against
it and no join is computed.

### 6.5 Optionals (DECISIONS 17)

A value of type `T?` is never used where `T` is expected. Code proves presence in one of
four ways:

| Form | Type | At evaluation |
|---|---|---|
| narrowing: `x != none` in a condition (§6.6) | `x` is `T` where the condition holds | |
| `x ?? y` | `T` if `y: T`; `T?` if `y: T?` | `y` is evaluated only if `x` is `none` |
| `x!` (postfix) | `T` | `E4001` at the `!` if `x` is `none` |
| `x?.f`, `x?.m(…)` | the chain's type, made optional | the rest of the chain is skipped if `x` is `none` |

Static errors on optionals:

- `.f`, `.m(…)`, `[i]`, a call, iteration (`for … in x`, `… in x`), or arithmetic on a value
  of type `T?` is `E3402`. The message suggests `?.`, `!`, `??` or a `!= none` test.
- A `T?` where a `T` is expected (argument, field, return, operand of `<`, condition, …) is
  `E3403`.
- `!`, `?.` or `??` on a non-optional left side, and `x == none` / `x != none` with a
  non-optional `x`, are `W3401` (redundant; the comparison is constant). For `?.` the left side
  is the **receiver segment** as the chain has typed it so far: after an earlier `?.` has
  unwrapped the chain, a `?.` whose receiver segment is not optional is `W3401` (`w?.item?.id`
  with `item: ref itemH` is `W3401` at the second `?.`; write `w?.item.id`).
- `==` and `!=` accept `T?` against `T` or `T?` (§7.5). `x in xs` accepts `x: T?` (a `none` is
  never a member). Interpolation prints `none`.

**Optional chaining.** A postfix chain is a primary followed by suffixes `.f`, `?.f`, `[i]`,
`(args)` and `!`. When a `?.` finds `none`, the **whole rest of the chain** is skipped and the
chain is `none` (as in Swift, C# and TypeScript). Statically, each `?.` unwraps its receiver
for the suffixes after it, and the chain's type is made optional once at the end: the type of
the last segment `T` becomes `T?`, and a last segment that is already optional (`U?`) stays `U?`
(never `U??`). So `etcJobs.first(…)?.jobBase`, with `jobBase: String?`, is `String?`.

```
w?.item.id ?? ""          // w: Weapon?, item: ref itemH: `w?.item.id` is String?, the whole is String
a?.b.c                    // b: B?: `.c` is E3402; write a?.b?.c
(w?.item).id              // parentheses end the chain: E3402
a?.b!.c                   // none if a is none; E4001 if a is present and b is none
```

`!` binds as a postfix operator (SPEC §7.1 level 10). `x!=y` is `x != y`: write `x! == y`.

### 6.6 Flow narrowing

A **stable path** is:

- a root: a local `let`, a `var` (see kills below), a parameter of a function, method or
  lambda, a loop or comprehension variable, a comprehension `let`, a match binding, `self`, a
  field name in scope in a record body (it means `self.f`), a top-level `let` or `const` of any
  package, or `it`;
- followed by any number of `.f` segments, where `f` is a record or case field, or a table or
  keyed-list entry key (`flags.sensitive.hint`). Going through a `ref` is allowed (implicit
  dereference).

Index segments, method calls, `!` and parenthesised expressions end a stable path. Two paths
are the same when they have the same root declaration and the same segments.

A **fact** is "path `p` is not `none`" (and, for `is`, "path `p` is case `c`", §8.3).
Every Boolean expression `C` has two fact sets: `T(C)`, true when `C` evaluates to `true`, and
`F(C)`, true when it evaluates to `false`:

| `C` | `T(C)` | `F(C)` |
|---|---|---|
| `p != none`, `none != p` | `{p}` | `{}` |
| `p == none`, `none == p` | `{}` | `{p}` |
| `p == e`, `e == p`, with `e` of non-optional type | `{p}` | `{}` |
| `p != e`, `e != p`, with `e` of non-optional type | `{}` | `{p}` |
| `p is c` | `{p, p is c}` | `{}` |
| `not C1` | `F(C1)` | `T(C1)` |
| `C1 and C2` | `T(C1) ∪ T(C2)` | `F(C1) ∩ F(C2)` |
| `C1 or C2` | `T(C1) ∩ T(C2)` | `F(C1) ∪ F(C2)` |
| `(C1)` | `T(C1)` | `F(C1)` |
| anything else | `{}` | `{}` |

When `p` is written as an optional chain of stable segments (`w?.item != none`), the fact
covers every prefix of the chain (`w` and `w.item`).

Facts apply as follows. A fact `{p}` gives `p` the type `T` instead of `T?`. A fact `p is c`
gives `p` the case type `V.c`.

| Construct | Facts that hold |
|---|---|
| `C1 and C2` | `C2` is checked under `T(C1)` |
| `C1 or C2` | `C2` is checked under `F(C1)` |
| `if C { A } else { B }`, statement or expression | `A` under `T(C)`, `B` under `F(C)`; `else if C2` is checked under `F(C)` |
| `while C { B }` | `B` under `T(C)` |
| comprehension clause `if C` | every later clause and the element (or key and value) under `T(C)` |
| `check C else "m"`, `warn C else "m"` | the message template under `F(C)` |
| `match p { none => … , arms }` | every other arm under `{p}`; an arm `c(x)` or `c` on a variant under `p is c` |
| lambda | its body under the facts holding where the lambda is written (captures are copies) |

**Facts after a statement.** In a block, facts flow from one statement to the next:

- After `if C1 { A1 } else if C2 { A2 } … else { B }` (a missing `else` is an empty one), the
  facts are the **intersection**, over the branches that can complete normally, of the facts
  holding at the start of that branch: `T(C1)` for the first branch, `F(C1) ∪ T(C2)` for the
  second, …, `F(C1) ∪ … ∪ F(Cn)` for the `else`. So after `if x == none { return … }` the rest
  of the block has `{x}`.
- A block **cannot complete normally** if its last statement is `return`, `break` or
  `continue`, or an `if` with an `else` whose branches all cannot complete normally, or a
  `match` statement whose arms all cannot complete normally. `fail(…)` and `warn(…)` complete
  normally.
- After `while C { B }` with no `break` of this loop in `B`: `F(C)`, plus the facts before
  the loop.

**Kills.** A `var` can be narrowed, but an assignment ends it:

- after an assignment to `v` (`v = e`, `v op= e`), every fact rooted at `v` is removed. The
  right-hand side is checked **before** the kill, so `at = etcJobs.first(j => j.id ==
  at.jobBase)` still sees `at` narrowed;
- at the start of a loop body, and after the loop, facts rooted at any `var` assigned
  anywhere in that loop body are removed (the loop may have run the assignment before);
- element assignment (`v[i] = e`) kills nothing: index paths are never narrowed;
- facts rooted at immutable roots are never killed.

Examples (all well typed):

```
// a var narrowed by a while condition (sweep_plan.canon's lineage walk, written as a loop)
var at = etcJobs.first(j => j.id == job)            // EtcJob?
while at != none and at.jobBase != none {
  out += [at.jobBase]                               // at: EtcJob, at.jobBase: String
  at = etcJobs.first(j => j.id == at.jobBase)       // kill after the right-hand side
}

// or-chain (sweep_plan.canon's linkAccepts writes it as an if statement)
link == none or kind in (LINK_FAMILIES.get(link) ?? [link])

// rules.canon: and-chain on a local
if tier != none and not tier.totems { … }

// adventurequest.canon: comprehension if-clause
let priorities = [a.priority for a in amps if a.priority != none]   // [Int]

// early exit
fn label(f: Flag) -> String {
  if f.hint == none { return f.label }
  return f.hint                                     // String
}
```

Not narrowed: `xs[0] != none` (index), `f() != none` (call), a `var` inside a loop that
assigns it, and anything after `expect x != none` (a failed `expect` does not stop the test).

---

## 7. Scalars and operators

### 7.1 Operator table

`Num` is `Int` or `Float`. Operands must have the listed types after narrowing. An integer
literal may stand for a `Float` operand. Anything else is `E3007`, naming the operator and both
types.

| Operator | Operands | Result |
|---|---|---|
| `+ - * /` | `Int, Int` | `Int` (`/` truncates toward zero) |
| `+ - * /` | `Float, Float` | `Float` |
| `%` | `Int, Int` | `Int` (sign of the left operand) |
| `+ -` | `Duration, Duration` | `Duration` |
| `*` | `Duration, Int` or `Int, Duration` | `Duration` |
| `/` | `Duration, Int` | `Duration` (truncates toward zero, in ms) |
| `/` | `Duration, Duration` | `Float` |
| `+` | `String, String` | `String` |
| `+` | `[S], [T]` | `[S ⊔ T]` (a plain list) |
| unary `-` | `Int`, `Float`, `Duration` | same |
| `== !=` | comparable (§7.5) | `Bool` |
| `< <= > >=` | orderable (§7.5) | `Bool` |
| `and or` | `Bool, Bool` | `Bool` |
| `not` | `Bool` | `Bool` |
| `in` | `T` and `[T]`, keyed list, table, `{T: V}`, `Range` | `Bool` |
| `is` | variant or `V?`, case name | `Bool` |
| `..`, `..=` | `Int, Int` | `Range` |
| `??` | `T?, T` / `T?, T?` | `T` / `T?` |

Not defined, and therefore `E3007`: `%` on `Float` or `Duration`; `Duration * Float`; `+` on
maps; arithmetic mixing `Int` and `Float` variables; arithmetic on `Bool`, enums or refs.
Evaluation errors (overflow, division by zero, NaN) are in EVALUATION.md §6.

Compound assignment `x op= e` is `x = x op e`, typed with the table above and assigned back
to `x`'s declared type.

### 7.2 Sized integers (TYP-03)

In expressions every integer type is `Int` (64-bit signed). A sized type is `Int` plus an
implicit range: `Int8` −128..=127, `Int16`, `Int32` likewise, `UInt8` 0..=255, `UInt16`,
`UInt32`, and `UInt64` limited to `0..=9223372036854775807` in this version. The range is
checked at storage points (`E3201`) and, for literals, statically. `code: UInt16` holding
70000 is `E3201`. `code + 1` is an `Int`.

**Durations** follow the same model. A `Duration` is a signed 64-bit count of milliseconds in
expressions (overflow is `E4101`, EVALUATION.md §6.3), and a stored `Duration` is limited to
**±9,223,372,036,854 ms**, the range of Go's `time.Duration`, which generated Go getters return
(CODEGEN.md §4.1). The limit is an implicit range, checked like a sized type: at storage points
(`E3201`), statically for literals, on loaded values (WIRE.md §5.1) and on the parameters and
results of translated functions (CONFORMANCE.md §2.3).

### 7.3 Floats

`Float` is IEEE 754 binary64. `Float32` is `Float` in expressions, and a value stored into a
`Float32` is rounded to the nearest binary32 value (TYP-11). NaN and infinities are never
values: storing one is `E3202`, and producing one is `E4104` (EVALUATION.md §6.2).

### 7.4 Refinements

A refinement never changes the static type (§6.1). It is a predicate checked at storage
points (§6.2), reported at the value's location (EVALUATION.md §4.3):

| Refinement | Allowed on | Holds when | Code |
|---|---|---|---|
| `(lo..hi)`, `(lo..=hi)`, `(lo..)`, `(..=hi)`, `(..hi)` | `Int` (all widths), `Float`, `Duration` | the value is in the range | `E3204` |
| the same | `String` | the length in **bytes** is in the range | `E3204` |
| the same | `[T]`, keyed lists, maps | the number of elements is in the range | `E3204` |
| `(/re/)` | `String` | RE2 **search** finds a match (STD-03) | `E3205` |
| `where p` | any type | `p` evaluates to `true` with `it` bound to the value | `E3206` |
| implicit sized range (§7.2) | integer types | | `E3201` |
| implicit finiteness | `Float`, `Float32` | | `E3202` |
| asset (§13.4) | `String` | the file exists | `E3701`–`E3703` |

Rules:

- Bounds are constant expressions (§15) of the base type (`E3015`); an integer literal is
  accepted as a `Float` bound. A range with `lo > hi` (after normalising `..=`) is `E3023`.
- A range refinement on `Bool`, an enum or a record, or a regex on anything but `String`, is
  `E3023`. An invalid RE2 pattern is `E1114` (GRAMMAR.md).
- A refinement on a named type adds to the named type's own refinements: `Penya(..=1000)` with
  `type Penya = Int(0..)` means `0..=1000`. Both are checked. Chained refinements in one type
  expression (`Int(0..)(..=5)`) are `E1103` (GRAMMAR.md).
- On `T?`, a refinement or `where` applies to the non-`none` value only: `Int? where it > 0`
  accepts `none` (GRM-06). In a union, `where` binds to the last alternative.
- `it` in a `where` predicate has the refined type without the refinement being defined (the
  predicate may call functions). `it` is never optional.
- A literal checked against a refined type that it violates is reported statically with the
  same code (`stack: Int(1..) = 0` is a static `E3204`).

### 7.5 Equality and ordering (TYP-08)

**Comparable** (`==`, `!=`): both operands have the same type (§6.1) up to optionality and up
to `ref T`/`T`. `none == none` is `true`; `none` equals only `none`. Comparing refs into two
different collections is `E3309`. Comparing function values is `E3007`. Other mismatches are
`E3002`.

Equality at evaluation:

| Values | Equal when |
|---|---|
| `Bool`, `Int`, `String`, `Duration`, enum | same value (strings byte-wise) |
| `Float` | IEEE `==` (`-0.0 == 0.0`) |
| both carry an identity (refs, entries) | same collection and same key |
| records, cases | same type and case, and every field equal (identities ignored) |
| lists, keyed lists | same length and elements equal in order (a keyed list equals a plain list with the same elements) |
| maps | same key set and equal values, **order ignored** |
| tables | same keys in the same order and equal entries |
| ranges | same start and end |
| pairs | component-wise |

**Orderable** (`<`, `<=`, `>`, `>=`): both operands `Int`, both `Float` (an integer literal may
stand for a `Float`), both `Duration`, both `String` (byte order), or both the same `ordered`
enum (declaration order). Anything else, including `Bool`, refs, lists, records and
non-`ordered` enums, is `E3310`. Optionals are not orderable (`E3403`). Comparisons do not chain
(GRAMMAR.md).

---

## 8. Enums and variants

### 8.1 Enums

- Members are written `Enum.m` or bare with an expected type (§4).
- Members have built-in members: `.name: String` (Canon name), `.index: Int` (position from
  0, retired members included), `.wire: String` (the wire value), and `.code: Int` with
  `@codes` only (`E3003` otherwise).
- `ordered` makes the enum orderable (§7.5).
- A retired member (`retired FIRE = 1`) still exists for `match` exhaustiveness (§12.6) and
  for generated code. Using it in a value, in Canon source or loaded data, is `E3506`, except
  inside a retired table entry (LOCK.md: a retired entry may name anything).
- An enum with `@codes(T)` requires every member to have an `= integer` value that fits `T`
  (`E3201`); codes are unique (`E3102`).

### 8.2 Variant values

- A case literal is `c { fields }` or `V.c { fields }`. A case whose fields all have defaults
  (or that has no fields) may be written bare: `nothing`, `item` ≡ `item {}` (TYP-19). A bare
  case with a required field is `E3302`.
- The static type of a case literal is the case type `V.c`, which is assignable to `V`.
- `self` inside a case body is the case value, typed `V.c`.

### 8.3 Working with variant values

- `v.kind` has the type `Kind(V)`: an enum whose members are the case names, in declaration
  order. It is not writable in Canon source (generated code calls it `<Variant>Kind`). Its
  members are resolved contextually: `v.kind == item`.
- `v is c` tests the case. `c` must be a case of `v`'s variant (`E3605`). `is` on a
  non-variant is `E3605`; on an enum, write `==`.
- Fields and methods of a case are reachable only on a value typed `V.c`: a case literal, a
  match binding (`item(r) => r.count`) or a path narrowed by `is` (§6.6):

```
if reward is item { total += reward.count }        // reward narrowed to Reward.item
```

- On a value typed `V`, `.f` for a case field is `E3003`, with the hint "narrow with `is` or
  `match`". Built-in members (`.kind`) are always available.

---

## 9. Collections

### 9.1 Lists and keyed lists

- `[T]` is an ordered list. `xs[i]` indexes from 0; a negative index counts from the end.
  `xs[r]` with a `Range` is a slice (GRM-13).
- `[T] keyed by f` requires `T` to be a record with a field `f` whose type is `String`, an
  integer type, an enum or a `ref`, possibly refined (`E3012`). Keys are unique (`E3102`, at
  evaluation).
- On a keyed list, `xs[k]` and `xs.k` look up **by key** (like a table), and positional access
  is `xs.at(i)`. A `Range` index is still a positional slice. The list methods of STDLIB.md
  apply to its elements. This keeps expression indexing consistent with value paths (API-02),
  where `[n]` on a keyed list is a key.
- A keyed-list `let` may receive elements from `entry` declarations, like a table (§9.3).

### 9.2 Maps

- `{K: V}` keeps insertion order. `K` must be `String`, an integer type, an enum, a `ref`, a
  `Kind`, a `LitUnion` whose base is one of these, or a `DepUnion` (`E3011`). Refinements on
  `K` are allowed and checked per key.
- `m[k]` reads an entry (`E4002` if missing); `m.get(k)` returns `V?`.

### 9.3 Tables

- `table T` and `stable table T` require `T` to be a record type (`E3013`).
- `stable table T` is allowed only as the whole declared type of a top-level `let` (`E6003`,
  LOCK.md), because the lock names it by value.
- Every entry has `.id: String` (its key) and `.retired: Bool`.
- `t.k` and `t[k]` look up by key (`k` an identifier, `String` or `ref T`); `t.at(i)` by
  position.
- `entry t.k { … }` requires `t` to be a top-level `let` of the same package whose type is a
  table or a keyed list, and whose initializer is a table literal or a list literal (possibly
  empty: `{}`, `[]`); anything else is `E3103`. Entries are added to that literal, so a loaded or
  computed collection takes none. Its body is a record literal of `T`. For a table, `k` is the
  entry key; for a keyed list, `k` is the value of the key field, which the body must then omit
  (it is filled from `k`; giving it is `E3321`). Keys must be unique across the literal and every
  `entry` (`E3101` for tables, `E3102` for keyed lists, naming both locations; static when both
  keys are written in source). `retired entry` is allowed only on tables (keyed lists have no
  retirement, LOCK.md §1): elsewhere it is `E3103`.
- The keys of a table literal are identifiers; any reserved word the grammar accepts as a name
  is allowed.

### 9.4 Collections of T

A **collection of `T`** is a table of `T` or a keyed list of `T`. Load.defines tables are
collections of `Define`. Only collections can be referenced (§10).

---

## 10. References

### 10.1 Meaning

`ref T` holds the key of an entry of one collection of `T`. Statically it behaves like `T`
wherever `T` is expected (implicit dereference, §6.2), and `r.f` reads the entry's field. `r.id`
is the key for a table target. Wire form: WIRE.md.

### 10.2 Target resolution (RES-03)

`ref X` is resolved from `X`:

- If `X` names a top-level `let` (of this package, `local` included, or imported, or qualified
  `pkg.v`), or a path `v.f.g` through record fields of a top-level `let`, the target is that
  collection. It must be a collection (§9.4), else `E3504`.
- If `X` names a record type `T`, the target is searched **level by level**. The first level
  with at least one candidate decides; it must have exactly one (`E2103`, listing the
  candidates and asking for `ref name`):
  1. **Enclosing instance.** Let `D` be the record or case whose field type contains this
     `ref`. Candidates: fields, declared in records of the same package, whose type is a
     collection of `T`, in a record type that contains `D` (through fields, lists, maps,
     optionals and variant cases). The ref is resolved **per instance**: against that field of
     the nearest enclosing instance (EVALUATION.md §3.4). A ref in a function signature or a
     `let` annotation skips this level.
  2. **Package.** Top-level lets of the package, `local` included, whose type is a collection of
     `T`.
  3. **Imports.** Public top-level lets of imported packages whose type is a collection of `T`.
- No candidate at any level is `E2103`. Anything else after `ref` is `E3504`.
- A level-1 ref is bound per instance at evaluation; dereferencing or verifying one that no
  enclosing instance has bound is `E3505`, as EVALUATION.md §3.4 specifies.

Examples: `next: [ref Status]` in taxonomy.canon resolves at level 2 to `statuses`. `parent: ref
TalentNode?` in rules.canon resolves at level 1 to `GuildTalentTree.nodes`. `ref items` names
the collection directly.

A public type may refer to a `local` collection of its package (`Param` uses `ref monsters`,
RES-07). Other packages then validate against that collection.

### 10.3 Keys and existence

- A ref is written with a key: a contextual identifier, a string or integer literal of the key
  type (§4.1), a value of type `T` converted to `ref T` (§6.2), or another `ref` of the same
  target.
- Existence is checked at evaluation: a missing key is `E3501` at the ref's location. A ref
  from a live entry to a retired entry is `E3502`. A converted entry that does not belong to the
  target is `E3503`. EVALUATION.md §5 says when.
- Refs may form cycles (`next: [ref Status]`), since a ref is a key.

---

## 11. Parameterized records and dependent types

### 11.1 Declarations

- `record R(p: P, …) { … }` and `type F(p: P, …) = type` take **value** parameters. `P` is a
  record type or a `ref` type (DEP-05).
- The parameters are in scope in field types, defaults and checks of `R`, and in the body of
  `F`.
- `R(args)` and `F(args)` need exactly the declared number of arguments, each checked against
  its parameter type (a `ref` argument dereferences, so `Param(eventType)` with `eventType: ref
  eventTypes` is fine). Wrong arity or type is `E3806`.
- An argument must be a stable path rooted at a type parameter, an **earlier** field of the
  same record, or the binder of a dependent map (§11.5). A later field, the field itself, or any
  other expression is `E3805` (for fields) or `E3803` (otherwise).

### 11.2 Type functions

The body of a type function is a type, possibly a **type-level `match`**:

```
type Param(e: EventType) = match e.param {
  monster     => ref monsters
  none_, stat => Never
  …
}
```

- The scrutinee must be a stable path of enum or `Bool` type rooted at a parameter (`E3803`).
- Patterns are members of that enum (or `true`/`false`), or `_`. Exhaustiveness follows §12.6
  (`E3601`); duplicates are `E3602`.
- Result types may not have a refinement that depends on a parameter (`E3803`). They may be
  any other type, including `Never`, refs to `local` collections and other type applications.

### 11.3 Static view of parameterized records (TYP-18)

`R(x)` has the static identity `R(_)` (§6.1). Field reads on it give the field's declared type
with parameters erased: a field whose type mentions a parameter has the static type of §11.4.

### 11.4 Static view of dependent values (DEP-01)

A field or value whose declared type is a type application `F(args)` (or contains one) has the
static type `DepUnion(F)`, printed `F(*)`. It supports only:

- `==` and `!=` with the same `DepUnion` or with `none`, and flow narrowing on `!= none`;
- interpolation and `String(x)`;
- being passed to `fail`/`warn`, stored in a field or list of the same declared type, or
  returned from a function whose return type is that `DepUnion`'s type application.

Anything else (field access, arithmetic, passing it as a `ref items`) is `E3804`. There is no
branch narrowing in this version: `match` on the discriminating field (`e.param`), not on the
dependent value.

**Literals.** When a literal gives a value to a dependent field, the value is checked against
the union of the branches: a bare identifier stays **symbolic**, and string and integer
literals are kept as written. The value is converted to the computed branch at evaluation
(§11.6).

### 11.5 Dependent maps

`{k in c: T(k)}` is a map keyed by `ref c`, where `c` names a collection (§10.2). The value type
`T(k)` may use the binder `k`. Statically the map is `{ref c: DepUnion}` if `T(k)` depends on
`k`, else `{ref c: T}`. Its literal keys are resolved against `ref c`. Each value is checked at
evaluation against `T` applied to its key (§11.6). A map whose **key** type is a dependent
type (`{SpecificKey(e): …}`) has `DepUnion` keys.

### 11.6 Evaluation of dependent types (DEP-02)

- Type functions are evaluated at evaluation time, in verification (EVALUATION.md §5), once per
  value, with the actual argument values. The steps count toward the budget.
- A value that does not fit the computed type is `E3802`: "value does not match
  `Param(COMBAT_KILL_FFA)` = `Never`". Symbolic identifiers are resolved against the computed
  type there (member, case or key), and fail with `E3802` if the computed type has no such
  name. A key that names no entry of a computed `ref` target is `E3501`.
- `none` is always valid for an optional dependent field.
- A **non-optional** field whose computed type is `Never` makes the value unbuildable:
  `E3801` at the value (TYP-13).

### 11.7 What other documents do with them

CODEGEN.md emits the union of the branches with a branch enum named `<Alias>Branch` (DEP-03,
DEP-04, CODEGEN.md §5.6); VIEWMODEL.md encodes the type function (VM-02). Neither changes the rules
above.

---

## 12. Functions, lambdas and control flow

### 12.1 Declarations

- Parameter and return types are required. A parameter default is a constant expression
  (§15, `E3015`) checked against the parameter type.
- Top-level functions, methods and `export fn`s share the namespace rules of §3. There is no
  overloading of user functions.
- Every path through a function body that can complete normally must end in `return e`
  (`E3006`). `return` without a value in a function is `E3019`.
- Recursion, including mutual recursion, is allowed (the budget bounds it).
- Methods (`fn m(self, …)`) exist in record and case bodies. Inside them, fields are in scope
  (§3.3). `self` must be the first parameter and only there.

### 12.2 Calls

- `f(a, b, name: c)`: positional arguments first, then named ones. An unknown name, a
  parameter given twice, a missing parameter without default, or too many arguments is
  `E3004`. Calling something that is not a function is `E3005`.
- Built-in functions and methods are generic (STDLIB.md). Type parameters are bound from the
  arguments left to right, **non-lambda arguments first**, then lambdas are checked with the
  bound parameter types. A type parameter that appears only in a lambda's result is taken from
  the lambda body. If a type parameter is still unbound, the expected type of the call is used,
  else `E3008`. Refinements are dropped when binding (`[Int(1..)]` binds `T = Int`). Refs bind
  as refs: `reachable(from: initialStatus, next: .next)` binds `T = ref Status`.
- The only overloads are those listed in STDLIB.md (arity overloads such as `first()` /
  `first(pred)`, and `next:` accepting `fn(T) -> [T]` or `fn(T) -> T?`).

### 12.3 Function types (GRM-15)

- `fn(T, U) -> R` is a type. It may be used only as the type of a function parameter or of a
  local `let` or `var`. Anywhere else (record field, top-level `let`, `const`, element of a
  list or map, return type) is `E3306`. Function values are never emitted.
- A top-level `fn` name used as a value has its function type. Methods and built-in functions
  cannot be used as values (`E3016`); write a lambda (`x => abs(x)`) or the shorthand
  (`.minutes()`).

### 12.4 Lambdas

- `x => e`, `(a, b) => e`, and the shorthand `.f` / `.m(args)` (meaning `x => x.f`).
- A lambda needs an expected function type; its parameters take the expected parameter types.
  Without one it is `E3008`. The body is checked against the expected result type if that type
  is known, else synthesized.
- A two-parameter lambda checked against `fn(Pair(A, B)) -> R` destructures the pair.
- A lambda captures by value (§6.6 for facts). It cannot assign: its body is an expression.

### 12.5 Pairs

`Pair(A, B)` is produced by `enumerate()`, `zip()`, `pairs()` and map iteration. It cannot be
written as a type. It is consumed by two-name binders: `for i, x in …`, comprehension `for k,
v in …`, and two-parameter lambdas. Binding a pair to one name is allowed; the name can then
only be passed on. Its text form is `(a, b)` (STDLIB.md).

### 12.6 `match` (TYP-20)

- Scrutinee types: an enum, a variant, `Bool`, `Kind(V)`, or an optional of these (`E3604`
  otherwise).
- Patterns: members (enum, `Kind`), cases with an optional binding `c(x)` (variants), `true` /
  `false`, `none` (optionals only), `_`. Qualified forms `E.m`, `V.c` are accepted. A pattern that
  is not of the scrutinee's type, or a binding on a non-variant pattern, is `E3603`. No other
  literal patterns in this version.
- A binding `c(x)` gives `x` the case type `V.c`.
- **Exhaustiveness**: every member or case, **retired ones included**, plus `none` for an
  optional, must be covered, or `_` must be present (`E3601`, listing what is missing). A
  pattern already covered by an earlier arm is `E3602`. A `_` when everything is covered is
  `W3601`.
- A `match` expression's arms are checked against the expected type, or joined (§6.4). A
  `match` statement's arms may be blocks.

### 12.7 Statements

| Statement | Rule |
|---|---|
| `let x = e`, `let x: T = e` | `x` is immutable; without `T`, `e` is synthesized |
| `var x = e`, `var x: T = e` | the declared or synthesized type is fixed for `x`'s scope |
| `x = e`, `x op= e` | `x` must be a `var` (`E3017` for lets, parameters, loop variables, fields) |
| `x[i]… = e` | the root must be a `var`; any depth of indexes (`m[k][0] = v`) |
| `x.f = e` | `E3307` (records are values: rebuild with spread) |
| `for x in e` | `e` is a list, keyed list, table (entries), `Range` (`Int`) or a list of pairs |
| `for a, b in e` | `e` is a map (key, value) or a sequence of pairs; anything else `E3018`. A map needs two names: `for k in m` is `E3018` (use `m.keys()`) |
| `while c`, `if c` | `c ⇐ Bool` |
| `break`, `continue` | inside a loop (`E1134`, GRAMMAR.md) |
| `return e` | only in a function (`E1135`, GRAMMAR.md); a function's `return` needs a value (`E3019`) |
| expression statement | must be a call to a user function, `fail` or `warn`; any other expression is `E3020` (it would have no effect) |
| `expect …` | only in test blocks (`E1130`, GRAMMAR.md; EVALUATION.md §10) |

---

## 13. Other types

### 13.1 Aliases

- `type A = T` names `T`; `A` is the same type as `T` everywhere (§6.1).
- A type function (§11) is an alias with parameters.
- An alias that refers to itself, directly or through other aliases, without passing through a
  record, is `E3021`.
- A record that contains itself through fields without an optional, list, map or ref on the
  way can never be built: `E3022`.
- An alias of a record can be used in a typed literal (`Cfg { … }`). An alias is never callable
  as a conversion function.

### 13.2 String-literal unions (TYP-09)

- `A | "lit" | …`: the alternatives other than the first are string literals. `A` must have a
  string wire form: `String` (possibly refined), an enum, a `ref`, or a type application whose
  branches do (`E3002` otherwise). `Never | "default"` accepts only `"default"`.
- A value of the union is either an `A` or one of the literals. Such unions are valid map keys.
- A string literal equal to one of the literals is that literal, even if `A` would also accept
  it (TYP-09: "the literal wins").
- Operations: `==` / `!=` with a string literal of the union or with an `A`, interpolation,
  `String(x)`.

### 13.3 `Range` (TYP-12)

- A `Range` value is an **integer** range: `start`, `end` (exclusive) and whether the end is
  open. `a..=b` stores `end = b + 1` (`E4101` if `b` is the largest `Int`). `a..` is open.
- `r.start: Int`; `r.end: Int` (`E4002` on an open range at evaluation).
- Ranges of `Float` or `Duration` exist only inside refinements; as values they are `E3025`
  (this narrows TYP-12, which allowed `Duration` range values).
- A range without a start (`..b`) is allowed only as a slice index; as a value it is `E3025`.

### 13.4 Assets (TYP-21, DECISIONS 19)

- `asset(root, ext: [a, b])` is `String` with an asset refinement. `root` is a path string in
  the `load` path syntax (`"@resource/Icon/Item"`), `ext` a list of symbols (bare extensions,
  without the dot). Bad arguments are `E3704`.
- A value is a path relative to the root, with `/` separators and no empty segment, `.`, `..`,
  leading `/` or `\` (`E3703`).
- Its extension (after the last `.` of the last segment) must be one of `ext`, compared exactly
  (`E3702`).
- The file must exist under the root, matched **exactly and case-sensitively**, byte for byte,
  on every platform. A file that differs only in letter case is a missing file: `E3701`. Legacy
  case drift is fixed by a script, never tolerated by the language (DECISIONS 19).
- Checked at evaluation, at storage points (EVALUATION.md §5). Directory listings may be cached
  per build.

### 13.5 `Never`

`Never` has no values. `Never?` accepts only `none`. A non-optional field of type `Never`
(directly or computed, §11.6) is `E3801`.

### 13.6 `_`

`_` is allowed only as (part of) a widget parameter type (`E3002` elsewhere). It matches any
type (VIEWMODEL.md).

---

## 14. Runtime inputs: typing (TYP-17)

EVALUATION.md §11 gives the declaration rules and their codes (`E19xx`). For typing:

- An input field (`apiKey: input String(1..)? from env "…"`) **does not exist in Canon
  values**. It is absent from literals, defaults, spreads, loaded data and the text form.
- Giving it in a record literal is `E3312`. A loaded object with a key equal to its wire name
  is `E3312` (WIRE.md).
- Reading it (`config.gen.apiKey`, or `apiKey` in the record body) is `E3313`, in any
  expression, check, function or view.
- A record whose only fields are inputs still has a value: `gen: Gen = {}` is valid.

---

## 15. Constants and top-level values

- `const NAME = e` is inferred by synthesis. `e` is a **constant expression**: literals, other
  constants (any package), operators, collection literals, and built-in functions and methods
  (TYP-14). A call to a user function, `load`, a `let`, or anything else is `E3015`.
- Constant expressions are also required for refinement bounds, parameter defaults,
  `@codes` values and enum member values (`E3015`).
- Constants are evaluated when first needed, including during type checking. A cycle between
  constants is `E4301` (EVALUATION.md §3).
- A public top-level `let` needs a type annotation (`E3001`). A `local let` without one is
  synthesized when first needed; a cycle among such inferences is `E3008`.
- A field default (TYP-15) may use constants, **earlier** fields of the same record, record
  parameters, and built-in functions; anything else (a package `let`, a user function, a later
  field) is `E3010`.

---

## 16. `@deprecated` (TYP-22)

- A `@deprecated("why")` field, member or entry is still loaded, type-checked, verified,
  emitted and fingerprinted. It has no other effect.
- In **Canon source**, giving a deprecated field a value in a literal, using a deprecated enum
  member, referencing a deprecated entry, or reading a deprecated field in an expression is
  `W3301` at that use. Loaded data never triggers it (legacy files are full of such fields).
- `W3301` replaces the `W1003` proposed in TYP-22, because FMT-03 also claims `W1003`.

---

## 17. Examples under strict optionals

Every place in `examples/` that DECISIONS 17 affected proves presence explicitly. The forms in use
(lines as of 2026-09-23):

| File:line | Written | Why it type-checks |
|---|---|---|
| teamboard/taxonomy.canon:325 | `columns.active().first()!.statuses[0]` | `!` (§6.5); `E4001` if the deck had no column |
| teamboard/taxonomy.canon:329 | `severities.active().first(.default)!` | `!`; the package check guarantees one default |
| teamboard/taxonomy.canon:346 | `check columns.active().first()!.statuses.len() == 1` | `!` |
| teamboard/taxonomy.canon:396–397 | `export fn columnOf(s: ref Status) -> ref Column?` returns `columns.active().first(…)` | a `Column?` is a `ref Column?` (rows `T ≤ ref T` and `S? ≤ T?` of §6.2) |
| balance/parity/sweep_plan.canon:143 | `etcJobs.first(j => j.id == job)!` | `!` |
| balance/parity/sweep_plan.canon:148–149 | `let base = etcJobs.first(…)?.jobBase`, then `if base != none and base != job` | chaining ends on the optional `jobBase` (`String?`, §6.5); `and` narrows `base` |
| balance/parity/sweep_plan.canon:169 | `if link != none { return kind in (LINK_FAMILIES.get(link) ?? [link]) }` | `if` narrowing of a parameter; `??` |
| balance/parity/sweep_plan.canon:260–261 | `w?.item.id ?? ""`, `w?.item.value ?? 0` | one `?.` on the optional `w`; `item` is not optional (§6.5) |
| resource/rules/rules.canon:86–88 | `let anyJob = jobs.values().first()`, then `if anyJob != none { … anyJob.compared() … }` | `if` narrowing of a local |
| resource/rules/rules.canon:171 | `if tier != none and not tier.totems` | `and` narrowing |
| resource/adventurequest/adventurequest.canon:183 | `[a.priority for a in amps if a.priority != none]` | comprehension `if` narrowing: `[Int]` |
| resource/events/event.canon:172 | `itemId: II_GEN_MAT_MOONSTONE` | a symbolic key (§4.1): no optional involved |

---

## Diagnostics

Severity `error` unless stated. "When" is static unless it says "evaluation".

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E2001 | error | two packages in one directory, neither being the directory's (§3.1) |
| E2002 | error | cyclic imports |
| E2003 | error | import of a package that does not exist |
| E2004 | error | selective import of a missing or `local` name |
| E2005 | error | two imports, or an import and a declaration, bind one name |
| E2006 | error | package line of a descendant or unrelated package |
| E2101 | error | §4.2 |
| E2102 | error | name found by no step of §3.3 |
| E2103 | error | §10.2 |
| E2104 | error | §3.6 |
| E2105 | error | field or method `id`/`retired` on a table element, `kind` on a case |
| E2106 | error | duplicate declaration, field, member, case, method or parameter |
| E2107 | error | redeclaration in one block |
| E2108 | error | `self` outside a record or case body |
| E2109 | error | `it` outside a refinement predicate |
| E2110 | error | wrong kind of name for the position |
| E3001 | error | SPEC §4.2 |
| E3002 | error | assignability failure (§6.2), and the misc cases listed in §5.2, §13 |
| E3003 | error | unknown member, case field on an un-narrowed variant, `.code` without `@codes`, `m.k` on a map |
| E3004 | error | §12.2 (a positional argument after a named one, or a name given twice, is `E1121`, GRAMMAR.md) |
| E3005 | error | calling something that is not a function |
| E3006 | error | §12.1 |
| E3007 | error | §7.1, function equality |
| E3008 | error | `none`, `[]`, lambda without expected type, unbound type parameter, both operands context-dependent, inference cycle |
| E3010 | error | §15 |
| E3011 | error | §9.2 |
| E3012 | error | §9.1 |
| E3013 | error | §9.3 |
| E3015 | error | §15, §7.4 |
| E3016 | error | §12.3 |
| E3017 | error | §12.7 |
| E3018 | error | §12.7 |
| E3019 | error | bare `return` in a function (§12.1) |
| E3020 | error | expression statement that is not a call |
| E3021 | error | §13.1 |
| E3022 | error | §13.1 |
| E3023 | error | §7.4, empty range |
| E3025 | error | §13.3 |
| E3101 | error | static or evaluation (§9.3) |
| E3102 | error | evaluation: keyed list keys, `@codes`, `@stable` (LOCK.md) |
| E3103 | error | §9.3 |
| E3201 | error | sized integer, `Duration` range (§7.2) or `@codes` range; static for literals, else evaluation |
| E3202 | error | NaN or infinity stored (evaluation) |
| E3204 | error | range or length refinement (static for literals, else evaluation) |
| E3205 | error | regex refinement |
| E3206 | error | `where` predicate is false (evaluation) |
| E3301 | error | unknown field in a literal (and in loaded data, WIRE.md) |
| E3302 | error | missing required field |
| E3303 | error | §5.2 |
| E3304 | error | §5.2 |
| E3305 | error | §5.2 |
| E3306 | error | §12.3 |
| E3307 | error | §12.7 |
| E3308 | error | §6.4 |
| E3309 | error | §7.5 |
| E3310 | error | §7.5 |
| E3311 | error | §5.3 |
| E3312 | error | §14 |
| E3313 | error | §14 |
| E3314 | error | STDLIB.md `sum` |
| E3320 | error | §5.2 |
| E3321 | error | §5.2 |
| E3322 | error | map literal or comprehension (static or evaluation) |
| E3323 | error | §5.2 |
| E3401 | error | `T??` is not a type |
| E3402 | error | §6.5 |
| E3403 | error | §6.5 |
| E3501 | error | dangling ref (evaluation) |
| E3502 | error | evaluation |
| E3503 | error | `T → ref T` conversion (evaluation) |
| E3504 | error | §10.2 |
| E3505 | error | level-1 ref outside an instance (evaluation) |
| E3506 | error | retired member or case used in a value (evaluation) |
| E3601 | error | §12.6, §11.2 |
| E3602 | error | a `match` pattern already covered |
| E3603 | error | a pattern that is not a member or case of the scrutinee's type |
| E3604 | error | `match` on a type that cannot be matched |
| E3605 | error | §8.3 |
| E3701 | error | missing file, including a letter-case-only match (evaluation) |
| E3702 | error | evaluation |
| E3703 | error | evaluation |
| E3704 | error | static |
| E3801 | error | evaluation |
| E3802 | error | evaluation |
| E3803 | error | §11.1, §11.2 |
| E3804 | error | §11.4 |
| E3805 | error | §11.1 |
| E3806 | error | §11.1 |
| W3301 | warning | §16 |
| W3401 | warning | §6.5 |
| W3601 | warning | §12.6 |

Codes referenced but owned elsewhere: `E1103`, `E1105`, `E1114`, `E1121`, `E1130`, `E1134`, `E1135`
(GRAMMAR.md); `E6003` (LOCK.md); `E3203` and `E3315`–`E3318` (WIRE.md §13: wire decoding and
encoding; `E3316` also covers an `@json` form that does not apply to the field's type, WIRE.md
§4.1), `E7002` (WIRE.md); `E4001`, `E4002`, `E4101`, `E4301` (EVALUATION.md); `E19xx`
(EVALUATION.md). The single catalogue of every code is [ERRORS.md](ERRORS.md).
