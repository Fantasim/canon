package edit

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// fromJSON decodes raw by WIRE.md's rules for t in the destination field's scope (its unit, int,
// bits, none marker), keeping what only the re-check reports (V2, V3), a failing finding being
// the refusal's detail; fields left to their defaults are left out.
func (tc *typing) fromJSON(raw FromJSON, t types.Type) (value.Value, error) {
	var fs source.FileSet
	f, err := fs.Add(jsonName, jsonName, raw)
	if err != nil {
		return nil, tc.refuse(t, describe(raw), err.Error())
	}
	bag := diag.NewBag(&fs, tc.ty.Pkg)
	root, err := jsonsrc.Parse(f, bag)
	if err != nil {
		return nil, tc.refuse(t, describe(raw), firstFinding(&fs, bag, err.Error()))
	}
	dec := wire.Decoder{Bag: bag, Pkg: tc.ty.Pkg, Host: tc.ty.Host, Keep: true, Outer: tc.ty.Outer, Field: tc.ty.scope}
	v, ok, err := dec.Decode(tc.ctx, wire.Selection{Node: root}, t)
	switch {
	case tc.ctx.Err() != nil:
		return nil, tc.ctx.Err()
	case err != nil:
		return nil, tc.refuse(t, describe(raw), err.Error())
	case !ok:
		return nil, tc.undecoded(raw, t, &fs, bag)
	}
	written(v)
	tc.ty.marks.noteTokens(v, f)
	return v, nil
}

// undecoded is the ValueError of a raw that did not decode (V1): its first finding of the static
// shape, else its first, which no value holds (an integer past Int64, a float past Float64).
func (tc *typing) undecoded(raw FromJSON, t types.Type, fs *source.FileSet, bag *diag.Bag) error {
	found := diag.Locate(fs, bag.Findings())
	for _, f := range found {
		if !recheckCodes[f.Code] {
			return tc.refuse(t, describe(raw), located(f))
		}
	}
	if len(found) == 0 {
		return tc.refuse(t, describe(raw), detailUndecoded)
	}
	return tc.refuse(t, describe(raw), located(found[0]))
}

// firstFinding is the first finding's pointer and message, or orElse when there is none.
func firstFinding(fs *source.FileSet, bag *diag.Bag, orElse string) string {
	found := diag.Locate(fs, bag.Findings())
	if len(found) == 0 {
		return orElse
	}
	return located(found[0])
}

// located is a finding's pointer and message.
func located(f diag.Located) string {
	if f.Pointer == "" {
		return f.Message
	}
	return f.Pointer + detailSep + f.Message
}

// written drops, in every record of v, the fields the JSON did not write (API.md V3).
func written(v value.Value) {
	switch x := v.(type) {
	case *value.Record:
		for i, set := range x.Set {
			x.Fields[i] = writtenField(x.Fields[i], set)
		}
	case *value.List:
		writtenAll(x.Elems)
	case *value.Map:
		writtenAll(x.Vals)
	case *value.Table:
		for _, e := range x.Entries {
			written(e)
		}
	}
}

// writtenField is a field's value with its own unwritten fields dropped, nil when it was not written.
func writtenField(v value.Value, set bool) value.Value {
	if !set {
		return nil
	}
	written(v)
	return v
}

func writtenAll(vs []value.Value) {
	for _, v := range vs {
		written(v)
	}
}
