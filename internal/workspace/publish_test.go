package workspace_test

import (
	"context"
	"testing"
)

// API.md S10: each snapshot published, by a refresh or a writer, is numbered after every one
// published before it, and a snapshot keeps its number once a newer one is current; revisions
// are hashes, so this is what orders them.
func TestPublishedOrder(t *testing.T) {
	m := newMemFS(lawFiles())
	p := open(t, m)
	ctx := context.Background()
	first := read(t, p)
	if _, err := first.Revision(ctx); err != nil { // the sources enter the snapshot
		t.Fatal(err)
	}
	m.put("/law/a/a.canon", []byte(srcA+"\n"))
	changed := read(t, p)
	if err := p.SetOverlay("b/b.canon", []byte(srcB+"\n")); err != nil {
		t.Fatal(err)
	}
	overlaid := read(t, p)
	same := read(t, p)
	steps := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"the first snapshot", first.Published(), 0},
		{"a refresh's", changed.Published(), 1},
		{"an overlay's", overlaid.Published(), 2},
		{"a read with nothing changed", same.Published(), 2},
	}
	for _, s := range steps {
		if s.got != s.want {
			t.Errorf("%s is published %d, want %d", s.name, s.got, s.want)
		}
	}
	if changed == first || overlaid == changed || same != overlaid {
		t.Errorf("snapshots: first %p, changed %p, overlaid %p, same %p", first, changed, overlaid, same)
	}
}
