# canon audit rules

The rulebook `internal/rules/rules.go` holds; `TestRulesDocInSync` keeps this file equal to it,
order included. Limits live in `internal/rules/constants.go`. Modes:

- **enforce**: any finding fails `check`.
- **ratchet**: findings recorded in `.sovaudit/baseline.tsv` pass; a new one, or one whose measured
  value grew (a longer function, a higher ratio), fails.
- **observe**: counted by `audit`, never gates.
- **off**: the repo turned it off in `.sovaudit/state.tsv` (a maintainer's call).

A repo overrides a mode in `.sovaudit/state.tsv` (`<rule>\t<mode>`). Silence one finding with
`// sovaudit:ignore <rule> -- <reason>` on or above its line (`sovaudit:ignore-file` for a file;
in Markdown, an HTML comment opening its line); an ignore without a reason ignores nothing and is
itself a finding.

Never audited: `vendor/`, `testdata/`, `examples/_fixtures/`, every `expected/` tree under
`examples/` (generated goldens), nested modules, and generated Go files (the
`// Code generated ... DO NOT EDIT.` marker, or a `.gen.go` name).

## Size and shape

### `fn-length`
ratchet · custom: function body over 60 lines or 40 statements (tests: 100 lines).
A function a reader can't hold on one screen gets patched instead of understood.

### `fn-params`
ratchet · custom: more than 5 parameters.
Long parameter lists hide a missing type and invite argument-order bugs. A method in a _test.go
file is exempt: a mock mirrors the interface it stands in for.

### `fn-results`
ratchet · custom: more than 3 return values.
More than three results is a struct that hasn't been named yet. Test-file methods are exempt,
as for fn-params.

### `fn-complexity`
ratchet · gocognit: cognitive complexity over 15.
Cognitive complexity is what makes a reviewer (human or agent) miss a branch.

### `fn-nesting`
ratchet · custom: control flow nested deeper than 3 levels.
Deep nesting buries the happy path; early returns keep it at the left margin.

### `naked-return`
ratchet · custom: bare return in a function longer than 5 lines.
In a long function a bare return hides which values leave it.

### `file-length`
ratchet · custom: file over 500 lines.
Big files are read whole into context; split by concern.

### `pkg-size`
observe · custom: package over 15 files or 4000 lines.
An oversized package is several packages that share a directory.

## Magic values and constants

### `magic-string`
ratchet · custom: string literal used 2+ times.
A literal typed twice drifts; a named constant is found by one grep. Not judged: test code (e2e
harness and *stub packages included), log and log/slog arguments (logger methods resolved by
type), map keys and map indexes, and the pattern of a router's Get/Post/Handle/HandleFunc/Method
etc.

### `magic-number`
ratchet · custom: number other than 0 and 1 outside a const declaration.
A bare number says nothing about its unit or why it has that value. Not judged: a shift count, a
count of a time unit (`24 * time.Hour`), a strconv base or bit size, anything in constants.go,
test code.

### `const-placement`
ratchet · custom: const, or package-level literal var, outside <pkg>/constants.go or errors.go.
One constants.go per package: a reader knows where to look and where to add.
A SQL statement (SELECT, INSERT, WITH...) stays beside its function; a fragment does not.

### `const-dup`
ratchet · custom: one string value declared under two constant names.
Two names for one value will be changed one at a time. Upper-case SQL keyword fragments
(" AND ", "ORDER BY") are exempt: they name nothing.

### `env-key`
ratchet · custom: environment read outside cmd/ or a config package, or with a literal key.
Configuration is read in one place, so every knob is discoverable; a library that reads the
environment behind its caller's back cannot be embedded.

## Errors

### `err-inline`
ratchet · custom: errors.New inside a function, or fmt.Errorf without %w.
Callers can only errors.Is against a sentinel; inline errors are untestable strings.

### `err-placement`
ratchet · custom: error sentinel declared outside <pkg>/errors.go.
One errors.go per package lists every way the package can fail.

### `err-compare`
ratchet · errorlint: == or type assertion on an error.
== breaks as soon as anyone wraps the error.

### `err-unchecked`
ratchet · errcheck: error result not checked.
An ignored error is a silent failure. `(*sql.Rows).Close`, `(*sql.Tx).Rollback` and
`(io.ReadCloser).Close` are exempt (cleanup nothing can act on); `(*os.File).Close` is not.

### `err-unwrapped`
ratchet · wrapcheck: error from another module returned unwrapped, or formatted with %v.
An unwrapped error loses where it happened. A row-scan callback, `func(*sql.Rows) (T, error)`, is
exempt: its caller wraps.

### `nil-err`
ratchet · nilerr: returns nil inside an err != nil branch.
Returning nil on the error branch reports success for a failure.

### `err-style`
enforce · staticcheck ST1005: error string capitalised or ending in punctuation.
Error strings are composed with others; capitals and periods break the sentence.

## API surface

### `exported-but-local`
ratchet · custom: exported identifier used only inside its own package.
Exported surface is a promise to other packages; unused promises are noise. An external `_test`
package counts as another package, so an Example test keeps a public API export legitimate.

### `unused-param`
ratchet · unparam: parameter always unused or always the same value.
A parameter nobody varies is dead API.

### `pkg-doc`
ratchet · custom: package without a doc.go.
The package comment lives in one known file, the only place a file header is allowed.

### `pkg-example`
ratchet · custom: package other than main without an Example test.
An Example is documentation the test run keeps true; a package without one is learnt by reading
its code.

## Dead code

### `dead-unexported`
ratchet · unused: unused unexported identifier.
Dead code is read, maintained and tested for nothing.

### `dead-unreachable`
ratchet · deadcode: function no main or test can reach.
Code no entry point reaches is dead even if something references it.

### `dead-file`
ratchet · deadcode: file whose every function is unreachable.
A whole dead file is the cheapest deletion there is.

### `commented-code`
ratchet · gocritic: commented-out code.
Git keeps history; commented-out code only confuses the next reader.

### `dead-assign`
ratchet · ineffassign, wastedassign: value assigned and never read.
A value written and never read is a bug or a leftover.

### `todo-in-code`
ratchet · custom: TODO, FIXME, XXX or HACK in code.
TODOs in code are never scheduled; the issue tracker is where work is tracked.

## Duplication

### `reimplements-stdlib`
ratchet · custom, modernize: hand-written stdlib helper, or pre-modern Go (interface{}, int loops, loop var copies).
The standard library comes first: its version is tested, known, and shorter. Test code is not
judged.

### `dup-in-repo`
ratchet · dupl: duplicated block of ~15+ lines in one repo.
Duplicated blocks get fixed in one place and not the other.

## Comments

### `comment-decl`
ratchet · custom: doc comment over 3 lines on a declaration.
Long doc comments cost every reader on every read and rot silently.

### `comment-file-header`
ratchet · custom: comment block above package/imports outside doc.go.
File headers narrate context that belongs in doc.go, DECISIONS.md or git.

### `comment-pkg`
ratchet · custom: package doc over 10 lines.
The package doc says what the package is, not its history.

### `comment-block`
ratchet · custom: more than 3 consecutive comment lines inside a body.
A paragraph inside a function means the code should say it itself.

### `comment-ratio`
ratchet · custom: comments over 20% of a file's non-blank lines.
Readers imitate the density they read; bloated files breed bloated changes. Counts only comments
no placement rule (decl/field/block/file-header/pkg) already flagged, and skips doc.go: it catches
a file full of many small comments, not the same long comment a second time.

### `comment-field`
ratchet · custom: comment over 1 line on a field or constant.
A field that needs a paragraph needs a better name or type.

### `comment-adr-narration`
ratchet · custom: comment citing a SPEC §, decision, ADR or DOCTRINE that narrates it.
The spec is the narration; the code keeps only the pointer. Judged only on a comment no placement
rule already flagged, so one comment is one finding.

### `comment-history`
observe · custom: comment narrating what changed.
What changed belongs in the commit message, not the code.

## Go consistency and idioms

### `fmt`
enforce · gofumpt, goimports: file not gofumpt/goimports formatted.
One format, zero review comments about it.

### `log-direct`
ratchet · custom: log.* or fmt.Print* outside cmd/ and package main.
A library that prints steals its caller's output; diagnostics are returned, and only the command
prints them.

### `bare-goroutine`
ratchet · custom: bare go statement: a panic in it kills the host program.
A caller can recover a panic on its own goroutine, never on one the library started. A package
named safego, the one helper that recovers, is exempt.

### `ctx-first`
ratchet · revive, contextcheck: ctx not first (revive), or a call chain that drops the caller's ctx (contextcheck, one per final callee).
Cancellation and deadlines only work if ctx flows first, everywhere. A contextcheck issue in, or
on a call of, a function taking a *http.Request is dropped: it derives ctx from r.Context().

### `http-no-ctx`
ratchet · noctx: outgoing HTTP request without a context.
A request without a context cannot be cancelled.

### `resource-close`
ratchet · sqlclosecheck, rowserrcheck, bodyclose: rows, statement or body not closed, or rows.Err unchecked.
Unclosed rows and bodies leak connections under load.

### `switch-exhaustive`
ratchet · exhaustive: switch on an enum-like type missing a case.
A new enum value must break the build, not fall through silently.

### `naming`
ratchet · revive, staticcheck ST1003: initialism, receiver or variable naming.
One naming convention across the project (ID, URL, receivers).

### `import-boundary`
ratchet · custom: import across a boundary declared in .sovaudit/imports.tsv.
Layers stay layers only if imports are checked.

### `security`
ratchet · gosec: gosec finding.
gosec's findings are cheap to fix before they ship.

### `staticcheck`
ratchet · staticcheck: staticcheck finding.
The Go community's baseline for correctness.

## Project layout

### `dead-link`
ratchet · custom: Markdown link to a path that does not exist.
A dead link sends the reader to a file that does not exist. Every audited .md file is checked;
external links and pure anchors are not.

### `root-clutter`
ratchet · custom: stray file or directory at the repo root.
The repo root is the first thing every reader lists. The project's own intentional entries are
listed in .sovaudit/root-allow.txt.

## Audit integrity

### `ratchet`
enforce · mechanism: check fails on a finding that is new or grew against .sovaudit/baseline.tsv.
Existing debt is recorded once; the gate only fails on new or grown debt.

### `baseline-guard`
enforce · custom: baseline or state loosened against HEAD.
Nobody silences the gate by editing its baseline.

### `decision-dropped`
enforce · custom: a removed comment recorded a decision and DECISIONS.md did not change.
Cutting a comment to its budget must not delete the only record of a choice (a rule, a trade-off,
a rejected alternative). Judged on the Go diff since the base (HEAD unless --base): a removed
comment worded as a decision ("was rejected", "considered and", "trade-off", "judgement call",
"not an oversight", "an interpretation", "we chose", ...) whose wording the same file's added
comments do not carry again fails, unless DECISIONS.md changed too, or a commit since the base
says `sovaudit:decision-obsolete`. Intensifiers ("deliberately", "on purpose") are not markers.

### `ignore-reason`
enforce · custom, nolintlint: ignore directive without a rule and a reason, or one that ignores nothing.
An ignore without a reason is a finding hidden, not a decision made; a stale one hides the next.

### `ignore-count`
ratchet · custom: ignore directives in the repo.
Ignores are debt too; their number only goes down.

### `rule-state`
enforce · custom: unknown rule id or mode in .sovaudit/state.tsv.
A typo in state.tsv must not silently disable a rule.

### `census-complete`
enforce · mechanism: audit prints every rule with its count, zeros included.
A rule nobody breaks stays visible, so zero is also news.
