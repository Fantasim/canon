package verify_test

import (
	"bytes"
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"golang.org/x/tools/txtar"
)

const pkg = "teamboard"

// object is a fixture check.Object, as tests build them before the checker exists.
type object struct {
	kind check.ObjKind
	name string
	typ  types.Type
	decl syntax.Node
	file *syntax.File
}

func (o *object) Kind() check.ObjKind { return o.kind }
func (o *object) Name() string        { return o.name }
func (o *object) Pkg() string         { return pkg }
func (o *object) Type() types.Type    { return o.typ }
func (o *object) Decl() syntax.Node   { return o.decl }
func (o *object) File() *syntax.File  { return o.file }

// evaluator is a fixture evaluator: it serves top-level values and records invalid marks.
type evaluator struct {
	values   map[eval.Root]value.Value
	poisoned map[eval.Root]bool
	invalid  []value.Value
	holds    bool
	hard     bool
}

func (e *evaluator) Force(_ context.Context, r eval.Root) (value.Value, bool) {
	v, ok := e.values[r]
	return v, ok && !e.poisoned[r]
}

func (e *evaluator) MarkInvalid(v value.Value) { e.invalid = append(e.invalid, v) }

func (e *evaluator) Invalid(v value.Value) bool { return slices.Contains(e.invalid, v) }

func (e *evaluator) Where(context.Context, *types.Predicate, value.Value) (bool, bool) {
	return e.holds, !e.hard
}

// assets is a fixture listing of rooted asset roots, displayed as written: root -> exact paths.
type assets map[string][]string

func (a assets) Exists(root, _, p string) (string, bool) {
	return root, slices.Contains(a[root], p)
}

// fixture is one package of one parsed .canon file, its values built by hand.
type fixture struct {
	t       *testing.T
	fs      *source.FileSet
	file    *syntax.File
	src     string
	prog    *check.Program
	ev      *evaluator
	bag     *diag.Bag
	assets  assets
	archive *txtar.Archive // the txtar case the fixture was read from, nil for none
	out     []byte         // a case's findings when it ran a whole build, nil otherwise
}

func newFixture(t *testing.T, name string, data []byte) *fixture {
	t.Helper()
	fs := &source.FileSet{}
	src, err := fs.Add(name, "/"+name, data)
	if err != nil {
		t.Fatal(err)
	}
	parsed := diag.NewBag(fs, pkg)
	file := syntax.Parse(src, syntax.FileSource, parsed)
	if n := len(parsed.Findings()); n != 0 {
		t.Fatalf("%s: %d parse findings", name, n)
	}
	fx := &fixture{t: t, fs: fs, file: file, src: string(src.Content), bag: diag.NewBag(fs, pkg)}
	fx.ev = &evaluator{values: map[eval.Root]value.Value{}, poisoned: map[eval.Root]bool{}}
	fx.prog = &check.Program{
		Packages: []*check.Package{{Path: pkg, Files: []*syntax.File{file}}},
		Info:     &check.Info{TypeExprs: map[syntax.Type]types.Type{}},
	}
	return fx
}

// fromArchive is the fixture of a txtar case's one .canon file.
func fromArchive(t *testing.T, a *txtar.Archive) *fixture {
	t.Helper()
	for _, f := range a.Files {
		if path.Ext(f.Name) == ".canon" {
			fx := newFixture(t, f.Name, f.Data)
			fx.archive = a
			return fx
		}
	}
	t.Fatal("no .canon file in the case")
	return nil
}

// at is the span of the first occurrence of text inside the first occurrence of within.
func (fx *fixture) at(text string, within ...string) source.Span {
	fx.t.Helper()
	base := 0
	for _, w := range within {
		i := strings.Index(fx.src[base:], w)
		if i < 0 {
			fx.t.Fatalf("no %q in the source", w)
		}
		base += i
	}
	i := strings.Index(fx.src[base:], text)
	if i < 0 {
		fx.t.Fatalf("no %q in the source", text)
	}
	start := base + i
	return source.Span{File: fx.file.Src.ID, Start: source.Pos(start), End: source.Pos(start + len(text))}
}

// lit is the provenance of a literal written as text (inside within).
func (fx *fixture) lit(text string, within ...string) *value.Prov {
	fx.t.Helper()
	return &value.Prov{Kind: value.ProvLiteral, Span: fx.at(text, within...)}
}

// computed is the provenance of a value an operator computed, written as text.
func (fx *fixture) computed(text string, within ...string) *value.Prov {
	fx.t.Helper()
	return &value.Prov{Kind: value.ProvComputed, Span: fx.at(text, within...)}
}

