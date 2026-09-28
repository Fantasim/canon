package edit_test

import (
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md S7: a snapshot answers concurrent reads, each the same as alone.
func TestSnapshotConcurrentReads(t *testing.T) {
	f := lawFixture(t, []string{"dev"}, "a", "b")
	paths := []string{"a:items[#1].n", "a:viaName.n", "a:base.x", "a:derived.sub.n", "a:holder.data.n", "a:entries.two.v"}
	want := make([]edit.Editability, len(paths))
	for i, in := range paths {
		want[i] = mustEdit(t, f, resolve(t, f, in), edit.OpSet, "dev")
	}
	var wg sync.WaitGroup
	got := make([][]edit.Editability, runtime.GOMAXPROCS(0))
	for g := range got {
		wg.Go(func() {
			for _, in := range paths {
				p, _ := edit.Parse(in)
				r, _ := edit.Resolve(f.Snapshot, p)
				e, _ := f.Editable(r, edit.OpSet, "dev")
				got[g] = append(got[g], e)
			}
		})
	}
	wg.Wait()
	for g := range got {
		if !reflect.DeepEqual(got[g], want) {
			t.Errorf("goroutine %d: %+v, want %+v", g, got[g], want)
		}
	}
}
