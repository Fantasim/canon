package build

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// WIRE.md §8.1, DECISIONS 196: identical bytes at one path collide but for a runtime file.
func TestCollisions(t *testing.T) {
	for _, c := range []struct {
		name, a, b  string
		kept, found int
	}{
		{"runtime", "/o/g/rt/rt.go", "/o/g/rt/rt.go", 1, 0},
		{"C++ runtime", "/o/c/canon_runtime.h", "/o/c/canon_runtime.h", 1, 0},
		{"data file", "/o/v.json", "/o/v.json", 1, 1},
		{"letter case", "/o/g/rt/rt.go", "/o/G/rt/rt.go", 1, 1},
	} {
		bag := diag.NewBag(&source.FileSet{}, "a")
		r := &run{bags: check.Bags{"a": bag}}
		kept := r.collisions([]*output{
			{Output: Output{Path: c.a, Abs: c.a, Package: "a", Content: []byte("x")}},
			{Output: Output{Path: c.b, Abs: c.b, Package: "a", Content: []byte("x")}},
		})
		if len(kept) != c.kept || len(bag.Findings()) != c.found {
			t.Errorf("%s: kept %d, %d findings", c.name, len(kept), len(bag.Findings()))
		}
	}
}

// CODEGEN.md §2.4, WIRE.md §8.4, VIEWMODEL.md V1: adopting headers; the view marker.
func TestAdoptAndViewMarker(t *testing.T) {
	adopt := []string{"@source/x.h", "@out/v.json"}
	if !adoptable("@source/x.h", adopt) || adoptable("@out/v.json", adopt) || adoptable("@source/y.h", adopt) {
		t.Error("adoptable")
	}
	view := Output{Abs: "/o/a.view.json", Target: ir.TargetView}
	data := Output{Abs: "/o/a.json", Target: ir.TargetJSON}
	vm, fp := []byte(`{"$schema": "canon-vm/1"}`), []byte(`{"$schema": "a.v@0123abcd"}`)
	if !marked(view, vm) || marked(view, fp) || marked(data, vm) || !marked(data, fp) {
		t.Error("markers")
	}
}

// DECISIONS 196: a generator never sees a value stage E left empty.
func TestComplete(t *testing.T) {
	if err := complete(&ir.Package{Name: "a", Values: []*ir.Value{{Name: "v"}}}); !errors.Is(err, ErrInternal) {
		t.Errorf("empty value: %v", err)
	}
	if err := complete(&ir.Package{Name: "a", Consts: []*ir.Const{{Name: "C"}}}); !errors.Is(err, ErrInternal) {
		t.Errorf("empty const: %v", err)
	}
}

// failingDir lists no directory: every listing fails with errListing.
type failingDir struct{ project.FS }

var errListing = errors.New("listing failed")

func (failingDir) ReadDir(string) ([]fs.DirEntry, error) { return nil, errListing }

// TYPES.md §13.4: an unlistable directory is an error of the run, listed once.
func TestAssetListing(t *testing.T) {
	h := &evalHost{}
	a := &assets{fs: failingDir{}, host: h, dirs: map[string][]string{}}
	a.files("/o")
	a.files("/o")
	if len(h.errs) != 1 || !errors.Is(h.errs[0], errListing) {
		t.Errorf("errors %v", h.errs)
	}
}
