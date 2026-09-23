package rules_test

import (
	"bytes"
	"context"
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"golang.org/x/tools/txtar"
)

const pkg = "teamboard"

// object is a fixture check.Object.
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

// script is a hand-written check run: the outcome of one check on one instance (nil at
// package level).
type script func(self value.Value) rules.Run

// evaluator is a fixture evaluator for stages B to D: it serves top-level values, keeps the
// invalid marks verification makes and runs checks from scripts.
type evaluator struct {
	scripts map[*syntax.CheckDecl]script
	invalid map[value.Value]bool
	values  map[eval.Root]value.Value
	calls   []*syntax.CheckDecl
}

func (e *evaluator) Invalid(v value.Value) bool { return e.invalid[v] }

func (e *evaluator) MarkInvalid(v value.Value) { e.invalid[v] = true }

func (e *evaluator) Force(_ context.Context, r eval.Root) (value.Value, bool) {
	v, ok := e.values[r]
	return v, ok
}

func (e *evaluator) Where(context.Context, *types.Predicate, value.Value) (bool, bool) {
	return true, true
}

func (e *evaluator) Run(_ context.Context, c *syntax.CheckDecl, self value.Value) rules.Run {
	e.calls = append(e.calls, c)
	if s, ok := e.scripts[c]; ok {
		return s(self)
	}
	return rules.Run{}
}

// fixture is one package of one parsed .canon file, its values built by hand.
type fixture struct {
	t     *testing.T
	fs    *source.FileSet
	file  *syntax.File
	src   string
	prog  *check.Program
	ev    *evaluator
	bag   *diag.Bag
	roots []eval.Root
	vals  []value.Value
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
		t.Fatalf("%s: %v", name, parsed.Findings())
	}
	fx := &fixture{t: t, fs: fs, file: file, src: string(src.Content), bag: diag.NewBag(fs, pkg)}
	fx.ev = &evaluator{scripts: map[*syntax.CheckDecl]script{}, invalid: map[value.Value]bool{}, values: map[eval.Root]value.Value{}}
	fx.prog = &check.Program{
		Packages: []*check.Package{{Path: pkg, Files: []*syntax.File{file}}},
		Info: &check.Info{
			Uses: map[*syntax.IdentExpr]check.Object{}, Selections: map[*syntax.SelectorExpr]*check.Selection{},
			TypeExprs: map[syntax.Type]types.Type{}, Broken: map[check.Object]bool{},
		},
	}
	for _, d := range file.Decls {
		if c, ok := d.(*syntax.CheckDecl); ok {
			fx.prog.Packages[0].Decls = append(fx.prog.Packages[0].Decls, &object{kind: check.ObjCheck, decl: c, file: file})
		}
	}
	return fx
}

func fromArchive(t *testing.T, a *txtar.Archive) *fixture {
	t.Helper()
	for _, f := range a.Files {
		if path.Ext(f.Name) == ".canon" {
			return newFixture(t, f.Name, f.Data)
		}
	}
	t.Fatal("no .canon file in the case")
	return nil
}

func (fx *fixture) at(text string, within ...string) source.Span {
	fx.t.Helper()
	base := 0
	for _, w := range append(within, text) {
		i := strings.Index(fx.src[base:], w)
		if i < 0 {
			fx.t.Fatalf("no %q in the source", w)
		}
		base += i
	}
	return source.Span{File: fx.file.Src.ID, Start: source.Pos(base), End: source.Pos(base + len(text))}
}

func (fx *fixture) lit(text string, within ...string) *value.Prov {
	fx.t.Helper()
	return &value.Prov{Kind: value.ProvLiteral, Span: fx.at(text, within...)}
}

// check is the check declaration whose source starts with text.
func (fx *fixture) check(text string) *syntax.CheckDecl {
	fx.t.Helper()
	var found *syntax.CheckDecl
	syntax.Inspect(fx.file, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CheckDecl); ok && found == nil && strings.HasPrefix(fx.text(c), text) {
			found = c
		}
		return found == nil
	})
	if found == nil {
		fx.t.Fatalf("no check %q", text)
	}
	return found
}

