# Canon grammar

Status: **normative** companion of [SPEC.md](../SPEC.md) v0.1. It replaces SPEC §2.3, §2.4 (the
identifier rule), §2.5 (the lexical rules of literals), Appendix A and Appendix B. Where this
document and SPEC.md disagree on syntax, this document wins; the SPEC errata will point here.

It covers: the lexer (§2), the newline and separator algorithm (§3), keywords (§4), the full
grammar (§5), disambiguation rules (§6), the project file (§7), the annotation catalogue (§8),
naming conventions and doc comments (§9), what the parser must produce (§10), the examples that do
not parse (§11) and the diagnostics this document owns (§12).

Other documents this one relies on. Each assumption is stated where it is used.

| Document | What it decides that this grammar leaves open |
|---|---|
| TYPES.md | classification of brace literals (§6.4), refinement vs type application (§6.6), narrowing and `x!`, `?.` chain semantics, validity of assignment targets (`E3307`), `T??` (`E3401`) |
| STDLIB.md | meaning of format specs; canonical text of values other than durations |
| WIRE.md | meaning of every `@json` argument; canonical JSON bytes |
| LOCK.md | `@stable`, `@codes`, `retired` |
| CODEGEN.md | `@cpp`, `@go`, `@ts`, `@reload`, the `go_module` project key |
| VIEWMODEL.md | meaning of view items, field props and widgets (this document owns their syntax, §5.3 and §5.6) |
| I18N.md | the translation key catalogue (this document only fixes the key syntax) |
| EVALUATION.md | layers (`amend`) |
| FORMATTER.md | the canonical layout |

Positions in diagnostics are 1-based lines and 1-based columns counted in UTF-8 bytes.

---

## 1. Source text

- A source file is UTF-8. Invalid UTF-8 is `E1124`. A byte order mark at the start is `E1123`
  (`canon fmt` removes it).
- `\r\n` is normalized to `\n` before lexing. A `\r` not followed by `\n` is `E1124`.
- Outside string literals and comments, the only characters allowed are ASCII. Any other
  character, and the ASCII characters `` ` `` `#` `$` `&` `;` `^` `~` `\` `'` outside a literal, is
  `E1106`. The one exception is the `#` of a position segment `[#n]` in an amend path (§5.7): the
  lexer produces a `#` token there (directly after `[`), and a `#` token anywhere else is `E1106`.
- Inside string literals and comments, the control characters U+0000–U+001F other than tab (and
  newline in multiline strings and block comments) are `E1124`.
- Tabs and spaces are whitespace. Tabs have no width meaning for the parser.

---

## 2. Lexer

### 2.1 Token classes

| Class | Examples | Notes |
|---|---|---|
| `IDENT` | `heal` `Tone` `none_` `_other` `II_WEA_AXE_ANGEL` | §2.3; excludes reserved words and `_` |
| keyword | `record` `check` `in` | the reserved words of §4.1, each its own token kind |
| `_` | `_` | a lone underscore is punctuation, never an identifier |
| `INT` | `42` `1_000_000` `0x1F` `0b1010` | §2.4 |
| `FLOAT` | `3.5` `0.15` `1e-3` `2.5e6` | §2.4 |
| `DURATION` | `250ms` `90s` `1h30m` | §2.5 |
| `STRING` | `"Heal {heal} HP"` | §2.6; carries parts |
| `RAW` | `r"C:\path\{x}"` | §2.6 |
| `MLSTRING` | `"""` … `"""` | §2.6; carries parts |
| `RAWML` | `r"""` … `"""` | §2.6 |
| `REGEX` | `/^II_[A-Z0-9_]+$/` | §2.7 |
| `DOC` | `/// text` | §2.2; attachable trivia |
| punctuation | §2.8 | |
| `NL` | | produced by the separator pass (§3), never by the lexer directly |

Whitespace, newlines, `//` comments and `/* */` comments are **trivia**. Every token records its
leading trivia (for the formatter) and the number of line breaks between it and the previous
token (`0`, `1`, or `2+` meaning "a blank line").

### 2.2 Comments and doc comments

```
// line comment, to the end of the line
/* block comment: may span lines, does not nest */
/// doc comment
```

- `/*` without a matching `*/` is `E1108`. `/*` inside a block comment has no meaning.
- A line whose first non-whitespace characters are exactly `///` is a **doc comment line**
  (`DOC`). `////` (four or more slashes) is an ordinary line comment.
- `///` that is not the first token of its line (after code) is an ordinary line comment, and
  `W1001` is reported.
- **Normalized text** of a doc comment (LEX-07): for each line, remove `///` and then one space if
  one follows; remove trailing whitespace; join consecutive lines with `\n`; drop leading and
  trailing empty lines. `///` alone produces an empty line (a paragraph break).
- Consecutive doc lines (no blank line and no code between them; ordinary comments between them are
  allowed) form one **doc block**. Attachment is decided by the parser (§9.1).

Example (taxonomy.canon):

```
/// The reporter makes ONE classification: WHERE did you see it (area). Everything
/// finer is said in the text; routing to a technical area is the triager's move.
package teamboard
```

The package doc is the two lines without their `/// ` prefix, joined by `\n`.

### 2.3 Identifiers and words

```
IDENT = letter { letter | digit | "_" }       (not a reserved word, not "_")
letter = "A".."Z" | "a".."z" | "_"
digit  = "0".."9"
```

- Identifiers are ASCII and case-sensitive.
- A lone `_` is the `_` token (match wildcard, ignored binder, any-type).
- In this document **`WORD`** means "an `IDENT` or any reserved word". The grammar uses `WORD` in
  the positions where reserved words are accepted as names (§4.3).

### 2.4 Numbers

```
INT      = decimal | "0x" hexDigit { [ "_" ] hexDigit } | "0b" binDigit { [ "_" ] binDigit }
decimal  = digit { [ "_" ] digit }
FLOAT    = decimal "." decimal [ exponent ] | decimal exponent
exponent = ( "e" | "E" ) [ "+" | "-" ] decimal
```

- `_` may only stand between two digits: `1_000` is valid; `1__000`, `1_`, `_1`, `0x_1F` are
  `E1110`.
- A decimal literal with more than one digit may not start with `0` (`007`, `0_7`: `E1110`). `0`,
  `0.5` and `0e3` are valid.
- `.` belongs to a float only when a digit follows it: `1..2` is `1`, `..`, `2`; `1.5.abs()` is a
  method call on `1.5`; `1.` and `.5` are not floats.
- The prefixes are lowercase (`0x`, `0b`); hex digits may be either case.
- The character right after a numeric token must not be a letter, digit or `_` (`5min`, `1e`,
  `0x1G`, `12abc`: `E1110`), except where §2.5 makes it a duration.
- **Arbitrary precision** (LEX-05). An `INT` token holds its exact integer value; a `FLOAT`
  token holds its exact decimal value. Nothing is rounded or range-checked by the lexer. Range and
  rounding are applied against the expected type (TYPES.md; `E3201` for an integer that does not
  fit). The parser folds unary `-` applied directly to a numeric or duration literal (with no
  postfix operator on the literal) into a negative literal, so `-9223372036854775808` is valid for
  `Int`.
- The formatter keeps numeric literals exactly as written, `_` included (FORMATTER.md).

### 2.5 Durations

```
DURATION = decimal unit { decimal unit }
unit     = "ms" | "s" | "m" | "h" | "d"
```

- A duration token starts where a `decimal` is directly followed by one of the letters `m s h d`.
  Units are matched longest first (`ms` before `m`): `5ms` is 5 ms, `5m5s` is 5 min 5 s.
- Units appear in strictly decreasing order `d h m s ms`, each at most once. `30m1h`, `1h1h`,
  `1h30` (trailing digits without a unit) are `E1111`.
- Any integer is allowed per unit (`90m`, `1500ms`). A fraction (`1.5h`) is `E1111`; write
  `1h30m`. Hex and binary digits are not durations.
- Values: `ms` = 1, `s` = 1 000, `m` = 60 000, `h` = 3 600 000, `d` = 86 400 000 milliseconds. A
  total that does not fit in a signed 64-bit count of milliseconds is `E1111`.
- A negative duration is written with unary `-` (`-5m`) and folded like numbers.

**Canonical text** (LEX-04). Used by `canon fmt`, string interpolation, `String(x)`, messages and
the view model. For a value of `v` milliseconds:

1. `v == 0` → `0s`.
2. Otherwise write `-` if `v < 0`, then decompose `|v|` into days, hours, minutes, seconds and
   milliseconds (`d = |v| / 86 400 000`, and so on with the remainders), and write each non-zero
   part followed by its unit, largest first, with no separators.

| Literal | Canonical |
|---|---|
| `90s` | `1m30s` |
| `60s` | `1m` |
| `1500ms` | `1s500ms` |
| `36h` | `1d12h` |
| `0ms` | `0s` |
| `1_000ms` | `1s` |
| `-(90m)` evaluated | `-1h30m` |

### 2.6 Strings

