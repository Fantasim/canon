# Canon for VS Code

Registers the `canon` language (`.canon`), highlights it with a hand-written TextMate grammar
(`syntaxes/canon.tmLanguage.json`, kept in step with spec/GRAMMAR.md) and starts `canon lsp`
(CLI.md 3.13) over stdio. The `canon` executable must be on the PATH, or set `canon.server.path`.

## Grammar subset

The grammar uses only what Go's RE2 also accepts (no lookahead, lookbehind or backreference, no
`\G`), so a pure-Go test can tokenise every example (see Tests). Two rules follow: `^` only anchors a rule at the
start of the line, and the regex literal (GRAMMAR 2.7, previous token `(` or `,`) consumes its
`(` or `,`. A regex after a `(` or `,` on an earlier line is not highlighted.

## Dependencies

Approved by Louis (IMPLEMENTATION-PLAN section 11): `vscode-languageclient` ^9.0.1 (runtime, the
LSP client of `extension.js`) and `@vscode/vsce` ^3 (dev, packages the `.vsix`). `vscode-textmate`
and `vscode-oniguruma` were declined: the grammar is tested by the pure-Go engine instead, so the
real Oniguruma scoring of the grammar is an accepted, unverified risk. No `node_modules` or
lockfile is committed; run `npm install` here to build.

## Tests

`internal/testkit/vscodegrammar` (`go test ./internal/testkit/vscodegrammar`): a scope snapshot of
every example (`testdata/scopes/`, `-update` rewrites it and deletes orphans), every GRAMMAR 4.1
reserved word, the lexical rules and this manifest.

## Watched files and limits

The client starts `canon lsp` with no argument and watches `**/*.{canon,json,csv,h,txt}` in the
workspace folders for `workspace/didChangeWatchedFiles`. Files read under another extension
(`format:`), assets, and roots outside the workspace are not watched: their changes show after the
next edit of a buffer (IMPLEMENTATION-PLAN 8.4). GRAMMAR 4.2 contextual keywords are not
highlighted.
