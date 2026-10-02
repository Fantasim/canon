package canon

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/fantasim/canonlang/internal/edit"
)

// pathKeyLit is a key as an edit's JSON form gives it, read as a path key (rule E25); only
// UnmarshalJSON makes one.
type pathKeyLit struct{ key string }

func (pathKeyLit) isLit() {}

// operation is o as edit applies it; an unknown kind is opUnknown, which edit refuses in its
// turn as ErrBadOp (rules E1, E2).
func (o Op) operation() edit.Operation {
	kind := opUnknown
	for k := edit.OpSet; k <= edit.OpRenameName; k++ {
		if opKinds[k] == o.Kind {
			kind = k
		}
	}
	return edit.Operation{Kind: kind, Path: o.Path, Value: editLit(o.Value), Key: editLit(o.Key), Index: o.Index, Case: o.Case, Name: o.Name}
}

// operations are ops as edit applies them.
func operations(ops []Op) []edit.Operation {
	out := make([]edit.Operation, len(ops))
	for i, op := range ops {
		out[i] = op.operation()
	}
	return out
}

// opJSON is o as edit's codec writes it; an unknown kind is errOpKind (rule E24).
func (o Op) opJSON() (edit.Operation, error) {
	op := o.operation()
	if op.Kind == opUnknown {
		return op, fmt.Errorf(fmtUnknown, errOpKind, o.Kind)
	}
	return op, nil
}

// opOf is an operation edit gave back, an Undo's, in the API's form (rule E23).
func opOf(o edit.Operation) Op {
	return Op{Kind: opKinds[o.Kind], Path: o.Path, Value: apiLit(o.Value), Key: apiLit(o.Key), Index: o.Index, Case: o.Case, Name: o.Name}
}

// editLit is v as edit reads it; nil stays nil (API.md §8.2).
func editLit(v Lit) edit.Lit {
	switch x := v.(type) {
	case boolLit:
		return edit.Bool(x.v)
	case intLit:
		return edit.Int(x.v)
	case floatLit:
		return edit.Float(x.v)
	case strLit:
		return edit.Str(x.v)
	case durLit:
		return edit.Dur(x.v)
	case memberLit:
		return edit.Member(x.name)
	case keyLit:
		return edit.Key(x.key)
	case intKeyLit:
		return edit.IntKey(x.key)
	case pathKeyLit:
		return edit.PathKey(x.key)
	case noneLit:
		return edit.None{}
	case jsonLit:
		return edit.FromJSON(slices.Clone(x.raw))
	case sourceLit:
		return edit.Source(x.text)
	}
	return editComposite(v)
}

// editComposite is a list, record, map or case value as edit reads it.
func editComposite(v Lit) edit.Lit {
	switch x := v.(type) {
	case listLit:
		out := make(edit.List, len(x.elems))
		for i, e := range x.elems {
			out[i] = editLit(e)
		}
		return out
	case Obj:
		return editObj(x)
	case mapLit:
		out := make(edit.Map, len(x.entries))
		for i, kv := range x.entries {
			out[i] = edit.KV{Key: editLit(kv.Key), Value: editLit(kv.Value)}
		}
		return out
	case caseLit:
		return edit.Case{Name: x.name, Fields: editObj(x.fields)}
	}
	return nil
}

// editObj is o's fields as edit reads them; nil stays nil (a case without fields).
func editObj(o Obj) edit.Obj {
	if o == nil {
		return nil
	}
	out := make(edit.Obj, len(o))
	//canon:unordered each field is converted under its own name
	for name, v := range o {
		out[name] = editLit(v)
	}
	return out
}

// apiLit is a value edit gave back in the API's form; nil stays nil.
func apiLit(v edit.Lit) Lit {
	switch x := v.(type) {
	case edit.Bool:
		return boolLit{bool(x)}
	case edit.Int:
		return intLit{int64(x)}
	case edit.Float:
		return floatLit{float64(x)}
	case edit.Str:
		return strLit{string(x)}
	case edit.Dur:
		return durLit{time.Duration(x)}
	case edit.Member:
		return memberLit{string(x)}
	case edit.Key:
		return keyLit{string(x)}
	case edit.IntKey:
		return intKeyLit{int64(x)}
	case edit.PathKey:
		return pathKeyLit{string(x)}
	case edit.None:
		return None
	case edit.FromJSON:
		return jsonLit{json.RawMessage(slices.Clone(x))}
	case edit.Source:
		return sourceLit{string(x)}
	}
	return apiComposite(v)
}

// apiComposite is a list, record, map or case value edit gave back in the API's form.
func apiComposite(v edit.Lit) Lit {
	switch x := v.(type) {
	case edit.List:
		out := listLit{elems: make([]Lit, len(x))}
		for i, e := range x {
			out.elems[i] = apiLit(e)
		}
		return out
	case edit.Obj:
		return apiObj(x)
	case edit.Map:
		out := mapLit{entries: make([]KV, len(x))}
		for i, kv := range x {
			out.entries[i] = KV{Key: apiLit(kv.Key), Value: apiLit(kv.Value)}
		}
		return out
	case edit.Case:
		return caseLit{name: x.Name, fields: apiObj(x.Fields)}
	}
	return nil
}

// apiObj is o's fields in the API's form; nil stays nil.
func apiObj(o edit.Obj) Obj {
	if o == nil {
		return nil
	}
	out := make(Obj, len(o))
	//canon:unordered each field is converted under its own name
	for name, v := range o {
		out[name] = apiLit(v)
	}
	return out
}
