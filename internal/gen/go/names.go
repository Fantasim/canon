package gogen

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// upperCamel is Go's UpperCamel(x) (CODEGEN.md §3.2), ir's own copy so the two generators cannot drift (decision 120).
func upperCamel(name string) string { return ir.GoUpperCamel(name) }

// lowerCamel is lowerCamel(x): the first word lower case, then UpperCamel of the rest (CODEGEN.md §3.2).
func lowerCamel(name string) string { return ir.GoLowerCamel(name) }

// escapeLower suffixes `_` to a name reserved in a Go lower-case position (CODEGEN.md §3.4).
func escapeLower(name string) string { return ir.GoEscapeLower(name) }

// local escapes a parameter or local named like an imported package (§3.4, decision 182).
func (g *gen) local(name, origin string) string {
	if !g.canonGo[name] {
		return name
	}
	if g.canonGo[name+underscore] {
		g.failf(ErrNameCollision, "%s: the imports %s and %s%s", origin, name, name, underscore)
	}
	return name + underscore
}

// storageName is the unexported, escaped name of a field, parameter or value (CODEGEN.md §6.1).
func storageName(canon string) string {
	return escapeLower(lowerCamel(canon))
}

// exportedName is UpperCamel(canon), or the @go(name:) override when there is one (§3.5).
func exportedName(override, canon string) string {
	if override != "" {
		return override
	}
	return upperCamel(canon)
}

// accessorName is Get<V>, or the whole @go(name:) override: valueSlot, accessors and findBy share it.
func accessorName(v *ir.Value) string {
	if v.Go.Name != "" {
		return v.Go.Name
	}
	return getPrefix + upperCamel(v.Name)
}

// firstUpper upper-cases the first letter of a Canon type name (CODEGEN.md §3.3).
func firstUpper(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// validIdent reports whether name can be declared in Go: an identifier that is not a keyword.
func validIdent(name string) bool {
	return token.IsIdentifier(name)
}

// scope is one namespace of generated Go and what each name was generated for (§3.5).
type scope struct {
	what  string
	names map[string]string
}

func newScope(what string) *scope {
	return &scope{what: what, names: map[string]string{}}
}

// add declares name for origin: a name that is not a Go identifier, or that the scope already
// holds, is an error naming both origins.
func (s *scope) add(name, origin string) error {
	if !validIdent(name) {
		return fmt.Errorf("%w: %q (from %s)", ErrName, name, origin)
	}
	if prev, ok := s.names[name]; ok {
		return fmt.Errorf("%w: %s %s: %s, %s", ErrNameCollision, s.what, name, prev, origin)
	}
	s.names[name] = origin
	return nil
}

// checkOverrides refuses a @go(name:) override that is not exported (§1.3, decision 182).
func (g *gen) checkOverrides() {
	for _, t := range g.p.Types {
		g.typeOverrides(t)
	}
	for _, c := range g.p.Consts {
		g.override(c.Go, c.Name)
	}
	for _, v := range g.p.Values {
		g.override(v.Go, v.Name)
	}
	g.bodyOverrides(g.p.Name, nil, g.p.Fns)
}

func (g *gen) typeOverrides(t ir.Type) {
	switch t := t.(type) {
	case *ir.Record:
		g.override(t.Go, t.QName())
		g.bodyOverrides(t.QName(), t.Fields, t.Methods)
	case *ir.Enum:
		g.override(t.Go, t.QName())
		for _, m := range t.Members {
			g.override(m.Go, t.QName()+dot+m.Name)
		}
	case *ir.Variant:
		g.override(t.Go, t.QName())
		for _, c := range t.Cases {
			g.override(c.Go, t.QName()+dot+c.Name)
			g.bodyOverrides(t.QName()+dot+c.Name, c.Fields, c.Methods)
		}
	case *ir.Dependent:
		g.override(t.Go, t.QName())
	}
}

func (g *gen) bodyOverrides(owner string, fields []*ir.Field, fns []*ir.ExportFn) {
	for _, f := range fields {
		g.override(f.Go, owner+dot+f.Name)
	}
	for _, fn := range fns {
		g.override(fn.Go, owner+dot+fn.Name)
	}
}

func (g *gen) override(o ir.NameOptions, origin string) {
	if o.Name != "" && !token.IsExported(o.Name) {
		g.fail(newDetail(ErrName, origin, "@go(name: %q) of %s is not exported", o.Name, origin))
	}
}
