package check_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// world is a set of sources on one persistent file set, as an editor session keeps them: an
// edited file is added again and gets a new, higher FileID, the others keep their ASTs.
type world struct {
	t     *testing.T
	proj  *project.Project
	fs    *source.FileSet
	src   map[string]string
	files map[string]*syntax.File
}

func newWorld(t *testing.T, srcs map[string]string) *world {
	t.Helper()
	return newWorldIn(t, srcs, slices.Sorted(maps.Keys(srcs)))
}

// newWorldIn adds the files to the file set in the given order, which sets their FileIDs.
func newWorldIn(t *testing.T, srcs map[string]string, order []string) *world {
	t.Helper()
	w := &world{t: t, proj: exampleProject(), fs: &source.FileSet{}, src: map[string]string{}, files: map[string]*syntax.File{}}
	for _, path := range order {
		w.commit(path, w.parse(path, srcs[path]))
	}
	return w
}

// parse adds a version of a file to the file set and parses it.
func (w *world) parse(path, text string) *syntax.File {
	w.t.Helper()
	src, err := w.fs.Add(path, "/"+path, []byte(text))
	if err != nil {
		w.t.Fatal(err)
	}
	return syntax.Parse(src, syntax.FileSource, diag.NewBag(w.fs, ""))
}

// commit makes f the current version of its path.
func (w *world) commit(path string, f *syntax.File) {
	w.src[path], w.files[path] = string(f.Src.Content), f
}

// edit is a new version of path with the first old replaced by new, not committed.
func (w *world) edit(path, old, new string) *syntax.File {
	w.t.Helper()
	if !strings.Contains(w.src[path], old) {
		w.t.Fatalf("%s holds no %q", path, old)
	}
	return w.parse(path, strings.Replace(w.src[path], old, new, 1))
}

// list is the current files in path order.
func (w *world) list() []*syntax.File {
	out := make([]*syntax.File, 0, len(w.files))
	for _, path := range slices.Sorted(maps.Keys(w.files)) {
		out = append(out, w.files[path])
	}
	return out
}

// bags are new bags holding the parse findings of every current file, as a build gives Check.
func (w *world) bags() check.Bags {
	bags := check.Bags{}
	for _, f := range w.list() {
		pkg := ""
		if f.Package != nil {
			pkg = syntax.Qualified(f.Package)
		}
		if bags[pkg] == nil {
			bags[pkg] = diag.NewBag(w.fs, pkg)
		}
		syntax.Parse(f.Src, syntax.FileSource, bags[pkg])
	}
	return bags
}

// session checks the current files keeping a session.
func (w *world) session() (*check.Program, *check.Session, check.Bags) {
	bags := w.bags()
	prog, s := check.CheckSession(context.Background(), w.proj, w.list(), bags, eval.NewFolder(bags, eval.Options{}))
	if s == nil {
		w.t.Fatal("no session")
	}
	return prog, s, bags
}

// cold is the canonical text of Check over the current files.
func (w *world) cold() string {
	bags := w.bags()
	prog := check.Check(context.Background(), w.proj, w.list(), bags, eval.NewFolder(bags, eval.Options{}))
	return canonical(w.fs, prog, bags)
}

// recheck rechecks s with fs in place of their paths' files; when it holds, they are committed.
func (w *world) recheck(s *check.Session, fs ...*syntax.File) (*check.Program, *check.Session, check.Bags, bool) {
	bags := check.Bags{}
	w.parseInto(bags, fs...)
	prog, next, ok := s.Recheck(context.Background(), fs, bags, eval.NewFolder(bags, eval.Options{}))
	for _, f := range fs {
		if ok {
			w.commit(f.Src.Path, f)
		}
	}
	return prog, next, bags, ok
}

// parseInto puts the parse findings of every current file, fs in place of their paths', in bags.
func (w *world) parseInto(bags check.Bags, fs ...*syntax.File) {
	for _, cur := range w.list() {
		for _, f := range fs {
			if cur.Src.Path == f.Src.Path {
				cur = f
			}
		}
		pkg := ""
		if cur.Package != nil {
			pkg = syntax.Qualified(cur.Package)
		}
		if bags[pkg] == nil {
			bags[pkg] = diag.NewBag(w.fs, pkg)
		}
		syntax.Parse(cur.Src, syntax.FileSource, bags[pkg])
	}
}

// staleRefs names each object Info of prog points at whose file is not one of prog's.
func staleRefs(prog *check.Program) []string {
	current := map[*syntax.File]bool{}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			current[f] = true
		}
	}
	var out []string
	note := func(o check.Object) {
		if o != nil && o.File() != nil && !current[o.File()] {
			out = append(out, objText(o))
		}
	}
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			note(o)
		}
	}
	for _, o := range prog.Info.Defs {
		note(o)
	}
	for _, o := range prog.Info.Uses {
		note(o)
	}
	for _, o := range prog.Info.NameUses {
		note(o)
	}
	for _, s := range prog.Info.Selections {
		note(s.Obj)
	}
	for _, c := range prog.Info.Calls {
		note(c.Obj)
	}
	for o := range prog.Info.Broken {
		note(o)
	}
	return out
}
