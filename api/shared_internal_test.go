package canon

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
)

// sharedLaw is a package with a table and a view, so both Value and ViewModel have work.
var sharedLaw = map[string][]byte{
	"/law/project.canon": []byte("project a {\n  canon: \"0.1\"\n}\n"),
	"/law/a/a.canon": []byte("/// A.\npackage a\n\n/// A use.\nrecord Use {\n  /// How many.\n  count: Int\n}\n\n" +
		"/// Uses.\nlet uses: table Use = {\n  first { count: 20 }\n  second { count: 3 }\n}\n\n" +
		"view Use {\n  title \"{id}: {count}\"\n}\n\nemit view { out: \"a.view.json\" }\n"),
}

// API.md S7, S8, R4, R9: Value reads and ViewModel run at once on one shared Analysis, every
// value and model as a lone call gives it. Run with -race.
func TestValueAndViewModelShareAnAnalysis(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(sharedLaw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, err := analyze(ctx, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.ViewModel(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	want, err := viewgen.Write(m)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := edit.Parse("a:uses")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			snap := &snapshot{a: a, s: edit.NewSnapshot(a), types: newTypeEncoder(ctx, a)}
			v, err := snap.resolve("a:uses", parsed)
			if err != nil || len(v.Children()) != 2 || v.Type.VM == nil || v.Origin.Kind == "" {
				t.Errorf("Value on the shared analysis: %v, %+v", err, v)
			}
		}()
		go func() {
			defer wg.Done()
			m, err := a.ViewModel(ctx, "a")
			if err != nil {
				t.Error(err)
				return
			}
			if got, err := viewgen.Write(m); err != nil || !bytes.Equal(got, want) {
				t.Errorf("a concurrent view model differs: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if v, err := p.Value(ctx, "a:uses.first.count"); err != nil || v.Text != "20" {
				t.Errorf("Value: %v, %+v", err, v)
			}
		}()
	}
	wg.Wait()
}
