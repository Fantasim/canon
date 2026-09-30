package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// keyN is the nth distinct key.
func keyN(n int) edit.VerdictKey {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n))
	return edit.VerdictKey{Sum: sha256.Sum256(b[:])}
}

// API.md M9: the memo keeps at most verdictLimit verdicts and drops the least recently used; a
// verdict asked for is used again, so it outlives one never asked for.
func TestVerdictMemoBound(t *testing.T) {
	var m verdictMemo
	if m.Fixed(keyN(0)) {
		t.Fatal("an empty memo holds a verdict")
	}
	for n := range verdictLimit {
		m.Keep(keyN(n))
	}
	if !m.Fixed(keyN(0)) { // key 0 is now the most recently used; key 1 the least
		t.Fatal("a kept verdict is missing")
	}
	m.Keep(keyN(verdictLimit))
	m.Keep(keyN(verdictLimit)) // kept twice: still one entry
	if m.Fixed(keyN(1)) {
		t.Error("the least recently used verdict outlived the bound")
	}
	if !m.Fixed(keyN(0)) || !m.Fixed(keyN(2)) || !m.Fixed(keyN(verdictLimit)) {
		t.Error("a recently used verdict was dropped")
	}
	if len(m.entries) != verdictLimit || m.order.Len() != verdictLimit {
		t.Errorf("%d entries, %d listed, want %d", len(m.entries), m.order.Len(), verdictLimit)
	}
	for n := range 3 * verdictLimit {
		m.Keep(keyN(verdictLimit + 1 + n))
	}
	if len(m.entries) != verdictLimit || m.Fixed(keyN(0)) {
		t.Errorf("%d entries after the flood, the oldest kept %v", len(m.entries), m.Fixed(keyN(0)))
	}
}

// The project and the parse kind are part of the key: the same hash as a project file is another
// verdict.
func TestVerdictMemoKind(t *testing.T) {
	var m verdictMemo
	k := keyN(1)
	m.Keep(k)
	k.Project = true
	if m.Fixed(k) {
		t.Error("a source's verdict answered for a project file")
	}
}

// Concurrent edits share one memo (run under -race): every key kept is found, and the bound holds.
func TestVerdictMemoConcurrent(t *testing.T) {
	var m verdictMemo
	var wg sync.WaitGroup
	const workers, each = 8, 2 * verdictLimit
	for w := range workers {
		wg.Go(func() {
			for n := range each {
				k := keyN(w*each + n)
				m.Keep(k)
				m.Fixed(k)
				m.Fixed(keyN(n))
			}
		})
	}
	wg.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) != verdictLimit || m.order.Len() != verdictLimit {
		t.Errorf("%d entries, %d listed, want %d", len(m.entries), m.order.Len(), verdictLimit)
	}
}

const (
	memoPackage = "/// X.\npackage x\n\n/// N.\nlet n: Int = 1\n"
	memoProject = "project a {\n  canon: \"0.1\"\n}\n"
)

// API.md M9: an edit records the canonical text it checked in its project's memo, and a second
// edit of the same text, whose snapshot is another parse, adds nothing.
func TestEditKeepsVerdicts(t *testing.T) {
	b, err := build.Open(&clockFS{MapFS: fstest.MapFS{
		"law/project.canon": {Data: []byte(memoProject)},
		"law/x/x.canon":     {Data: []byte(memoPackage)},
	}}, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	defer p.Close()
	set := EditRequest{DryRun: true, Changes: Changes{Host: build.EditHost, Ops: []edit.Operation{
		{Kind: edit.OpSet, Path: "x:n", Value: edit.Int(2)},
	}}}
	for range 3 {
		if _, err := p.Edit(context.Background(), set); err != nil {
			t.Fatal(err)
		}
		if n := len(p.verdicts.entries); n != 1 {
			t.Fatalf("the memo holds %d verdicts after a dry run, want 1", n)
		}
	}
}
