# Code budget (project doctrine, enforced by canon audit)

Every Go package of this repository follows this section; `make check` runs `canon audit check`
and fails on anything new or grown. The full rulebook, with the reason for each rule, is
[rules.md](rules.md).

1. **Size.** Functions <= 60 lines, <= 5 parameters, <= 3 results, nesting <= 3. Files <= 500
   lines. Past a limit, split by concern; never raise the limit. Every limit of this page is a
   value of [thresholds.tsv](thresholds.tsv), the one place the tool reads it from.
2. **Constants.** No literal twice, no bare number but 0 and 1. Constants live in
   `<pkg>/constants.go`; one shared by two packages lives in the lowest package both import.
   The environment is read in `cmd/` (or one config package) only, and passed down.
3. **Errors.** Sentinels in `<pkg>/errors.go`, wrapped with `%w`, compared with `errors.Is`.
4. **Surface.** Nothing exported that only its own package uses (an Example test counts as a
   user). No dead code, no commented-out code, no TODO in code (the issue tracker tracks work).
   Every package has a `doc.go` and an Example test.
5. **Reuse.** The standard library first, then code already in this repository, then new code.
   The same block twice in the repository is one function waiting to be extracted.
6. **Comments.** A comment says why, in as few lines as it takes: <= 3 lines on a declaration,
   <= 3 consecutive lines inside a body, no file headers outside `doc.go`, <= 20% of a file.
   The spec is cited as `// SPEC §19` or `// decision 25`, never narrated. What changed belongs
   in the commit message. A decision recorded only in a comment (a rule, a trade-off, a rejected
   alternative) moves to `DECISIONS.md` before the comment is cut; the code keeps the pointer.
7. **Library manners.** Library code never prints (only `cmd/` does) and never starts a
   goroutine whose panic it does not recover: the caller cannot.
8. **The ratchet.** Existing findings are in `.sovaudit/baseline.tsv`; new or grown ones fail.
   Only `baseline --tighten` writes the baseline, and it only shrinks. `// sovaudit:ignore
   <rule> -- <reason>` is the only way to silence a finding, and every ignore is counted, the
   linters' own (`//nolint`, `#nosec`...) included: the count never grows.
9. **Diagnostics.** A finding of the language is reported only as `diag.E3501.At(span,
   args...)`: no code string, message text, `fmt` call or English argument outside
   `internal/diag`, and every code the compiler reports has a `<CODE>_<n>.txtar` test in its
   owning package.

Generated code is not judged: the goldens under `examples/*/expected/`, `testdata/`, and any Go
file marked `// Code generated ... DO NOT EDIT.` or named `*.gen.go`.
