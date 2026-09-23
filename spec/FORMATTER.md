# Canon formatter

Status: **normative** companion of [SPEC.md](../SPEC.md) v0.1 (§2.1) and [CLI.md](../CLI.md) §3.6
and §5.3. It defines the single canonical layout that `canon fmt` writes, the rules the edit API
uses to re-print only what changed, and the canonical layout of JSON source files.

Depends on:

- [GRAMMAR.md](GRAMMAR.md): the syntax, the concrete syntax tree with trivia (GRAMMAR §10), the
  canonical text of durations (GRAMMAR §2.5) and doc comment normalization (GRAMMAR §2.2).
- WIRE.md §7.2 (number text) and §7.3 (string escaping), reused for JSON sources (§14).
- API.md: which edits exist, how paths address values, and "a field set to its default is
  removed" (API-06). This document only fixes how the resulting text is printed (§13).

DECISIONS 18 applies throughout: **the formatter never aligns columns.**

---

## 1. Guarantees

- **One layout, no options.** The output depends only on the input's syntax tree, its comments,
  its blank lines, and one bit per brace list: whether the list was written on one line (§6.1).
- **Meaning is preserved.** Parsing the output gives the same AST as parsing the input, except for
  the changes listed in §10 (duration spelling, separators, import order, doc comment spacing,
  project map sugar). No token is added, removed or reordered otherwise.
- **Comments are preserved**, each one kept next to the token it was attached to (§8).
- **Idempotence.** `fmt(fmt(x)) == fmt(x)` byte for byte, for every `x` that parses.
- **Every file in `examples/` is a fixed point** (§15).
- A file with a syntax error is not changed: `canon fmt` prints the errors (GRAMMAR §12) and
  exits 1. The one exception is a leading byte order mark (`E1123`), which `canon fmt` removes
  (§10) when the file has no other syntax error. `canon fmt --check` reports unformatted files by exit code only; it produces no finding.
- Naming conventions are not the formatter's business: they are `W1003`, reported by `canon check`
  (GRAMMAR §9.2).

---

## 2. Characters and lines

- Output is UTF-8 without BOM, with `\n` line endings, and ends with exactly one `\n`.
- No line ends with a space or a tab. No tab appears outside string literals and comments.
- **Width** is 100 columns. The width of a line is its number of Unicode code points (every code
  point counts 1, including `×`, `é` and wide characters).
- A line may exceed 100 columns only when it holds a piece that cannot be broken: a string, a
  comment, a name, an annotation, a translation entry, a type.

---

## 3. Indentation

- One level is **2 spaces**. The body of every broken `{ }`, `( )` and `[ ]` list is indented one
  level more than the line holding its opening bracket.
- A **continuation line** (an item that does not fit on one line and is broken at a break point of
  §7) is indented one level (2 spaces) more than the item's first line, unless the break is inside a
  bracket list, whose own indentation applies.
- Own-line comments and doc comments take the indentation of the item that follows them (§8).
- **Multiline strings.** The opening `"""` stays where it is written in the expression. The content
  lines and the closing `"""` are re-indented to the indentation of the line holding the opening
  `"""` plus 2: the old indentation prefix (GRAMMAR §2.6) is replaced by the new one on every
  non-empty content line, so the string's value never changes. Empty content lines stay empty.

---

## 4. Blank lines

- At most one blank line in a row, anywhere.
- No blank line at the start or end of a file, right after an opening `{`, or right before a closing
  `}`.
- Between two items of a brace list, two top-level declarations, two statements, or an own-line
  comment and what follows it: one blank line if the input had at least one there, none otherwise.
- Inside expressions and inside `( )` and `[ ]` lists, blank lines are removed.
- File header:
  - comments before `package` keep their blank lines (at most one). A doc block directly above
    `package` stays directly above it; a blank line between them is kept, since removing it would
    attach a detached comment (GRAMMAR §9.1);
  - `package p` is followed by one blank line if anything follows, except that `layer name` or
    `translation lang` comes on the very next line;
  - the import block has no blank lines inside and is followed by one blank line if anything
    follows.

