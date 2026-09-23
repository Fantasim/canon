# A trimmed catalogue

## E10xx: Project file

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E1001 | error | project | GRAMMAR.md §7.1 | unsupported version |
| W1001 | warning | syntax | GRAMMAR.md §9.1 | a doc comment attaches to nothing |
| E1002 | error | project | GRAMMAR.md §7.1 | unknown key in `project.canon` |
| E1003 | error | project | GRAMMAR.md §7.1 | no `project.canon` |

| Code | Variant | Args | Template |
|---|---|---|---|
| E1001 | - | version:Text, supported:Names | `project requires Canon {version}; this compiler supports {supported}` |
| W1001 | - | - | `doc comment is not attached to anything` |
| E1002 | - | key:Name | `unknown project key "{key}"` |
| E1003 | - | dir:Path | `no project.canon found in {dir} or its parents` |

## E4xxx: Evaluation

| Code | Severity | Package | Owner | Meaning |
|---|---|---|---|---|
| E4101 | error | eval | TYPES.md §9 | integer overflow |
| E8303 | runtime | gen | CODEGEN.md §8 | outside the safe range |

| Code | Variant | Args | Template |
|---|---|---|---|
| E4101 | - | op:Name | `integer overflow in {op}` |
| E8303 | - | - | `integer outside the TypeScript safe range` |

| Code | Text |
|---|---|
| E4101 | `integer overflow in +` |
