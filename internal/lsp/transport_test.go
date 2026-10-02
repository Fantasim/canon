package lsp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// IMPLEMENTATION-PLAN §8.4 Transport: LSP 3.17 base protocol framing and its errors.
func TestReadFrame(t *testing.T) {
	cases := []struct {
		name, in string
		want     string
		err      error
	}{
		{"one message", "Content-Length: 2\r\n\r\n{}", "{}", nil},
		{"another header too", "Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: 2\r\n\r\n{}", "{}", nil},
		{"end of input", "", "", io.EOF},
		{"no length", "Content-Type: x\r\n\r\n{}", "", errHeader},
		{"a negative length", "Content-Length: -1\r\n\r\n", "", errHeader},
		{"too large and cut short", "Content-Length: 999999999999\r\n\r\n", "", errHeader},
		{"a body cut short", "Content-Length: 5\r\n\r\n{}", "", errHeader},
		{"headers cut short", "Content-Length: 2\r\n", "", errHeader},
		{"a header line without a colon", "nonsense\r\n\r\n", "", errHeader},
	}
	for _, tc := range cases {
		body, err := readFrame(bufio.NewReader(strings.NewReader(tc.in)))
		if string(body) != tc.want || !errors.Is(err, tc.err) || (tc.err == nil) != (err == nil) {
			t.Errorf("%s: got %q, %v; want %q, %v", tc.name, body, err, tc.want, tc.err)
		}
	}
}

// IMPLEMENTATION-PLAN §8.4 Transport: a frame over the cap is skipped whole; the next reads.
func TestReadFrameTooLarge(t *testing.T) {
	n := maxMessage + 1
	in := io.MultiReader(strings.NewReader(fmt.Sprintf(headerFormat, n)), io.LimitReader(zeros{}, int64(n)),
		strings.NewReader(fmt.Sprintf(headerFormat, 2)+"{}"))
	r := bufio.NewReader(in)
	if _, err := readFrame(r); !errors.Is(err, errTooLarge) {
		t.Fatalf("first frame: %v, want errTooLarge", err)
	}
	if body, err := readFrame(r); err != nil || string(body) != "{}" {
		t.Fatalf("next frame: %q, %v", body, err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// IMPLEMENTATION-PLAN §8.4 Transport: a write that fails sticks, so the session stops.
func TestWriterSticks(t *testing.T) {
	w := &writer{out: failing{}}
	if err := w.write([]byte("{}")); !errors.Is(err, errWrite) {
		t.Fatalf("write = %v, want errWrite", err)
	}
	if err := w.failed(); !errors.Is(err, errWrite) {
		t.Fatalf("failed = %v, want errWrite", err)
	}
}

type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
