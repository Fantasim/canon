package rules

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// CheckUnits reports E1610 for each field's `unit:` naming no entry of units, the studio's
// evaluated `units` table, which is not a table when it did not evaluate (VIEWMODEL.md G16).
func CheckUnits(ctx context.Context, prog *check.Program, bags check.Bags, studio string, units value.Value) {
	c := newChecker(prog, bags, studio)
	keys, ok := c.unitKeys(units)
	if !ok {
		return
	}
	for _, p := range prog.Packages {
		if ctx.Err() != nil {
			return
		}
		if bag := bags[p.Path]; bag != nil {
			c.eachView(p, bag, false, func(v *view) { v.units(keys) })
		}
	}
}

// unitKeys are the keys of studio's units table: none when studio declares no `units`; false
// when they cannot be told.
func (c *checker) unitKeys(units value.Value) (map[string]bool, bool) {
	if c.studio == nil {
		return nil, false
	}
	keys := map[string]bool{}
	if o := c.studio.decls[syntax.StudioUnits]; o == nil || o.Kind() != check.ObjLet {
		return keys, true
	}
	t, ok := units.(*value.Table)
	if !ok {
		return nil, false
	}
	for _, e := range t.Entries {
		if e.Ident != nil {
			keys[e.Ident.Key.Text()] = true
		}
	}
	return keys, true
}

// units checks the unit of each field item the static pass checked the properties of.
func (v *view) units(keys map[string]bool) {
	if !slices.Contains(allowed[v.kind], syntax.KindViewField) {
		return
	}
	v.quiet = true
	for _, it := range v.decl.Items {
		switch it := it.(type) {
		case *syntax.ViewField:
			v.unit(it, keys)
		case *syntax.ViewGroup:
			v.groupUnits(it, keys)
		}
	}
}

func (v *view) groupUnits(g *syntax.ViewGroup, keys map[string]bool) {
	for _, m := range g.Members {
		if f, ok := m.(*syntax.ViewField); ok {
			v.unit(f, keys)
		}
	}
}

// unitTwice records a unit property, true when the item has one already (E1613 twice).
func (v *view) unitTwice(n named, fi *syntax.FieldItem) bool {
	if _, dup := v.c.givenAt(n.keys, syntax.PropUnit); dup {
		return true
	}
	for _, k := range n.keys {
		v.c.given[propKey{k, syntax.PropUnit}] = v.span(fi.Name)
	}
	return false
}

func (v *view) unit(f *syntax.ViewField, keys map[string]bool) {
	if f.Name == nil || f.Props == nil {
		return
	}
	n, ok := v.resolve(f.Name)
	if !ok || n.kind != diag.KindField || !holdsNumbers(n.field.Type) {
		return // E1611 or another static finding says it already
	}
	for _, it := range f.Props.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok || fi.Name == nil || fi.Name.Name != syntax.PropUnit || v.unitTwice(n, fi) {
			continue
		}
		if id, isName := fi.Value.(*syntax.IdentExpr); isName && !keys[id.Name] {
			diag.E1610.At(v.span(id), diag.KindUnit, id.Name, v.c.studio.path).Report(v.bag)
		}
	}
}
