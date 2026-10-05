package canon

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

// API.md S8, DECISIONS 313: a view model is kept on its snapshot, shared by later calls, and gone with it.
func TestViewModelKeptPerSnapshot(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	s, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.KeptView("a"); ok {
		t.Fatal("API.md S8: a view model kept before any call")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.ViewModel(ctx, "a"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, ok := s.KeptView("a"); !ok {
		t.Fatal("API.md S8: no view model kept after ViewModel")
	}
	d1, err1 := keptViewModel(ctx, s, "a")
	d2, err2 := keptViewModel(ctx, s, "a")
	if err1 != nil || err2 != nil || &d1[0] != &d2[0] {
		t.Errorf("API.md S8: two calls on one snapshot do not share the kept bytes: %v %v", err1, err2)
	}
	first, _ := p.ViewModel(ctx, "a")
	if err := p.SetOverlay("a/a.canon", append(bytes.Clone(sharedLaw["/law/a/a.canon"]), []byte("\n/// Z.\nlet z: Int = 1\n")...)); err != nil {
		t.Fatal(err)
	}
	s2, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.KeptView("a"); ok || s2 == s {
		t.Error("API.md S8: a new snapshot kept the old view model")
	}
	second, err := p.ViewModel(ctx, "a")
	if err != nil || second.Revision == first.Revision {
		t.Errorf("API.md S3: revision %q after %q, %v", second.Revision, first.Revision, err)
	}
	second.JSON()[0] = 'X' // a caller's copy is its own
	if again, _ := p.ViewModel(ctx, "a"); again.JSON()[0] == 'X' {
		t.Error("API.md S8: a kept view model shares its bytes with a caller")
	}
}
