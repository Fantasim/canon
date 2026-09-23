# Canon view model

Status: **normative**, for language version 0.1. Companion to [SPEC.md](../SPEC.md) §16 (Views)
and §17 (Translations). This document is the whole contract between the compiler, which writes
the view model, and the **studio**, the generic editor that renders it. Two teams build from it
without talking to each other: one writes `internal/gen/view` (and the view checks of
`internal/check`), the other writes the studio.

Settled here: AUDIT VIEW-01…10 and VM-01…07, the 50 view gaps of MOCKUP-GAPS.md (accepted by
DECISIONS 20; gap 34 follows DECISIONS 19), and the view parts of DECISIONS 7 and 9.
Translation keys, translation files and `canon i18n` are in [I18N.md](I18N.md).
The machine-checkable form of §12 is [viewmodel.schema.json](viewmodel.schema.json) (JSON Schema
2020-12). The golden is [`examples/pipeline/expected/potion.view.json`](../examples/pipeline/expected/potion.view.json).

"MUST", "MUST NOT", "SHOULD" and "MAY" are used in their RFC 2119 sense. Every numbered rule
(`G3`, `C12`, …) is a test target.

Documents this one relies on, and what it assumes from them:

| Document | Assumed here |
|---|---|
| GRAMMAR.md | the view grammar (§5.6), the widget grammar with `default` (§5.3), `check … at` (§5.5), contextual keywords (§4.2), `@menu` (§8.3) |
| TYPES.md | type identity, aliases, refinements (§7.4), refs and their targets (§10.2), type functions and dependent maps (§11), assets (§13.4), `Never`, `@deprecated` (§16); view expressions are type-checked with its rules |
| EVALUATION.md | evaluation of view expressions, poisoning, provenance, layers |
| WIRE.md | wire names, units, `@json(int)`, `@json(bits)`, canonical JSON bytes (§7) |
| API.md | `ViewModel` (§5.4), value paths (§6), editability (§7), `Edit` and its cascades (§8), `Evaluate` (§11), the finding JSON (§4.2) |
| I18N.md | the key catalogue, source texts, fallbacks, number formatting per language |

---

## Contents