**Plain strings** `"…"`:

- End at the next unescaped `"`. A newline or the end of the file before it is `E1107`, located at
  the string's opening quote, interpolations included (DECISIONS 286).
- Escapes: `\n` `\t` `\r` `\\` `\"` `\{` `\}` and `\u{H…}` with 1 to 6 hex digits naming a Unicode
  scalar value (not U+D800–U+DFFF, at most U+10FFFF). Any other `\x` is `E1109`.
- `{{` is a literal `{` and `}}` is a literal `}`. A single `}` in the text is `E1112`.
- A single `{` opens an **interpolation** (below).
- The token carries a list of **parts**: text parts (escapes decoded) and interpolation parts.

**Interpolation** (LEX-02). `{` expression [ `:` spec ] `}`:

- The lexer pushes an interpolation mode and lexes ordinary tokens (including nested strings,
  which may interpolate in turn) while tracking `(` `[` `{` depth, until it meets, at depth 0, a
  `}` (end) or a `:` (format spec).
- A `:` at depth 0 starts a **format spec**, which must match `\+?,?(\.[0-9]{1,2})?` and be
  non-empty, followed directly by `}`. The number after `.` is at most 20. Anything else is
  `E1101`. The spec components are: `+` always signed, `,` thousands separator, `.N` fixed
  decimals (their meaning: STDLIB.md).
- A `:` inside brackets belongs to the expression: `{f(x, n: 1)}`, `{ {a: 1}.a }` are valid.
- An empty interpolation `{}`, a newline inside an interpolation, a comment inside an
  interpolation, or end of input before the closing `}` is `E1112`.
- The expression is parsed with the ordinary expression grammar (§5.11). Its tokens never produce
  `NL` separators.

```
"status {s.id} is claimed by several columns: {owners.map(.id).join(", ")}"
"{heal:,} HP"            // spec ","
"{ratio:.2}"             // spec ".2"
"{delta:+}"              // spec "+"
"{a:x}"                  // E1101
```

**Raw strings** `r"…"`: `r` immediately followed by `"`. No escapes, no interpolation; `{`, `}` and
`\` are literal. A raw string cannot contain `"` or a newline (`E1107` at its opening `r`, as for
every string; DECISIONS 286).

**Multiline strings** `"""` … `"""` (LEX-03, Swift rules):

- The opening `"""` must be followed by optional whitespace and a newline; other text after it is
  `E1122`.
- The closing `"""` must be the first non-whitespace text of its line; otherwise `E1122`. The
  whitespace before it is the **indentation prefix**. A prefix that mixes tabs and spaces is
  `E1102`.
- Every content line must start with exactly the indentation prefix, which is removed. A content
  line that is empty or whitespace-only and not longer than the prefix becomes empty. Any other
  line that does not start with the prefix is `E1122`.
- The value is the content lines joined with `\n`. The newline after the opening `"""` and the
  newline before the closing line are not part of the value. Trailing whitespace of content lines
  is kept.
- Escapes and interpolation work as in plain strings; `"` and `""` may appear unescaped; `"""`
  inside the content is written `\"""`. An interpolation must stay on one line.

```
    let help = """
      First line.
        Indented line.
      """
```

`help` is `First line.\n  Indented line.`

**Raw multiline strings** `r"""` … `"""`: the same layout rules, no escapes, no interpolation.

`stringLit` in the grammar means any of `STRING`, `RAW`, `MLSTRING`, `RAWML`. A **constant string**
is a `stringLit` with no interpolation part; positions that require one report `E1132` (or `E1119`
inside an annotation).

### 2.7 Regular expressions and the division rule

LEX-01. `/` starts a `REGEX` token when the previous significant token is `(` or `,`. Everywhere
else `/` is the division operator (or `/=`). `//` and `/*` always start comments, so an empty regex
cannot be written.

- The regex body runs to the first `/` that is not escaped and not inside a character class
  `[…]`. `\` escapes the next character; `\/` stands for `/` and every other escape is passed to the
  regex engine unchanged. A newline or end of input before the closing `/` is `E1113`.
- There are no flags after the closing `/`; use RE2 inline flags (`(?i)`).
- The body must compile as an RE2 pattern (`E1114`). Matching semantics: STDLIB.md.
- The parser accepts a `REGEX` in two places only (anywhere else: `E1115`): as the only argument of
  a named type (`String(/^IDS_/)`), and as the only argument of a method call named `matches`
  (`name.matches(/^II_/)`).

```
id: String(/^[A-Za-z0-9_]{1,64}$/)     // regex: previous token is "("
let half = (a + b) / 2                 // division: previous token is ")"
```

### 2.8 Operators and punctuation

Matched longest first:

```
...  ..=  ..  ??  ?.  =>  ->  ==  !=  <=  >=  +=  -=  *=  /=
+  -  *  /  %  <  >  =  !  ?  .  ,  :  (  )  [  ]  {  }  @  |  _  #
```

- `#` is a token only directly after `[` in an amend path (`[#0]`, §5.7); anywhere else it is
  `E1106` (§1).

- `!` is the postfix presence assertion (`x!`, DECISIONS 17). `x!=y` lexes as `x`, `!=`, `y`.
- `?` appears only in types (`T?`). `??` in a type position is `E3401` (TYPES.md).
- `@` must be directly followed by the annotation name (no whitespace): `@ json` is `E1116`.

### 2.9 Lexer modes

```
modes := [CODE]
loop:
  switch top(modes):
  CODE, INTERP:
    skip whitespace and comments (recording trivia; in INTERP, a newline or comment is E1112)
    '"""' / 'r"""'                  -> read a multiline string (its interpolations push INTERP)
    '"'  -> push STRING;  'r"' -> read a raw string
    '/' and previous significant token is '(' or ',' -> read REGEX
    digit -> read INT | FLOAT | DURATION
    letter or '_' -> read IDENT / keyword / '_'
    '(' '[' '{' -> emit; depth++ ;  ')' ']' -> emit; depth--
    '}' -> if INTERP and depth == 0: pop INTERP (back to STRING) else emit; depth--
    ':' -> if INTERP and depth == 0: read format spec, expect '}', pop INTERP
    '#' directly after '[' -> emit '#' (an amend-path position segment, §5.7; E1106 elsewhere)
    otherwise longest-match punctuation, or E1106
  STRING:
    read text until '"' (pop), '{{' / '}}' (literal brace), '{' (push INTERP with depth 0),
    '\' escape, or newline / EOF (E1107)
```

Each `INTERP` has its own bracket depth, starting at 0.

---

## 3. Newlines and separators

A newline ends a field, entry, statement, declaration or other list item, except in the cases
below. The **separator pass** turns line breaks into `NL` tokens after lexing (GRM-02, GRM-03).

### 3.1 Rules

A line break between the previous significant token `P` and the next significant token `N` produces
one `NL` token if and only if all of the following hold:

1. **The innermost open bracket is `{`, or there is none.** Inside `(` or `[` (and inside string
   interpolation) line breaks never separate, even when that `(`/`[` is nested in a `{`. Inside a
   `{` they separate, even when that `{` is nested in `(` or `[`. Only the innermost bracket
   decides.
2. **`P` can end an item.** `P` is not one of: `+ - * / % == != < <= > >= ?? = += -= *= /= => -> .
   ?. , : | ( [ { ...` or the keywords `and or not in is else where as`. A keyword directly after
   `.` or `?.` is a name (§ member access), so it can end an item: `a: Leg.in` newline `b: 1` is two
   items (DECISIONS 302).
3. **`N` does not continue the previous line.** `N` is not one of: `. ?. ?? + * / % == != < <= > >=
   = += -= *= /= => -> |` or the keywords `and or in is else where`; and `N` is not an `@` that
   continues (rule 4).
