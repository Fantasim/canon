package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"golang.org/x/tools/txtar"
)

// CLI.md §3.8, §4, API.md R7, R8: references at an entry are canon refs' list, in its order.
func TestReferencesMatchRefs(t *testing.T) {
	// The fixture of testdata/references.txtar, its request sent before any pass published.
	data, err := os.ReadFile(filepath.Join("testdata", "references.txtar"))
	must(t, err)
	s := newSession(t, txtar.Parse(data))
	done := s.start()
	must(t, s.initialize(nil))
	must(t, s.open([]string{"proj/r/r.canon"}))
	req := `{"jsonrpc":"2.0","id":1,"method":"textDocument/references","params":{"textDocument":{"uri":"$URI/proj/r/r.canon"},"position":{"line":21,"character":2}}}`
	must(t, s.sendLine([]string{req}))
	got := s.locations(t, 1)
	must(t, s.in.Close())
	<-done
	p, err := canon.Open(filepath.Join(s.dir, "proj"), canon.Options{})
	must(t, err)
	defer func() { _ = p.Close() }()
	res, err := p.Refs(context.Background(), "statuses.done")
	must(t, err)
	var want []string
	for _, r := range res.Refs {
		want = append(want, fmt.Sprintf("%s:%d:%d", r.File, r.Line, r.Col))
	}
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Fatalf("references\n%q\nwant canon refs'\n%q", got, want)
	}
}

// locations are the locations answering request id, as project-relative `file:line:col`, 1-based
// (the fixture is ASCII: UTF-16 characters are bytes).
func (s *session) locations(t *testing.T, id int) []string {
	t.Helper()
	var out []string
	for _, m := range s.rec.since(0) {
		var r struct {
			ID     *int       `json:"id"`
			Result []location `json:"result"`
		}
		if json.Unmarshal(m, &r) != nil || r.ID == nil || *r.ID != id {
			continue
		}
		for _, l := range r.Result {
			out = append(out, fmt.Sprintf("%s:%d:%d", strings.TrimPrefix(l.URI, s.uri+"/proj/"), l.Range.Start.Line+1, l.Range.Start.Character+1))
		}
	}
	return out
}

// log-2026-10-02 L1 NIT: an error finding the project other than none is reported, wrapped.
func TestProjectAboveError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	must(t, os.WriteFile(file, nil, 0o644))
	// Windows reports a path under a regular file as not found, not as ENOTDIR.
	if runtime.GOOS != "windows" {
		root, err := projectAbove(filepath.ToSlash(filepath.Join(file, "x.canon")))
		if root != "" || !errors.Is(err, errFind) {
			t.Fatalf("projectAbove under a file = %q, %v; want errFind", root, err)
		}
	}
	root, err := projectAbove(filepath.ToSlash(filepath.Join(dir, "x.canon")))
	if root != "" || err != nil {
		t.Fatalf("projectAbove in no project = %q, %v; want none", root, err)
	}
}