1. [Division of labour](#1-division-of-labour)
2. [Terms](#2-terms)
3. [Views in the language](#3-views-in-the-language)
4. [Choosing a control](#4-choosing-a-control)
5. [Layout of a record](#5-layout-of-a-record)
6. [Variants and dependent values](#6-variants-and-dependent-values)
7. [Collections](#7-collections)
8. [Pickers and the search index](#8-pickers-and-the-search-index)
9. [Texts, numbers and durations](#9-texts-numbers-and-durations)
10. [Navigation](#10-navigation)
11. [Editing](#11-editing)
12. [The view model document](#12-the-view-model-document)
13. [Evaluate](#13-evaluate)
14. [Versioning](#14-versioning)
15. [Worked examples](#15-worked-examples)
16. [Diagnostics](#16-diagnostics)

---

## 1. Division of labour

The studio renders three inputs and derives no rule from the language:

| Input | Produced by | Holds | Changes |
|---|---|---|---|
| the **view model** of each package (§12) | `emit view`, or `Project.ViewModel` (API.md §5.4) | types, resolved views and controls, values, usage, search indexes, texts in every language, findings | per build |
| **values** | `Project.Value` (API.md §5.2) | the data, its origin (set or defaulted), its editability (read-only, layered, key) | per edit |
| **live state** | `Project.Evaluate` (API.md §11, §13 here) | rendered titles and `show` lines, `when` results, findings of a draft | per committed edit |

- **R1.** The compiler resolves every rule that depends on the language: the control of every
  field (thresholds included, VIEW-05), labels (humanized defaults included), section order,
  relevance (usage and the "More" cut), search index rows, rendered templates, findings. The
  studio MUST NOT re-derive any of them from types.
- **R2.** The studio applies only the **display rules** this document states for it, each of
  which reads the view model, a value and the live state: which case fields exist for the
  current case (§5.7), promotion of rare fields that a value sets (§5.3), un-hiding fields that
  carry a finding (§5.10), number and duration formatting (§9), filters and sorting over loaded
  rows (§7).
- **R3.** The studio MUST NOT evaluate Canon expressions. The single exception is the `{index}`
  placeholder of a `step` text (§7.7), which is plain substitution.
- **R4.** Views never change validation, generated code or data (SPEC §16): deleting every view
  changes only the view model.

## 2. Terms

| Term | Meaning |
|---|---|
| view | a `view` declaration (§3) |
| target | what a view describes: a record, a variant, a variant case, an enum, or a define table |
| item | one line of a view: `title`, `group`, a field line, `show`, … |
| field key | the name of a field in a view's `fields` map: the field name, or for a case field of an inline variant `<variantField>.<case>.<field>` (§5.7) |
| shape | a record type together with the current case of each of its inline variant fields (§5.7) |
| shape key | the text form of a shape: `""`, or `kind=IK1_WEAPON`, or `kind=IK1_GENERAL/kind2=IK2_MATERIAL` |
| active | not retired (enum members, variant cases, table entries) |
| text reference | a string `"<package>:<key>"` naming a translatable text (§12.2) |
| studio chrome | texts the studio owns and translates itself: "More", "Other", "Unused fields", "Add", "Any", "(none)", "none", "Unset", "Yes", "No", "Clear", "Reset to default", "No menu", "min", "max", "#1" |

---

## 3. Views in the language

### 3.1 Grammar

The syntax of views, widgets and `check … at` is GRAMMAR.md's (§5.6 `viewDecl`, §5.3
`widgetDecl`, §5.5 `checkDecl`); this document gives their meaning. In short:

```
view Type { … }        view Variant.case { … }
  title "tpl"   subtitle "tpl"   singular "text"   plural "text"   preview expr
  menu Menu icon Icon   columns { f 200, g }   search { e, … }   filters { f multi, g }
  group id "Label" ["intro"] [advanced] [when expr] { field items and show lines }
  show [id] "Label" "tpl"
  [field] name ["Label"] [{ prop: value, … }]
widget name(value: T [, siblings: [T]]) [default]
check [name:] cond [at field] else "message"
```

`value`, `siblings` and `default` are contextual keywords of `widget` (GRAMMAR.md §4.2).

- **G1.** Inside a view the words `title subtitle singular plural preview menu icon columns search
  filters group show advanced when field multi` are keywords only at the start of an item or
  where the grammar expects them (GRM-17, GRAMMAR.md §4.2). A field whose name is one of them is
  written with the `field` escape: `field title "Title"`. `field` followed by a string, `{` or a separator is a
  field named `field`.
- **G2.** `show` takes an optional id: `show avg "Average hit" "{(attackMin + attackMax) / 2}"`.
  `default` in `widgetDecl` and `at` in `checkDecl` are contextual.
- **G3.** Views are top-level declarations; a `view` inside another declaration is a syntax error.

### 3.2 Targets

`view X { … }` and `view X.c { … }` resolve `X` in the package namespace:

| `X` names | Target | Allowed items |
|---|---|---|
| a record type | the record | every item |
| a variant type, `view V` | the variant | `title subtitle singular plural preview search columns filters menu`, and member items naming **cases** (label, `help`, `icon`, `tone`) |
| a variant case, `view V.c` | the case | `title subtitle singular plural preview` and every layout item (groups, `show`, field lines) over the case's fields |
| an enum type | the enum | member items naming **members** (label, `help`, `icon`, `tone`) |
| a top-level `let` whose value comes from `load.defines` (MOCKUP-GAPS 19) | the define table's entries | `title subtitle singular plural search` |

- **G4.** A view MUST be declared in the package that declares its target (`E1603`, VIEW-04).
- **G5.** A target has at most one view (`E1607`). `view V` and `view V.c` have different targets
  and may coexist.
- **G6.** A view on any other `let`, or on a `let` that is not a define table, is `E1626`. An item
  not allowed for the target is `E1627`.
- **G7.** A define-table view's expressions see `id` (the define name), `value` (its integer) and
  the package scope. Example: `view texts { title "{names.get(id) ?? id}" }`.

### 3.3 Names in a record or case view

A member item, a group entry, a column or a filter names one of:

1. a **field** of the target;
2. a **method** of the target that takes no parameter besides `self` (MOCKUP-GAPS 24); a method
   with parameters is `E1616`; a method in `columns` or `filters` is `E1606` (VIEW-09);
3. for a record with an `@json(inline)` variant field `v`: a **field of one or more cases** of `v`
   (MOCKUP-GAPS 14), searched when (1) and (2) fail. Nested inline variants inside those cases are
   searched too, depth first.

- **G8.** A name found by none of these is `E1602`. A case-field name found in several cases with
  different types (after alias expansion, refinements kept) is `E1628`: name them in the case
  views instead.
- **G9.** A name placed twice in one view (two groups, or the same group twice) is `E1605`. A view-level field line without a group (`heal "Heal" { unit: hp }`) sets
  presentation only; it does not place the field.
- **G10.** Only the target's own view and, for case fields, the view of the parent record that
  inlines the variant may give a field its label. A field labelled in both is `E1614`.

### 3.4 Scope of view expressions

`title`, `subtitle`, `preview`, `search` terms, `show` texts, `when` conditions and `step` texts
are expressions or templates (VIEW-02). Their scope is, in lookup order:

1. the magic names, when the value being shown is in such a position: `id` (the key of a
   **table** entry or of a define-table entry), `key` (the key of a **map** value), `index` (the
   **1-based** position of a list element, MOCKUP-GAPS 27);
2. `self` and the fields and methods of the target (a case view: the case's fields);
3. the package scope and imports, as in a method body.

- **G11.** A field named `key` or `index` hides the magic name: the template reads the field, and
  the compiler reports `W1604` at the first template that uses the name. (`id` cannot be hidden:
  a table element may not declare a field `id`, RES-08.)
- **G12.** A magic name that has no value in the position being rendered (`{key}` for a value that
  is not a map value) makes that rendering fail (§9.2).
- **G13.** View expressions are type-checked with TYPES.md: `when` MUST be `Bool`; `preview`
  MUST be an asset type or its optional (`E1612`); each `search` term MUST be a `String`, an
  integer, an enum, a `ref`, an asset, an optional of one of these, or a list of one of these
  (`E1622`). Input fields cannot be read (`E3313`).
- **G14.** Templates are `title`, `subtitle`, the second string of `show`, `step`, and check
  messages. Every other string of a view (labels, `help`, group labels and intros,
  `singular`, `plural`, `placeholder`, `none`) is **plain text**: an unescaped `{` in it is
  `E1615`.

### 3.5 Field properties

A member item's `{ … }` holds properties. Unknown properties, or properties not valid for what the
item names, are `E1613`.

| Property | Value | Valid on | Meaning |
|---|---|---|---|
| `help` | plain text | field, method, enum member, case | overrides the doc comment as help text |
| `unit` | a unit of the studio package | field whose type (optional stripped) is an integer type, `Float`, `Float32`, or a list or map whose elements (values) are | §9.3; other types `E1611`; unknown unit `E1610` |
| `control` | a control name of §4.5 | field | control hint; not accepting the type `E1601`; unknown name `E1609` |
| `widget` | a widget of the studio package | field | §4.6; unknown `E1610`; type mismatch `E1608` |
| `readonly` | `true` / `false` | field | shown read-only (§4.7) |
| `hidden` | `true` / `false` | field, method | not shown (§5.5) |
| `placeholder` | plain text | field with a text or number control | shown in the empty input |
| `when` | `Bool` expression | field, method | shown only while it holds (§5.5) |
| `none` | plain text | optional field | the label of `none` (MOCKUP-GAPS 12) |
| `step` | template over `{index}` only | list field | name of each element in step, list and positional controls (§7.7); another name is `E1623` |
| `icon` | an `Icon` member of the studio package | enum member, case | UI symbol |
| `tone` | a `Tone` member of the studio package | enum member, case | UI tone |

- **G15.** `control` and `widget` on the same field is `E1634`.
- **G16.** Studio names (`unit`, `widget`, `icon`, `tone`, `menu`) resolve against the package named
  by `project.studio`, whether or not it is imported (RES-06). An unknown name is `E1610`.

### 3.6 Other view items

| Item | Meaning |
|---|---|
| `title "<template>"` | the name of a value in lists, pickers, headers, card titles |
| `subtitle "<template>"` | second line in lists and pickers |
| `preview <expr>` | the asset shown next to the value in lists and pickers |
| `singular "<text>"` | used in "Add …" actions |
| `plural "<text>"` | noun shown after the count of a collection of this type (§7.2) |
| `menu <Menu> icon <Icon>` | navigation (§10) |
| `columns { f w?, … }` | table columns for a collection of this type (§7.2); `w` is a width in pixels, 16 to 2000 (`E1613` otherwise) |
| `search { e, … }` | extra search terms of pickers and search boxes (§8) |
| `filters { f multi?, … }` | filters above a table of this type (§7.3) |
| `group id "Label" "intro"? advanced? (when e)? { … }` | a named section (§5.2) |
| `show id? "Label" "<template>"` | a read-only computed line (§5.8) |

- **G17.** Group and show ids are unique within a view (`E1617`). Ids starting with `_` are
  reserved (`E1618`); `_other` names the final group (§5.3) and `_0`, `_1`, … name unnamed `show`
  lines (I18N.md).
- **G18.** Parallel legacy fields (MOCKUP-GAPS 49) are one list of records through
  `@json(pairs:)` (DECISIONS 21, WIRE.md §5.14): `stats: [StatBonus](..=6)`. The view shows that
  list with the ordinary list rules (§5.9); there is no `row` item.

### 3.7 `check … at`

- **G19.** A one-line `check` or `warn` in a record or case MAY end its condition with `at f`,
  naming a field of that record or case (MOCKUP-GAPS 31). Its finding is then reported at the
  value of `f` (its `path` ends with `.f`) instead of at the record. An unknown field is `E1633`.

```
warn unreachable_levels: maxLevel <= levels.len() at maxLevel
  else "maxLevel is {maxLevel} but only {levels.len()} levels exist"
```

### 3.8 Widgets

A widget is a custom editor implemented by the studio under the same name (SPEC §16.11).

- **G20.** Matching (VIEW-07): a field matches parameter type `P` when, after expanding aliases
  and removing refinements, `where` predicates and `keyed by` on both sides, the field's type
  equals `P`, or is `T?` with `T` equal to `P`. `_` matches any type, at any depth
  (`[_]` matches every list). A mismatch is `E1608`.
- **G21.** `siblings: [P]` (MOCKUP-GAPS 28), where `P` is exactly the `value` type, asks the studio
  to pass, next to the value, the values of the same field in every element of the nearest
  collection (list, keyed list, table, map values) that contains the record holding the field, in
  collection order, the value itself included. A record not inside a collection gets
  `[value]`. Another `siblings` type is `E1631`.
- **G22.** `default` (MOCKUP-GAPS 5) makes the widget the control of every field whose type
  matches `P` by G20, in every package, unless the field has a `control` or `widget` property.
  `P` MUST be a named record, variant or enum type, optionally in a list (`E1630`); two default
  widgets for one type are `E1629`. Widgets, default or not, are declared in the studio package
  (DECISIONS 22); a view may still override a default widget with `widget:` or `control:`. The
  studio package declares:

```
/// Hour and minute on one clock input.
widget time_of_day(value: TimeOfDay) default
```

### 3.9 `@menu`

- **G23.** `@menu(<Menu>, icon: <Icon>, label: "<text>")` on a public top-level `let` places that
  value in the navigation (VIEW-03). `icon` and `label` are optional. It overrides the `menu` of
  its type's view. On anything else it is `E1632`.

---

## 4. Choosing a control

The compiler writes a resolved **control** (§12.5) for every field of every view, every value,
every collection element and every map key and value. The studio renders it.

### 4.1 Precedence

- **C1.** For a field, the first rule that applies decides:
  1. the field's `widget` property (§4.6);
  2. the field's `control` property (§4.5);
  3. a default widget matching the field's type (G22);
  4. the type rules of §4.3.
- **C2.** The optional wrapper (§4.4), the unit (§4.8) and read-only reasons (§4.7) are then added
  to the result.

### 4.2 Choices and thresholds

A **choice** control picks one of a finite set: `Bool?` values, enum members, variant cases, or
entries of a `ref` target.

- **C3.** Thresholds count **active** choices (SPEC §16.2): active members, active cases, active
  entries of the target collection as evaluated in this build.
- **C4.** Enums and variant cases share one scale (MOCKUP-GAPS 1): 1 to 4 choices → `segmented`;
  5 to 10 → `select`; more than 10 → `search`.
- **C5.** A `ref` whose target has at most 10 active entries → `select`, each option showing the
  entry's title and preview; more → `search` (a picker over the target's index, §8).
- **C6.** A `ref` into a collection of the enclosing instance (TYPES.md §10.2 level 1) is always a
  `select` whose options are the current keys of that collection in the value being edited.

### 4.3 Type rules

`T` below is the field's type with aliases expanded and the outer optional removed (the optional is
§4.4). "Scalar" means `Bool`, an integer type, `Float`, `Float32`, `String`, `Duration`, an enum,
a `ref`, an asset, or a literal union.

| # | Type | Control |
|---|---|---|
| C7 | `Bool` | `switch` (a `checkbox` in table cells) |
| C8 | enum | choice by C4 over its members |
| C9 | variant | `variant`: a case selector (choice by C4 over its cases) and the current case's fields (§6.1) |
| C10 | `ref` | choice by C5 / C6 |
| C11 | `String` | `input`, with `minLen`, `maxLen` (bytes) and `pattern` (RE2 search, STD-03) from the type |
| C12 | integer type, `Float`, `Float32` | `number`, with `min`/`max` from the type (sized-integer limits included); for an integer type whose range has both bounds and at most 20 values, `stepper: true` (MOCKUP-GAPS 6). `Float` never gets a stepper. |
| C13 | `Duration` | `duration`: a number plus a unit selector (§9.4) |
| C14 | asset | `file`: a file picker limited to the asset root and extensions, with a thumbnail |
| C15 | `T \| "lit" …` | the control of `T` with `pinned` literals (§6.4) |
| C16 | a dependent type `F(args)` | `dependent`: one control per branch (§6.3) |
| C17 | `Never` | `never` (§6.3) |
| C18 | record with at most 6 fields | `section` (inline) |
| C19 | record with more than 6 fields | `card` (collapsible) |
| C20 | `[E]`, `E` an enum, declared a set | `checkboxes` if at most 6 active members, else `chips` |
| C21 | `[E]`, not a set | `chips` |
| C22 | `[ref T]` | `chips`, with a picker over the target |
| C23 | `[S]`, `S` scalar, refined `where it.len() == 2 and it[0] <= it[1]` | `range`: a min and a max input (`strict: true` for `<`) |
| C24 | `[S]`, `S` scalar, whose length refinement has an upper bound `n ≤ 6` | `positional`: `n` inputs in a row, labelled by position (§7.5) |
| C25 | `[S]`, `S` scalar, otherwise | `tags` |
| C26 | `[R]`, `[R] keyed by f`, `table R`, `stable table R`, `R` a record or a variant | `table` (§7.1) |
| C27 | any other list (lists of lists, of maps, of optionals) | `list`: one element control per element, numbered |
| C28 | `{E: S}`, `E` an enum, `S` scalar | `enumRow`: one input per active member, in member order |
| C29 | `{K: R}`, `R` a record or variant (dependent maps included) | `cards`: one card per entry (§7.6) |
| C30 | any other map | `map`: a key/value table; the key and the value get their own controls by these rules |

- **C31.** A record's field count (C18, C19) counts every declared field, inputs and deprecated
  fields included.
- **C32.** A list is **declared a set** (C20) when its `where` predicate is exactly `it.isUnique()`
  or a conjunction (`and`) that has `it.isUnique()` as one operand, or when the field has
  `@json(bits)` (MOCKUP-GAPS 3, 7). `@json(int)` on a `Bool` changes nothing here.
- **C33.** The range form (C23) is recognized in the predicate's conjunction, in any order and
  among other conjuncts: `it.len() == 2` (or `2 == it.len()`) together with `it[0] <= it[1]`
  (or `it[0] < it[1]`, `it[1] >= it[0]`, `it[1] > it[0]`). Nothing else is recognized.
- **C34.** A field whose type admits exactly one value is shown read-only with that value
  (MOCKUP-GAPS 40): an integer, `Float` or `Duration` range with equal bounds; a `where` that is
  `it == c` or `c == it`, or a conjunction with such an operand, where `c` is a constant
  expression; an enum with exactly one active member. The field's view entry carries `single`
  (§12.4). This applies to non-optional fields only.

### 4.4 Optional values

- **C35.** `T?` uses the control of `T` with an `optional` wrapper:
  - `Bool?` is `segmented` over Yes, No, Unset;
  - when the control of `T` is `segmented`, `none` is a final "Unset" segment
    (`optional.unset: "segment"`, MOCKUP-GAPS 2);
  - otherwise a clear button (`optional.unset: "clear"`).
- **C36.** A `T?` whose default is not `none` has three states (MOCKUP-GAPS 11): **default** (the
  field is not set; the control shows the default as placeholder), **value**, and **explicit
  `none`** (shown as the `none` label, visibly different from empty). The wrapper has
  `threeState: true`. "Clear" writes `none` (`Set(path, None)`, API.md E7); "Reset to default"
  removes the field (`Reset(path)`); both are offered.
- **C37.** A `T?` whose default is `none` has two states. "Clear" is `Set(path, None)`, which
  removes the field (API.md E7).
- **C38.** The label of `none` is the field's `none` property if any (translatable,
  `Type.field.none`), else the studio chrome "none".
- **C39.** `Never?` (a dependent branch or a declared type) is the `never` control: the field is
  not shown while its value is `none` (MOCKUP-GAPS 16), and is shown read-only with its finding
  otherwise.

### 4.5 Control hints

The closed list of built-in hints (SPEC §16.3), and what each accepts (VIEW-07 comparison, the
optional stripped):

| `control:` | Accepts | Resolved `kind` |
|---|---|---|
| `switch`, `checkbox` | `Bool` | same |
| `segmented`, `radio`, `select`, `search` | enum, `ref`, variant (applies to the case selector) | same |
| `checkboxes`, `chips` | `[E]` (enum), `[ref T]` | same |
| `input`, `textarea`, `code` | `String` | same |
| `number` | integer types, `Float`, `Duration` | `number` (`duration` for `Duration`) |
| `stepper` | integer types, `Float`, `Duration` | `number` with `stepper: true` (`duration` for `Duration`) |
| `slider` | integer types, `Float`, `Duration`, with both bounds (`E1619` otherwise) | `slider` |
| `color` | `String` whose regex source is exactly `^#[0-9a-fA-F]{6}$`, `UInt32` | `color` |
| `text` | anything | `text` (read-only rendering) |

- **C40.** Any other hint name is `E1609`; a hint that does not accept the type is `E1601`.

### 4.6 Widgets

- **C41.** A widget control is `{kind: "widget", widget, siblings?, fallback}`. `fallback` is the
  control the field would get without the widget (C1 steps 2 to 4). A studio that does not
  implement the widget MUST render `fallback` and SHOULD log it once.
- **C42.** The studio passes the widget the value, the resolved control of its type, the unit, and
  when declared the siblings (G21).

### 4.7 Read-only

- **C43.** A field is shown read-only when its view entry has `readonly` (§12.4): `"view"` (the
  `readonly: true` property), `"deprecated"`, `"single"` (C34) or `"input"` (a runtime input, no
  value at build time, shown as "from environment variable NAME"). It is also read-only when
  `Value.Editable.Mode` is `none` (API.md §7.2 reasons: `computed`, `layered`, `format`, `input`,
  `key`, `pseudo`), and then the studio shows the reason (for `layered`, the layer's name,
  MOCKUP-GAPS 48). A value set by an active layer is `layered` unless `Options.EditLayer` names
  that layer, in which case the edit writes into the layer (API.md W10, W11).
- **C44.** Keys are read-only everywhere; they change through a "Rename…" action that sends
  `Rename` (API.md §8.4, MOCKUP-GAPS 23).

### 4.8 Units

- **C45.** `unit` applies to the scalar itself, to each element of a list, and to each value of a
  map (MOCKUP-GAPS 30, VIEW-08). The resolved control carries `unit` on the control that shows
  the number (the element or value control for collections). Formatting is §9.3.

### 4.9 Controls in table cells

- **C46.** The control of a cell is the field's control, with these substitutions: `switch` →
  `checkbox`; `segmented` and `radio` → `select`; the optional wrapper keeps `unset: "clear"`.
  Which columns are editable is §7.2.

---

## 5. Layout of a record

### 5.1 Sections

- **L1.** A record or case is shown as sections, in this order (MOCKUP-GAPS 9, 36):
  1. the **named groups**, in view order, each with all its entries whatever their usage;
  2. the **`_other`** group: the fields named in no group, in usage order (§5.3), preceded by
     the view-level `show` lines and methods (§5.8). Labelled with the studio chrome "Other";
  3. **More**: the rare fields of `_other` (§5.3), collapsed;
  4. **Unused fields**: the deprecated fields, collapsed, read-only (SPEC §16.2).
- **L2.** A section with nothing to show is not shown. When the only section shown is `_other`,
  the studio MAY omit its heading.
- **L3.** The view model lists these sections for each record and case view (§12.4). The
  `_other` label is studio chrome, not a translation key.

### 5.2 Named groups

- **L4.** A group shows its entries in view order: fields, methods and `show` lines. A field that
  does not exist in the current shape (a case field, §5.7) is skipped; a group left with no
  existing field, method or `show` line is not shown (MOCKUP-GAPS 14).
- **L5.** An `advanced` group is collapsed until opened; the open or closed state is a per-user
  studio preference (MOCKUP-GAPS 37). A group `when` hides the whole group while the condition is
  false (§5.5).
- **L6.** A deprecated field named in a group is not shown there: it goes to "Unused fields"
  (`W1642`).

### 5.3 `_other`, More and usage

- **L7.** For each shape of a record type used by the package's values, the view model's `usage`
  (§12.7) lists the `_other` fields of that shape split in two ordered lists, `main` and `more`
  (MOCKUP-GAPS 8, DECISIONS 7):
  - a field **is set** in an instance when it is present in its source (a field of the record
    literal, a key of the JSON object), after spreads (VM-06); a field filled by its default is not
    set;
  - `count` is the number of instances of the shape; `set(f)` the number of them where `f` is set;
  - `f` is in `main` when `4 × set(f) ≥ count` (at least 25 %), else in `more`;
  - both lists are ordered by `set(f)` descending, then by the order of the view's `_other` list
    (declaration order).
- **L8.** When the package's `usage` has no entry for the shape (no instance), every `_other` field
  is in `main`, in declaration order.
- **L9.** The studio shows `main` in the `_other` group, then, still in `_other`, every field of
  `more` that **this value sets** (`Value.Origin.Kind` is not `default`, API.md §5.2) or that
  carries a finding (§5.10), in `more` order. The remaining `more` fields go under "More".
- **L10.** Usage counts instances reachable from the package's **public** values (VM-06 narrowed:
  test fixtures and local lets do not count). Instances of a type declared in another package are
  counted in this package's `usage` under that type's qualified name: the studio uses the usage of
  the package whose value it shows.
- **L11.** The threshold 25 % (MOCKUP-GAPS 8) replaces the 10 % of VIEW-06.

### 5.4 Unused fields

- **L12.** Every `@deprecated` field of the shape is listed in "Unused fields", read-only, with the
  deprecation text as its help (`Type.field.deprecated`), collapsed by default.

### 5.5 `hidden` and `when`

- **L13.** A `hidden: true` field or method is not shown anywhere, whatever group it is in
  (VIEW-06), except when it carries a finding (§5.10).
- **L14.** A field, method or group with `when` is shown only while its condition is true. The
  condition is evaluated by the compiler (`EvalResult.When`, API.md §11.2); a condition that
  fails to evaluate counts as true (API.md V12). `when` is display only (SPEC §16.6).
- **L15.** `W1641`: a required field without a default that is `hidden` (new values cannot be
  completed in the studio).

### 5.6 Flattening

- **L16.** A group whose only entry is one non-optional field whose control is `section` or `card`,
  and that field has no `when`, is **flattened** (MOCKUP-GAPS 35): the group label replaces the
  record's heading, the record's `title` is not repeated, and the record's own sections become
  subsections of the group. The view model marks such groups `flatten: true`.

### 5.7 Inline variants and case fields

- **L17.** The fields of the cases of an `@json(inline)` variant field `v` are laid out in the
  **parent's** sections, as if they were the parent's fields: named in the parent's groups, or in
  the parent's `_other`. Their field key is `v.<case>.<field>`; for a nested inline variant `w` of
  that case, `v.<case>.w.<case2>.<field>`.
- **L18.** Each such field belongs to one **case path** (`v=<case>` or
  `v=<case>/w=<case2>`). It exists in a value when the value's shape starts with that case path.
  A parent group entry that names a case field expands to every field key with that name, in case
  declaration order (G8 guarantees one type).
- **L19.** The shape key of a value is built from its current cases: for each inline variant field
  of the record in declaration order, `<field>=<case>`, followed by the shape of that case's own
  inline variant fields, joined with `/`. A record without inline variant fields has shape `""`.
  The studio computes it from `Value.Case()` of each inline variant field.
- **L20.** A variant field that is **not** inline is one control (`variant`, C9): its current case's
  fields are shown nested under it, laid out by the case view (§12.4).

### 5.8 `show` lines and methods

- **L21.** A `show` line or a method named in a view is shown read-only as "Label: text", where the
  text is rendered by the compiler (`EvalResult.Show`, API.md §11). It is recomputed after each
  committed edit (§11). A rendering that fails shows "—" and produces no finding
  (MOCKUP-GAPS 38).
- **L22.** A `show` line or method written inside a group appears there, in order. One written at
  view level appears at the top of `_other`, in view order.
- **L23.** A method's label is its view label or its humanized name; its help is the `help`
  property or its doc comment.

### 5.9 Parallel legacy fields

- **L24.** A `@json(pairs:)` field (DECISIONS 21) is an ordinary list of two-field records: its
  control follows §4.3 (C26 `table` for `[StatBonus](..=6)`, with `step` naming its elements), its
  columns are the two fields, and "Add" is refused by the control once the list holds the slot
  count `N` (the type's upper length bound, carried by the type expression's `max`). The wire slots
  are invisible to the studio; the field's `wire` carries them only for display (§12.3).

### 5.10 Findings

Findings come from the view model (`findings`, last build) and from `Evaluate` and `Edit` (live).
Each has a `path` (API.md F1).

- **L25.** Placement (MOCKUP-GAPS 31, 32, 33):
  - a finding whose path is a field, or inside a field (an element, a map entry, a nested value
    that is not shown as its own record), is shown inline under that field;
  - a finding whose path is a record or case value (a one-line check without `at`) is shown as a
    banner at the top of that record's panel, and the fields listed in the finding's `reads`
    (§12.11) are highlighted; a record that is a table row also marks its row;
  - a finding whose path is a collection element marks that element's row and is shown at the top
    of its detail panel;
  - a finding with no path, or with a path the studio does not display, is shown in the findings
    list only, which lists every finding.
- **L26.** A field that carries a finding, directly or below it, is always shown: it leaves "More"
  (L9), `hidden` and `when` do not hide it, its `advanced` group and "Unused fields" open, and a
  collapsed card or section containing it opens (MOCKUP-GAPS 33).
- **L27.** Guidance, not a rule: a check about the elements of a collection uses the block form
  with one `warn(x, …)` or `fail(x, …)` per offending element, so rows can be marked
  (MOCKUP-GAPS 32).

---

## 6. Variants and dependent values

### 6.1 Changing a case

- **D1.** The case selector of a `variant` control sends `SetCase(path, case, nil)` (API.md §8.3,
  E14). The fields kept are those with the same name and type (MOCKUP-GAPS 13).
- **D2.** Before changing the case of a value that sets at least one field, the studio runs the
  edit with `DryRun: true`, and if `Dropped` is not empty it names the fields that would be dropped
  and asks for confirmation. After the change it offers Undo (`EditResult.Undo`).
- **D3.** Changing a case usually leaves required fields of the new case missing; the studio sends
  the edit with `AllowErrors: true` and the missing-field findings guide the user (API.md E19).
- **D4.** In a table, a case column is read-only; the case is changed in the detail panel
  (MOCKUP-GAPS 15).

### 6.2 Views of cases

- **D5.** A case is laid out by `view V.c` when it exists, else by a default layout (all fields in
  `_other`). Case labels, help, icons and tones come from `view V` (member items naming cases);
  a case without a label shows its Canon name (MOCKUP-GAPS 39).

### 6.3 Dependent fields

- **D6.** A field whose type is `F(args)` (TYPES.md §11) has a `dependent` control: one control per
  branch of `F`, in branch order, each resolved by §4.3 (`never` for `Never`). The studio shows the
  control of the branch that applies to the current driver value (§12.3, type functions).
- **D7.** Changing the driver (for example `eventType`) is an ordinary `Set`; the compiler clears a
  dependent optional value that no longer fits in the same edit and reports it in `Dropped`
  (API.md E15). The studio names the cleared value and offers Undo (MOCKUP-GAPS 16).
- **D8.** A branch `Never` is shown per C39: not shown while `none`, shown read-only with its
  `E3802` finding otherwise.

### 6.4 Literal unions

- **D9.** A `T | "lit" …` value uses T's control with one **pinned** option per literal
  (MOCKUP-GAPS 18): for choice, `search` and `chips` controls, the literals are listed first, above
  the other options and search results, whatever the query; for any other control, a `segmented`
  pre-selector offers the literals and "Value", and T's control is shown when "Value" is chosen.

### 6.5 Dependent maps

- **D10.** A dependent map `{k in c: T(k)}` is a `cards` control with `keyFixed: true`
  (MOCKUP-GAPS 17): "Add" asks for the key first (a choice over the active keys of `c` not yet in
  the map), then shows the typed editor for `T(key)`; the key of an existing entry cannot change
  (API.md E13); entries can be removed.

---

## 7. Collections

### 7.1 Tables

A `table` control (C26) shows a collection of records or variants as rows, with a detail panel for
the selected row (SPEC §16.9).

- **T1.** The control carries: `of` (the element type), `key` (`"$id"` for tables, the key field
  for keyed lists, absent for plain lists), `orderable`, the resolved `columns` and `filters`,
  `search` (the index id, §8), and `singular`.
- **T2.** `orderable` is true when rows can be moved (`Move`): false for tables and keyed lists
  whose order comes from file paths (`load.dir`, entry files; API.md reason `order`).
- **T3.** "Add" is labelled with the element view's `singular` ("Add a level"), else the chrome
  "Add". For keyed collections the key is asked first. The new element is sent with
  `AllowErrors: true` (D3).
- **T4.** Retired entries are shown with a "retired" badge and cannot be un-retired from the
  studio (retirement is one-way, LOCK.md §4.3; API.md `Unretire` is refused); they are never
  offered by pickers (§8).
- **T5.** The detail panel shows the element with its record layout (§5).

### 7.2 Columns

- **T6.** The first column is the **entry column** (`$entry`) when the collection is keyed or the
  element view has a `title`: it shows the preview, the title (the key when there is no title) and
  the subtitle (the key when there is a title and no subtitle) (MOCKUP-GAPS 23).
- **T6a.** For a collection of variants, the next column is the case (mode `case`). A variant
  view's `columns` and `filters` name case fields (§3.3 rule 3 applied to the variant's cases).
- **T7.** The declared `columns` follow, in order. A declared column naming the key field of a
  keyed list is not repeated: its width goes to the entry column. Without `columns`, the columns
  are the first 6 scalar fields in declaration order, skipping the key field, hidden fields and
  deprecated fields (VIEW-09).
- **T8.** Each column has a **mode**, resolved by the compiler:

  | Mode | For | Cell |
  |---|---|---|
  | `entry` | `$entry` | T6 |
  | `edit` | a scalar field that is not read-only by C34/C43, not a key, not dependent | the cell control (C46), editable inline |
  | `value` | a read-only scalar field (deprecated, single, `readonly`, input) | the value, formatted by §9 |
  | `count` | a list, map or table field | its element count, followed by the element view's `plural` when there is one ("9 levels") (MOCKUP-GAPS 25) |
  | `case` | a variant field | the current case's label, read-only (D4) |
  | `text` | a record field, a dependent field, a literal union | text rendered by the compiler (`Heading.Cells`, §13): a record's title, a ref's target title, a value's canonical text |

- **T9.** A case field column shows an empty cell for rows whose shape lacks the field.
- **T10.** Every change made in a cell, or in a column across selected rows, is an ordinary edit
  (SPEC §16.9).

### 7.3 Filters

- **T11.** Each filter (MOCKUP-GAPS 43) is resolved to one of:

  | Field type (optional stripped) | Filter `kind` | Control | Row matches when |
  |---|---|---|---|
  | enum, `ref` | `choice` | single choice over "Any" + choices, by C4/C5 (`segmented`, `select`, `search`); with `multi`: `checkboxes` (≤ 6 choices) or `chips` | the value equals the choice (multi: is one of them) |
  | variant field | `case` | as `choice`, over cases | the value's case is the choice |
  | `Bool` | `bool` | `segmented` Any / Yes / No | the value equals it |
  | integer types, `Float`, `Duration` | `range` | a min and a max input, bounded by the minimum and maximum of the field over the collection's current rows | `min ≤ value ≤ max`; an empty bound is open |
  | `[E]`, `[ref T]` | `contains` | as `choice` | the list contains the choice (multi: any of them) |

- **T12.** An optional field adds the choice "(none)" (range filters: a "(none)" checkbox), which
  matches rows whose value is `none`. With a choice other than "Any" or "(none)", a `none` value
  does not match.
- **T13.** A case field used as a filter matches only rows whose shape has it; "Any" matches every
  row.
- **T14.** Filtering on another type is `E1620`; `multi` on a filter that is not `choice`, `case` or
  `contains` is `E1621`.
- **T15.** All filters combine with `and`. The default state is "Any" / empty.

### 7.4 Search box and sorting

- **T16.** A table whose control has `search` shows a search box (MOCKUP-GAPS 44). It keeps the rows
  whose index row matches the query in any tier of S3, in the table's current order.
- **T17.** Every column is sortable (SPEC §16.9): numbers and durations numerically; strings by code
  point; enums, cases and booleans by declaration index (`false` first); refs by the displayed
  title; `count` columns numerically; `none` and empty cells last in both directions. Ties keep
  collection order.

### 7.5 Lists of scalars

- **T18.** `positional` (C24) shows `n` inputs labelled `1`…`n` (or by `step`, §7.7). Filled inputs
  must be contiguous from the first; inputs past the current length are empty, and clearing the
  last filled input shortens the list. The type's lower length bound marks the first inputs as
  required.
- **T19.** `range` (C23) shows two inputs labelled with the chrome "min" and "max".
- **T20.** `tags` (C25) shows each element as a tag, editable with the element control, reorderable
  and removable; "Add" appends.

### 7.6 Maps

- **T21.** `enumRow` (C28): one input per active member, labelled by the member's label, in member
  order. An empty input means the key is absent.
- **T22.** `cards` (C29): one collapsible card per entry present, in map order, titled by the value
  view's `title` rendered with `{key}` (MOCKUP-GAPS 29). Without a title, the card shows the key:
  for an enum key its label (default: its Canon name) with the wire value as subtitle when it
  differs from the Canon name; for a ref key the target's title. "Add" offers, for an enum key,
  the active members not yet present; for any other key, the key control. Cards apply to every
  map of records, not only `{Enum: Record}`.
- **T23.** `map` (C30): a two-column table, key and value, with the key and value controls.

### 7.7 Steps

- **T24.** In `list`, `positional`, `table` rows of a plain list, and the `ordered_steps` widget,
  each element is named by the field's `step` text with `{index}` replaced by its 1-based
  position, else by the chrome "#1", "#2", … (MOCKUP-GAPS 27). The replacement is done by the
  studio on the translated text (R3).

---

## 8. Pickers and the search index

### 8.1 The index

- **S1.** The view model of a package holds a **search index** (VM-04) for:
  - every public top-level `let` whose value is a table, stable table, keyed list or define table;
  - every top-level `let` of the package (`local` included) whose value is such a collection or a
    define table, and that is the target of a `ref` type anywhere in the package.

  An index lives only in the view model of the package that declares the collection; other view
  models refer to it by its id `"<package>:<let>"`.
- **S2.** One row per entry, in collection order, retired entries included and flagged:
  `key`; `title` (the rendered view `title`, absent when the element has no view title); `subtitle`;
  `terms` (the rendered `search` expressions: strings as is, integers in decimal, enums by Canon
  name, refs by key, assets by file name, lists flattened, `none` omitted); `preview` (the asset
  file name); `retired`; and `tr`, the title and subtitle in each other language where the
  translated template renders differently (§12.8).

### 8.2 Ranking

- **S3.** A picker or search box matches the query `q` against each active row (MOCKUP-GAPS 20).
  Comparisons are case-insensitive: both sides are lower-cased code point by code point with the
  Unicode simple lowercase mapping. `t` is the displayed title (the title, else the key). Tiers:
  1. `t` starts with `q`;
  2. a word of `t` starts with `q`, where words are split at white space and at
     `( ) [ ] { } - _ / . , : ;`;
  3. `t` contains `q`;
  4. the key or a search term contains `q`.
- **S4.** Results are sorted by tier, then by the length of `t` in code points, then by collection
  order. A picker shows the first **60** results and the total number of matches. An empty query
  shows the first 60 active rows in collection order and the number of active rows.
- **S5.** Retired entries are never offered (MOCKUP-GAPS 21). A value that refers to one shows it
  with a "retired" badge next to its `E3502` finding.
- **S6.** A picker stores the key and shows, for each result and for the chosen value, the preview,
  the displayed title and the subtitle (SPEC §16.8). A target without a view title (a define table
  without a view, MOCKUP-GAPS 19) shows keys only, in a monospace font.
- **S7.** Guidance: when a package could load the same defines as another package's collection that
  has a view, it should import that collection instead (MOCKUP-GAPS 19, C5).

### 8.3 Titles

- **S8.** A `ref` interpolated in a template renders as its target's title when the target type has
  a view `title`, else as its key; `{r.id}` always renders the key (MOCKUP-GAPS 22, API.md V7).
- **S9.** When two entries of one collection render the same title, each of them is shown as
  `<title> (<key>)` (MOCKUP-GAPS 26), where `<key>` is the key's canonical text (API.md P9, without
  JSON quotes) or `#<n>` (1-based position) in a plain list. The compiler applies it in index rows
  and in `Evaluate` headings; the comparison uses the source-language rendering.

---

## 9. Texts, numbers and durations

### 9.1 Labels and fallbacks

- **X1.** Every field, method, value, enum member and variant case has a label key (I18N.md). Its
  source text is the view label, else a default (MOCKUP-GAPS 39):
  - fields, methods and values: the name **humanized**: insert a space between a lower-case ASCII
    letter or digit and a following upper-case ASCII letter; replace each `_` with a space;
    collapse runs of spaces; trim; lower-case the whole; upper-case the first character.
    `farmPurchasePrice` → "Farm purchase price", `minLevel` → "Min level", `id` → "Id",
    `stage1Rate` → "Stage1 rate", `adventureQuests` → "Adventure quests";
  - enum members and variant cases: the Canon name as is (`IK1_WEAPON`, `local_budget`).
- **X2.** The compiler writes these source texts into the view model; the studio never humanizes.
- **X3.** Fallback display (MOCKUP-GAPS 45, I18N.md §6): in a language other than the source, a
  text without a translation is shown in the source language with a small marker naming the
  source language ("EN"); when the package has no translation file for that language at all,
  no per-text marker is shown and one banner says so. Developers MAY hide the markers.

### 9.2 Rendering templates

Templates are rendered by the compiler: search rows, `Evaluate` titles, `show` lines, headings and
cells, finding messages.

- **X4.** A template renders each interpolation with the canonical text form (STD-06, SPEC §2.5)
  and its format spec, with these differences for views: a `ref` renders per S8; an enum member
  renders as its label in the rendering language (its Canon name without a label); `none` renders
  as the field's `none` text when the interpolation is a field that has one, else as `none`.
- **X5.** Numbers in templates are not localized: the `{x:,}` separator is `,` in every language.
  (Only controls localize numbers, §9.3.)
- **X6.** In a language other than the source, the translated template is used when there is one,
  else the source template (API.md V8).
- **X7.** A template whose evaluation fails (an error, a magic name without a value, a read of
  `none`) renders as nothing and is shown as "—"; it produces no finding (API.md V11).

### 9.3 Numbers and units

- **X8.** A number control shows `stored × scale` of its unit (VIEW-08, default scale 1) with
  exactly `decimals` fraction digits when the unit has a `decimals` greater than 0 (else as many as
  needed), grouped by thousands when `thousands` is true, followed by the unit's `suffix`
  (translated, I18N.md `units.<unit>.suffix`, MOCKUP-GAPS 46).
- **X9.** Decimal mark and grouping separator follow the studio language's number conventions
  (CLDR, as `Intl.NumberFormat(lang)` gives them): `1,234.5` in `en`, `1 234,5` in `fr`
  (MOCKUP-GAPS 46). Input accepts the language's decimal mark and ignores grouping characters.
- **X10.** A value typed by the user is stored as `shown ÷ scale`; for an integer type it is rounded
  half away from zero. A result outside the type's range is refused by the control before any
  edit (the type encoding has the bounds).
- **X11.** Without a unit, integers are shown in decimal with grouping, floats with the shortest
  text that reads back (WIRE.md §7.2), both with the language's separators.

### 9.4 Durations

- **X12.** A `duration` control offers the units of `units` (§12.5): the units among `ms s m h d`
  that are not smaller than the field's wire unit (`ms` without `@json(unit:)`) and not larger than
  the type's upper bound when there is one (a unit is kept when 1 unit ≤ max). The wire unit is
  always offered.
- **X13.** A duration is shown in the **largest** offered unit that divides it exactly
  (MOCKUP-GAPS 41): 3 600 000 ms is "1 h", 5 400 000 ms is "90 m". Zero is shown in `s` when `s` is
  offered, else in the smallest offered unit.
- **X14.** Choosing another unit keeps the typed number and re-reads it in the new unit (3 m → 3 h),
  which is a new value committed as an edit.

### 9.5 Defaults and placeholders

- **X15.** An empty input shows as placeholder: the view `placeholder`; else the field's default
  (from the type encoding), formatted like a value; else, for an optional with default `none`, the
  `none` label (C38).
- **X16.** A field set to a value equal to its default is not distinguished from one left at its
  default once saved (API.md E6 removes it).

---

## 10. Navigation

- **N1.** A public top-level value appears in the navigation under a menu when (VIEW-03):
  - it has `@menu(…)` (G23), which wins; or
  - its type is a record type `T`, or a list, keyed list, table or stable table of `T` (refined or
    not), and the view of `T`, declared in the **same package** as the value, has `menu` (this
    extends VIEW-03 from "exactly that type" to collections of it).
- **N2.** Menus appear in the declaration order of the studio package's `Menu` enum, labelled by its
  member labels (`Menu.<member>`, translated in the studio package). Inside a menu, values are
  ordered by package name (byte order), then by declaration order in the package (`order`, §12.6).
- **N3.** A value's navigation label is its label (`@menu(label:)`, else the humanized name) and
  its icon is the menu's `icon`.
- **N4.** Public values without a menu appear in a final "No menu" section, grouped by package
  (MOCKUP-GAPS 42). An editable one (`editable` is not `none`) outside the studio package gets
  `W1640`, reported only when its package is selected and has an `emit view` (the scope of
  `W1701`, I18N.md W1): a package without `emit view` publishes no view model, so its navigation
  is not a studio concern. One finding per value, at the value's name.
- **N5.** A value is opened as a screen: a record value with its layout (§5), a collection value
  with its table (§7), any other value with its control.

---

## 11. Editing

The studio edits values through API.md §8; this section only lists what the view layer adds.

- **E1.** Every change is sent as an `Edit`. After each **committed** edit (blur, Enter, a choice, an
  add, a remove, a move), including drafts written with `AllowErrors`, the studio calls `Evaluate`
  (or passes `Edit.Evaluate`) and re-renders `when`, `show`, titles and findings; never on each
  keystroke (MOCKUP-GAPS 47, API.md V14).
- **E2.** While typing, the studio MAY check locally what the type encoding states: integer and
  range bounds, string byte length, and patterns with an RE2-compatible engine (VIEW-10). A
  `predicate` (`where`) is checked only by the compiler.
- **E3.** Edits go to base sources; a value set by an active layer is read-only and names the layer
  (MOCKUP-GAPS 48, API.md W10), unless `Options.EditLayer` names that layer: then the edit is
  written as an amendment of that layer (API.md W11).
- **E4.** Undo uses `EditResult.Undo` (API.md §8.7).

---

## 12. The view model document

### 12.1 Envelope and bytes

A view model is one JSON document per package, written by `emit view` (the `out` file) and
returned by `Project.ViewModel` (API.md R9), byte for byte the same.

- **J1.** Bytes: WIRE.md §7.1 to §7.3, and the **pretty** form of WIRE.md §7.4 for the whole
  document (the layout of `JSON.stringify(vm, null, 2)`), ending with one LF.
- **J2.** Member order: an object whose member names come from the program (qualified type names,
  value ids, field keys, shape keys, languages, text keys, unit and widget names, group ids, show
  ids) has its members sorted by the **byte order** of the names. Every other object has its
  members in the order this section lists them.
- **J3.** A member listed as optional is **omitted** when it has no value, is `false`, or is an
  empty array or object, unless marked "always present".
- **J4.** The view model is produced even when the package has errors (VM-07, API.md R9, B1). A
  declaration that is broken (TYPES.md §1), or that sits in a file that does not parse, is left out
  of `types`, `views`, `values` and the catalogue; a value that failed to evaluate is listed with
  `failed: true` and contributes no usage and no index rows. Every finding is in `findings`.
- **J5.** Determinism: the document depends only on the sources, the loaded files, the layers and
  the compiler version (not on `Options.Lang`, time, or the machine).

Top level, every member always present except `studio`:

| Member | Content |
|---|---|
| `$schema` | `"canon-vm/1"` |
| `package` | the package's qualified name |
| `language` | the project's `canon` version, `"0.1"` |
| `requires` | packages whose view models this one refers to (types, views, indexes, texts, units, widgets), byte order |
| `types` | §12.3: every type declared in the package that is public, or reachable from a public type or value |
| `views` | §12.4: one resolved view per record, variant (with its cases) of `types` |
| `values` | §12.6: every public top-level `let` |
| `usage` | §12.7 |
| `search` | §12.8 |
| `assets` | §12.9 |
| `units` | §12.9: the units used by this package's views |
| `widgets` | §12.9: the widgets used by this package's views, and the default widgets that apply |
| `studio` | §12.9: only in the view model of the project's studio package |
| `i18n` | §12.10 |
| `findings` | §12.11 |

- **J6.** A studio MUST load the view model of `project.studio` and of every package in `requires`,
  recursively, through `Project.ViewModel` or the emitted files. It MUST refuse a document whose
  `$schema` it does not support, and MUST ignore members it does not know.

### 12.2 Names, texts and values

- **J7.** Types are named by qualified name `<package>.<Type>` (VM-03): `resource.farm.Global`. A
  type declared in another package is referenced, never copied.
- **J8.** Values and search indexes are named by `"<package>:<let>"`, the package-qualified value
  path of API.md §6: `pipeline:potions`.
- **J9.** A **text reference** is either a string `"<package>:<key>"`, naming key `<key>` of
  package `<package>` (I18N.md), or an object `{"text": "…"}` for a language-neutral text (a text
  with no letter, I18N.md §2). The studio resolves a string reference in the view model of
  `<package>`: `i18n.languages[<lang>].texts[<key>]`, else the source language's text (X3). The
  source-language table always has the key.
- **J10.** **Value encoding** in the view model (defaults, `single`, literals): `Bool` → boolean;
  integers → number, or a decimal string when outside ±(2^53−1); `Float` → number (WIRE.md §7.2);
  `String` and assets → string; `Duration` → integer milliseconds; enum → Canon member name;
  `ref` → key (string, or number for integer keys); `none` → `null`; list → array; map → array of
  `[key, value]` pairs in map order; record → object of Canon field names in declaration order,
  every field present; variant → object with `"$case": "<case>"` first, then the case's fields;
  literal-union literal → string.

### 12.3 `types`

`types` maps qualified names to **type definitions**. Field types use **type expressions**.

**Type expressions** (VM-02), one object tagged by `kind`:

| `kind` | Members | Notes |
|---|---|---|
| `bool` | | |
| `int` | `bits` (8, 16, 32, 64), `signed`, `min`?, `max`? | inclusive bounds, sized limits included; `min`/`max` omitted when they equal the int64 limits; `Int(0..100)` has `max` 99 |
| `float` | `bits` (32, 64), `min`?, `max`?, `minExclusive`?, `maxExclusive`? | `Float(0.0..1.0)` has `max` 1 and `maxExclusive` |
| `string` | `minLen`?, `maxLen`?, `pattern`? | lengths in UTF-8 bytes; `pattern` is RE2, search semantics (STD-03, VIEW-10) |
| `duration` | `min`?, `max`? | inclusive, integer milliseconds |
| `enum` | `ref` | qualified enum name |
| `record` | `ref`, `bind`? | `bind` maps each parameter of a parameterized record to its driver (below) |
| `variant` | `ref` | |
| `list` | `of`, `min`?, `max`?, `keyedBy`?, `unique`? | `min`/`max`: length bounds; `unique`: declared a set (C32) |
| `table` | `of`, `stable`? | |
| `map` | `key`, `value`, `min`?, `max`?, `dependent`? | `dependent`: a dependent map `{k in c: T(k)}` |
| `optional` | `of` | |
| `ref` | `collection` or `sibling`, `element`, `keyType`, `count`, `active` | below |
| `union` | `of`, `literals` | `T \| "lit" …` |
| `asset` | `root`, `ext` | `root` as written (`"@resource/Icon/Item"`), `ext` without dots |
| `never` | | |
| `dependent` | `fn`, `on` | an application of a type function (below) |
| `any` | | `_`, only in widget parameter types (§12.9) |

Every type expression MAY also carry `predicate`: the canonical source text of its `where`
predicate, when it has one (VM-02).

- **J11.** Aliases are expanded (VIEW-07); a type function with a `match` body is kept as a
  `typeFunction` definition and applied with `dependent`; any other parameterized alias is expanded
  with its arguments substituted (`SpecificKey(e)` becomes a `union` of `dependent` and
  `"default"`).
- **J12.** `ref`: `collection` is the index id of the target (`"resource.vocab:items"`), or
  `sibling: {"up": n, "field": f}` for a per-instance target (TYPES.md §10.2 level 1): walk `n`
  enclosing records up from the record that declares the ref (0 = that record) and use its field
  `f`. `element` is the element type's qualified name, or `"$define"` for a define table.
  `keyType` is `"string"` or `"int"`. `count` and `active` are the target's entry counts in this
  build (0 for `sibling`).
- **J13.** Drivers, used by `on` and `bind`: `{"field": f}` (an earlier field of the same record or
  case), `{"param": p}` (a parameter of the enclosing parameterized record, bound at its use), or
  `{"key": true}` (the key of the enclosing dependent map). The studio resolves `param` by walking
  out to the `record` expression's `bind`.

**Type definitions**, tagged by `kind`:

- `record`: `name`; `help`? (text reference, the type's doc); `params`? (`[{name, type}]`);
  `fields` (always present, declaration order); `methods`? (parameterless methods named by a view:
  `[{name, returns}]`).
- **Field**: `name`; `type`; `required`? (no default and not optional); `default`? (J10, or
  `{"computed": true}` when the default is not constant, TYP-15); `help`?; `deprecated`? (text
  reference); `wire` (always present: `name`, and when set `path`, `unit`, `none` (the JSON value),
  `inline`, `int`, `bits`, `pairs`); `stable`?; `input`? (`{"env": "VAR"}`). For a
  `@json(pairs:)` field, `pairs` is `{"keys": [k, v], "slots": N}` (the two templates as written,
  `{i}` included) and `name` is the first template.
- `variant`: `name`; `help`?; `tag` (the wire tag key); `cases` (always present): each
  `{name, wire, retired?, label, help?, icon?, tone?, fields, methods?}`.
- `enum`: `name`; `help`?; `ordered`?; `codes`? (the `@codes` type name, `"UInt8"`); `members`
  (always present): each `{name, wire, index, code?, retired?, label, help?, icon?, tone?}`.
  `wire` is the JSON wire value (a number with `@json(codes)`).
- `typeFunction`: `name`; `params` (`[{name, type}]`); `select` (the discriminant path relative to
  the single parameter, `""` for the parameter itself); `branches` (`[{match: [members], type}]`,
  in `match` order, `_` written as `["_"]`); `drivers` (for each collection whose entries are
  passed as the parameter in the loaded program: the collection id → `{key: discriminant member}`
  for every entry).

- **J14.** The studio picks the branch of a `dependent` value by: the driver's key → `drivers` →
  discriminant member → the first branch whose `match` contains it or `_`. A driver that is an enum
  field is its own discriminant (`select` is `""`).

### 12.4 `views`

`views` maps qualified type names to resolved views, for every record and variant in `types`.

**Record view** (also the shape of a **case view**):

| Member | Content |
|---|---|
| `kind` | `"record"` (`"case"` for a case view) |
| `declared`? | true when a `view` declaration exists |
| `title`?, `subtitle`? | `{template, text?}`: the source template, and its text reference when it is a key |
| `preview`? | `{expr, root, ext}`: the preview expression's source and its asset root and extensions |
| `singular`?, `plural`? | text references |
| `menu`? | `{menu, icon?}` as declared (resolution per value is in `values`) |
| `sections` | always present: named groups, then `{"kind": "other"}`, `{"kind": "more"}`, `{"kind": "unused"}` (§5.1) |
| `fields` | always present: field key → field view, for every field and every inline case field |
| `methods`? | method name → `{label, help?, hidden?, when?}` |
| `shows`? | show id → `{label, text}`: `text` is `{template, text?}` |

**Sections**:

- group: `{kind: "group", id, label, intro?, advanced?, when?, flatten?, entries}`; each entry is
  `{field}`, `{method}` or `{show}`;
- other: `{kind: "other", entries?, fields}`: `entries` are the view-level `show` lines and methods,
  `fields` (always present) the `_other` field keys in declaration order (the L8 fallback);
- more: `{kind: "more"}` (its content comes from `usage`, L9);
- unused: `{kind: "unused", fields?}`.

**Field view**:

| Member | Content |
|---|---|
| `label` | text reference |
| `help`? | text reference |
| `control` | §12.5 |
| `readonly`? | `"view"`, `"deprecated"`, `"single"`, `"input"` (C43) |
| `single`? | the only admissible value (C34), J10 |
| `hidden`? | true |
| `when`? | the condition's source text |
| `placeholder`?, `none`?, `step`? | text references |
| `case`? | the case path of a case field (L18): `"kind=IK1_WEAPON"` |
| `path`? | for a case field, the relative value path from the record: `"kind.attackMin"` |
| `env`? | for an input field, the variable name |

**Variant view**:

| Member | Content |
|---|---|
| `kind` | `"variant"` |
| `declared`? | true when `view V` exists |
| `title`?, `subtitle`?, `preview`?, `singular`?, `plural`?, `menu`? | as for records |
| `selector` | the case selector control (C9) |
| `cases` | always present: case name → case view |

### 12.5 Controls

A control is an object tagged by `kind`. Members are present only where they apply.

| Member | On kinds | Content |
|---|---|---|
| `kind` | all | one of `switch checkbox segmented radio select search checkboxes chips input textarea code number slider duration color text file section card tags positional range table list enumRow cards map variant dependent never widget` |
| `source` | choice kinds, `checkboxes`, `chips`, `enumRow`, `cards` | `{"bool": true}`, `{"enum": name}`, `{"cases": name}`, `{"collection": id}`, or `{"sibling": {up, field}}` |
| `pinned` | choice kinds and any control of a literal union | the literals (D9) |
| `optional` | any | `{unset: "segment" \| "clear", threeState?}` (C35, C36) |
| `min`, `max`, `minExclusive`, `maxExclusive` | `number`, `slider`, `duration` | bounds as in the type expression |
| `stepper` | `number` | true (C12) |
| `unit` | `number`, `slider` | unit name (§9.3) |
| `units` | `duration` | offered units, largest last (X12) |
| `minLen`, `maxLen`, `pattern` | `input`, `textarea`, `code` | from the type |
| `root`, `ext` | `file` | asset root and extensions |
| `of` | `section`, `card`, `table`, `cards`, `variant` | qualified type name |
| `element` | `tags`, `positional`, `range`, `list` | the element control |
| `count`, `minCount` | `positional` | number of inputs; required leading inputs |
| `strict` | `range` | true for `<` |
| `key` | `table` | `"$id"` or the key field |
| `orderable` | `table`, `list`, `tags` | T2 |
| `columns` | `table` | `[{field, width?, mode, cell?}]` (§7.2); `field` is a field key or `"$entry"` |
| `filters` | `table` | `[{field, kind, multi?, none?, control}]` (§7.3) |
| `search` | `table` | index id (T16) |
| `singular` | `table`, `list`, `cards` | text reference (T3) |
| `keyControl`, `value` | `map`, `cards`, `enumRow` (`value` only) | key and value controls |
| `keyFixed` | `cards`, `map` | true for dependent maps (D10) |
| `inline` | `variant` | true for an `@json(inline)` field (L17) |
| `selector` | `variant` | the case selector control |
| `fn`, `on`, `branches` | `dependent` | type function, driver, one control per branch |
| `widget`, `siblings`, `fallback` | `widget` | C41 |

### 12.6 `values`

`values` maps value ids (`"<package>:<let>"`) to:

| Member | Content |
|---|---|
| `name` | the `let` name |
| `order` | 0-based declaration position among the package's public lets (files in path order, then source order) |
| `type` | type expression |
| `label`, `help`? | text references |
| `menu`? | `{menu, icon?}`, resolved (N1) |
| `control` | the control of the value (§4) |
| `editable` | `"canon"`, `"json"` or `"none"` (API.md §7: the mode at the value's root) |
| `reason`? | when `none`: `"computed"` or `"format"` |
| `reload`? | true for `@reload` |
| `layers`? | active layers that amend this value, in application order |
| `failed`? | true when the value could not be evaluated (J4) |
| `sources` | always present: `{files, glob?, count?, entries?}`: `files` the declaring file, then the loaded file of a single-file `load`; `glob` (project-relative) and `count` (files matched) for `load.dir`; `entries` the number of `entry` declarations in other files |

### 12.7 `usage`

`usage` maps qualified record and variant names to `{shapes}`. `shapes` maps shape keys (L19; for a
variant, the case name followed by the case's own inline shape, `spawn_item` or
`IK1_GENERAL/kind2=IK2_MATERIAL`) to:

| Member | Content |
|---|---|
| `count` | instances of that shape reachable from public values (L10) |
| `fields` | field key → number of those instances that set it (every field of the shape) |
| `main` | always present: `_other` field keys shown (L7) |
| `more`? | `_other` field keys under "More" (L7) |

Only shapes with at least one instance appear (L8).

### 12.8 `search`

`search` maps index ids (S1) to:

| Member | Content |
|---|---|
| `type` | element type's qualified name, or `"$define"` |
| `keyType` | `"string"` or `"int"` |
| `count`, `active` | entries, active entries |
| `preview`? | `{root, ext}` of the preview asset |
| `rows` | always present: `[{key, title?, subtitle?, terms?, preview?, retired?, tr?}]` in collection order; `tr` maps a language to `{title?, subtitle?}` |

Titles in rows are already disambiguated (S9).

### 12.9 `assets`, `units`, `widgets`, `studio`

- `assets` maps each asset root used by the package's types (`"@resource/Icon/Item"`) to
  `{dir}`, the root's directory relative to the project directory, with `/`. Files are matched
  exactly and case-sensitively (DECISIONS 19). The studio lists `dir` for the file picker and reads
  thumbnails from it.
- `units` maps unit names to `{suffix, scale, thousands, decimals}` copied from the studio
  package's `units` table; `suffix` is a text reference (`"studio:units.hp.suffix"`), absent when
  the suffix is empty.
- `widgets` maps widget names to `{value, siblings?, default?, help?}`: the parameter types as type
  expressions (`_` is `{"kind": "any"}`), and the doc comment as a plain string (widget docs are not
  translated).
- `studio`, only in the studio package's view model: `{menus, icons, tones, units, widgets}`:
  `menus` the `Menu` enum's qualified name, `icons` and `tones` the member names of `Icon` and
  `Tone` in declaration order, `units` and `widgets` the full catalogues in the shapes above.

### 12.10 `i18n`

```json
"i18n": {
  "source": "en",
  "languages": {
    "en": { "texts": { "Potion.heal": "Heal" } },
    "fr": { "files": ["resource/farm/farm.fr.canon"], "missing": 12, "texts": { … } }
  }
}
```

- `source` is the first language of `project.languages`.
- `languages` has one member per project language. The source language has `texts`: every key of
  the package's catalogue (I18N.md §3) with its source text. Every other language has `files`
  (always present: the package's translation files for it, display paths, byte order), `missing`
  (always present: catalogue keys without a non-empty translation) and `texts` (always present:
  the translated keys, non-empty only).

### 12.11 `findings`

- **J15.** `findings` is every finding of the package from the build (errors included, VM-07), in
  API.md F2 order, each as the JSON object of API.md §4.2 (F5 key order, which ends with `reads`),
  followed by one view-model member when not empty:
  - `messages`: the message in each non-source language where the check has a translation
    (`{"fr": "…"}`).

  `reads` (`Finding.Reads`, API.md §4.1) is, for a finding of a one-line record check without
  `at`, the fields its condition reads (identifiers and `self.f` that resolve to fields of the
  record or case, in first-use order).
- **J16.** Findings do not count toward the view model size target (NFR-01) and are capped as in
  API.md F7.

---

## 13. Evaluate

`Evaluate` is specified by API.md §11. The view layer relies on it as follows; API.md §11 carries
each point:

- **Q1.** `EvalResult.Show` holds one line per `show` item and per view-named method of every value
  in the form; `ShowLine.Key` is the line's label key (I18N.md: `Type.show.<id>`, unnamed lines
  `Type.show._<n>`; a method's `Type.<method>`).
- **Q2.** `EvalResult.When` holds every field, method and group with a `when`: fields and methods by
  relative path (`kind.attackMin`, `name`), groups by `<relative path>#<group id>`.
- **Q3.** `Heading` gains `Cells map[string]Text`: for each column of mode `text` (T8), keyed by
  field key, the rendered cell. `Heading.Title` is disambiguated per S9.
- **Q4.** When `Path` is itself a collection, its elements get headings (so a table screen needs one
  `Evaluate`).
- **Q5.** Every `Finding` carries `Reads` as J15 describes (so live findings highlight fields too).

An illustrative exchange (Go field names in lowerCamel, as the studio's web client receives
them): task 0 filters on a monster, and the draft switches its event to an item event.

```json
{"path": "resource.heistia:heistia.tasks[0]", "lang": "fr", "draft": [
  {"op": "set", "path": "heistia.tasks[0].eventType", "value": "ECONOMY_DROP_ITEM"}]}
```

```json
{"revision": "r1:…", "path": "resource.heistia:heistia.tasks[0]",
 "title": {"value": "Kill Aibatt", "ok": true, "fallback": false},
 "when": {}, "show": [], "headings": {},
 "types": {"filterParam": {"kind": "ref", "collection": "resource.vocab:items", "element": "resource.vocab.Item", "keyType": "string", "count": 6944, "active": 6944}},
 "findings": [], "dropped": [{"path": "heistia.tasks[0].filterParam", "value": "MI_AIBATT1"}]}
```

The studio switches the field to the `items` branch (J14), shows an empty picker, and names the
cleared `MI_AIBATT1` with Undo (D7).

---

## 14. Versioning

- **V1.** `$schema` is `canon-vm/<N>`. This document is `canon-vm/1` (NFR-03). A change that a
  studio implementing version N could misread (a renamed or re-typed member, a new control kind, a
  new required member) increments N. Adding an optional member that can be ignored does not.
- **V2.** `viewmodel.schema.json` is strict (`additionalProperties: false`): it is the compiler's
  conformance test. Studios read leniently (J6).

---

## 15. Worked examples

### 15.1 The pipeline golden

`examples/pipeline/potion.canon` declares `record Potion` (five documented fields, a named `warn`),
`let potions: [Potion] keyed by id = load.dir("data/*.json")` with two files, and:

```
view Potion {
  title "{id}"
  menu items icon gem
  columns { id 200, heal 90, cooldown 90, stack 70 }
  heal "Heal" { unit: hp }
  cooldown "Cooldown"
}
```

What the golden `examples/pipeline/expected/potion.view.json` shows, rule by rule:

- `requires: ["studio"]`: the view uses the unit `hp`, the menu `items` and the icon `gem`.
- `types["pipeline.Potion"]`: `heal` is `{"kind": "int", "bits": 64, "signed": true, "min": 1,
  "max": 100000}`; `cooldown` is `{"kind": "duration", "min": 0, "max": 600000}` with wire unit
  `ms`; `stack` has `default: 99` and no `required`.
- `views["pipeline.Potion"]`: no group, so one `other` section listing the five fields; `heal` is a
  `number` with `unit: "hp"` (no stepper: 100 000 values); `cooldown` is a `duration` offering
  `ms`, `s`, `m` (X12: `h` exceeds the 10 m bound); `id` and `name` are `input`s with their
  patterns.
- `values["pipeline:potions"]`: a `table` with `key: "id"`, not orderable (`load.dir`, T2),
  columns `$entry` (width 200, taken from `id`, T7), `heal`, `cooldown`, `stack` in `edit` mode;
  `editable: "json"`, `reload: true`; menu `items` with icon `gem`.
- `usage`: 2 instances; `stack` is set in 1 of 2 (`II_POT_HEAL_S.json` omits `nStack`), which is
  50 % ≥ 25 %, so every field is in `main`, `stack` last.
- `search["pipeline:potions"]`: a public keyed list is indexed (S1); titles render `{id}`.
- `i18n`: 14 English texts; French has no file and 14 missing keys.
- `findings`: one `W1701` (I18N.md §5) for French.

### 15.2 A dependent field (heistia)

`Task.filterParam: Param(eventType)? = none` resolves to:

```json
"filterParam": {
  "label": "resource.heistia:Task.filterParam",
  "help": "resource.heistia:Task.filterParam.help",
  "control": {
    "kind": "dependent",
    "fn": "resource.vocab.Param",
    "on": {"field": "eventType"},
    "branches": [
      {"kind": "search", "source": {"collection": "resource.vocab:monsters"}},
      {"kind": "search", "source": {"collection": "resource.vocab:items"}},
      {"kind": "search", "source": {"collection": "resource.vocab:worlds"}},
      {"kind": "select", "source": {"enum": "resource.vocab.Element"}},
      {"kind": "select", "source": {"enum": "resource.vocab.QuestStyle"}},
      {"kind": "search", "source": {"collection": "resource.vocab:upgradeTypes"}},
      {"kind": "input"},
      {"kind": "never"}
    ],
    "optional": {"unset": "clear"}
  }
}
```

and `types["resource.vocab.Param"]` in vocab's view model carries `select: "param"`, the eight
branches, and `drivers["resource.vocab:eventTypes"]` mapping `COMBAT_KILL_MONSTER` to `monster`,
`ECONOMY_DROP_ITEM` to `item`, `COMBAT_KILL_FFA` to `none_`, and so on. With the driver
`COMBAT_KILL_FFA` the studio picks the `never` branch and hides the field while it is `none`.
`Element` (5 members) and `QuestStyle` (6) get `select` by C4.

---

## 16. Diagnostics

Codes `E16xx` / `W16xx`.

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E1601 | error | C40, §4.5 |
| E1602 | error | G8 |
| E1603 | error | G4 |
| W1604 | warning | G11 |
| E1605 | error | G9 |
| E1606 | error | §3.3, VIEW-09 |
| E1607 | error | G5 |
| E1608 | error | G20 |
| E1609 | error | C40 |
| E1610 | error | G16 |
| E1611 | error | §3.5 |
| E1612 | error | G13 |
| E1613 | error | §3.5, §3.6 |
| E1614 | error | G10 |
| E1615 | error | G14 |
| E1616 | error | §3.3 |
| E1617 | error | G17 |
| E1618 | error | G17 |
| E1619 | error | §4.5 |
| E1620 | error | T14 |
| E1621 | error | T14 |
| E1622 | error | G13 |
| E1623 | error | §3.5 |
| E1626 | error | G6 |
| E1627 | error | G6 |
| E1628 | error | G8 |
| E1629 | error | G22 |
| E1630 | error | G22 |
| E1631 | error | G21 |
| E1632 | error | G23 |
| E1633 | error | G19 |
| E1634 | error | G15 |
| W1640 | warning | N4 |
| W1641 | warning | L15 |
| W1642 | warning | L6 |

Type errors in view expressions use the codes of TYPES.md (for example `E3002` for a `when` that is
not `Bool`); reading an input field is `E3313`. `E1624` and `E1625` belonged to the view `row`
item, which DECISIONS 21 removed; the numbers are not reused. The single catalogue is
[ERRORS.md](ERRORS.md).