---

## 5. Spacing

One space, no more, wherever this table says "space". No padding for alignment, ever.

| Construct | Canonical form |
|---|---|
| binary operators `+ - * / % == != < <= > >= ?? and or in is` | space on both sides: `a + b`, `x ?? 0` |
| ranges `..` `..=` | no spaces: `0..=100`, `1..`, `..=256`, `start..end - WEEK`, `i + 1..` |
| unary `-`, spread `...` | no space: `-5`, `-x`, `...base` |
| `not` | `not x` |
| postfix `.` `?.` `[ ]` `( )` `!` | no spaces: `x.f`, `x?.f`, `xs[i]`, `f(a)`, `x!` |
| `,` | no space before, one after (single-line lists); line end in broken `( )`/`[ ]` lists |
| `:` in fields, brace items, arguments, annotations, map types | no space before, one after: `heal: Int`, `f(at: "items")`, `{K: V}` |
| `=` `+=` … `=>` `->` | space on both sides: `stack: Int = 99`, `x => x.id`, `-> Bool` |
| `?` of optional types | none: `ref TalentNode?` |
| `\|` in types | space on both sides: `Param(e) \| "default"` |
| single-line brace list | `{ a, b }`; empty: `{}` |
| single-line `( )` / `[ ]` list | `(a, b)`, `[a, b]`; empty `()`, `[]` |
| keyword followed by a name or `{` | one space: `record Rect {`, `emit json {`, `check {` |
| name followed by a parameter list | none: `fn at(day: Weekday)`, `record HourlyTarget(e: EventType)`, `widget ordered_steps(value: [_])` |
| annotation | `@name` or `@name(args)`, one space before it: `heal: Int @json("nHeal")` |
| table entry, typed literal, case with body | one space before `{`: `open { … }`, `Reward.item { … }` |
| trailing comment | one space before `//` |
| view items | `title "{id}"`, `menu events icon chest`, `columns { id 200, kind 170 }`, `heal "Heal" { unit: hp }` |
| translation entry | `Global.visitCost "Coût d'une visite"` |
| doc comment | `/// text` (one space after `///` when the text is not empty; `///` alone for an empty line) |

Interpolations inside strings are part of the string and are kept byte for byte (§10).

---

## 6. Lists

### 6.1 Brace lists `{ }`

A **brace list** is every `{ … }` of the grammar: declaration bodies (record, enum, variant, view,
group, amend, project), statement blocks, match arms, brace literals, the entries of an import, the
items of `search`, `filters` and `columns`, `if` expression branches, and brace comprehensions.

Layout:

- **Single-line** `{ a, b }` when **both**: (1) the list's `{` and `}` were on the same line in the
  input, and (2) the whole single-line form fits within the width in its context (§7). A comment
  inside the list, or an item that must itself be broken, makes (2) false.
- Statement blocks (function bodies, `check { }`, `test { }`, `if`/`for`/`while` blocks, block
  arms) may be single-line only if they hold **exactly one** statement. An `if`/`else` chain is
  single-line only if every block of the chain qualifies; otherwise every block of the chain is
  broken.
- Otherwise **broken**: `{` ends the header line, each item is on its own line one level deeper,
  **with no commas**, and `}` is on its own line at the header's indentation.
- An empty list is `{}`, unless it holds comments (then broken, with the comments inside).

A list written on several lines stays broken even if it would fit: that is the author's choice, and
it keeps one item per line for one-line diffs. A list written on one line that no longer fits
becomes broken.

### 6.2 Parenthesis and bracket lists `( )` `[ ]`

Argument lists, parameter lists, list literals, and `[ ]` comprehensions are laid out by width only
(line breaks in the input do not matter there, GRAMMAR §3.1):

- **flat** `(a, b)` / `[a, b]` if it fits;
- otherwise **broken**: the opening bracket ends the line, one element per line one level deeper,
  each followed by `,` (including the last), the closing bracket on its own line.
