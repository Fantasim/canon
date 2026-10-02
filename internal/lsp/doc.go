// Package lsp is the Canon language server, `canon lsp` (spec/IMPLEMENTATION-PLAN.md, CLI.md):
// LSP 3.17 over stdio, JSON-RPC on the standard library alone. Each open buffer is an overlay
// of the project whose project.canon is nearest above it; diagnostics of every file with
// findings, loaded JSON, CSV and header files included, are published once the buffers are quiet
// for 150 ms, in UTF-16 positions. Hover, definition and references read the project's analysis,
// formatting is canon fmt. The server is a viewer (DECISIONS 274): it never writes a file.
package lsp
