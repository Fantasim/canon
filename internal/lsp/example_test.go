package lsp_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/lsp"
)

// frame wraps a JSON-RPC body in the base protocol's header.
func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func Example() {
	in := strings.NewReader(frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`) +
		frame(`{"jsonrpc":"2.0","method":"exit"}`))
	var out bytes.Buffer
	err := lsp.Serve(context.Background(), in, &out)
	msgs := strings.Split(out.String(), "Content-Length: ")
	_, initialized, _ := strings.Cut(msgs[1], "\r\n\r\n")
	_, shutdown, _ := strings.Cut(msgs[2], "\r\n\r\n")
	fmt.Println(initialized)
	fmt.Println(shutdown, err)
	// Output:
	// {"jsonrpc":"2.0","id":1,"result":{"capabilities":{"positionEncoding":"utf-16","textDocumentSync":{"openClose":true,"change":1},"hoverProvider":true,"definitionProvider":true,"referencesProvider":true,"documentFormattingProvider":true},"serverInfo":{"name":"canon"}}}
	// {"jsonrpc":"2.0","id":2,"result":null} <nil>
}