- A broken comprehension has the element and then each `for`/`if`/`let` clause on its own line,
  without commas.
- **Hugging.** A call whose only argument is a brace literal, a typed literal or a list literal is
  printed `f({` … `})`: the argument's own brackets break, not the parentheses.
- Annotation argument lists and type argument lists (`Int(0..=100)`, `@json("x", unit: s)`) are
  always flat.

### 6.3 New lists

A brace list created by the edit API (it has no input text) is single-line if it fits and none of
its items contains a nested list (a brace literal or a list literal); otherwise it is broken.

---

## 7. Breaking long lines

### 7.1 The layout model

The printer is the Wadler/Prettier algorithm over a document built from the syntax tree.

| Document | Flat | Broken |
|---|---|---|
| `text(s)` | `s` | `s` |
| `line` | one space | newline, then the current indentation |
| `softline` | nothing | newline, then the current indentation |
| `hardline` | newline (always) | newline; forces every enclosing group to break |
| `ifBreak(b, f)` | `f` | `b` |
| `indent(d)` | `d` | `d` with the indentation increased by 2 |
| `group(d)` | printed flat if it fits, else broken | |
| `rhs(op, value)` | rule A (§7.3) | |

```
print(root):
  stack := [(indent 0, BREAK, root)]; col := 0
  while stack not empty:
    (ind, mode, d) := pop(stack)
    text s      -> write s; col += width(s)
    concat      -> push children in reverse order with (ind, mode)
    indent x    -> push (ind + 2, mode, x)
    ifBreak b f -> push (ind, mode, mode == BREAK ? b : f)
    line        -> if mode == FLAT: write " "; col += 1  else: newline(ind)
    softline    -> if mode == FLAT: nothing             else: newline(ind)
    hardline    -> newline(ind)
    group x     -> if mode == FLAT:                          push (ind, FLAT, x)
                   else if x contains a hardline:            push (ind, BREAK, x)
                   else if fits(x, ind, stack, 100 - col):   push (ind, FLAT, x)
                   else:                                     push (ind, BREAK, x)

fits(x, ind, rest, w):
  simulate printing x in FLAT mode, then the commands of `rest` in their own modes
  (a group met in `rest` counts as FLAT unless it contains a hardline);
  subtract the width of every text and of every flat line from w;
  return false as soon as w < 0; return true at the first line or hardline printed in
  BREAK mode, or at the end of input.

newline(ind): remove trailing spaces, write "\n", write ind spaces; col := ind
```

Groups are decided from the outside in: an outer group breaks before any group inside it is
considered.

### 7.2 Documents of the constructs

`seq(xs, sep)` joins `xs` with `sep`. `BL(items)` is a brace list: `group("{", indent(line,
seq(items, [ifBreak("", ","), line])), line, "}")`, forced broken by §6.1 (a hardline placed in the
group) when the list was not single-line in the input or holds a comment; items separated by a
blank line in the input get an extra empty line between them in broken form. `PL(o, items, c)` is
`group(o, indent(softline, seq(items, [",", line]), ifBreak(",", "")), softline, c)`.

