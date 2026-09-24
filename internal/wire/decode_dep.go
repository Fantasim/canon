package wire

import (
	"maps"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// frame is what a type may name (TYPES.md §11.1): parameters, map binders, earlier fields.
type frame struct {
	params  map[*types.Param]value.Value
	binders map[string]value.Value
	rec     *value.Record
	failed  bool // rec has a field that failed: its later defaults are not evaluated
}

// bind is fr with a dependent map's binder bound to one of its keys (TYPES.md §11.5).
func (fr *frame) bind(binder string, key value.Value) *frame {
	b := maps.Clone(fr.binders)
	if b == nil {
		b = map[string]value.Value{}
	}
	b[binder] = key
	return &frame{params: fr.params, binders: b, rec: fr.rec}
}

// dependent decodes an application as its selected branch; on Never, a symbol (DECISIONS 175).
func (r *run) dependent(sel Selection, t types.Type, sc wscope) value.Value {
	app := t.Base().(*types.TypeAppType)
	branch, fr, ok := r.branch(app, sc.fr)
	n := sel.Node
	switch {
	case !ok:
		return nil
	case branch.Base().Kind() == types.Never && n.Kind != jsonsrc.Null && !sel.Star:
		return &value.Symbol{Name: symbolText(n), T: t, P: prov(n)}
	}
	return r.value(sel, branch, wscope{unit: sc.unit, asInt: sc.asInt, bits: sc.bits, root: sc.root, fr: fr})
}

// branch is the type app selects, and the frame binding its function's parameters (DEP-02).
func (r *run) branch(app *types.TypeAppType, fr *frame) (types.Type, *frame, bool) {
	fn := app.Fn
	params, ok := r.bind(fn.Params, app.Args, fr)
	if !ok {
		return nil, nil, false
	}
	inner := &frame{params: params}
	if fn.Scrutinee == nil {
		return fn.Body, inner, true
	}
	v, ok := r.follow(params[fn.Scrutinee.Param], fn.Scrutinee.Path)
	if !ok {
		return nil, nil, false
	}
	if i, ok := armIndex(v); ok && fn.Arm(i) != nil {
		return fn.Arm(i).Result, inner, true
	}
	r.misuse(ErrShape, app)
	return nil, nil, false
}

// bind evaluates each argument in fr and binds it to its parameter (TYPES.md §11.1).
func (r *run) bind(params []*types.Param, args []*types.Arg, fr *frame) (map[*types.Param]value.Value, bool) {
	if len(params) != len(args) {
		r.misuse(ErrShape, nil)
		return nil, false
	}
	out := make(map[*types.Param]value.Value, len(params))
	for i, a := range args {
		v, ok := r.arg(a, fr)
		if !ok {
			return nil, false
		}
		out[params[i]] = v
	}
	return out, true
}

// arg is an argument's value: a stable path from a parameter, an earlier field or a binder.
func (r *run) arg(a *types.Arg, fr *frame) (value.Value, bool) {
	var v value.Value
	path := a.Path
	switch a.Source {
	case types.ArgParam:
		v = fr.params[a.Param]
	case types.ArgKey:
		v = fr.binders[a.Binder]
	case types.ArgField:
		if len(path) > 0 && fr.rec != nil && path[0].Index < len(fr.rec.Fields) {
			v, path = fr.rec.Fields[path[0].Index], path[1:]
		}
	}
	if v == nil {
		return r.silent()
	}
	return r.follow(v, path)
}

// follow reads fields along path, dereferencing refs through the host.
func (r *run) follow(v value.Value, path []*types.Field) (value.Value, bool) {
	for _, f := range path {
		rec, ok := r.deref(v)
		if !ok {
			return nil, false
		}
		if f.Index >= len(rec.Fields) || rec.Fields[f.Index] == nil {
			return r.silent()
		}
		v = rec.Fields[f.Index]
	}
	return v, v != nil
}

// deref is the record v is, or the entry it names; the host reports a missing one.
func (r *run) deref(v value.Value) (*value.Record, bool) {
	switch x := v.(type) {
	case *value.Record:
		return x, true
	case *value.Ref:
		if r.d.Host == nil {
			r.misuse(ErrNoHost, x.T)
			return nil, false
		}
		rec, ok := r.d.Host.Deref(r.ctx, x)
		if !ok || rec == nil {
			r.failed = true
			return nil, false
		}
		return rec, true
	}
	r.misuse(ErrShape, v.Type())
	return nil, false
}

// silent is a value a failure already reported leaves missing; without one, a misuse.
func (r *run) silent() (value.Value, bool) {
	if !r.failed {
		r.misuse(ErrShape, nil)
	}
	return nil, false
}

// armIndex is what a type-level match selects on: a member's index, false 0 and true 1.
func armIndex(v value.Value) (int, bool) {
	switch x := v.(type) {
	case *value.Member:
		return x.Index, true
	case *value.CaseKind:
		return x.Index, true
	case *value.Bool:
		if x.V {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// symbolText is a JSON value as a symbol: a string's text, else its compact JSON.
func symbolText(n *jsonsrc.Node) string {
	if n.Kind == jsonsrc.String {
		return n.Text
	}
	return string(sourceNode(n).compact(nil))
}

// sourceNode is a source value as an encoder node, for its compact text (WIRE.md §7.4).
func sourceNode(n *jsonsrc.Node) *node {
	switch n.Kind {
	case jsonsrc.Array:
		a := arrayNode(len(n.Elems))
		for _, e := range n.Elems {
			a.elems = append(a.elems, sourceNode(e))
		}
		return a
	case jsonsrc.Object:
		o := objectNode()
		for _, m := range n.Members {
			o.add(m.Key, sourceNode(m.Value))
		}
		return o
	case jsonsrc.String:
		return stringNode(n.Text)
	default:
		return text(n.Text)
	}
}
