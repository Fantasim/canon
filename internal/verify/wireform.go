package verify

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// wireForm reports each part of field f's value v that has no wire form, at its own path (E8102, WIRE.md §5.1).
func (w *walker) wireForm(f *types.Field, v value.Value, at *Path, e *env) {
	if w.stopped() {
		return
	}
	for _, x := range wire.FieldForms(f, v) {
		s := SiteOf(x.Site)
		w.flag(s, x.At(s.Span, f.Name), x.Value, w.stepsPath(at, f.Type, x.Steps, e))
	}
}

// stepsPath is at extended by steps, naming elements and map entries as the walk does (API.md P8, P9).
func (w *walker) stepsPath(at *Path, t types.Type, steps []wire.Step, e *env) *Path {
	for _, st := range steps {
		switch in := st.In.(type) {
		case *value.List:
			at = at.Index(st.Index)
			if lt := listOf(Declared(t), in.T); lt != nil {
				t = lt.Elem
			}
		case *value.Map:
			var kt types.Type
			switch mt := underOf(Declared(t), in.T).(type) {
			case *types.MapType:
				kt, t = mt.Key, mt.Value
			case *types.DepMapType:
				t = mt.Value
			}
			at = w.keyPath(at, st.Key, kt, e)
		}
	}
	return at
}
