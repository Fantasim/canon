package check_test

import (
	"context"
	"fmt"
	"math/big"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// object is a fixture Object, as tests of later packages build them before the checker exists.
type object struct {
	kind      check.ObjKind
	name, pkg string
	typ       types.Type
	decl      syntax.Node
	file      *syntax.File
}

func (o *object) Kind() check.ObjKind { return o.kind }
func (o *object) Name() string        { return o.name }
func (o *object) Pkg() string         { return o.pkg }
func (o *object) Type() types.Type    { return o.typ }
func (o *object) Decl() syntax.Node   { return o.decl }
func (o *object) File() *syntax.File  { return o.file }

// The checker hands every conclusion over once: for `let limit: UInt16? = 3`, the declared
// name's object and the conversion the literal needs where it is stored.
func Example() {
	name, three := &syntax.Ident{Name: "limit"}, &syntax.IntLit{Value: big.NewInt(3)}
	limit := &object{kind: check.ObjLet, name: "limit", pkg: "teamboard", typ: &types.OptionalType{Elem: types.UInt16Type}}
	limit.decl = &syntax.LetDecl{Name: name, Value: three}
	prog := &check.Program{
		Packages: []*check.Package{{Path: "teamboard", Decls: []check.Object{limit}}},
		Info: &check.Info{
			Types: map[syntax.Expr]types.Type{three: types.IntType},
			Defs:  map[*syntax.Ident]check.Object{name: limit},
			Conv:  map[syntax.Expr]*check.Conversion{three: {Kind: check.ConvWrap, From: types.IntType, To: limit.typ}},
		},
	}
	for _, d := range prog.Packages[0].Decls {
		init := d.Decl().(*syntax.LetDecl)
		conv := prog.Info.Conv[init.Value]
		fmt.Println(prog.Info.ObjectOf(init.Name).Kind(), d.Pkg()+"."+d.Name(), prog.Info.Types[init.Value], conv.Kind, conv.To)
	}
	// Output: Let teamboard.limit Int Wrap UInt16?
}

// A refinement bound is folded as soon as it is typed; eval.NewFolder does it for a build.
func ExampleFolder() {
	var fold check.Folder = literalFolder{}
	bound := &syntax.IntLit{Value: big.NewInt(100)}
	owner := &object{kind: check.ObjField, name: "percent", pkg: "resource.farm", typ: types.IntType}
	v, ok := fold.Fold(context.Background(), owner, bound, &check.Info{Types: map[syntax.Expr]types.Type{bound: types.IntType}})
	fmt.Println(v.CanonText(), v.Type(), ok)
	// Output: 100 Int true
}

// Check types a package: here a table whose `next` refs resolve to it, and a let reading it.
func ExampleCheck() {
	fs := &source.FileSet{}
	src, _ := fs.Add("board/board.canon", "/board/board.canon", []byte(`package board

local record Status {
  next: [ref Status]
}

local let statuses: table Status = {
  open { next: [done] }
  done { next: [] }
}

local let first = statuses.active().first()
`))
	bag := diag.NewBag(fs, "board")
	file := syntax.Parse(src, syntax.FileSource, bag)
	prog := check.Check(context.Background(), project.New("demo", project.Version{Minor: 1}), []*syntax.File{file}, check.Bags{"board": bag}, literalFolder{})
	decls := prog.Packages[0].Decls
	fmt.Println(decls[1].Name(), decls[1].Type(), decls[2].Name(), decls[2].Type(), len(bag.Findings()))
	// Output: statuses table board.Status first board.Status? 0
}
