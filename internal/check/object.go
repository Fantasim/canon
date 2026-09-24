package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// object is the one Object implementation: a pointer per declaration, so == is identity.
type object struct {
	kind    ObjKind
	name    string
	pkg     string
	typ     types.Type
	decl    syntax.Node
	file    *syntax.File
	id      int
	local   bool
	mutable bool
	state   resolveState

	field  *types.Field
	member *types.Member
	owner  types.Type // the record, case, enum or variant holding a member of it
	target *pkgState  // ObjPackage: the imported package
	parent *object    // ObjEntry: the let whose collection holds it
	keys   *entryKeys // ObjLet initialized by a table literal (TYPES.md §4.1)
	body   *recordCtx
}

func (o *object) Kind() ObjKind { return o.kind }

func (o *object) Name() string { return o.name }

func (o *object) Pkg() string { return o.pkg }

func (o *object) Type() types.Type { return o.typ }

func (o *object) Decl() syntax.Node { return o.decl }

func (o *object) File() *syntax.File { return o.file }

// resolveState tracks lazy resolution of a top-level declaration, so a cycle is seen.
type resolveState uint8

// entryKeys are the keys of a table known statically: its literal's entries and every `entry`
// declaration of the package, in that order.
type entryKeys struct {
	byName map[string]*object
	order  []*object
}

func (k *entryKeys) add(o *object) (*object, bool) {
	if first, ok := k.byName[o.name]; ok {
		return first, false
	}
	k.byName[o.name] = o
	k.order = append(k.order, o)
	return nil, true
}

// newObject records a declaration of pkg in file; its id orders facts and hints. A declaration
// holding a syntax error is broken from the start (syntaxErrors).
func (c *checker) newObject(kind ObjKind, name string, pkg *pkgState, decl syntax.Node, file *syntax.File) *object {
	c.nextID++
	o := &object{kind: kind, name: name, decl: decl, file: file, id: c.nextID}
	if pkg != nil {
		o.pkg = pkg.path
	}
	if c.syntaxHeld[decl] {
		c.breakObj(o) // DECISIONS 214: it holds a syntax error
	}
	return o
}
