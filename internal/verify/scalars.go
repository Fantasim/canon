package verify

import (
	"math"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// finite checks that a Float is a number and that a Float32 did not overflow (TYPES.md §7.3).
func (w *walker) finite(v *value.Float, b types.Basic, at *Path) {
	s := SiteOf(v)
	switch {
	case math.IsNaN(v.V) || math.IsInf(v.V, 0):
		w.flag(s, diag.E3202.AtNonFinite(s.Span, v), v, at)
	case b == types.Float32Type && math.IsInf(float64(float32(v.V)), 0):
		w.flag(s, diag.E3202.AtFloat32(s.Span, &value.Float{V: v.V, T: types.FloatType}), v, at)
	}
}

func (w *walker) member(m *value.Member, at *Path, sc scope) {
	e := m.Enum
	if e == nil || m.Index < 0 || m.Index >= len(e.Members) || !e.Members[m.Index].Retired || sc.retired {
		return
	}
	name, s := e.Members[m.Index].Name, SiteOf(m)
	b := diag.E3506.At(s.Span, w.local(e.Pkg, e.Name), name)
	w.flag(s, w.src.relatedNode(b, e.Decl, memberNode(e, name)), m, at)
}

// retiredCase reports a retired case outside a retired entry (TYPES.md §8.1).
func (w *walker) retiredCase(r *value.Record, c *types.CaseType, at *Path, sc scope) {
	if !c.Retired || sc.retired {
		return
	}
	v, s := c.Variant, SiteOf(r)
	b := diag.E3506.At(s.Span, w.local(v.Pkg, v.Name), c.Name)
	w.flag(s, w.src.relatedNode(b, v.Decl, caseNode(v, c.Name)), r, at)
}

// local names a declaration of pkg, qualified outside the finding's package (ERRORS.md §1.3).
func (w *walker) local(pkg, name string, fields ...string) string {
	if pkg != w.pkg && pkg != "" {
		name = pkg + dot + name
	}
	return strings.Join(append([]string{name}, fields...), dot)
}
