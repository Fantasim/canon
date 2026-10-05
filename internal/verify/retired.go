package verify

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// retiredUse is a retired member, case or entry named outside a past slot, judged per scope (TYPES.md §8.4, §10.3; LOCK.md §4.3).
type retiredUse struct {
	v       value.Value    // the member, the case record or the ref; marked invalid when reported
	path    string         // where it is, under the prefix of the instance keeping it
	entry   string         // the innermost entry around it, "" for none, as scope.entry
	retired string         // the innermost retired entry around it, "" for none, as scope.retired
	ref     *types.RefType // E3502's ref type; nil for E3506
	pkg     string         // E3506: the enum or variant, its member or case, and their declarations
	typ     string
	name    string
	decl    syntax.Node
	node    syntax.Node
}

// member is E3506 for a retired member outside a past slot (TYPES.md §8.1, §8.4).
func (w *walker) member(m *value.Member, at *Path, sc scope) {
	e := m.Enum
	if sc.past || e == nil || m.Index < 0 || m.Index >= len(e.Members) || !e.Members[m.Index].Retired {
		return
	}
	name := e.Members[m.Index].Name
	w.use(retiredUse{v: m, path: at.String(), pkg: e.Pkg, typ: e.Name, name: name, decl: e.Decl, node: memberNode(e, name)}, sc)
}

// retiredCase is E3506 for a retired case outside a past slot (TYPES.md §8.1, §8.4).
func (w *walker) retiredCase(r *value.Record, at *Path, sc scope) {
	c, ok := r.T.Base().(*types.CaseType)
	if !ok || !c.Retired || sc.past {
		return
	}
	v := c.Variant
	w.use(retiredUse{v: r, path: at.String(), pkg: v.Pkg, typ: v.Name, name: c.Name, decl: v.Decl, node: caseNode(v, c.Name)}, sc)
}

// retiredTarget is E3502 for a ref to a retired entry outside a past ref slot (TYPES.md §10.3, §8.4).
func (w *walker) retiredTarget(r *value.Ref, rt *types.RefType, target *value.Record, at *Path, sc scope) {
	if target.Ident.Retired && !sc.past {
		w.use(retiredUse{v: r, path: at.String(), ref: rt}, sc)
	}
}

// use keeps u, in sc's entries, for each instance being verified around it, then judges it.
func (w *walker) use(u retiredUse, sc scope) {
	u.entry, u.retired = sc.entry, sc.retired
	for _, r := range w.recording {
		r.uses = append(r.uses, u)
	}
	w.judgeUse(u)
}

// judgeUse reports u unless a retired entry encloses it, keeping the finding per scope (LOCK.md §4.3).
func (w *walker) judgeUse(u retiredUse) {
	if u.retired != "" {
		return
	}
	s := SiteOf(u.v)
	b := w.retiredFinding(u, s.Span, u.entry)
	s.reportAt(b, u.path, w.bag)
	w.noteFound(b)
	w.invalid(u.v) // the value, wherever else it is held (EVALUATION.md §7.3)
	w.keepScoped(b, u.path)
}

// retiredFinding is u's finding: E3502 naming the live entry around it, if any, else E3506.
func (w *walker) retiredFinding(u retiredUse, span source.Span, entry string) *diag.Builder {
	switch {
	case u.ref == nil:
		return w.src.relatedNode(diag.E3506.At(span, w.local(u.pkg, u.typ), u.name), u.decl, u.node)
	case entry == "":
		return w.src.related(diag.E3502.AtValue(span, u.v), u.ref)
	}
	return w.src.related(diag.E3502.AtEntry(span, u.v, entry), u.ref)
}

// rebased is uses of the instance at prefix, reached at here in sc: their paths, and the entries
// inside the instance, moved under here; sc's entries for those outside it. Nil for none.
func rebased(uses []retiredUse, prefix, here string, sc scope) []retiredUse {
	if len(uses) == 0 {
		return nil
	}
	out := make([]retiredUse, len(uses))
	for i, u := range uses {
		u.path = rebase(u.path, prefix, here)
		u.entry = inside(u.entry, prefix, here, sc.entry)
		u.retired = inside(u.retired, prefix, here, sc.retired)
		out[i] = u
	}
	return out
}

// rebase is path, under prefix, moved under here; "" stays "".
func rebase(path, prefix, here string) string {
	if path == "" {
		return path
	}
	return here + strings.TrimPrefix(path, prefix)
}

// inside is the entry at p moved under here when it lies inside the instance at prefix (the
// instance itself is an entry of its reaching scope), else outer.
func inside(p, prefix, here, outer string) string {
	rest, ok := strings.CutPrefix(p, prefix)
	if !ok || !strings.HasPrefix(rest, dot) && !strings.HasPrefix(rest, keyOpen) {
		return outer
	}
	return here + rest
}
