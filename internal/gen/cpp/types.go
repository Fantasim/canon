package cppgen

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/public_decode.txt
var publicDecodeText string

// typesRefusals refuses in types mode what stage E refuses first: a stored export fn and a computed default (E8014, no data to read), an input field (E8019 InputField: CODEGEN.md §2.2 writes no LoadInputs in this mode).
func (g *gen) typesRefusals() {
	if !g.types() {
		return
	}
	for _, c := range g.declared() {
		fields, fns := c.shape()
		if i := slices.IndexFunc(fns, func(fn *ir.ExportFn) bool { return fn.Kind != ir.FnTranslated }); i >= 0 {
			g.malformed(typesStoredFn, c.canonName()+qnameSep+fns[i].Name) // E8014
		}
		if i := slices.IndexFunc(fields, func(f *ir.Field) bool { return f.Computed }); i >= 0 {
			g.malformed(typesComputed, c.canonName()+qnameSep+fields[i].Name) // E8014
		}
	}
	if len(g.inputs) > 0 {
		g.malformed(typesInputs, g.inputs[0].rec.Name+qnameSep+g.inputs[0].f.Name)
	}
}

// publicDecodeDecl declares a record's or variant's static Decode, first in its class (CODEGEN.md §5.13, §7.2).
func (g *gen) publicDecodeDecl(sc *scope, name string) {
	g.fail(sc.add(ir.CppDecode, name))
	g.h.linef(1, publicDecodeDeclFormat, name)
}

// publicDecoders define each record's and variant's static Decode, in class order (CODEGEN.md §2.7, §5.13): a decoder of its own whose paths start at `$`, and no value unless all of the JSON decodes.
func (g *gen) publicDecoders() {
	if !g.types() {
		return
	}
	for _, c := range g.classes {
		if c.cs == nil && c.dependent == nil {
			name := g.className(c)
			g.c.printf(publicDecodeText, name, ir.CppDecode, quote(name), quote(dollar))
		}
	}
}

// fieldDefault is the constant default a types-mode decoder gives f when its key is absent, nil for none or without one (CODEGEN.md §5.13; WIRE.md §5.4).
func (g *gen) fieldDefault(f *ir.Field) value.Value {
	if _, none := f.Default.(*value.None); none || !g.types() {
		return nil
	}
	return f.Default
}

// absentDefault writes the default an absent key takes, and returns the `} else ` the key's read then opens with; "" without a default.
func (g *gen) absentDefault(depth int, obj, key string, l leaf) string {
	if l.def == nil {
		return ""
	}
	g.c.linef(depth, absentOpenFormat, obj, key)
	g.c.linef(depth+1, assignFormat, l.dst, g.defaultLit(l.t, l.def))
	return elseLead
}

// defaultLit is the C++ expression of a constant default v of type t; a list is written with its storage type, so that it assigns a std::optional too (CODEGEN.md §5.1).
func (g *gen) defaultLit(t ir.TypeRef, v value.Value) string {
	switch {
	case ir.HeldApp(t) != nil && present(v):
		g.malformed(dependentDefault, g.at) // E8019 DependentType
		return cppInvalid
	case t.Kind == types.List:
		return g.storage(t) + g.literal(t, v)
	}
	return g.element(t, v)
}

// present reports a value a literal writes: not none, and a list holding one.
func present(v value.Value) bool {
	switch x := v.(type) {
	case nil, *value.None:
		return false
	case *value.List:
		return slices.ContainsFunc(x.Elems, present)
	}
	return true
}

// wireKeys are the keys a decoder checks an object's against (strict.go's Keys); none in types mode, which ignores unknown keys (CODEGEN.md §5.13).
func (g *gen) wireKeys(keys []string) []string {
	if g.types() {
		return nil
	}
	return keys
}

// decodeDuration reads a Duration: an integer count of units, or, in types mode, any number that is a whole number of milliseconds (WIRE.md §5.1, §5.13).
func (g *gen) decodeDuration(depth int, src, key string, l leaf) {
	if g.types() {
		g.c.linef(depth, sourceDurationFormat, src, key, l.unit.Millis(), l.dst)
		return
	}
	g.c.linef(depth, decCallFormat, asDuration, src, key, g.shortcutExtra(l), l.dst)
}

// unionMembership refuses in types mode a union's text that is no literal nor enum wire (CODEGEN.md §5.13).
func (g *gen) unionMembership(depth int, key, dst string, t ir.TypeRef) {
	e := g.unionEnum(t)
	if e == nil {
		return
	}
	var conds []string
	for _, w := range t.Literals {
		conds = append(conds, fmt.Sprintf(differsFormat, dst, quote(w)))
	}
	for _, m := range e.Members {
		conds = append(conds, fmt.Sprintf(differsFormat, dst, quote(m.Wire)))
	}
	g.c.linef(depth, unionCheckFormat, strings.Join(conds, andSep), key, dst)
}

// unionEnum is the enum arm of a union whose membership types mode checks, so which no read shortcut takes; nil otherwise.
func (g *gen) unionEnum(t ir.TypeRef) *ir.Enum {
	if !g.types() || t.Kind != types.LitUnion || t.Elem == nil {
		return nil
	}
	e, _ := t.Elem.Named.(*ir.Enum)
	return e
}
