package canon

import (
	"context"
	"testing"
)

// API.md S10: Revision called after an Edit returned is the edit's, even when a read begun before
// the edit ends between the refresh and the read of p.rev of Revision (canon.go), replayed here
// step by step; TestStress meets that interleaving only at random.
func TestStressRevisionAfterLateRead(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	ctx := context.Background()
	if _, err := p.Check(ctx); err != nil {
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
	p.refresh() // Revision, its first step
	if _, err := p.revision(ctx, late); err != nil {
		t.Fatal(err) // the read begun before the edit ends, as every read ends
	}
	p.mu.Lock()
	got := p.rev // Revision, its second step
	p.mu.Unlock()
	if got != res.Revision {
		t.Errorf("Revision after the edit returned %s is %s, the revision before it (API.md S10)", res.Revision, got)
	}
}
