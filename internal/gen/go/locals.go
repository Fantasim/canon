package gogen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"

	"github.com/fantasim/canonlang/internal/ir"
)

// locals are the escaped locals of data mode's loaders, decoders and resolvers (CODEGEN.md §3.4, decision 182).
type locals struct {
	Name, Path, Raw, Out, Obj, Err, F, Rows, Values, Keys, I, ID, Retired, Dir, S, Ctx, Tag, C          string
	Key, K, R, OK, Bad, Want, Dst, N, Lo, Hi, V, Kr, Vr, HasK, HasV, First, Empty, Marker, A, M, Af, Mf string
	At                                                                                                  string
}

func (g *gen) newLocals() locals {
	return locals{
		Name: g.local(localName), Path: g.local(localPath), Raw: g.local(localRaw), Out: g.local(localOut),
		Obj: g.local(localObj), Err: g.local(localErr), F: g.local(localFile),
		Rows: g.local(ir.GoRows), Values: g.local(localValues), Keys: g.local(localKeys), I: g.local(localIndex),
		ID: g.local(ir.GoIDStore), Retired: g.local(ir.GoRetiredStore), Dir: g.local(localDir), S: g.local(localSnap), Ctx: g.local(localCtx),
		Tag: g.local(localTag), C: g.local(localCase),
		Key: g.local(keyArg), K: g.local(tempKey), R: g.local(tempRaw), OK: g.local(tempOK),
		Bad: g.local(localBad), Want: g.local(localWant), Dst: g.local(localDst), N: g.local(tempInt), Lo: g.local(localLo),
		Hi: g.local(localHi), V: g.local(tempValue), Kr: g.local(localKr), Vr: g.local(localVr), HasK: g.local(localHasK),
		HasV: g.local(localHasV), First: g.local(localFirst), Empty: g.local(tempEmpty), Marker: g.local(localMarker),
		A: g.local(localA), M: g.local(tempMember), Af: g.local(localAf), Mf: g.local(localMf),
		At: g.local(localAt),
	}
}

// importedNames are the Go package names of the imported Canon packages' go emits.
func importedNames(p *ir.Package) map[string]bool {
	out := map[string]bool{}
	for _, ref := range p.Imports {
		for _, e := range ref.Emits {
			if e.Target == ir.TargetGo {
				out[e.GoPackage] = true
			}
		}
	}
	return out
}

// local is name, with `_` added until no imported Canon package is called so.
func (g *gen) local(name string) string {
	for g.taken[name] {
		name += underscore
	}
	return name
}

// temp is a fresh local of the function being written: base and a number.
func (g *gen) temp(base string) string {
	g.temps++
	return g.local(base + strconv.Itoa(g.temps))
}

// checkNames refuses a file declaring a name twice in one scope, a function's parameters included; in data mode also a parameter named like an import (decision 203: ir plans baked names only).
func checkNames(src []byte, data bool) error {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("%w: %w", errFormat, err)
	}
	c := nameCheck{scopes: map[string]map[string]bool{}, imports: map[string]bool{}, data: data}
	for _, imp := range f.Imports {
		c.importName(imp)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			c.funcName(d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				c.specNames(spec)
			}
		}
	}
	return c.err
}

// nameCheck is the names declared so far, per scope (a type's name, or packageScope), and the first repeat.
type nameCheck struct {
	scopes  map[string]map[string]bool
	imports map[string]bool
	data    bool // a parameter may not be named like an import (CODEGEN.md §3.4)
	err     error
}

func (c *nameCheck) add(scope, name string) {
	names := c.scopes[scope]
	if names == nil {
		names = map[string]bool{}
		c.scopes[scope] = names
	}
	if names[name] && c.err == nil {
		c.err = newDetail(ErrNameCollision, name, dataCollisionFormat, scope, name)
	}
	names[name] = true
}

func (c *nameCheck) importName(imp *ast.ImportSpec) {
	p, err := strconv.Unquote(imp.Path.Value)
	name := path.Base(p)
	if imp.Name != nil {
		name = imp.Name.Name
	}
	if err == nil {
		c.add(packageScope, name)
		c.imports[name] = true
	}
}

// funcName adds a function to the package, a method to its receiver's type, and checks the
// names its receiver, parameters and results declare.
func (c *nameCheck) funcName(d *ast.FuncDecl) {
	c.locals(d)
	if d.Recv == nil || len(d.Recv.List) == 0 {
		c.add(packageScope, d.Name.Name)
		return
	}
	if recv := recvType(d); recv != "" {
		c.add(recv, d.Name.Name)
	}
}

// recvType is the name of a method's receiver type, "" for a function.
func recvType(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return ""
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// specNames adds a type, its struct fields, or constants and variables.
func (c *nameCheck) specNames(spec ast.Spec) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		c.add(packageScope, s.Name.Name)
		st, ok := s.Type.(*ast.StructType)
		if !ok {
			return
		}
		for _, field := range st.Fields.List {
			for _, n := range field.Names {
				c.add(s.Name.Name, n.Name)
			}
		}
	case *ast.ValueSpec:
		for _, n := range s.Names {
			c.add(packageScope, n.Name)
		}
	}
}

// locals checks one function's receiver, parameters and results: one scope.
func (c *nameCheck) locals(d *ast.FuncDecl) {
	scope := d.Name.Name + funcScopeSuffix
	if recv := recvType(d); recv != "" {
		scope = recv + dot + scope
	}
	names := map[string]bool{}
	for _, list := range []*ast.FieldList{d.Recv, d.Type.Params, d.Type.Results} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			for _, n := range field.Names {
				c.local(scope, n.Name, names)
			}
		}
	}
}

func (c *nameCheck) local(scope, name string, names map[string]bool) {
	if (names[name] || c.data && c.imports[name]) && c.err == nil && name != underscore {
		c.err = newDetail(ErrNameCollision, name, dataCollisionFormat, scope, name)
	}
	names[name] = true
}