| Construct | Document | Break points |
|---|---|---|
| binary chain (a maximal run of operators of one precedence level; `??` included) | `group(a, indent(" op", line, b, " op", line, c …))` | after each operator: `a and` / `b` |
| comparison, `is`, `in` | `group(a, indent(" op", line, b))` | after the operator |
| range | `a..b` | none |
| postfix chain with **two or more call steps** | `group(head, indent(softline, step1, softline, step2 …))` | before each `.`/`?.` step |
| other postfix chains | concatenation | inside arguments only |
| call arguments | `PL("(", args, ")")`, or hugging (§6.2) | per argument |
| list literal | `PL("[", elems, "]")` | per element |
| comprehension `[ ]` | `group("[", indent(softline, elem, line, clause1, line, clause2 …), softline, "]")` | before each clause |
| brace literal, record body, enum body, block, view body … | `BL(items)` | per item |
| brace comprehension | `group("{", indent(line, item, line, clause …), line, "}")` with §6.1's single-line rule | before each clause |
| `let`/`const`/`var`, field default, parameter default, type alias, assignment | `… rhs("=", value)` | rule A |
| lambda | `params rhs("=>", body)` | rule A |
| match arm | `patterns rhs("=>", body)` | rule A |
| brace-literal item, amend item | `name rhs(":", value)` | rule A |
| field declaration | `group(name ": " type [rhs("=", default)], indent(line, ann1, line, ann2 …))`; when the next item of the body starts with a declaration keyword of GRAMMAR.md §3.1 rule 4 (a method or check), each `line` before an annotation is a plain space instead | before each annotation, then rule A; never before an annotation when a method or check follows (GRAMMAR.md §3.1 rule 4 would read it as that item's prefix) |
| one-line check | `group("check " [name ": "] cond [" at " field], indent(line, "else ", message))` | before `else`, then inside `cond` |
| `expect` statement | `"expect " subject [" passes" \| " fails " target \| " warns " target]` | inside the subject only |
| `if` statement or expression | one group for the whole chain: header, `" {"`, bodies, `"} else {"` … | per block (§6.1) |
| `for`, `while`, `match` | header then `BL` | inside the header, per item |
| function declaration | `"fn " name PL("(", params, ")") " -> " type " " block` | parameters, then the body |
| index `x[i]`, parentheses `(e)` | no break points of their own | |
| multiline string | its lines joined by `hardline` (so every enclosing group breaks) | none inside |
| trailing line comment | `text(" // …")` followed by a `hardline` | after the comment |
| types, annotations, `import` paths, view item headers, translation entries | no break points | |

The header of an `if`, `for`, `while` or `match` breaks like any expression; its continuation
lines are indented one level more than the keyword's line.

An `expect` keeps its keyword, its subject and its verdict (`passes`, `fails …`, `warns …`) on one
logical item: the verdict is never moved to a line of its own. When the subject is a brace literal
that is broken (§6.1: written on several lines, or too wide), the verdict follows its closing `}`
on the same line; the width check of §7.1 counts the verdict as part of the rest of that line:

```
  expect {
    ...sample
    schedule: [at(Mon, 20, 22), at(Mon, 21, 23)]
  } fails "overlaps an earlier window"
  expect { ...sample, schedule: [at(Mon, 20, 22), at(Mon, 22, 23)] } passes
```

### 7.3 Rule A (`rhs`)

For `prefix op value` (with `op` one of `=`, `+=`…, `=>`, `:`):

1. If the whole item fits on the current line (`fits` of §7.1, flat), print it flat.
2. Else, if the value is a brace literal, a typed literal, a list literal or comprehension, an
   `if` or `match` expression, or a multiline string, print `op`, a space, and the value in its
   broken layout (the value's brackets break).
3. Else, if the value printed flat fits on a new line indented one level deeper, print `op`, break,
   and the value flat on that line.
4. Else print `op`, a space, and the value, letting its own groups break.

### 7.4 Examples

adventurequest.canon, rule A step 3:

```
let adventureQuests: AdventureQuestConfig =
  load("@resource/Server/Quest/adventure_quest_config.json")
```

sweep_plan.canon, a chain with two calls, an argument that does not fit, and rule A step 4 on the
lambda:

```
// before
  return weapons
    .filter(w => w.kind3 == kind and w.handed in ["HD_ONE", "HD_TWO"] and w.job in jobs
                 and w.sex in ["=", "SEX_ANY", ""] and (w.limit ?? 1) <= level)
    .maxBy(w => w.limit ?? 1)

// after
  return weapons
    .filter(
      w => w.kind3 == kind and
        w.handed in ["HD_ONE", "HD_TWO"] and
        w.job in jobs and
        w.sex in ["=", "SEX_ANY", ""] and
        (w.limit ?? 1) <= level,
    )
    .maxBy(w => w.limit ?? 1)
```

rules.canon, a call broken per argument and a string concatenation broken after `+`:

```
// before
          fail(jobs[missing[0]].compared()[category],
               "{category}: set id {id} is missing from {missing.join(", ")}; " +
               "the loader refuses the whole file until every job declares the same ids")

// after
          fail(
            jobs[missing[0]].compared()[category],
            "{category}: set id {id} is missing from {missing.join(", ")}; " +
              "the loader refuses the whole file until every job declares the same ids",
          )
```

taxonomy.canon, a comprehension that does not fit:

```
// before
  return [{ group: g, areas: areasVisibleTo(role).filter(a => a.group == g) }
          for g in areaGroups.active()
          if areasVisibleTo(role).any(a => a.group == g)]

// after
  return [
    { group: g, areas: areasVisibleTo(role).filter(a => a.group == g) }
    for g in areaGroups.active()
    if areasVisibleTo(role).any(a => a.group == g)
  ]
```

A check whose message does not fit:

```
check columns.active().first()!.statuses.len() == 1
  else "the first column must hold exactly one status: it is the initial status"
```

farm.canon, a field whose annotation does not fit:

```
  maxModels: Int? = none
    @deprecated("never read by LoadFromFile: the real cap is FARM_MAX_MODELS = 100. Kept as is; changing it changes nothing in game.")
```

---

## 8. Comments

### 8.1 Attachment

Every comment is attached to one token of the syntax tree:

- a comment that starts on the same line as the end of a previous token is a **trailing comment**
  of that token;
- any other comment is a **leading comment** of the next token (own-line comment).

A leading comment of the first token of an item belongs to that item; a leading comment of a closing
`}` belongs to the end of the list. An item's comments move with the item (import sorting, edits)
and are removed only with it.

Doc comments (`///`) are leading comments with the attachment rules of GRAMMAR §9.1. The formatter
never moves a doc comment across a blank line and never adds or removes the blank line between a
doc block and the next item, since that changes attachment.

### 8.2 Placement

- **Own-line comments** are printed on their own lines, indented like the item that follows them.
  At the end of a list they are indented like the list's items. Blank lines between them follow §4.
- **Trailing line comments** stay on the line of their token, one space after it. A line comment
  ends its line: the token after it starts a new line (the break falls at the same token boundary
  as in the input, which the grammar accepted). A line comment inside a group forces that group and
  every enclosing group to break; in particular, a comment inside a brace list makes it broken.
- **Block comments** `/* … */` on one line, between tokens, stay inline with one space on each side.
  A block comment spanning several lines is printed as an own-line comment; its inner lines are kept
  byte for byte.
- The text of a comment is never changed, except that trailing whitespace is removed and a doc line
  `///text` becomes `/// text`.

```
// before
record Item {
  id:      String   @json("dwID")          // "II_GEN_MAT_MOONSTONE": what gets stored
  nameKey: String   @json("szName")        // "IDS_PROPITEM_TXT_003930"
}

// after
record Item {
  id: String @json("dwID") // "II_GEN_MAT_MOONSTONE": what gets stored
  nameKey: String @json("szName") // "IDS_PROPITEM_TXT_003930"
}
```

---

## 9. Declarations and files

### 9.1 Imports

- Imports are sorted by their package path, compared as bytes, then by alias (no alias first).
- The names in `{ }` are sorted as bytes (upper case before lower case).
- The import block is contiguous (no blank lines) and keeps each import's comments.

```
// before
import sovcommon.ui { Tone, Icon }
import sovcommon.roles { Role }

// after
import sovcommon.roles { Role }
import sovcommon.ui { Icon, Tone }
```

### 9.2 Items in brace lists

Record fields, enum members, variant cases, table entries, view items and translation entries are
printed with single spaces and no alignment (DECISIONS 18). A line that held several items separated
by commas in a broken list becomes one item per line.

```
// before (event.canon)
record Rect {
  left: Float, top: Float, right: Float, bottom: Float

  check left < right and top < bottom else "empty spawn region (Rect2D::IsValid)"
}

// after
record Rect {
  left: Float
  top: Float
  right: Float
  bottom: Float

  check left < right and top < bottom else "empty spawn region (Rect2D::IsValid)"
}
```

```
// before (vocab.canon)
type Param(e: EventType) = match e.param {
  monster      => ref monsters
  game_mode    => String         // KeyToGameMode's keys are still C++ (EventParamTypes.h)
  none_, stat  => Never          // no parameter: only "no filter" is valid
}

// after
type Param(e: EventType) = match e.param {
  monster => ref monsters
  game_mode => String // KeyToGameMode's keys are still C++ (EventParamTypes.h)
  none_, stat => Never // no parameter: only "no filter" is valid
}
```

The taxonomy statuses table. `open` and `wont_do` were single-line and still fit. `taken` and
`fixed` were written on two lines, so they are broken. `verified` (102 columns) and `duplicate`
(105) were single-line but do not fit in 100 columns, so they are broken too.

```
// before
let statuses: stable table Status = {
  open      { tone: warning, label: "Open",      terminal: false, next: [taken, wont_do, duplicate] }
  taken     { tone: info,    label: "Taken",     terminal: false, next: [open, fixed, wont_do, duplicate],
              requires: [assignee] }
  fixed     { tone: accent,  label: "Fixed",     terminal: false, next: [verified, open],
              optional: [fixed_in] }
  verified  { tone: success, label: "Verified",  terminal: true,  next: [open], by: reporter_or_triager }
  wont_do   { tone: neutral, label: "Won't do",  terminal: true,  next: [open], requires: [reason] }
  duplicate { tone: neutral, label: "Duplicate", terminal: true,  next: [open], requires: [duplicate_of] }
}

// after
let statuses: stable table Status = {
  open { tone: warning, label: "Open", terminal: false, next: [taken, wont_do, duplicate] }
  taken {
    tone: info
    label: "Taken"
    terminal: false
    next: [open, fixed, wont_do, duplicate]
    requires: [assignee]
  }
  fixed {
    tone: accent
    label: "Fixed"
    terminal: false
    next: [verified, open]
    optional: [fixed_in]
  }
  verified {
    tone: success
    label: "Verified"
    terminal: true
    next: [open]
    by: reporter_or_triager
  }
  wont_do { tone: neutral, label: "Won't do", terminal: true, next: [open], requires: [reason] }
  duplicate {
    tone: neutral
    label: "Duplicate"
    terminal: true
    next: [open]
    requires: [duplicate_of]
  }
}
```

Changing `wont_do`'s label is now a one-line diff, whatever the other labels are.

Views and translations lose their padding the same way:

```
// before (farm.view.canon, farm.fr.canon)
    farmPurchasePrice "Farm purchase price" { unit: penya }
    visitCost         "Cost of a visit"     { unit: penya }
Global.farmPurchasePrice              "Prix d'achat de la ferme"

// after
    farmPurchasePrice "Farm purchase price" { unit: penya }
    visitCost "Cost of a visit" { unit: penya }
Global.farmPurchasePrice "Prix d'achat de la ferme"
```

### 9.3 Other declarations

- Top-level declarations are separated by a line break; blank lines follow §4.
- Prefix annotations (`@reload`) are printed on their own line(s) above the declaration, one per
  line, in the written order. Annotations in header, field, member, case and entry positions stay
  on the item's line unless §7.2 moves them to continuation lines. Annotation order is never
  changed.
- `if`/`else`: `} else {` and `} else if … {` on one line.
- `project.canon`: `key: value` items; a map-valued key is printed in block form `roots { … }`
  (`roots: { … }` is rewritten).
- Layer files: `package p`, then `layer name` on the next line, then one blank line.
- Translation files: `package p`, then `translation lang` on the next line, then one blank line;
  entries keep their order.

---

## 10. Literal canonicalization

| Literal | Rule |
|---|---|
| duration | rewritten in canonical text (GRAMMAR §2.5): `60s` → `1m`, `90s` → `1m30s`, `1_000ms` → `1s` |
| integer, float | kept exactly as written: `1_000_000`, `0x1F`, `0.0000001`, `1e-3` |
| string, raw string, regex | kept byte for byte, escapes and interpolations included |
| multiline string | content kept; indentation re-based (§3) |
| trailing and separator commas | removed in single-line lists and in broken brace lists; added after every element of a broken `( )`/`[ ]` list |
| empty lists | `{}`, `()`, `[]` |
| parentheses | never added or removed |
| doc comment | `///text` → `/// text` |
| project map value | `key: { … }` → `key { … }` |

The formatter also removes a leading BOM, converts `\r\n` to `\n`, and replaces indentation tabs by
the canonical spaces.

---

## 11. What the formatter never does

It never reorders items (other than imports), merges or splits declarations, renames anything,
adds or removes doc comments, changes a string, number or regex, adds or removes parentheses,
changes annotation order, or reports naming-convention warnings.

---

## 12. Testing requirements

- Corpus `testdata/fmt/`: pairs `name.in.canon` / `name.out.canon`, at least one pair per rule of
  §§3–10, plus every file of `examples/` as its own expected output.
- For every corpus input and every example: `fmt(x) == expected`, `fmt(expected) == expected`,
  and `AST(fmt(x)) ≡ AST(x)` modulo §10.
- Property test: for any file that parses, re-joining all its tokens with arbitrary legal line
  breaks and spaces and formatting gives the same output as formatting the original, except for the
  single-line bit of §6.1.

---

## 13. Minimal re-printing for the edit API

DECISIONS 12, API-03. The studio edits values; the compiler writes text. The contract:

- **Untouched text is byte-identical.** Bytes outside the re-printed units never change, even if
  they are not in canonical layout.
- **Item.** A top-level declaration, an item of a brace list (field, member, case, table entry,
  brace-literal item, statement, view item, amend item, project item), a translation entry, or an
  element of a `( )`/`[ ]` list.
- **Unit.** For each change, the unit is the smallest item whose text changes. For an insertion it
  is the new item; for a removal, the removed item.

Algorithm for one edit (all operations of an atomic edit are applied, then units are re-printed):

1. **Print the unit** with the printer of §7, starting at the unit's current column, with the text
   that follows the unit on its last line as the rest for `fits`. Brace lists that already exist
   keep their single-line bit from the current text; new ones follow §6.3.
2. **Splice** the printed text over the unit's bytes, from its first token to its last token. Its
   leading comments, doc comments and trailing comment stay where they are.
3. **Escalate.** If a line touched by the splice is now wider than 100 columns, or the unit now
   spans several lines, and the unit lies inside a list printed on one line, the unit becomes the
   item that contains that list, and the algorithm restarts at step 1. Escalation stops at a
   top-level declaration.
4. **Insert** into a broken brace list: a new line at the insertion point, at the list's item
   indentation, without blank lines. Into a single-line brace list: `, item` before ` }` (`{}`
   becomes `{ item }`). Into a broken `( )`/`[ ]` list: a new line `item,`. Into a single-line one:
   `, item` before the closing bracket. Then step 3 applies.
5. **Remove** from a broken list: the item's lines, with its leading comments, doc comments and
   trailing comment; if this leaves two blank lines in a row, or a blank line after `{` or before
   `}`, one is removed. From a single-line list: the item and one adjacent `, `. A list left empty
   becomes `{}` or `[]`.
6. **Retire**: `retired ` is inserted before the key (LOCK.md).
7. **New files** (a table entry added through `@files`): printed entirely in canonical layout.

Where an insertion goes (a defaulted field inserted in declaration order, API-02) and when a field
is removed instead of set (API-06) are defined by API.md.

```
// Set(statuses.wont_do.label, "Will not do")      one line changes
  wont_do { tone: neutral, label: "Will not do", terminal: true, next: [open], requires: [reason] }

// Add(statuses.open.next, fixed): the single-line entry is now 98 columns, so only the list
// changes. Adding `verified` as well would make it 108 columns: the unit escalates to the
// entry, which is re-printed broken, one field per line.
  open { tone: warning, label: "Open", terminal: false, next: [taken, wont_do, duplicate, fixed] }
```

---

## 14. JSON source files

FMT-02. JSON files read by `load` are normalized once by `canon fmt --json-sources` (CLI §3.6) and
from then on edited by the same minimal re-printing. This layout is for **sources**; the files that
`emit json` writes have their own layout (WIRE.md).

### 14.1 Canonical layout

- UTF-8 without BOM, `\n` line endings, one final `\n`.
- Indentation 2 spaces per level.
- A non-empty object is `{`, then one member per line as `"key": value`, then `}`; a non-empty
  array is `[`, then one element per line, then `]`. Members and elements are separated by `,` at
  the end of the line; there is no trailing comma. Empty containers are `{}` and `[]`.
- Strings, including keys, use the canonical JSON string escaping of WIRE.md §7.3.
- Numbers: a number read into a Canon field (an integer, a float, a `Duration` with
  `@json(unit:)`, a `@json(int)` Bool, a `@json(bits)` list, a `@codes` enum with `@json(codes)`)
  is written in the canonical number text of WIRE.md §7.2 for that field (integers exact, floats
  shortest round-trip). Any other number (an unknown key kept by `partial: true`, a value no Canon
  type reads) is kept exactly as written, so nothing is lost.
- `true`, `false`, `null` as usual.
- **Key order is preserved.** Normalization never reorders members.
- **Unknown keys are preserved** in place, with their values.

A file that is not valid JSON, or has duplicate keys (`E7104`), is not changed.

### 14.2 Edits

- A **new key** goes right after the member holding the nearest *earlier* key in the declaration
  order of the object's Canon type (wire names). If no earlier declared key is present, it goes
  right before the first later declared key present; if no declared key is present at all, at the
  end of the object.
- `@json(path: "a.b")`: missing intermediate objects are created with the same placement rule,
  using the first field that uses the prefix (LOD-09).
- The unit is the smallest member or element whose text changes; adding or removing the `,` of the
  neighbouring line is part of the change. A unit inside a container written on one line (a file
  never normalized) escalates to that container, which is re-printed in canonical layout.

```
// data/II_POT_HEAL_S.json, Set(potions.II_POT_HEAL_S.stack, 20). nStack follows dwCooldownMs in
// Potion's declaration order.

// before
{
  "dwID": "II_POT_HEAL_S",
  "szName": "IDS_PROPITEM_TXT_POT_S",
  "nHeal": 500,
  "dwCooldownMs": 3000
}

// after
{
  "dwID": "II_POT_HEAL_S",
  "szName": "IDS_PROPITEM_TXT_POT_S",
  "nHeal": 500,
  "dwCooldownMs": 3000,
  "nStack": 20
}
```

---

## 15. The examples as fixed points

Every `.canon` file under `examples/` is a fixed point of this document: formatting it changes no
byte. The examples errata removed column alignment (DECISIONS 18), and the consistency pass of
2026-09-23 applied the rest: one item per line in broken brace lists, sorted imports and imported
names, canonical durations (`60s` → `1m`), lines within 100 columns, and a trailing comma in
broken `( )` and `[ ]` lists. The examples are part of the formatter's corpus (§12), so a rule
change that would move any of them shows in the golden tests.

Lines longer than 100 columns that hold only an unbreakable piece (a long string after
`else`, an annotation, a group header with a long intro, a translation entry, a comment) stay as
they are (§2).

---

## 16. Diagnostics

The formatter owns no diagnostic codes. It reports the syntax errors of GRAMMAR §12 (`E11xx`) for
a `.canon` file that does not parse, and WIRE.md's `E7109` (invalid JSON) or `E7104` (duplicate
key) for a JSON source; in each case the file is left unchanged. The single catalogue of every code
is [ERRORS.md](ERRORS.md).

`canon fmt --check` signals unformatted files by exit code 1 and the list of paths (`--diff` prints
the changes); these are not findings.
