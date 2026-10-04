package build

import (
	"errors"
	"io/fs"
	"strings"
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

// CODEGEN.md §2.4, §2.9, WIRE.md §8.4, VIEWMODEL.md V1: adopting headers and text files; the view marker.
func TestAdoptAndViewMarker(t *testing.T) {
	adopt := []string{"@source/x.h", "@out/v.json", "@out/sql/a.sql"}
	header, json := Output{Path: "@source/x.h", Target: ir.TargetCpp}, Output{Path: "@out/v.json", Target: ir.TargetJSON}
	other, text := Output{Path: "@source/y.h", Target: ir.TargetCpp}, Output{Path: "@out/sql/a.sql", Target: ir.TargetText}
	if !adoptable(header, adopt) || adoptable(json, adopt) || adoptable(other, adopt) || !adoptable(text, adopt) {
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

// failingRead is a project.FS whose ReadFile always fails with an absolute path.
type failingRead struct{ project.FS }

var errReadDenied = &fs.PathError{Op: "open", Path: "/o/v.json", Err: fs.ErrPermission}

func (failingRead) ReadFile(string) ([]byte, error) { return nil, errReadDenied }

// DECISIONS 201: a read error on an existing output names the display path, not r.p.fs's
// absolute one.
func TestPlaceReadErrorNamesDisplayPath(t *testing.T) {
	r := &run{p: &Project{fs: failingRead{}}}
	_, err := r.place([]*output{{Output: Output{Path: "@out/v.json", Abs: "/o/v.json", Package: "a"}}}, nil)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("place: %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "@out/v.json: open: ") || strings.Contains(msg, "/o/v.json") {
		t.Errorf("place error = %q", msg)
	}
}

// listingDenied reads every file but a `.canon-text`, whose read fails with a permission error.
type listingDenied struct{ project.FS }

func (listingDenied) ReadFile(name string) ([]byte, error) {
	if strings.HasSuffix(name, "/"+textListing) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return []byte("-- old"), nil
}

// CODEGEN.md §2.9, DECISIONS 201: a .canon-text that exists but cannot be read is an I/O error naming its display path, never E8001.
func TestPlaceListingReadError(t *testing.T) {
	r := &run{p: &Project{fs: listingDenied{}}}
	o := &output{Output: Output{Path: "@out/sql/a.sql", Abs: "/o/sql/a.sql", Package: "a", Target: ir.TargetText, Content: []byte("new")}}
	_, err := r.place([]*output{o}, nil)
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "@out/sql/.canon-text") {
		t.Fatalf("place: %v", err)
	}
	same := &output{Output: Output{Path: "@out/sql/a.sql", Abs: "/o/sql/a.sql", Package: "a", Target: ir.TargetText, Content: []byte("-- old")}}
	if _, err := r.place([]*output{same}, nil); err != nil || same.Status != StatusUnchanged {
		t.Errorf("an unchanged output read its listing: %v, status %d", err, same.Status)
	}
}

// failingDir lists no directory: every listing fails with errListing.
type failingDir struct{ project.FS }

var errListing = errors.New("listing failed")

func (failingDir) ReadDir(string) ([]fs.DirEntry, error) { return nil, errListing }

// TYPES.md §13.4: an unlistable directory is an error of the run, listed once.
func TestAssetListing(t *testing.T) {
	h := &evalHost{}
	layout, _ := project.NewLayout(&project.Project{}, "/p", nil, diag.NewBag(nil, ""))
	a := &assets{fs: failingDir{}, layout: layout, host: h, dirs: map[string]dirListing{}}
	a.list("/o")
	a.list("/o")
	if len(h.errs) != 1 || !errors.Is(h.errs[0], errListing) {
		t.Errorf("errors %v", h.errs)
	}
}

// memDirFS lists a fixed set of file names for one directory, nothing else.
type memDirFS struct {
	project.FS
	dir   string
	names []string
}

func (m memDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != m.dir {
		return nil, fs.ErrNotExist
	}
	entries := make([]fs.DirEntry, len(m.names))
	for i, n := range m.names {
		entries[i] = dirEntryName(n)
	}
	return entries, nil
}

type dirEntryName string

func (n dirEntryName) Name() string             { return string(n) }
func (dirEntryName) IsDir() bool                { return false }
func (dirEntryName) Type() fs.FileMode          { return 0 }
func (dirEntryName) Info() (fs.FileInfo, error) { return nil, nil }

// TYPES.md §13.4: root already carries its own "@" (AssetSpec.Root); Exists must not prepend a second one.
func TestAssetExistsRootAlreadyMarked(t *testing.T) {
	p := &project.Project{Roots: []project.Root{{Name: "resource", Path: "res"}}}
	layout, ok := project.NewLayout(p, "/p", nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	mfs := memDirFS{dir: "/p/res/Icon", names: []string{"Item.dds"}}
	a := &assets{fs: mfs, layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}
	if display, found := a.Exists("@resource/Icon", "items", "Item.dds"); !found || display != "@resource/Icon" {
		t.Errorf("Exists = %q, %v; want @resource/Icon, true for a root already carrying its own @", display, found)
	}
	if _, found := a.Exists("@resource/Icon", "items", "Missing.dds"); found {
		t.Error("Exists: true for a name not listed")
	}
}

// failingDirAbs fails every listing with a *fs.PathError carrying an absolute path.
type failingDirAbs struct{ project.FS }

func (failingDirAbs) ReadDir(name string) ([]fs.DirEntry, error) {
	return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrPermission}
}

// DECISIONS 201: an asset directory listing that fails names its display path, project-relative
// and `/`-separated, never the absolute one project.FS carries (meta/decisions/log-2026-09-24.md
// "load.dir review (M2)").
func TestAssetListingErrorNamesDisplayPath(t *testing.T) {
	h := &evalHost{}
	layout, _ := project.NewLayout(&project.Project{}, "/p", nil, diag.NewBag(nil, ""))
	a := &assets{fs: failingDirAbs{}, layout: layout, host: h, dirs: map[string]dirListing{}}
	a.list("/p/assets/icons")
	if len(h.errs) != 1 {
		t.Fatalf("errors %v", h.errs)
	}
	if msg := h.errs[0].Error(); !strings.Contains(msg, "assets/icons") || strings.Contains(msg, "/p/assets/icons") {
		t.Errorf("listing error = %q", msg)
	}
}
