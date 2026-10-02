// Package vscodegrammar tests the TextMate grammar and manifest of editors/vscode
// (IMPLEMENTATION-PLAN.md 8.4, CLI.md 4) with a small TextMate engine over Go's RE2.
//
// The grammar stays in the subset RE2 and Oniguruma agree on: no lookahead, lookbehind,
// backreference or \G; a ^ only starts a pattern and is tried at column 0 only; \b, \w and \s
// are ASCII-only in RE2 but Unicode-aware in Oniguruma, so words are consumed whole. A search
// runs on a slice of the line, so a \b at the slice start sees a boundary. How Oniguruma scores
// the same grammar is not proved here: only a run in VS Code does.
package vscodegrammar
