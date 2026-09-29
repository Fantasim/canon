package canon_test

import (
	"context"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// API.md B4, API.md S8: LockCheck is a read that compares canon.lock with the stable collections
// alone: a lock missing an id is W6006, a failing check of another value is not reported.
func TestLockCheck(t *testing.T) {
	p, _ := openEdit(t, map[string]string{"a/a.canon": weightless, "a/canon.lock": lockHeader})
	checked, err := p.Check(context.Background(), "a")
	if err != nil || checked.Summary.Errors != 1 {
		t.Fatalf("Check: %+v, %v", checked, err)
	}
	res, err := p.LockCheck(context.Background(), "a")
	if err != nil || res.Summary.Errors != 0 || len(res.Findings) != 1 || res.Findings[0].Code != string(diag.W6006.Def().Code) {
		t.Fatalf("API.md B4: %+v, %v", res, err)
	}
	if res.Revision != p.Revision() || len(res.Packages) != 1 || res.Packages[0] != "a" {
		t.Errorf("API.md B4: revision %s, packages %v", res.Revision, res.Packages)
	}
}

// API.md P8, API.md V7, VIEWMODEL.md S9: a plain list of table entries names its elements by
// position, `[n]` and `#<n>`, never by the entries' keys.
func TestEvaluatePlainCopies(t *testing.T) {
	p, _ := openEdit(t, nil)
	res := evaluate(t, p, "a:picked", "")
	if len(res.Headings) != 2 || res.Headings["[0]"].Title.Value != "#1" || res.Headings["[1]"].Title.Value != "#2" {
		t.Errorf("API.md P8: headings %+v", res.Headings)
	}
	if one := evaluate(t, p, "a:picked[1]", ""); one.Path != "a:picked[1]" || one.Title.Value != "#2" {
		t.Errorf("API.md V7: %s titled %q", one.Path, one.Title.Value)
	}
	if entry := evaluate(t, p, "a:statuses.done", ""); entry.Title.Value != "done" {
		t.Errorf("API.md V7: an entry's title %q", entry.Title.Value)
	}
}

// pausingFS stops at its second rename, the first file of an edit already renamed, until
// released.
type pausingFS struct {
	*memFS
	mu      sync.Mutex
	renames int
	reached chan struct{}
	release chan struct{}
}

func (f *pausingFS) Rename(oldname, newname string) error {
	f.mu.Lock()
	f.renames++
	pause := f.renames == 2
	f.mu.Unlock()
	if pause {
		close(f.reached)
		<-f.release
	}
	return f.memFS.Rename(oldname, newname)
}

// API.md S9, API.md S10, API.md S7: while an edit renames its files, readers keep the snapshot
// they read, the old values; once it returns, they read the new one.
func TestEditReadersKeepSnapshot(t *testing.T) {
	fsys := &pausingFS{memFS: lawFS(), reached: make(chan struct{}), release: make(chan struct{})}
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	rev := p.Revision()
	done := make(chan error, 1)
	go func() {
		_, err := p.Edit(context.Background(), threeFiles)
		done <- err
	}()
	<-fsys.reached
	var wg sync.WaitGroup
	ports := make([]string, 4)
	for i := range ports {
		wg.Go(func() {
			if v, err := p.Value(context.Background(), "a:config.port"); err == nil {
				ports[i] = v.Text
			}
		})
	}
	wg.Wait()
	during := p.Revision()
	close(fsys.release)
	if err := <-done; err != nil {
		t.Fatalf("Edit: %v", err)
	}
	for _, port := range ports {
		if port != "8765" {
			t.Errorf("API.md S9: a reader during the edit read %q", port)
		}
	}
	if v, err := p.Value(context.Background(), "a:config.port"); err != nil || v.Text != "9000" || during != rev {
		t.Errorf("API.md S10: after the edit %v, %v; revision during it %s, before %s", v, err, during, rev)
	}
}