func (fx *fixture) text(n syntax.Node) string {
	s := fx.file.Span(n)
	return fx.src[s.Start:s.End]
}

// typeName declares a record or variant of the package.
func (fx *fixture) typeName(t types.Type) *object {
	obj := &object{kind: check.ObjTypeName, typ: t, file: fx.file}
	fx.prog.Packages[0].Decls = append(fx.prog.Packages[0].Decls, obj)
	return obj
}

// written records that the type written as text resolves to t (Info.TypeExprs).
func (fx *fixture) written(text string, t types.Type) types.Type {
	fx.t.Helper()
	found := false
	syntax.Inspect(fx.file, func(n syntax.Node) bool {
		if tn, ok := n.(syntax.Type); ok && !found && fx.text(tn) == text {
			fx.prog.Info.TypeExprs[tn], found = t, true
		}
		return !found
	})
	if !found {
		fx.t.Fatalf("no written type %q", text)
	}
	return t
}

// let adds a top-level value to stages B and C, in declaration order.
func (fx *fixture) let(name string, v value.Value) {
	root := eval.Root{Pkg: pkg, Name: name}
	fx.roots = append(fx.roots, root)
	fx.vals = append(fx.vals, v)
	fx.ev.values[root] = v
}

// entry is a table entry `key { … }` of coll.
func (fx *fixture) entry(coll *types.Collection, key string, t types.Type, fields ...value.Value) *value.Record {
	fx.t.Helper()
	return &value.Record{
		T: t, Fields: fields, Ident: &value.Identity{Coll: coll, Key: value.Key{S: key}},
		P: &value.Prov{Kind: value.ProvLiteral, Span: fx.at(key + " {")},
	}
}

// run is a build's stages B, C and D over the fixture's values.
func (fx *fixture) run() {
	fx.t.Helper()
	bags := map[string]*diag.Bag{pkg: fx.bag}
	v, r := verify.New(fx.ev, fx.prog, bags, nil), rules.New(fx.ev, fx.prog, bags)
	ctx := context.Background()
	errs := []error{r.Names(fx.prog.Packages[0])}
	for i, root := range fx.roots {
		_, err := v.Check(ctx, root, fx.vals[i])
		errs = append(errs, err)
	}
	for i, root := range fx.roots {
		errs = append(errs, r.Instances(ctx, root, fx.vals[i]))
	}
	errs = append(errs, r.Package(ctx, fx.prog.Packages[0]))
	if err := errors.Join(errs...); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *fixture) render() []byte {
	fx.t.Helper()
	var buf bytes.Buffer
	opt := diag.RenderOptions{Summary: fx.bag.Summary(), Golden: true}
	if err := diag.Render(&buf, fx.fs, fx.bag.Findings(), opt); err != nil {
		fx.t.Fatal(err)
	}
	return buf.Bytes()
}

func record(name string, fields ...*types.Field) *types.RecordType {
	for i, f := range fields {
		f.Index = i
	}
	return &types.RecordType{Pkg: pkg, Name: name, Fields: fields}
}

func field(name string, t types.Type) *types.Field { return &types.Field{Name: name, Type: t} }

func str(v string, p *value.Prov) *value.Str { return &value.Str{V: v, T: types.StringType, P: p} }

// fails is a one-line run whose condition is false.
func fails(message string) script {
	return func(value.Value) rules.Run { return rules.Run{Failed: true, Message: message} }
}

// failsFor is fails on the given instances only.
func failsFor(message string, on ...value.Value) script {
	return func(self value.Value) rules.Run {
		for _, v := range on {
			if v == self {
				return rules.Run{Failed: true, Message: message}
			}
		}
		return rules.Run{}
	}
}
