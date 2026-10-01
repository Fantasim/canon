package eval_test

import (
	"context"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
	"golang.org/x/tools/txtar"
)

// scopeDir is the project directory scopeArchive's data files are served under.
const scopeDir = "/p"

// scopeArchive gives each field of Cfg a load, by the Loader: bare and load.dir, as an item and as
// a default; Wait's own Duration is in ms; plain's load.dir has no field.
const scopeArchive = `-- a/a.canon --
/// A.
package a

/// A wait.
record Wait {
  /// In ms.
  t: Duration
}

/// Fields with wire forms, each given a load.
record Cfg {
  /// Whole, in seconds.
  d: Duration @json(unit: s)
  /// Whole, as 0 or 1.
  b: Bool @json(int)
  /// One file each, in seconds.
  ds: [Duration] @json(unit: s)
  /// One file each, as 0 or 1.
  bs: [Bool] @json(int)
  /// Rows keep their own forms.
  rows: table Wait
  /// A default.
  df: [Duration] = load.dir("ds/*.json") @json(unit: s)
}

/// Every field given a load.
let cfg: Cfg = {
  d: load("d.json")
  b: load("b.json")
  ds: load.dir("ds/*.json")
  bs: load.dir("bs/*.json")
  rows: load.dir("rows/*.json")
}

/// No field: the default unit.
let plain: [Duration] = load.dir("ds/*.json")
-- d.json --
5
-- b.json --
1
-- ds/a.json --
5
-- ds/b.json --
7
-- bs/a.json --
1
-- bs/b.json --
0
-- rows/w.json --
{"t": 5}
`

// rootedFS is a MapFS read by absolute, '/'-separated names, as the Loader names files.
type rootedFS struct{ m fstest.MapFS }

func (r rootedFS) ReadFile(name string) ([]byte, error) { return r.m.ReadFile(name[1:]) }

func (r rootedFS) Stat(name string) (fs.FileInfo, error) { return r.m.Stat(name[1:]) }

func (r rootedFS) ReadDir(name string) ([]fs.DirEntry, error) { return r.m.ReadDir(name[1:]) }

// loaderOf serves every load through the Loader over a's data files, its request decoding
// through the evaluator as build's host does (load.Request.Through); bag takes its findings.
func loaderOf(t *testing.T, p *program, a *txtar.Archive, bag *diag.Bag) loader {
	m := fstest.MapFS{}
	for _, f := range a.Files {
		if path.Ext(f.Name) != canonExt {
			m[scopeDir[1:]+"/"+f.Name] = &fstest.MapFile{Data: f.Data}
		}
	}
	layout, ok := project.NewLayout(&project.Project{}, scopeDir, nil, bag)
	if !ok {
		t.Fatal("layout")
	}
	l := &load.Loader{FS: rootedFS{m}, Layout: layout, Set: p.fs}
	return func(ev *eval.Evaluator, e *syntax.LoadExpr, typ types.Type) (value.Value, bool) {
		req := load.Request{Pkg: "a", Bag: bag}.Through(ev)
		v, ok, err := l.Load(context.Background(), req, e, typ)
		if err != nil {
			t.Error(err)
		}
		return v, ok
	}
}

// WIRE.md §4.1, §6.1, §6.5, DECISIONS 268: through the Loader, a load into a field decodes in its scope.
func TestLoadFieldScopeLoader(t *testing.T) {
	a := txtar.Parse([]byte(scopeArchive))
	p := fromArchive(t, a)
	bag := diag.NewBag(p.fs, "a")
	b := runBuildWith(t, p, eval.Options{}, loaderOf(t, p, a, bag))
	if n := len(bag.Findings()); n != 0 {
		t.Errorf("the loads reported %d findings: %+v", n, bag.Findings())
	}
	for _, c := range []struct{ path, want string }{
		{"a:cfg.d", "5s"}, {"a:cfg.b", "true"}, {"a:cfg.ds", "[5s, 7s]"}, {"a:cfg.bs", "[true, false]"},
		{"a:cfg.rows", "{w: Wait{t: 5ms}}"}, {"a:cfg.df", "[5s, 7s]"}, {"a:plain", "[5ms, 7ms]"},
	} {
		if got, _, _ := strings.Cut(b.explain(c.path), "\n"); got != c.path+" = "+c.want {
			t.Errorf("%s, want %s", got, c.want)
		}
	}
}

// DECISIONS 236, 268, WIRE.md §4.1: a load into a field reads bytes as FromJSON's decoder does.
func TestLoadFieldScopeAsFromJSON(t *testing.T) {
	a := txtar.Parse([]byte(scopeArchive))
	p := fromArchive(t, a)
	bag := diag.NewBag(p.fs, "a")
	b := runBuildWith(t, p, eval.Options{}, loaderOf(t, p, a, bag))
	cfg := namedRecord(b.checked, "Cfg")
	for i, name := range []string{"d", "b"} {
		f := cfg.Fields[i]
		src, err := p.fs.Add(name+".edit", "/edit/"+name, archiveFile(a, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		root, err := jsonsrc.Parse(src, bag)
		if err != nil {
			t.Fatal(err)
		}
		dec := wire.Decoder{Bag: bag, Pkg: "a", Keep: true, Field: f}
		v, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root}, f.Type)
		loaded, _, _ := strings.Cut(b.explain("a:cfg."+name), "\n")
		if err != nil || !ok || loaded != "a:cfg."+name+" = "+v.CanonText() {
			t.Errorf("%s: FromJSON's decoder gives %v (%v, %v), the load %s", name, v, ok, err, loaded)
		}
	}
}

// optArchive amends a path through a field that is none: the path reaches no field past it.
const optArchive = `-- a/a.canon --
/// A.
package a

/// A wait.
record Wt {
  /// Maybe.
  t: Duration?
}

/// Its wait is none.
record Po {
  /// None on the wire as "x".
  opt: Wt? @json(none: "x")
}

/// Its opt is none.
let po: Po = {}
-- a/dev.canon --
package a
layer dev

amend po {
  opt.t: load("x.json")
}
-- x.json --
"x"
`

// EVALUATION.md §9.3, DECISIONS 268: a path through none reaches no field; its load is in no scope.
func TestLoadAmendThroughNone(t *testing.T) {
	a := txtar.Parse([]byte(optArchive))
	p := fromArchive(t, a)
	bag := diag.NewBag(p.fs, "a")
	b := runBuildWith(t, p, eval.Options{Layers: []string{"dev"}}, loaderOf(t, p, a, bag))
	if f := bag.Findings(); len(f) != 1 || f[0].Code != diag.E7110.Def().Code {
		t.Errorf("the load reported %+v, want one type mismatch: opt's none marker is not t's", f)
	}
	if _, ok := b.values[eval.Root{Pkg: "a", Name: "po"}]; ok {
		t.Errorf("po holds a value, want it poisoned by its failed amendment")
	}
}
