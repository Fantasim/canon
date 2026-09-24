package ir_test

import (
	"bytes"
	"context"
	"io/fs"
	"maps"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

const (
	examplesDir = "../../examples"
	projectFile = "project.canon"
	canonExt    = ".canon"
	jsonExt     = ".json"
)

// world is a parsed and checked program with a fixture host: values come from source-wire
// JSON (`<pkg>.<name>.json`), decoded as their declared type; calls from a Go function.
type world struct {
	fs     *source.FileSet
	files  []*syntax.File
	parse  *diag.Bag
	proj   *project.Project
	bags   check.Bags
	prog   *check.Program
	values map[string][]byte
	cache  map[string]value.Value
	calls  func(fn check.Object, recv value.Value, args []value.Value) (value.Value, bool)
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{fs: &source.FileSet{}, values: map[string][]byte{}, cache: map[string]value.Value{}, bags: check.Bags{}}
	w.parse = diag.NewBag(w.fs, "")
	data, err := os.ReadFile(filepath.Join(examplesDir, projectFile))
	if err != nil {
		t.Fatal(err)
	}
	src, err := w.fs.Add(projectFile, "/"+projectFile, data)
	if err != nil {
		t.Fatal(err)
	}
	if w.proj, err = project.Load(src, w.parse); err != nil {
		t.Fatal(err)
	}
	return w
}

// add parses a .canon file, or keeps a `<pkg>.<name>.json` value.
func (w *world) add(t *testing.T, name string, data []byte) {
	t.Helper()
	if path.Ext(name) == jsonExt {
		w.values[strings.TrimSuffix(name, jsonExt)] = data
		return
	}
	src, err := w.fs.Add(name, "/"+name, data)
	if err != nil {
		t.Fatal(err)
	}
	w.files = append(w.files, syntax.Parse(src, syntax.FileSource, w.parse))
}

// examples adds every .canon file under the example directories dirs, in path order.
func (w *world) examples(t *testing.T, dirs ...string) {
	t.Helper()
	err := filepath.WalkDir(examplesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != canonExt || d.Name() == projectFile {
			return err
		}
		rel, err := filepath.Rel(examplesDir, p)
		rel = filepath.ToSlash(rel)
		if err != nil || !slices.ContainsFunc(dirs, func(d string) bool { return strings.HasPrefix(rel, d+"/") }) {
			return err
		}
		data, err := os.ReadFile(p)
		w.add(t, rel, data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (w *world) check(t *testing.T) {
	t.Helper()
	w.prog = check.Check(context.Background(), w.proj, w.files, w.bags, folder{})
}

// build runs stage E over the selected packages (all when none is named).
func (w *world) build(t *testing.T, selected ...string) []*ir.Package {
	t.Helper()
	if w.prog == nil {
		w.check(t)
	}
	in := ir.Input{Program: w.prog, Project: w.proj, Bags: w.bags, Host: w, Fold: folder{}}
	if len(selected) > 0 {
		in.Selected = selected
	}
	return ir.Build(context.Background(), in)
}

// Value decodes the fixture JSON of pkg.name as its declared type, once.
func (w *world) Value(ctx context.Context, pkg, name string) (value.Value, bool) {
	key := pkg + "." + name
	if v, ok := w.cache[key]; ok {
		return v, v != nil
	}
	w.cache[key] = nil
	root := w.node(key)
	obj := w.object(pkg, name)
	if root == nil || obj == nil {
		return nil, false
	}
	v, ok := w.decode(ctx, pkg, root, obj.Type())
	if ok {
		w.cache[key] = v
	}
	return v, ok
}

// node parses the fixture file key.json, or is nil.
func (w *world) node(key string) *jsonsrc.Node {
	data, ok := w.values[key]
	if !ok {
		return nil
	}
	src, err := w.fs.Add(key+jsonExt, "/"+key+jsonExt, data)
	if err != nil {
		return nil
	}
	root, err := jsonsrc.Parse(src, diag.NewBag(w.fs, ""))
	if err != nil {
		return nil
	}
	return root
}

func (w *world) decode(ctx context.Context, pkg string, n *jsonsrc.Node, t types.Type) (value.Value, bool) {
	dec := wire.Decoder{Bag: diag.NewBag(w.fs, pkg), Pkg: pkg}
	v, ok, err := dec.Decode(ctx, wire.Selection{Node: n}, t)
	return v, ok && err == nil
}

// fixtureCalls answers a call from `<pkg>.<fn>.calls.json`, an object keyed by the canonical
// texts of the arguments joined by a space.
func (w *world) fixtureCalls(fn check.Object, _ value.Value, args []value.Value) (value.Value, bool) {
	root := w.node(fn.Pkg() + "." + fn.Name() + ".calls")
	sig, ok := fn.Type().(*types.FuncType)
	if root == nil || !ok {
		return nil, false
	}
	texts := make([]string, len(args))
	for i, a := range args {
		texts[i] = a.CanonText()
	}
	key := strings.Join(texts, " ")
	for _, m := range root.Members {
		if m.Key == key {
			return w.decode(context.Background(), fn.Pkg(), m.Value, sig.Result)
		}
	}
	return nil, false
}

func (w *world) Call(_ context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
	if w.calls == nil {
		return nil, false
	}
	return w.calls(fn, recv, args)
}

func (w *world) object(pkg, name string) check.Object {
	for _, p := range w.prog.Packages {
		if p.Path != pkg {
			continue
		}
		for _, o := range p.Decls {
			if o.Name() == name && (o.Kind() == check.ObjLet || o.Kind() == check.ObjConst) {
				return o
			}
		}
	}
	return nil
}

// findings renders the parse findings, then every package's, with their summary.
func (w *world) findings(t *testing.T) string {
	t.Helper()
	all := w.parse.Findings()
	sum := w.parse.Summary()
	sum.Packages = 0
	for _, name := range slices.Sorted(maps.Keys(w.bags)) {
		all = append(all, w.bags[name].Findings()...)
		sum = sum.Merge(w.bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, w.fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// folder folds the literals, constants, members and lists field defaults and bounds use.
type folder struct{}

func (folder) Fold(_ context.Context, _ check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	return fold(e, info)
}

func fold(e syntax.Expr, info *check.Info) (value.Value, bool) {
	t := info.Types[e]
	switch x := e.(type) {
	case *syntax.ParenExpr:
		return fold(x.X, info)
	case *syntax.IdentExpr:
		return foldName(x, info)
	case *syntax.UnaryExpr:
		if n, ok := x.X.(*syntax.IntLit); ok && x.Op == syntax.TokMinus {
			return fold(&syntax.IntLit{Value: new(big.Int).Neg(n.Value)}, info)
		}
	case *syntax.IntLit:
		return &value.Int{V: x.Value.Int64(), T: t}, x.Value.IsInt64()
	case *syntax.FloatLit:
		f, err := strconv.ParseFloat(x.Coef.String()+"e"+strconv.FormatInt(x.Exp, 10), 64)
		return &value.Float{V: f, T: t}, err == nil && !x.Neg
	case *syntax.DurationLit:
		return &value.Dur{Ms: x.Millis}, true
	case *syntax.BoolLit:
		return &value.Bool{V: x.Value}, true
	case *syntax.NoneLit:
		return &value.None{T: t}, true
	case *syntax.StringLit, *syntax.RawStringLit:
		return foldString(x, t)
	case *syntax.ListLit:
		return foldList(x, info)
	}
	return nil, false
}

func foldName(x *syntax.IdentExpr, info *check.Info) (value.Value, bool) {
	o := info.Uses[x]
	switch {
	case o == nil:
		return nil, false
	case o.Kind() == check.ObjConst:
		return fold(o.Decl().(*syntax.ConstDecl).Value, info)
	case o.Kind() == check.ObjMember:
		e, ok := o.Type().Base().(*types.EnumType)
		for i := 0; ok && i < len(e.Members); i++ {
			if e.Members[i].Name == o.Name() {
				return &value.Member{Enum: e, Index: i}, true
			}
		}
	}
	return nil, false
}

func foldString(x syntax.Expr, t types.Type) (value.Value, bool) {
	switch s := x.(type) {
	case *syntax.RawStringLit:
		return &value.Str{V: s.Value, T: t}, true
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range s.Parts {
			if p.Interp != nil {
				return nil, false
			}
			b.WriteString(p.Text)
		}
		return &value.Str{V: b.String(), T: t}, true
	}
	return nil, false
}

func foldList(x *syntax.ListLit, info *check.Info) (value.Value, bool) {
	l := &value.List{T: info.Types[x]}
	for _, el := range x.Elems {
		v, ok := fold(el, info)
		if !ok {
			return nil, false
		}
		l.Elems = append(l.Elems, v)
	}
	return l, true
}
