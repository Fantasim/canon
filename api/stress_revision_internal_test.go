package canon

import (
	"context"
	"errors"
	"testing"
)

// API.md S10: after an Edit returned, a read begun before it and ending later keeps its own
// revision but never moves the project's back, and Revision is the edit's (TestStress: random).
func TestStressRevisionAfterLateRead(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	ctx := context.Background()
	before, err := p.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	late, err := p.read(ctx) // a read begins, before the edit
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Edit(ctx, Edit{Ops: []Op{Set("a:uses.first.count", Int(21))}})
	if err != nil || !res.Applied {
		t.Fatalf("Edit: %v, %+v", err, res)
	}
	if _, ok := p.refresh(); !ok { // Revision, its refresh
		t.Fatal("refresh failed on an open project")
	}
	got, err := p.revision(ctx, late) // the read begun before the edit ends, as every read ends
	if err != nil || got != before.Revision {
		t.Errorf("the late read's own revision is %s (%v), want %s, the snapshot it read", got, err, before.Revision)
	}
	p.mu.Lock()
	kept := p.rev
	p.mu.Unlock()
	if kept != res.Revision {
		t.Errorf("after the edit returned %s the project's revision is %s, the one before it (API.md S10)", res.Revision, kept)
	}
	if now := p.Revision(); now != res.Revision {
		t.Errorf("Revision after the edit returned %s is %s (API.md S10)", res.Revision, now)
	}
}

// API.md O6, S10: after Close, Revision is the revision of the newest snapshot a call read, and a
// read begun before an edit and ending after Close does not replace it.
func TestRevisionAfterClose(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := p.Check(ctx); err != nil {
		t.Fatal(err)
	}
	late, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Edit(ctx, Edit{Ops: []Op{Set("a:uses.first.count", Int(21))}})
	if err != nil || !res.Applied {
		t.Fatalf("Edit: %v, %+v", err, res)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.revision(ctx, late); err != nil {
		t.Fatal(err)
	}
	if got := p.Revision(); got != res.Revision {
		t.Errorf("Revision after Close is %s, want %s, the edit's (API.md O6, S10)", got, res.Revision)
	}
	if _, err := p.Check(ctx); !errors.Is(err, ErrClosed) {
		t.Errorf("Check after Close: %v, want ErrClosed (API.md O6)", err)
	}
}

// API.md O6, S10 (log M4 B6): the first snapshot, numbered 0 and never published, counts as read,
// so Revision after Close is Check's revision, not "".
func TestRevisionAfterCloseFirstSnapshot(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background())
	if err != nil || res.Revision == "" {
		t.Fatalf("Check: %v, revision %q", err, res.Revision)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if got := p.Revision(); got != res.Revision {
		t.Errorf("Revision after Close is %q, want %s, Check's (API.md O6, S10)", got, res.Revision)
	}
}
