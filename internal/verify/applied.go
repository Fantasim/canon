package verify

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// appliedArgs is E3802 at rec, stored where its type applies the record to other arguments than its own (TYPES.md §11.1).
func (w *walker) appliedArgs(rec *value.Record, t types.Type, at *Path, sc scope) {
	app, ok := appliedOf(t)
	bound := w.stage.Params(rec)
	if !ok || len(bound) == 0 || len(app.Args) != len(app.Rec.Params) {
		return
	}
	args := make([]value.Value, len(app.Args))
	differ := false
	for i, p := range app.Rec.Params {
		v, r := w.arg(app.Args[i], sc.env)
		b, has := bound[p]
		if r == aborted {
			w.settled(r, nil) // a failed dereference aborts the root (EVALUATION.md §5)
			return
		}
		if r != found || !has {
			return // an argument with no value (an input field) leaves nothing to compare
		}
		args[i], differ = v, differ || !value.Equal(v, b)
	}
	if !differ {
		return
	}
	s := SiteOf(rec)
	b := w.src.related(diag.E3802.At(s.Span, w.src.types[app].typ, applied{rec: app.Rec, args: args}), app)
	w.report(s, b, at)
	if sc.env != nil && sc.env.rec != nil {
		w.invalid(sc.env.rec) // the instance holding it, which stays valid where it was built
	}
}

// applied is a record applied to argument values, as the type an application computes.
type applied struct {
	rec  *types.RecordType
	args []value.Value
}

func (a applied) String() string {
	texts := make([]string, len(a.args))
	for i, v := range a.args {
		texts[i] = v.CanonText()
	}
	return a.rec.String() + argsOpen + strings.Join(texts, argsSep) + argsClose
}
