package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// builds is what a fold may build: its record and case literals, the defaults they leave out, the values of the consts it reads (EVALUATION.md §1).
type builds struct {
	c        *checker
	bodies   map[*recordCtx]bool // the bodies whose defaults this walk saw checked
	finished map[*object]bool    // the consts and defaults walked to their end
	stack    []hop               // the consts and defaults being walked, outermost first: the path
	file     *syntax.File        // the file of the expression being walked
	unready  bool                // a body still being resolved, or one whose defaults are
	broken   bool                // a record, case or const broken, by its defaults, a cycle or before (TYPES.md §1)
}

// hop is a const or a field default on the path, and what read it: a const's name, a default's literal.
type hop struct {
	obj  *object
	use  syntax.Node
	file *syntax.File
}

// buildsOf checks the defaults of every record and case a fold of e, written in file, may build before the fold runs.
func (c *checker) buildsOf(e syntax.Expr, file *syntax.File) *builds {
	b := c.newBuilds()
	b.expr(e, file)
	return b
}

func (c *checker) newBuilds() *builds {
	return &builds{c: c, bodies: map[*recordCtx]bool{}, finished: map[*object]bool{}}
}

// buildable reports e free to fold: a body it builds still being resolved is E3015 (DECISIONS 150), a broken one nothing (TYPES.md §1).
func (c *checker) buildable(env *env, e syntax.Expr) bool {
	b := c.buildsOf(e, env.file)
	if b.unready {
		c.report(env, diag.E3015.AtNotConstant(env.span(e), env.what))
	}
	return !b.done()
}

// constCycles is E4301 for each cycle of p's consts through record defaults that no fold met (EVALUATION.md §3.2).
func (c *checker) constCycles(p *pkgState) {
	for _, o := range p.all {
		if o.kind == ObjConst && !c.info.Broken[o] {
			c.newBuilds().read(nil, o)
		}
	}
}

func (b *builds) done() bool {
	return b.unready || b.broken
}

func (b *builds) expr(e syntax.Expr, file *syntax.File) {
	outer := b.file
	b.file = file
	syntax.Inspect(e, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.BraceLit:
			if k, ok := b.c.info.Literals[x]; ok && k == LitRecord {
				b.literal(x, b.c.info.Types[x])
			}
		case *syntax.IdentExpr:
			b.read(x, b.c.info.Uses[x])
		case *syntax.Ident:
			b.read(x, b.c.info.NameUses[x])
		}
		return !b.done()
	})
	b.file = outer
}

// literal is a record or case literal of type t.
func (b *builds) literal(lit *syntax.BraceLit, t types.Type) {
	if t == nil {
		return
	}
	switch x := t.Base().(type) {
	case *types.RecordType:
		if o := b.c.typeObjects[x]; o != nil {
			b.body(o, o.body, lit)
		}
	case *types.AppliedRecord:
		b.literal(lit, x.Rec)
	case *types.CaseType:
		b.body(b.c.typeObjects[x.Variant], b.c.caseBodies[x], lit)
	}
}

// enter puts h on the path; one already on it closes a cycle, one walked to its end is not walked again.
func (b *builds) enter(h hop) bool {
	if i := slices.IndexFunc(b.stack, func(s hop) bool { return s.obj == h.obj }); i >= 0 {
		b.cycle(i, h)
		return false
	}
	if b.finished[h.obj] {
		return false
	}
	b.stack = append(b.stack, h)
	return true
}

func (b *builds) leave() {
	b.finished[b.stack[len(b.stack)-1].obj] = true
	b.stack = b.stack[:len(b.stack)-1]
}

// read is a const the fold evaluates: what its value builds; memoised once a walk of it ends with nothing unready, broken or cyclic.
func (b *builds) read(use syntax.Node, o Object) {
	x, ok := o.(*object)
	if !ok || x.kind != ObjConst || x.built {
		return
	}
	d, isConst := x.decl.(*syntax.ConstDecl)
	if !isConst {
		return
	}
	if b.c.info.Broken[x] {
		b.broken = true
		return
	}
	if !b.enter(hop{obj: x, use: use, file: b.file}) {
		return
	}
	b.expr(d.Value, x.file)
	b.leave()
	x.built = !b.done()
}

// body checks a record or case body's defaults now, once its fields are resolved and unless they
// are being checked, then walks the defaults lit leaves out, each on the path while it is walked.
func (b *builds) body(o *object, body *recordCtx, lit *syntax.BraceLit) {
	if o == nil {
		return
	}
	if body == nil || !b.c.bodyReady(body) || body.defaults == stateResolving {
		b.unready = true
		return
	}
	if !b.bodies[body] && !b.checkDefaults(o, body) {
		return
	}
	if b.c.info.Broken[o] {
		b.broken = true
		return
	}
	for _, fo := range body.order {
		d := fo.field.Default
		if b.done() || d == nil || fo.field.Input != nil || gives(lit, fo.name) || !b.enter(hop{obj: fo, use: lit, file: b.file}) {
			continue
		}
		b.expr(d, fo.file)
		b.leave()
	}
}

// checkDefaults checks body's defaults; one building a body still being resolved breaks o silently, the fold is E3015.
func (b *builds) checkDefaults(o *object, body *recordCtx) bool {
	b.bodies[body] = true
	before := b.c.unbuilt
	b.c.fieldDefaults(o, body)
	if b.c.unbuilt != before {
		b.c.breakObj(o)
		b.unready = true
		return false
	}
	return true
}

// gives reports lit giving the field named name, or every field through a spread.
func gives(lit *syntax.BraceLit, name string) bool {
	if lit == nil {
		return false
	}
	return slices.ContainsFunc(lit.Items, func(it syntax.BraceItem) bool {
		switch x := it.(type) {
		case *syntax.SpreadItem:
			return true
		case *syntax.FieldItem:
			return x.Name.Name == name
		}
		return false
	})
}

// literalBody is the body of a record or case literal's type.
func (c *checker) literalBody(t types.Type) *recordCtx {
	switch x := t.(type) {
	case *types.RecordType:
		return c.bodyOf(x)
	case *types.AppliedRecord:
		return c.bodyOf(x.Rec)
	case *types.CaseType:
		return c.caseBodies[x]
	}
	return nil
}

// bodyReady reports every field of body resolved: a literal or fold of it may type its values (TYPES.md §7.4, §15).
func (c *checker) bodyReady(body *recordCtx) bool {
	return body == nil || !slices.ContainsFunc(body.order, func(fo *object) bool {
		_, pending := c.fieldJobs[fo.field]
		return pending || fo.field.Type == nil
	})
}
