# Go rules — Canon compiler

Operational checklist for every Go change. The law is [DOCTRINE.md](../../DOCTRINE.md) §3–§5 and
[tools/audit/DOCTRINE-code.md](../../tools/audit/DOCTRINE-code.md); this file says how to meet
it. Go only: there is no web, database or service code in this repository. ≤ 80 lines.

## 1. Before writing

- Read the spec section the task implements, whole, and the frozen contracts it consumes
  (IMPLEMENTATION-PLAN §4). Implement numbered rules; cite them (`// WIRE.md W3`), never narrate.
- `grep -rn` the repository before creating a helper: standard library first, then code already
  here, then new code. Edit only the packages your task owns (§5.3); a need elsewhere is reported.
- Plan files per construct up front (§12.4's table): a file that will pass 500 lines is two files.

## 2. Shape (the audit fails anything new or grown)

- Functions ≤ 60 lines (tests 100), ≤ 40 statements, ≤ 5 params, ≤ 3 results, nesting ≤ 3,
  cognitive complexity ≤ 15. Early returns; a long switch becomes a table.
- Lexer and parser dispatch is table-driven: an array of small handlers indexed by token or node
  kind; precedence and keyword tables are data in `constants.go` (DECISIONS 26).
- Only 0 and 1 appear bare; every other literal is a named constant in `<pkg>/constants.go`, and
  no string literal appears twice. Limits the documents fix are named where enforced (§12.4).
- Nothing exported that only its package uses; no dead code, commented-out code or TODO.
- Every package: `doc.go` (the only file header) and an `example_test.go` with a running Example.

## 3. Errors and diagnostics

- Language findings are catalogue data: `diag.E3501.At(span, args...)`. No code string and no
  message text outside `internal/diag` (DECISIONS 27). A new code goes to QA's registry, in the
  requesting package's range, after its owning document has it.
- Go errors (I/O, stale revision, API misuse): sentinels in `<pkg>/errors.go`, wrapped with
  `fmt.Errorf("...: %w", err)`, compared with `errors.Is`/`errors.As`. Never match error text.
- Library code never prints, never reads the environment, never calls `os.Exit`, never panics
  on input. Only `cmd/canon` and `internal/cli` touch stdout, stderr and exit codes.

## 4. Determinism and concurrency (DOCTRINE §5)

- Never let map order reach an output: sort keys; a genuinely order-free `range` over a map in
  an output package carries `//canon:unordered <reason>`.
- No clock, environment, absolute path or directory-listing order in any output; paths are `/`,
  files end with one `\n`, names match case-sensitively.
- `ctx` first where work is cancellable; no goroutine a panic can escape; shared state behind a
  mutex; every test passes under `-race`.

## 5. Tests and goldens (DOCTRINE §4)

- Table-driven tests beside the code; each names the spec rule it proves. Every new diagnostic
  code gets `internal/<pkg>/testdata/findings/<CODE>_<n>.txtar`.
- Goldens are written by the tool (`go test ./internal/testkit/golden -update`, or the package's
  `-update` flag), never by hand; read the resulting diff before reporting.
- Fixtures stay small and synthetic-or-trimmed under `examples/_fixtures/` or `testdata/`; real
  data only under the git-ignored `testdata-real/`, behind opt-in targets.
- Fuzz targets per IMPLEMENTATION-PLAN §7.7 for every parser and printer you add.

## 6. Verify

`GOTOOLCHAIN=local make check` from the repository root, and `go test -race ./<your pkgs>/...`.
Paste the command log in the report. A red `make check` is never "unrelated" without naming the
failing target and the file that causes it.

## 7. Forbidden

- A third-party import not in IMPLEMENTATION-PLAN §11; cgo; network access; `unsafe`.
- Raising an audit limit, editing `.sovaudit/baseline.tsv` or `state.tsv`, or an ignore without
  `-- <reason>`.
- Hand-editing a golden, a `*.gen.go`, `internal/diag/codes.go` or anything marked generated.
- Editing SPEC.md, CLI.md, DECISIONS.md, `spec/`, or anything outside this directory.