4. **Annotation lines.** If `N` is `@`, look ahead over a run of annotations (each `@` `WORD`
   with an optional balanced `( … )`), skipping line breaks (blank lines included) and comments,
   to the **next significant token after the run**, wherever it is. If that token is a
   **declaration keyword** — `let local const type enum record variant fn export entry
   view widget test emit check warn`, or `retired` directly followed by `entry` — the annotations
   are prefix annotations of that declaration and the line break before `N` is a separator (if rules
   1–3 allow). Otherwise the `@` line continues the previous line: the annotations belong to the
   item above (farm.canon's own-line `@deprecated`). So an own-line annotation run directly
   followed by a method or check (`fn`, `check`, `warn`, `export`) is always read as that item's
   prefix annotations, never as the trailing annotations of the field above it; FORMATTER.md §7.2
   never prints a field's annotations on their own line in that position.

Notes:

- `-` is not in the list of rule 3: a line starting with `-` starts a new item (a negative
  literal).
- `..` and `..=` are in neither list: an open range may end a line (`let r = 5..`), and a range
  must not be split across lines inside `{ }`.
- A run of several line breaks (blank lines, comment-only lines) produces at most one `NL`.
- Comments and doc comments are trivia: they are never `P` or `N`. A `//` comment always ends its
  line.
- `return`, `break` and `continue` can end an item: `return` followed by a line break returns no
  value.
- `{` on the line after an `if`/`for`/`while`/`match` header, a record name, etc. is not a
  continuation; it is a syntax error (`E1116`). The canonical layout never produces it.
- Between prefix annotations and their declaration, `NL` tokens are skipped by the parser (the
  grammar writes `annotation { NL }`), so `@reload` on its own line before `let` works.

**Separators.** Inside `{ }`, list items are separated by a **separator run**: one or more `NL`
and `,` tokens containing at most one `,` (`E1116` for `,,`). A separator run may also follow the
last item (trailing separator). Inside `( )` and `[ ]`, items are separated by exactly one `,` and a
trailing `,` is allowed. At top level, declarations are separated by `NL` only. Two items on one
line with no separator is `E1117`.

### 3.2 Pseudo-code

```
// tokens: significant tokens from the lexer; t.breaks = number of line breaks before t
func separate(tokens) []Token {
    var stack []Kind              // open brackets
    var out []Token
    for i, t := range tokens {
        if t.breaks > 0 && len(out) > 0 && (len(stack) == 0 || top(stack) == LBRACE) {
            p := out[len(out)-1]  // always a significant token: NL is only emitted before t
            if !cannotEnd(p) && !continuesBefore(t, tokens, i) {
                out = append(out, Token{Kind: NL, Pos: t.Pos})
            }
        }
        out = append(out, t)
        switch t.Kind {
        case LPAREN, LBRACK, LBRACE:
            stack = append(stack, t.Kind)
        case RPAREN, RBRACK, RBRACE:
            if len(stack) == 0 || !matches(top(stack), t.Kind) { report(E1116, t) } else { pop(&stack) }
        }
    }
    return out
}

func continuesBefore(t Token, tokens []Token, i int) bool {
    if t.Kind == AT {
        j := skipAnnotations(tokens, i)   // past "@" WORD ["(" balanced ")"], repeatedly
        return !isDeclKeyword(tokens, j)  // prefix annotation => not a continuation
    }
    return inContinuesBeforeSet(t.Kind)
}
```

### 3.3 Examples

```
check columns.active().first()!.statuses.len() == 1
  else "the first column must hold exactly one status: it is the initial status"
```

One item: the second line starts with `else` (rule 3).

```
local let weapons: [Weapon] = load("@resource/Server/Item/propItem.json", at: "items", partial: true)
  .filter(w => w.kind1 == "IK1_WEAPON")
```

One item: the second line starts with `.`.

```
combos: [
  {
    job: job
    level: level
  }
  for level in levels
]
```

Inside `[`, line breaks are ignored; inside the nested `{`, they separate `job: job` from
`level: level` (GRM-02).

```
maxModels: Int?
  @deprecated("never read by LoadFromFile")
```

One item: the `@` line is not followed by a declaration keyword, so it continues the field.

```
@reload
let potions: [Potion] keyed by id = load.dir("data/*.json")
```

`@reload` is a prefix annotation of `let` (rule 4).

---

## 4. Keywords

### 4.1 Reserved words

```
and amend as asset break check const continue else emit entry enum export expect false fn for if
import in input is layer let load local match none not or package project record ref retired
return self stable table test translation true type var variant view warn where while widget
```

This is the same set as SPEC Appendix B.

### 4.2 Contextual keywords

They are identifiers everywhere except in the listed position.

| Word | Keyword only |
|---|---|
| `keyed` `by` | after a list type: `[T] keyed by f` |
| `ordered` | after the name in `enum Name ordered` |
| `from` `env` | after an `input` field type: `input T from env "VAR"` |
| `fails` `warns` `passes` | after the subject of `expect` |
| `at` | in a one-line `check`/`warn`, after the condition (§5.5) |
| `title` `subtitle` `singular` `plural` `menu` `preview` `search` `filters` `columns` `group` `show` `field` | at the start of an item of a `view` body (and `show`, `field` inside a `group`) |
| `icon` | after `menu WORD` in a view |
| `advanced` `when` | in a view `group` header |
| `multi` | after a name in `filters { … }` |
| `value` `siblings` | as the parameter names of a `widget` (§5.3) |
| `default` | after the parameter list of a `widget` (§5.3) |
| `ext` | as the named argument of `asset(…)` |
| `past` | at the start of a type (not right after another `past`), followed on the same line by a token that can start a type other than `(`, `{`, `from` and `keyed` (`past GrantKind`, `past [E]`, `past ref items`; `past(…)`, `-> past {`, `input past from`, `past keyed by` keep `past` a name; TYPES.md §8.4) |

`it` (the value in a `where` predicate), `fail` and the stdlib functions are ordinary identifiers
with predeclared meanings, not keywords. `self` is reserved.

### 4.3 Reserved words as names (LEX-08)

A reserved word is accepted as a name in exactly these positions:

| Position | Grammar symbol | Examples |
|---|---|---|
| (a) data-symbol declarations: enum member, variant case, table-entry key (in a table literal and in `entry t.KEY`) | `WORD` | `enum Icon { check, package }` |
| (a) translation-key segments | `WORD` | `ModelType.check.unreachable_levels` |
| (a) view item names, `menu`/`icon` values, `filters`/`columns` items, group and show ids | `WORD` | `field check "Check"` |
| (a) match patterns, the right side of `is` | `WORD` | `v is item` |
| (a) the name before `:` in a brace-literal item and in a project-file item | `WORD` | `emit go { package: "teamboard" }` |
| (a) annotation names, argument names and symbol values | `WORD` | `@cpp(type: "DWORD")` |
| (a) the target of `emit`, amend path segments | `WORD` | `emit view { … }` |
| (b) after `.` or `?.` | `WORD` | `Icon.check`, `flags.blocking.retired` |
| (c) value position (a `primary`), for the **nameable** words only | `nameable` | `icon: check`, `icon: package` |

The **nameable** words are the reserved words that can neither start nor continue an expression:

```
amend asset check const emit entry enum expect export import input layer local package project
record ref retired stable table test translation type var variant view warn widget
```

In value position these are read as identifiers (`icon: check` names the `Icon` member `check`).
The other reserved words (`and as break continue else false fn for if in is let load match none
not or return self true where while`) cannot be used bare in value position; a member with such a
name is written qualified (`Icon.return`).

Everywhere else — names of types, records, enums, variants, aliases, functions, methods, lets,
consts, fields, parameters, locals, loop and lambda variables, imports, aliases, packages, layers,
widgets — a reserved word is `E1125`. A field whose wire name is a reserved word uses `@json`:
`kind: RewardType @json("type")`.

`none`, `true`, `false` and `self` may never name an enum member, variant case or table key
(`E1126`), because they would be unreachable in value position.

Inside a variant body, `check`, `warn`, `fn`, `export` and `retired` at the start of an item always
start a check, a method or a retired case, so they cannot name a case.

---

## 5. Grammar

### 5.1 Notation

EBNF: `{ x }` zero or more, `[ x ]` optional, `|` alternatives, `( … )` grouping, quoted strings are
terminals (keywords and punctuation), upper-case names are token classes (§2.1). Two macros:

```
BraceList(X) = "{" [ X { SepRun X } [ SepRun ] ] "}"        (* §3.1 separator runs *)
ParenList(X) = "(" [ X { "," X } [ "," ] ] ")"
```

`stringLit = STRING | RAW | MLSTRING | RAWML`. `WORD = IDENT | reserved word`.
`qualifiedIdent = IDENT { "." IDENT }`. `qualifiedWord = WORD { "." WORD }`.

### 5.2 Files

The kind of a file is decided by its first tokens:

| File | Starts with | Contains |
|---|---|---|
| `project.canon` at the project root | `project` | the project declaration only (§7) |
| source | `package p` | imports, then declarations |
| layer | `package p` NL `layer name` | `amend` declarations only |
| translation | `package p` NL `translation lang` | translation entries only |

```ebnf
projectFile     = { DOC } "project" IDENT BraceList( projectItem ) { NL } EOF ;   (* §7 *)

sourceFile      = { DOC } packageClause NL
                  { importDecl NL }
                  { topDecl ( NL | EOF ) } EOF ;
layerFile       = packageClause NL "layer" IDENT ( NL | EOF ) { amendDecl ( NL | EOF ) } EOF ;
translationFile = packageClause NL "translation" IDENT ( NL | EOF )
                  { translationEntry ( NL | EOF ) } EOF ;

packageClause   = "package" qualifiedIdent ;
importDecl      = "import" qualifiedIdent [ "as" IDENT ] [ BraceList( IDENT ) ] ;
```

- Comments may precede `package`. A doc block directly before `package` in a source file is the
  **package doc** (GRM-08, §9.1). In a layer or translation file it is `W1001`.
- A file that does not start with `package` (or `project` for `project.canon`), a second
  `package` clause, an `import` after a declaration, or a declaration in a layer or translation
  file is `E1127`. A `project` declaration anywhere but `project.canon`, or anything else in
  `project.canon`, is `E1011`.

### 5.3 Top-level declarations

```ebnf
topDecl         = { DOC } { annotation { NL } } topDeclBody ;
topDeclBody     = constDecl | typeDecl | enumDecl | recordDecl | variantDecl | fnDecl
                | letDecl | entryDecl | checkDecl | viewDecl | widgetDecl | testDecl
                | emitDecl ;

constDecl       = [ "local" ] "const" IDENT "=" expr ;
letDecl         = [ "local" ] "let" IDENT [ ":" type ] "=" expr ;
typeDecl        = [ "local" ] "type" IDENT [ typeParams ] "=" type ;
typeParams      = ParenList( param ) ;                          (* at least one *)
param           = IDENT ":" type [ "=" expr ] ;

fnDecl          = [ "local" | "export" ] "fn" IDENT "(" [ fnParams ] ")" "->" type block ;
fnParams        = ( "self" | param ) { "," param } [ "," ] ;

entryDecl       = [ "retired" ] "entry" IDENT "." entryKey braceLit ;
entryKey        = WORD | INT ;

testDecl        = "test" stringLit block ;                     (* constant string: E1132 *)
emitDecl        = "emit" WORD braceLit ;
widgetDecl      = "widget" IDENT "(" "value" ":" type [ "," "siblings" ":" type ] [ "," ] ")"
                  [ "default" ] ;
```

- `local` is allowed only on `const`, `let`, `type`, `fn`, `record`, `enum`, `variant`; `export`
  only on `fn`; `retired` only on `entry` (at top level). Otherwise `E1133`.
- Prefix annotations are allowed on every top-level declaration except `record`, `enum` and
  `variant`, whose annotations go after the name (§5.4); a prefix annotation there is `E1118`.
- `entry t.KEY` adds an entry to the table or keyed list `t` of the same package (AUDIT CLI-02 for
  keyed lists; `entryKey` is an `INT` only for keyed lists whose key field is an integer).
- `emit` targets are words (`go`, `cpp`, `ts`, `json`, `view`, `text`; DECISIONS 294); the options are a brace literal
  whose schema belongs to CODEGEN.md §2.1 / WIRE.md §8 (`package:` is a valid item name there,
  §4.3). Like `project.canon`, the options are a built-in schema: no option value is an
  expression or is resolved in scope (CODEGEN.md §2.1, "Typing of the options"). `out` is a string
  or, for every target but `view`, a list literal of strings (DECISIONS 229).
- A widget's parameter list is the words `value` and, optionally, `siblings` (MOCKUP-GAPS 28), in
  that order: they are contextual keywords there (§4.2). Any other name is `E1116` with the hint
  "a widget takes `value` and `siblings`". Which types are allowed belongs to VIEWMODEL.md
  (G20–G22).
- `default` after the parameter list makes the widget the default editor of its `value` type
  (DECISIONS 22, MOCKUP-GAPS 5): `widget time_of_day(value: TimeOfDay) default`. Its meaning, and
  the rule that only the studio package declares widgets, belong to VIEWMODEL.md G22.

### 5.4 Records, enums and variants

```ebnf
recordDecl      = [ "local" ] "record" IDENT [ typeParams ] { annotation } recordBody ;
recordBody      = BraceList( recordItem ) ;
recordItem      = { DOC } ( fieldDecl | { annotation { NL } } ( fnDecl | checkDecl ) ) ;
fieldDecl       = IDENT ":" fieldType [ "=" expr ] { annotation } ;
fieldType       = "input" type "from" "env" stringLit | type ;

enumDecl        = [ "local" ] "enum" IDENT [ "ordered" ] { annotation } BraceList( enumMember ) ;
enumMember      = { DOC } [ "retired" ] WORD [ "=" ( stringLit | [ "-" ] INT ) ] { annotation } ;

variantDecl     = [ "local" ] "variant" IDENT { annotation } BraceList( variantItem ) ;
variantItem     = { DOC } ( { annotation { NL } } ( fnDecl | checkDecl ) | variantCase ) ;
variantCase     = [ "retired" ] WORD { annotation } [ recordBody ] ;
```

- A field's annotations come last, after the default if there is one:
  `stack: Int(1..=9_999) = 99 @json("nStack")`. They may continue on the next line (§3.1 rule 4).
- A prefix annotation before a field (only possible as the first item of a body) is `E1118`.
- `input` without `from env`, or `from env` without `input`, is `E1131`. The environment variable
  name is a constant string.
- `fnDecl` inside a record or variant body may not be `local` (`E1133`); `export fn` is allowed.
- A case with no fields is written alone (`nothing`) or with an empty body.

```
record Global @json(case: snake) {
  completionRerollCooldown: Duration @json("completion_reroll_cooldown_sec", unit: s)
  levelDiff: LevelDiff @json("levelDiff")
}

enum Element @codes(UInt8) { FIRE = 1, WATER = 2, ELECTRICITY = 3, WIND = 4, EARTH = 5 }

variant EventKind @json(tag: "type") {
  spawn_item {
    itemId: ref items
    groundLifetime: Duration(1m..=1d)? @json("groundLifetimeSec", unit: s)
  }
  nothing
}
```

### 5.5 Checks

```ebnf
checkDecl       = ( "check" | "warn" )
                  ( block
                  | [ IDENT ":" ] expr [ "at" IDENT ] "else" stringLit ) ;
```

- `check {` always starts the block form (GRM-11). `check name: …` is recognized by `IDENT ":"`
  (one token of lookahead after the name).
- `at field` pins a record-level finding to a field of the same record or case (MOCKUP-GAPS 31);
  it must be on the same line as the end of the condition. Meaning: VIEWMODEL.md G19.
- The message is a string literal; it may interpolate (a template). A long message may be a
  multiline string.
- In statement position (inside blocks), `warn` followed by `(` is a call to the builtin `warn`
  (§5.10); only at the start of a record, variant, case or top-level item does `warn` start a
  check.

```
warn unreachable_levels: maxLevel <= levels.len()
  else "maxLevel is {maxLevel} but only {levels.len()} levels exist"
check minValue <= maxValue at maxValue else "min_value is above max_value"
```

### 5.6 Views

```ebnf
viewDecl        = "view" IDENT [ "." WORD ] BraceList( viewItem ) ;
viewItem        = { DOC }
                  ( "title" stringLit
                  | "subtitle" stringLit
                  | "singular" stringLit
                  | "plural" stringLit
                  | "menu" WORD "icon" WORD
                  | "preview" expr
                  | "search" BraceList( expr )
                  | "filters" BraceList( filterItem )
                  | "columns" BraceList( columnItem )
                  | groupItem
                  | showItem
                  | fieldItem ) ;
filterItem      = WORD [ "multi" ] ;
columnItem      = WORD [ INT ] ;
groupItem       = "group" WORD stringLit [ stringLit ] [ "advanced" ] [ "when" headerExpr ]
                  BraceList( groupMember ) ;
groupMember     = { DOC } ( showItem | fieldItem ) ;
showItem        = "show" [ WORD ] stringLit stringLit ;
fieldItem       = [ "field" ] WORD [ stringLit ] [ braceLit ] ;
```

- `view Type` or `view Variant.case` (MOCKUP-GAPS 14). A view of an enum lists members as
  `fieldItem`s: `FIRE "Fire" { icon: flame, tone: danger }`.
- This is the only view grammar; VIEWMODEL.md §3.1 refers to it. There is no `row` item: parallel
  legacy fields are one list of records through `@json(pairs:)` (DECISIONS 21, §8.3).
- **Vocabulary words win** (GRM-17): an item starting with `title`, `subtitle`, `singular`,
  `plural`, `menu`, `preview`, `search`, `filters`, `columns`, `group` or `show` is that vocabulary
  item. To name a field (or member) spelled like one, write `field`: `field title "Title"`. A
  vocabulary word followed by something its production does not accept is `E1116`, with the hint
  "write `field <name>`". `field` followed by a string, a `{` or a separator is a field named
  `field`; `field field` also names it.
- `show [id] "Label" "template"`: the optional id names the line for translations (I18N.md).
- Field props are a brace literal of `name: value` items (`unit: pct`, `widget: ordered_steps`,
  `control: slider`, `help: "…"`, `readonly: true`, `hidden: true`, `placeholder: "…"`,
  `when: expr`, `none: "Any"`, `step: "Winner {index}"`). Their names and value kinds belong to
  VIEWMODEL.md; the parser accepts any `WORD ":" expr` item.
- `plural` (the label of a collection's count, MOCKUP-GAPS 25) and `multi` (a multi-select filter,
  MOCKUP-GAPS 43) are new syntax; their meaning belongs to VIEWMODEL.md.

```
view Task {
  title "{description}"
  columns { eventType 240, filterParam 240, targetPerPlayer 120, maxDuration 110, description }
  filters { eventType multi, targetPerPlayer }
  group mission "Mission" { eventType "Event", filterParam "Only" }
  group text "Text" advanced when description.len() > 0 { description }
}
```

### 5.7 Layers and translations

```ebnf
amendDecl       = { DOC } "amend" IDENT BraceList( amendItem ) ;
amendItem       = { DOC } amendPath ":" expr ;
amendPath       = WORD { "." WORD | "[" expr "]" | "[" "#" INT "]" } ;

translationEntry = { DOC } translationKey stringLit ;
translationKey  = WORD { "." WORD } ;
```

```
amend config {
  paths.resourceRoot: "/home/louis/Desktop/Sovereign/Resource"
  server.port: 9000
}

ModelType.check.unreachable_levels "maxLevel vaut {maxLevel} mais seuls {levels.len()} paliers existent"
Item.show._0 "Dégâts moyens"
```

- `[#n]` in an amend path is a **position** (0-based, as in API paths, API.md §6.1); `[e]` is a
  key or a list index. EVALUATION.md §9.2 gives the meaning of each segment.
- A key segment is a `WORD`. The ids of unnamed `show` lines, `_0`, `_1`, …, are identifiers
  (`letter` includes `_`), so they need no special form.

The key catalogue (which keys exist) is I18N.md; the layer semantics are EVALUATION.md.

### 5.8 Annotations

```ebnf
annotation      = "@" WORD [ "(" [ annArg { "," annArg } [ "," ] ] ")" ] ;
annArg          = [ WORD ":" ] annValue ;
annValue        = stringLit
                | [ "-" ] ( INT | FLOAT | DURATION )
                | "true" | "false"
                | qualifiedWord                                    (* a symbol *)
                | "[" [ annValue { "," annValue } [ "," ] ] "]"
                | "{" "}" ;
```

- No whitespace between `@` and the name. The `(` of the argument list must directly follow the
  name; `@deprecated ("x")` is an annotation without arguments followed by a stray `(`.
- Symbols are **never resolved in scope** (GRM-18): in `@json(unit: s)`, `s` is the unit symbol
  even if a variable `s` exists. What each annotation accepts is the catalogue in §8.
- Positional arguments come before named ones (`E1121`).

### 5.9 Types

```ebnf
type            = unionType ;
unionType       = optType { "|" optType } ;
optType         = primType [ "?" ] [ "where" expr ] ;
primType        = namedType
                | pastType
                | listType
                | mapType
                | depMapType
                | tableType
                | refType
                | fnType
                | assetType
                | matchType
                | stringLit                                        (* literal alternative *)
                | "_"
                | "(" type ")" ;
namedType       = qualifiedIdent [ typeArgs ] ;
typeArgs        = "(" typeArg { "," typeArg } [ "," ] ")" ;
typeArg         = REGEX | expr ;
listType        = "[" type "]" [ typeArgs ] [ "keyed" "by" IDENT ] ;
mapType         = "{" type ":" type "}" [ typeArgs ] ;
depMapType      = "{" IDENT "in" expr ":" type "}" [ typeArgs ] ;
tableType       = [ "stable" ] "table" qualifiedIdent ;
refType         = "ref" qualifiedIdent ;
pastType        = "past" primType ;          (* not before "(", "{", "from", "keyed"; no nesting: §4.2 *)
fnType          = "fn" "(" [ type { "," type } [ "," ] ] ")" "->" primType [ "?" ] ;
assetType       = "asset" "(" stringLit [ "," "ext" ":" "[" extName { "," extName } [ "," ] "]" ]
                  [ "," ] ")" ;
extName         = IDENT | stringLit ;
matchType       = "match" headerExpr BraceList( typeArm ) ;
typeArm         = pattern { "," pattern } "=>" type ;
```

Examples, with the parse:

| Written | Means |
|---|---|
| `ref TalentNode?` | `(ref TalentNode)?` (GRM-04) |
| `Duration(1s..=1d)?` | optional of a refined duration |
| `[Event](..=256) keyed by id` | a keyed list with a length refinement |
| `[Int(1..)] where it.len() == 2 and it[0] <= it[1]` | a list with a predicate |
| `Param(e) \| "default"` | a named type application or a literal alternative |
| `Int? where it > 0` | `none`, or an `Int` greater than 0 (GRM-06) |
| `A \| B where p` | `A \| (B where p)` |
| `{e in eventTypes: HourlyTarget(e)}` | a dependent map |
| `fn(Int, Int) -> Int` | a function type (GRM-15) |
| `(fn(Int) -> Int)?` | optional function type (parentheses required) |
| `asset("@resource/Icon/Item", ext: [dds, png])` | an asset type |

- `namedType` with `typeArgs` is either a refinement or a type application; the parser does not
  decide (GRM-05, §6.6).
- A second `typeArgs` (`Int(0..)(..=5)`, `[T](1..)(..=3)`) is `E1103`. Use `where`.
- `keyed by` is accepted only after a list type (`E1137` elsewhere). `T??` is `E3401`.
- `where` binds to the `optType` it follows; its expression ends at the first token that cannot
  continue an expression (`=`, `@`, `from`, `,`, `)`, `}`, `NL`, …).
- `_` is accepted syntactically anywhere a type is; it is valid only in widget parameter types
  (TYPES.md).

### 5.10 Statements

Statements appear in function bodies, `check { }` blocks and `test { }` blocks.

```ebnf
block           = BraceList( statement ) ;
statement       = letStmt | varStmt | ifStmt | forStmt | whileStmt | matchStmt
                | "break" | "continue" | returnStmt | expectStmt | simpleStmt ;
letStmt         = "let" IDENT [ ":" type ] "=" expr ;
varStmt         = "var" IDENT [ ":" type ] "=" expr ;
ifStmt          = "if" headerExpr block [ "else" ( ifStmt | block ) ] ;
forStmt         = "for" binder [ "," binder ] "in" headerExpr block ;
whileStmt       = "while" headerExpr block ;
matchStmt       = "match" headerExpr BraceList( stmtArm ) ;
stmtArm         = pattern { "," pattern } "=>" ( block | expr ) ;
returnStmt      = "return" [ expr ] ;
expectStmt      = "expect" expr [ ( "fails" | "warns" ) ( stringLit | IDENT ) | "passes" ] ;
simpleStmt      = expr [ assignOp expr ] ;
assignOp        = "=" | "+=" | "-=" | "*=" | "/=" ;
binder          = IDENT | "_" ;
```

- At the start of a statement, `if` and `match` always start the statement forms (GRM-21), whose
  `{` after `=>` is a block (GRM-19).
- `return` takes an expression when the next token is not `NL`, `,` or `}`. `return` outside a
  function body is `E1135`. `break` and `continue` outside a `for` or `while` body are `E1134`.
- `expect` outside a `test` block is `E1130`.
- A call to the builtin `fail` or `warn` that is not lexically inside a `check { }` block is
  `E1105` (CHK-04). Inside one, `warn(at, "message")` is an ordinary call statement.
- The left side of an assignment is parsed as an expression; TYPES.md accepts only
  `name { "[" expr "]" }` (`E3307` otherwise, GRM-20).

### 5.11 Expressions

Precedence from lowest to highest (SPEC §7.1 plus postfix `!` and lambdas):

| Level | Construct | Associativity |
|---|---|---|
| 0 | lambda `x => e` | right (the body extends as far as possible) |
| 1 | `??` | right |
| 2 | `or` | left |
| 3 | `and` | left |
| 4 | `not` (prefix) | — |
| 5 | `==` `!=` `<` `<=` `>` `>=` `in` `is` | none |
| 6 | `..` `..=` | none |
| 7 | `+` `-` | left |
| 8 | `*` `/` `%` | left |
| 9 | unary `-` | — |
| 10 | postfix `.f` `?.f` `[i]` `(args)` `!` | left |

```ebnf
expr            = lambda | coalesce ;
lambda          = lambdaParams "=>" expr ;
lambdaParams    = binder | "(" [ binder { "," binder } [ "," ] ] ")" ;
coalesce        = orExpr [ "??" coalesce ] ;
orExpr          = andExpr { "or" andExpr } ;
andExpr         = notExpr { "and" notExpr } ;
notExpr         = "not" notExpr | compare ;
compare         = range [ compOp range | "is" qualifiedWord ] ;
compOp          = "==" | "!=" | "<" | "<=" | ">" | ">=" | "in" ;
range           = additive [ rangeOp [ additive ] ] | rangeOp additive ;
rangeOp         = ".." | "..=" ;
additive        = multiplicative { ( "+" | "-" ) multiplicative } ;
multiplicative  = unary { ( "*" | "/" | "%" ) unary } ;
unary           = "-" unary | postfix ;
postfix         = primary { postfixOp } ;
postfixOp       = "." WORD | "?." WORD | "[" expr "]" | callArgs | "!" ;
callArgs        = ParenList( arg ) ;
arg             = [ IDENT ":" ] expr ;

primary         = literal
                | IDENT | nameable                                 (* §4.3 (c) *)
                | "self"
                | "(" expr ")"
                | shorthand
                | listLit
                | braceLit
                | typedLit
                | ifExpr
                | matchExpr
                | loadCall ;
shorthand       = "." WORD { postfixOp } ;       (* the whole chain belongs to the implicit lambda *)
typedLit        = qualifiedWord braceLit ;        (* never at depth 0 of a header, §6.1 *)
ifExpr          = "if" headerExpr exprBody { "else" "if" headerExpr exprBody } "else" exprBody ;
exprBody        = "{" [ SepRun ] expr [ SepRun ] "}" ;
matchExpr       = "match" headerExpr BraceList( exprArm ) ;
exprArm         = pattern { "," pattern } "=>" expr ;
pattern         = "_" | "none" | qualifiedWord [ "(" binder ")" ] ;
loadCall        = "load" [ "." IDENT ] callArgs ;

listLit         = "[" [ expr { "," expr } [ "," ] ] "]"
                | "[" expr compClause { compClause } "]" ;
braceLit        = BraceList( braceItem )
                | "{" [ SepRun ] braceItem { NL } compClause { { NL } compClause } [ SepRun ] "}" ;
braceItem       = { DOC }
                  ( "..." expr                                     (* spread *)
                  | [ "retired" ] WORD { annotation } braceLit     (* table entry *)
                  | WORD ":" expr                                  (* named item *)
                  | expr ":" expr ) ;                              (* keyed item *)
compClause      = "for" binder [ "," binder ] "in" expr
                | "if" expr
                | "let" IDENT "=" expr ;
literal         = INT | FLOAT | DURATION | stringLit | "true" | "false" | "none" ;
headerExpr      = expr ;                                           (* parsed in header mode, §6.1 *)
```

- `a < b < c`, `a == b == c`, `a..b..c` and `a in b in c` are `E1128` (comparisons do not chain).
- `not` applies to a whole comparison: `not a in b` is `not (a in b)`.
- A range operand is omitted when the next token cannot start an expression: `0..`, `schedule[i +
  1..]`, `..=256`. `xs[a..b]` is an index whose value is a `Range` (GRM-13: there is no separate
  slice syntax; slicing is STDLIB.md).
- `x!` asserts presence (DECISIONS 17). The parser allows it on any postfix expression; TYPES.md
  decides where it is valid. Narrowing on `!= none` is semantic and needs no syntax.
- `x?.f` is an ordinary postfix step. Whether it short-circuits the rest of the chain
  (`w?.item.id`) is decided by TYPES.md; the AST keeps the chain as nested postfix nodes.
- `.name …` at the start of an expression is a **shorthand lambda**: `.default` means
  `x => x.default`, `.compared()[k]` means `x => x.compared()[k]`. It includes every postfix step
  written directly after it, and nothing else (`.a + 1` is `(x => x.a) + 1`, a type error).
- `load` is always followed by `(` or `.IDENT(`: `load("…")`, `load.dir("…")`,
  `load.defines("…", prefix: "MI_")`.
- A comprehension brace literal has exactly one item, which is a named or keyed item
  (`{ name: v for name, job in jobs }`); anything else is `E1136`. Line breaks before its clauses
  are allowed (the `{ NL }` in the production): the item is a comprehension as soon as a `for`
  follows it. Inside `[ ]` line breaks never matter.

### 5.12 Literals of composite values

The parser builds one generic node for every `{ … }` literal: `BraceLit` with its items (spread,
entry, named, keyed) or its comprehension. Whether it is a record, map, table, variant case or
comprehension, and what a named item's name refers to, is decided by TYPES.md (GRM-10). Examples
of how each item is parsed:

| Item | Parsed as |
|---|---|
| `...sample` | spread |
| `open { tone: warning }` | table entry `open` |
| `retired old { … }` | retired table entry `old` |
| `tone: warning` | named item `tone` |
| `package: "teamboard"` | named item `package` (§4.3) |
| `"Cap": [a, b]` | keyed item, key is a string expression |
| `(name): v` | keyed item, key is the expression `name` |
| `name: v for name, job in jobs` | map comprehension; the key is the expression `name` |

A typed literal (`TimeOfDay { hour: 8, minute: 30 }`, `Reward.item { … }`,
`monster_drop_inject { itemId: … }`) is a `qualifiedWord` directly followed by a brace literal on
the same line.

---

## 6. Disambiguation rules

### 6.1 The header rule (GRM-01)

The expression after `if`, `else if`, `while`, `for … in`, `match` (statement, expression and
type level) and a view `group … when` is parsed in **header mode**. In header mode, at bracket
depth 0 relative to the start of the header:

1. a `{` never continues the expression: it is never the body of a typed literal, and it never
   starts a brace literal. It ends the header and opens the block, body or arm list;
2. an `if` or `match` expression, or a `{` where an operand is expected, is `E1129`
   ("parenthesize it").

Inside any `(`, `[` or `{` opened within the header, header mode is off. To use a typed literal
or a brace literal in a header, parenthesize it:

```
if settings.includePuppeteer { ["JOB_PUPPETEER"] } else { [] }   // header is `settings.includePuppeteer`
match e.param { monster => ref monsters, … }                      // header is `e.param`
group combat "Combat" when kind is IK1_WEAPON { attackMin }       // header is `kind is IK1_WEAPON`
if origin == (Point { x: 0, y: 0 }) { … }                         // parenthesized typed literal
for s in statuses.filter(s => s == Status.open) { … }             // inside ( ): header mode off
```

Comprehension clauses are not headers.

### 6.2 Items starting with a word

At the start of an item the parser decides with at most two tokens of lookahead:

| Context | Rule |
|---|---|
| record body | `check`/`warn` → check; `fn`/`export`/`@` → method (or check) with prefix annotations; otherwise `IDENT ":"` → field |
| variant body | `check`/`warn` → check; `fn`/`export`/`@` → method; otherwise a case |
| brace literal | `...` → spread; `retired WORD` → retired entry; `WORD` followed by `{` or `@` → entry; `WORD ":"` → named item; otherwise keyed item |
| view body | vocabulary word → that item; `field` → field item; otherwise field item |
| statement | `let`, `var`, `if`, `for`, `while`, `match`, `break`, `continue`, `return`, `expect` → that statement; otherwise expression (then an optional assignment) |

### 6.3 Lambdas vs parenthesized expressions (GRM-12)

At the start of an `expr`: an `IDENT` or `_` directly followed by `=>` starts a lambda. A `(` starts
a lambda if the token after its matching `)` is `=>`; the parenthesized tokens must then be binders
separated by commas. Otherwise `(` starts a parenthesized expression. Match arms parse patterns, so
`monster => ref monsters` is an arm, never a lambda. A `{` right after a lambda's `=>` is a brace
literal (lambda bodies are expressions).

### 6.4 Brace literal classification

Delegated to TYPES.md (GRM-10). The parser only fixes the item kinds of §5.12.

### 6.5 `match` arms

In a `matchStmt`, `{` after `=>` starts a block. In a `matchExpr` and a `matchType`, it starts a
brace literal (GRM-19). A statement starting with `match` is a `matchStmt`; `return match x { … }`
is a `matchExpr`.

### 6.6 Refinement vs type application (GRM-05)

`Name(args)` is parsed once as `namedType` with `typeArgs`. After resolution (TYPES.md): if `Name`
is a built-in scalar or an alias of one, the arguments must be exactly one range or one regex and
it is a refinement; if it is a parameterized record or alias, the arguments are values. A list or
map type followed by `typeArgs` is always a length refinement.

### 6.7 Other rules

- `is` takes a `qualifiedWord`, resolved against the left side's variant (GRM-16): `v is item`,
  `v is Reward.item`.
- `ref` takes a `qualifiedIdent` only (GRM-04): `ref items`, `ref Status`, `ref vocab.items`.
- A named argument is recognized by `IDENT ":"` at the start of an argument. Positional arguments
  after named ones, and duplicate names, are `E1121`.
- A `.` followed by a `WORD` is always member access; after `.`, reserved words are names
  (`flags.blocking.retired`).
- A typed literal requires the `{` on the same line as its name (a line break there is a
  separator in a `{` context, and not allowed in header mode).
- `-` applied to a literal is folded into the literal (§2.4); `-x.f` is `-(x.f)`.

---

## 7. The project file

`project.canon` sits at the project root and contains only the project declaration (GRM-07).

```ebnf
projectFile     = { DOC } "project" IDENT BraceList( projectItem ) { NL } EOF ;
projectItem     = { DOC } WORD ( ":" pValue | BraceList( pEntry ) ) ;
pEntry          = { DOC } ( WORD | stringLit ) ":" pValue ;
pValue          = stringLit | INT | qualifiedIdent
                | "[" [ pValue { "," pValue } [ "," ] ] "]"
                | BraceList( pEntry ) ;
```

- `key { … }` is sugar for `key: { … }` (only for map-valued keys). Strings are constant strings
  (`E1132`). There are no expressions: no arithmetic, no `load`, no names other than the symbols
  below.

### 7.1 Built-in schema

| Key | Type | Default | Rules |
|---|---|---|---|
| `canon` | string `"MAJOR.MINOR"` | required (`E1004`) | must match `^[0-9]+\.[0-9]+$` (`E1010`); an unsupported version is `E1001` (NFR-03: same major, known minor) |
| `roots` | map name → path string | `{}` | names are `IDENT`s, unique (`E1005`); paths are non-empty, relative to the project directory, `/`-separated, not absolute, no `\` (`E1007`). Roots may point outside the project directory. Path resolution inside roots: SPEC §3.1 and GEN-04 |
| `languages` | list of language codes, at least one | `[en]` | each matches `^[a-z]{2,3}(_[A-Z][a-z]{3})?(_[A-Z]{2})?$`, written as an identifier (`en`, `pt_BR`); no duplicates (`E1008`). The first is the source language |
| `studio` | package path | none | the package holding the studio vocabulary (SPEC §16.11); it must exist (`E1012`) |
| `budget` | integer ≥ 1 | `100_000_000` | evaluation steps (EVALUATION.md); out of range `E1006` |
| `go_module` | map root name → Go module path string | `{}` | EMT-06: the Go import path prefix of outputs under that root. Keys must be declared roots and values non-empty with no spaces (`E1009`); use: CODEGEN.md |

- An unknown key is `E1002`. A duplicate key is `E1005`. A value of the wrong kind is `E1006`.
- Keys may appear in any order. Doc comments on keys are allowed and ignored by the compiler.
- `canon` looks for `project.canon` in the current directory and its parents (CLI §2.1); none
  found is `E1003` (exit 2).

```
/// The law repository of Sovereign.
project sovereign {
  canon: "0.1"

  /// Named roots used by `load` and `emit` paths: "@resource/Server/...".
  roots {
    resource: "../../../Resource"
    services: "../.."
    sovcommon: "../../sovcommon"
  }

  languages: [en, fr]
  studio: studio
  budget: 100_000_000
  go_module {
    services: "gitlab.com/sovereign15"
    sovcommon: "gitlab.com/sovereign15/sovcommon"
  }
}
```

---

## 8. Annotation catalogue

GRM-18. Every annotation, where it may appear, and what its arguments are. The parser (with the
resolver for type-dependent rules) checks names, positions, argument kinds and duplicates; the
meaning belongs to the document in the last column.

### 8.1 Positions

| Code | Position | Syntax |
|---|---|---|
| `TL` | top-level declaration, prefix | `@reload let …`, `@deprecated("…") entry items.X { … }` |
| `TH` | type header of `record`, `enum`, `variant`, after the name (and parameters) | `record Global @json(case: snake) {` |
| `FD` | field of a record or case, after the type and default | `heal: Int @json("nHeal")` |
| `EM` | enum member, after its value | `FIRE = 1 @json("fire")` |
| `VC` | variant case, after its name | `item @json("Item") { … }` |
| `TE` | table entry in a table literal, after its key | `old @deprecated("gone") { … }` |
| `MB` | method or check inside a record, variant or case body, prefix | `@go(name: "Str") fn toString(self) -> String { … }` |

An annotation in a position it does not allow is `E1118`. The same annotation twice at one position
is `E1120` (combine the arguments into one annotation).

### 8.2 Argument kinds

| Kind | Written as | Notes |
|---|---|---|
| `string` | constant string | interpolation is `E1119` |
| `template` | string whose interpolations are names or `.`-paths | no format specs, no calls |
| `int` | integer literal, optionally negative | |
| `symbol{…}` | a `WORD` from the listed closed set | never resolved in scope |
| `studio{Enum}` | a `WORD` naming a member of that enum of the project's studio package (RES-06) | not resolved in lexical scope |
| `literal` | string, number, duration, `true`, `false`, `{}` or `[]` | a value of the field's wire form |
| flag | a bare positional symbol | e.g. `inline` |

A missing required argument, a value of the wrong kind, a value outside its closed set, or two
mutually exclusive arguments is `E1119`. An unknown annotation name or an unknown argument name is
`E1104`.

### 8.3 Catalogue

| Annotation | Arguments | Positions | Meaning in |
|---|---|---|---|
| `@json("wire")` | positional `string` | `FD`, `VC`; `EM` only in an enum with `@codes` (other members use `= "wire"`) | WIRE.md |
| `@json(path: "a.b")` | `string` (dotted, no empty segment) | `FD` | WIRE.md (LOD-09) |
| `@json(case: …)` | `symbol{snake, camel, kebab, upper_snake}` | `TH` (record, variant), `VC` | WIRE.md (WIR-08) |
| `@json(tag: "key")` | `string` | `TH` (variant) | WIRE.md |
| `@json(inline)` | flag | `FD` (variant-typed field) | WIRE.md (WIR-07) |
| `@json(none: v)` | `literal` | `FD` (optional field) | WIRE.md (WIR-02) |
| `@json(unit: u)` | `symbol{ms, s, m, h, d}` | `FD` (a type holding `Duration` outside named types: `Duration`, `Duration?`, lists and map values of them; WIRE.md §4.1 decides) | WIRE.md (LOD-10) |
| `@json(int)` | flag | `FD` (`Bool` field): written `0`/`1` | WIRE.md (MOCKUP-GAPS 7) |
| `@json(bits)` | flag | `FD` (`[Enum]` field): written as a bitmask integer | WIRE.md (MOCKUP-GAPS 7) |
| `@json(codes)` | flag | `TH` (enum with `@codes`): the wire value is the code | WIRE.md (WIR-04) |
| `@json(pairs: [k, v])` | list of exactly two `template`s whose only variable is `{i}` (the position, from 0) | `FD` (list of a two-field record): element `i` is written as the parallel wire keys `k(i)` and `v(i)` | WIRE.md §5.14 (DECISIONS 21, MOCKUP-GAPS 49) |
| `@stable` | none | `FD` | LOCK.md (LCK-02) |
| `@codes(T)` | positional `symbol{Int8, Int16, Int32, Int, UInt8, UInt16, UInt32, UInt64}` | `TH` (enum) | LOCK.md, TYPES.md |
| `@deprecated` / `@deprecated("why")` | optional positional `string` | `FD`, `EM`, `VC`, `TE`, `TL` (`entry` only) | TYPES.md (TYP-22), VIEWMODEL.md |
| `@since(n)` | positional `int` ≥ 1 | every position | documentation only |
| `@reload` | none | `TL` (`let`) | CODEGEN.md (SPEC §15.5) |
| `@files("tpl")` | positional `template`; scope: `id` (the key), the entry's fields `{f}` and nested paths `{f.g}` through records, the current case of variants and a dependent field's branches (any branch's fields); a variant-typed segment stands for its case's wire name; a name outside that scope, or a field no case or branch declares, is the checker's unknown-name finding at the first name outside the scope (`x` in `{id.x}`; DECISIONS 277, 280) | `TL` (`let` of a table or keyed list) | API.md N2 (API-03) |
| `@menu(m, icon: i, label: "l")` | positional `studio{Menu}`, optional named `icon: studio{Icon}` and `label: string` | `TL` (`let`) | VIEWMODEL.md G23 (VIEW-03) |
| `@cpp(defines: "P")` | `string` | `TH` (enum) | CODEGEN.md (CPP-03) |
| `@cpp(struct: "S", header: "h", access: a)` | `string`, `string`, `symbol{fields, both, getters}` | `TH` (record) | CODEGEN.md (CPP-01) |
| `@cpp(field: "m", type: "T")` | `string`, `string` | `FD` | CODEGEN.md §7.8 |
| `@cpp(value: N)` | `int` | `VC` (case of a variant held by a legacy struct) | CODEGEN.md §7.8 |
| `@cpp(unit: u)` | `symbol{ms, s, m, h, d}` | `FD` (`Duration` field mapped to a legacy member) | CODEGEN.md §7.8 |
| `@text("file")` | positional `string` (a file name: not empty, `.` or `..`, no `/` or `\`) | `TL` (`fn`: public, package-level, no parameter, result `String` or any type with a wire form) | CODEGEN.md §2.9 (DECISIONS 294, 308) |
| `@cpp(name: "N")`, `@go(name: "N")`, `@ts(name: "N")` | `string` (a target identifier) | `TH`, `FD`, `EM`, `VC`, `MB`, `TL` (`let`, `const`, `type`, `fn`) | CODEGEN.md (CG-02) |
| `@ts(bigint)` | flag | `FD` (a field with an integer position outside a ref: the integer, list elements, map keys and values, optional contents; DECISIONS 279) | CODEGEN.md (SPEC §15.4) |

Combination rules checked here:

- One `@json`, one `@cpp`, one `@go`, one `@ts` per position; their arguments combine:
  `@json("monsterLifetimeSec", unit: s)`, `@cpp(struct: "ItemProp", access: fields)`.
- In one `@json`: the positional wire name, `path:` and `pairs:` exclude each other; `inline`,
  `int`, `bits`, `unit:` and `pairs:` exclude each other; `tag:` and `case:` may be combined on a
  variant. `@json("bPartsFile", int)` and `@json("dwFlag", bits)` are valid.
- `@json(codes)` without `@codes` on the same enum is `E1119`.
- In one `@cpp`: `struct:`, `header:` and `access:` go together on a record header (`header:`
  without `struct:` is `E1119`); `field:`, `type:` and `unit:` go on fields; `value:` on cases;
  `name:` combines with any of them.

Type-dependent rules (for example `unit:` on a non-Duration field, `none:` on a required field)
are checked by TYPES.md / WIRE.md, with their own codes.

---

## 9. Doc comments and naming conventions

### 9.1 Doc comment attachment

A doc block attaches to the item that directly follows it, when there is no blank line between the
block and the item. Ordinary comments between them do not break the attachment. Attachable items:

- every top-level declaration (including `check`, `warn`, `test`, `emit`, `view`, `widget`,
  `entry`), and `package` (package doc, GRM-08) in source files;
- record and case fields, record and variant methods and checks, enum members, variant cases,
  table entries in table literals;
- view items, group members, `amend` declarations and items, translation entries, project items.

A doc block followed by a blank line, by something it cannot attach to (a statement, a brace-literal
item other than a table entry, an `import`, a closing `}`, the end of the file), or written after
code on the same line is `W1001`, and is kept as an ordinary comment.

The package doc of a multi-file package is the concatenation of the package docs of its source
files in byte order of their paths, separated by a blank line (`\n\n`).

`W1002`: a public (not `local`) `record`, `enum`, `variant` or `type` alias, a field of a public
record or of a case of a public variant, or an `export fn`, without a doc comment.

### 9.2 Naming conventions (FMT-03)

Reported by `canon check` (not by `canon fmt`) as `W1003`, on declarations of the selected packages
only.

| Declared name | Convention | Pattern |
|---|---|---|
| record, enum, variant, type alias | UpperCamel | `^[A-Z][A-Za-z0-9]*$` |
| field, function, method, `let`, parameter, type parameter, local, loop and lambda variable, import alias, package segment | lowerCamel | `^[a-z][A-Za-z0-9]*$` |
| `const` | UPPER_SNAKE | `^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$` |
| widget | lower_snake | `^[a-z][a-z0-9]*(_[a-z0-9]+)*$` |
| enum member, variant case, table key, check name, group and show id, layer name | free (they are data) | |

Example: `const version = 7` is `W1003`; write `const VERSION = 7`, as taxonomy.canon does.

---

## 10. What the parser produces

- A **lossless concrete syntax tree**: every token with its span and leading trivia (whitespace,
  line-break count, comments, doc comments), so that printing the tree reproduces the file byte for
  byte. `canon fmt` and the edit API (FORMATTER.md) work on it.
- An **AST** derived from it, with one node kind per production of §5 (named after the
  production), each with its span. Doc blocks are attached to their item as normalized text
  (§2.2). Brace literals are the generic `BraceLit` node (§5.12). `namedType` keeps its unresolved
  `typeArgs`. String tokens keep their parts.
- **Error recovery.** A syntax error never stops parsing of the file. After an error the parser
  skips to the next separator run of the innermost enclosing `{ }` list (or the next top-level
  declaration keyword at the start of a line) and resumes. At most one `E1116` is reported per
  item. The rest of the build continues with the items that parsed (SPEC §10.3: every error is
  reported).

---

## 11. Examples that do not parse under this grammar

Checked against SPEC.md and every file under `examples/` as they stood on 2026-09-23, including
`features/` and `game/items/`, which the examples errata added while this document was written.
Layout differences (the examples are not yet fixed points of the formatter) are listed in
FORMATTER.md §15, not here.

| File | Line (snippet) | Problem | Needed change |
|---|---|---|---|
| SPEC.md §4.2 | `let statuses: stable table Status = { open { … }  taken { … } }` | two entries on one line without a separator (`E1117`) | `{ open { … }, taken { … } }` |
| SPEC.md §6.1 | `{ open { … }  taken { … } }` | same (`E1117`) | `{ open { … }, taken { … } }` |
| SPEC.md §5.4, §6.1, §4.3 | `fn lengthMinutes(self) -> Int { … }`, `IK1_WEAPON { … }` | `…` is elided text, not syntax | none (it is prose) |
| SPEC.md App. A, App. B | the whole appendices | superseded | replace by a pointer to this document |

Every file under `examples/` parses under this grammar. That includes the constructs the audit
flagged: ui.canon's `check`/`package` members and taxonomy.canon's `icon: check` /
`icon: package` (§4.3), farm.canon's own-line `@deprecated` (§3.1 rule 4), potion.canon's and
item.canon's own-line `@reload` / `@files` (rule 4), `project.canon` (§7), the `///` blocks before
`package` (§9.1), `emit go { … package: "…" }` (§4.3), sweep_plan.canon's
`if settings.includePuppeteer { … }` (§6.1) and vocab.canon's `match e.param {` (§6.1). It also
includes the newer `first()!` (§5.11), `@json(pairs: …)` and `@json("bPartsFile", int)` (§8.3),
`retired event = 3` members, `view ItemKind.IK1_WEAPON`, the `show` line,
`widget weight_share(value: Int, siblings: [Int])` and `widget time_of_day(value: TimeOfDay) default`.

Unnamed `show` lines are keyed `_0`, `_1`, … (§5.7, I18N.md §3.3): the key in
`game/items/item.fr.canon` is `ItemKind.IK1_WEAPON.show._0`, not `…show.0`.

Semantic problems in the examples (for example TYP-06 unannotated `load`, DECISIONS 17 unwraps,
GEN-03 package directories, GEN-04 emit roots) belong to the other documents' errata.

---

## 12. Diagnostics

Codes owned by this document: `E10xx` (project file), `E11xx` (lexer and parser), `W10xx` (doc
comments and naming). `E3307` and `E3401` are referenced here and defined in TYPES.md.

Messages (templates and typed arguments) are defined only in [ERRORS.md](ERRORS.md), the single
source of diagnostics (DECISIONS 27); this table says when each code fires.

| Code | Severity | Trigger |
|---|---|---|
| E1001 | error | `canon` names a version this compiler does not support (§7.1) |
| E1002 | error | a key not in §7.1 |
| E1003 | error | project search failed (exit 2) |
| E1004 | error | `canon` key missing |
| E1005 | error | a key or root name given twice |
| E1006 | error | a project value of the wrong kind or out of range |
| E1007 | error | bad root name or path (absolute, empty, `\`) |
| E1008 | error | `languages` item |
| E1009 | error | key is not a declared root, or empty/invalid module path |
| E1010 | error | malformed version |
| E1011 | error | misplaced `project` or extra content |
| E1012 | error | `studio` names a missing package |
| E1101 | error | depth-0 `:` in an interpolation not followed by a valid spec and `}` |
| E1102 | error | indentation prefix of a multiline string |
| E1103 | error | a second `typeArgs` |
| E1104 | error | name not in §8.3 |
| E1105 | error | `fail`/`warn` call elsewhere (CHK-04) |
| E1106 | error | a character no token can start with (§1), or a `#` outside an amend-path position segment |
| E1107 | error | end of line/file inside a string, raw string or multiline string |
| E1108 | error | `/*` without `*/` |
| E1109 | error | unknown escape, bad `\u{…}` |
| E1110 | error | misplaced `_`, leading zero, bad suffix, exponent without digits |
| E1111 | error | unit order, repeated unit, fraction, trailing digits, overflow |
| E1112 | error | empty `{}`, newline or comment inside, unterminated, lone `}` in text |
| E1113 | error | newline or EOF before the closing `/` |
| E1114 | error | RE2 rejects the pattern |
| E1115 | error | `REGEX` elsewhere |
| E1116 | error | any other syntax error |
| E1117 | error | two items on one line |
| E1118 | error | position not in §8.3; prefix annotation on a record/enum/variant or field |
| E1119 | error | wrong argument kind, value, missing or exclusive arguments |
| E1120 | error | duplicate annotation at one position |
| E1121 | error | call or annotation arguments |
| E1122 | error | text after the opening `"""`, closing `"""` not alone, under-indented line |
| E1123 | error | UTF-8 BOM |
| E1124 | error | §1 |
| E1125 | error | reserved word outside §4.3 positions |
| E1126 | error | `none`/`true`/`false`/`self` as data symbol |
| E1127 | error | file structure (§5.2) |
| E1128 | error | non-associative operators chained |
| E1129 | error | `if`/`match` expression or brace literal at depth 0 of a header |
| E1130 | error | `expect` elsewhere |
| E1131 | error | `input` syntax |
| E1132 | error | test name, project string, input variable name |
| E1133 | error | `local`/`export`/`retired` misuse |
| E1134 | error | `break`/`continue` outside `for`/`while` |
| E1135 | error | `return` in a check or test block |
| E1136 | error | malformed comprehension |
| E1137 | error | `keyed by` after another type |
| W1001 | warning | §9.1 |
| W1002 | warning | §9.1 |
| W1003 | warning | §9.2 |