// inCall is the provenance of text computed inside fn, called at call (within within).
func (fx *fixture) inCall(text, fn, call string, within ...string) *value.Prov {
	fx.t.Helper()
	p := fx.computed(text)
	p.Stack = []diag.Frame{{Fn: fn, Span: fx.at(call, within...)}}
	return p
}

// decl is the top-level declaration named name.
func (fx *fixture) decl(name string) syntax.Decl {
	fx.t.Helper()
	for _, d := range fx.file.Decls {
		switch d := d.(type) {
		case *syntax.EnumDecl:
			if d.Name.Name == name {
				return d
			}
		case *syntax.VariantDecl:
			if d.Name.Name == name {
				return d
			}
		}
	}
	fx.t.Fatalf("no declaration %s", name)
	return nil
}

// written records that the type written as text (after within) resolves to t (Info.TypeExprs).
func (fx *fixture) written(text string, t types.Type, within ...string) types.Type {
	fx.t.Helper()
	from := fx.at(text, within...).Start
	var found syntax.Type
	syntax.Inspect(fx.file, func(n syntax.Node) bool {
		if tn, ok := n.(syntax.Type); ok && found == nil && fx.text(tn) == text && fx.file.Span(tn).Start >= from {
			found = tn
		}
		return found == nil
	})
	if found == nil {
		fx.t.Fatalf("no written type %q", text)
	}
	fx.prog.Info.TypeExprs[found] = t
	return t
}

func (fx *fixture) text(n syntax.Node) string {
	s := fx.file.Span(n)
	return fx.src[s.Start:s.End]
}

// let declares a top-level value of the package and serves it to Force.
func (fx *fixture) let(name string, t types.Type, v value.Value) {
	obj := &object{kind: check.ObjLet, name: name, typ: t, file: fx.file}
	fx.prog.Packages[0].Decls = append(fx.prog.Packages[0].Decls, obj)
	fx.ev.values[eval.Root{Pkg: pkg, Name: name}] = v
}

// entry is a table entry of coll written `key { … }`.
func (fx *fixture) entry(coll *types.Collection, key string, t types.Type, fields ...value.Value) *value.Record {
	fx.t.Helper()
	return &value.Record{
		T: t, Fields: fields, Set: make([]bool, len(fields)),
		Ident: &value.Identity{Coll: coll, Key: value.Key{S: key}},
		P:     &value.Prov{Kind: value.ProvLiteral, Span: fx.at(key + " {")},
	}
}

func (fx *fixture) verifier() *verify.Verifier {
	var a verify.Assets
	if fx.assets != nil {
		a = fx.assets
	}
	return verify.New(fx.ev, fx.prog, map[string]*diag.Bag{pkg: fx.bag}, a)
}

// verify runs stage B on each named top-level value, in order, with one verifier.
func (fx *fixture) verify(names ...string) []verify.Result {
	fx.t.Helper()
	v := fx.verifier()
	var out []verify.Result
	for _, name := range names {
		root := eval.Root{Pkg: pkg, Name: name}
		res, err := v.Check(context.Background(), root, fx.ev.values[root])
		if err != nil {
			fx.t.Fatal(err)
		}
		out = append(out, res)
	}
	return out
}

// render is the bag's findings in the golden text form.
func (fx *fixture) render() []byte {
	fx.t.Helper()
	var buf bytes.Buffer
	opt := diag.RenderOptions{Summary: fx.bag.Summary(), Golden: true}
	if err := diag.Render(&buf, fx.fs, fx.bag.Findings(), opt); err != nil {
		fx.t.Fatal(err)
	}
	return buf.Bytes()
}

func str(v string, t types.Type, p *value.Prov) *value.Str { return &value.Str{V: v, T: t, P: p} }

func integer(v int64, t types.Type, p *value.Prov) *value.Int { return &value.Int{V: v, T: t, P: p} }

func from(lo int64) *types.Bound { return &types.Bound{Lo: types.Limit{I: lo}, HasLo: true} }

func record(name string, fields ...*types.Field) *types.RecordType {
	for i, f := range fields {
		f.Index = i
	}
	return &types.RecordType{Pkg: pkg, Name: name, Fields: fields}
}

func field(name string, t types.Type) *types.Field { return &types.Field{Name: name, Type: t} }

func collection(name string, elem types.Type) *types.Collection {
	return &types.Collection{Kind: types.CollLet, Pkg: pkg, Name: name, Elem: elem}
}
