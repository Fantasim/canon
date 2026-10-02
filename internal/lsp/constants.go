package lsp

import (
	"encoding/json"
	"time"
)

// debounce is how long a document stays quiet before diagnostics are recomputed (IMPLEMENTATION-PLAN §8.4 Sync).
const debounce = 150 * time.Millisecond

// Framing (LSP 3.17 base protocol).
const (
	headerLength  = "Content-Length"
	headerFormat  = "Content-Length: %d\r\n\r\n"
	fmtWrap       = "%w: %w"
	maxMessage    = 1 << 28 // 256 MiB; a larger frame is skipped and refused (log-2026-10-02 L1)
	jsonrpcV2     = "2.0"
	canonName     = "canon"
	encodingUTF16 = "utf-16"
	fileScheme    = "file"
	uriSep        = "/"
	uncPrefix     = "//"
	reservedNote  = "$/"
	newline       = '\n'
)

// Methods the server answers or sends.
const (
	methodInitialize  = "initialize"
	methodInitialized = "initialized"
	methodShutdown    = "shutdown"
	methodExit        = "exit"
	methodDidOpen     = "textDocument/didOpen"
	methodDidChange   = "textDocument/didChange"
	methodDidClose    = "textDocument/didClose"
	methodWatched     = "workspace/didChangeWatchedFiles"
	methodPublish     = "textDocument/publishDiagnostics"
	methodLog         = "window/logMessage"
)

// JSON-RPC 2.0 and LSP error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeNotInitialized = -32002
)

// LSP enumerations: TextDocumentSyncKind.Full, DiagnosticSeverity, MessageType.Error.
const (
	syncFull        = 1
	severityError   = 1
	severityWarning = 2
	messageError    = 1
)

// phase is where the server is in the LSP lifecycle.
type phase int

const (
	phaseNew phase = iota
	phaseRunning
	phaseShutdown
)

// nullID is the id of a response to a message whose id could not be read (JSON-RPC 2.0 §5).
var nullID = json.RawMessage("null")
