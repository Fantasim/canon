package lsp_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/lsp"
)

// IMPLEMENTATION-PLAN §8.4 Transport: how Serve ends with the LSP 3.17 lifecycle.
func TestServeEnds(t *testing.T) {
	initialize := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	shutdown := frame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	cases := []struct {
		name, in string
		ok       func(error) bool
	}{
		{"end of input before shutdown", initialize, func(err error) bool { return errors.Is(err, lsp.ErrNoShutdown) }},
		{"end of input after shutdown", initialize + shutdown, func(err error) bool { return err == nil }},
		{"a broken header", "Content-Length: x\r\n\r\n", func(err error) bool { return err != nil && !errors.Is(err, lsp.ErrNoShutdown) }},
	}
	for _, tc := range cases {
		if err := lsp.Serve(context.Background(), strings.NewReader(tc.in), io.Discard); !tc.ok(err) {
			t.Errorf("%s: Serve = %v", tc.name, err)
		}
	}
}

// IMPLEMENTATION-PLAN §8.4 Transport: an oversize frame is refused, the session goes on.
func TestServeOversize(t *testing.T) {
	const big = 1<<28 + 1
	in := io.MultiReader(
		strings.NewReader(frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)+fmt.Sprintf("Content-Length: %d\r\n\r\n", big)),
		io.LimitReader(spaces{}, big),
		strings.NewReader(frame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)+frame(`{"jsonrpc":"2.0","method":"exit"}`)))
	var out bytes.Buffer
	if err := lsp.Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve = %v", err)
	}
	for _, want := range []string{`{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"lsp: message too large: 268435457 bytes"}}`, `{"jsonrpc":"2.0","id":2,"result":null}`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no %s in\n%s", want, out.String())
		}
	}
}

type spaces struct{}

func (spaces) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

// IMPLEMENTATION-PLAN §8.5: a cancelled context stops Serve while it waits for input.
func TestServeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in, w := io.Pipe()
	defer func() { _ = w.Close() }()
	stop := time.AfterFunc(10*time.Millisecond, cancel)
	defer stop.Stop()
	if err := lsp.Serve(ctx, in, io.Discard); !errors.Is(err, context.Canceled) {
		t.Errorf("Serve = %v, want context.Canceled", err)
	}
}
