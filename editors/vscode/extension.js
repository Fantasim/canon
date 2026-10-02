'use strict';

// Starts `canon lsp` (CLI.md 3.13) over stdio. The server is read-only (DECISIONS 274): the
// client offers no completion, code action or rename. No `transport` is set: vscode-languageclient
// would append `--stdio`, which `canon lsp` refuses; a bare command speaks stdio.
const vscode = require('vscode');
const { LanguageClient } = require('vscode-languageclient/node');

let client;

function activate(context) {
  const command = vscode.workspace.getConfiguration('canon').get('server.path', 'canon');
  const serverOptions = { command, args: ['lsp'] };
  // workspace/didChangeWatchedFiles for the files a project loads (IMPLEMENTATION-PLAN 8.4).
  const watcher = vscode.workspace.createFileSystemWatcher('**/*.{canon,json,csv,h,txt}');
  context.subscriptions.push(watcher);
  const clientOptions = {
    documentSelector: [{ scheme: 'file', language: 'canon' }],
    synchronize: { fileEvents: watcher },
  };
  client = new LanguageClient('canon', 'Canon', serverOptions, clientOptions);
  return client.start();
}

function deactivate() {
  return client ? client.stop() : undefined;
}

module.exports = { activate, deactivate };
