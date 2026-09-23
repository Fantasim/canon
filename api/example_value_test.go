package canon_test

import (
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// describe prints a value the way a detail panel shows it, by kind (API.md §5.2).
func describe(v *canon.Value) {
	switch v.Kind {
	case canon.KindBool:
		b, _ := v.Bool()
		fmt.Println(v.Path, b)
	case canon.KindInt:
		n, _ := v.Int()
		fmt.Println(v.Path, n)
	case canon.KindFloat:
		f, _ := v.Float()
		fmt.Println(v.Path, f)
	case canon.KindString, canon.KindAsset:
		s, _ := v.Str()
		fmt.Println(v.Path, s)
	case canon.KindDuration:
		d, _ := v.Dur()
		fmt.Println(v.Path, d)
	case canon.KindEnum:
		enum, name, _ := v.Member()
		fmt.Println(v.Path, enum, name)
	default:
		describeComposite(v)
	}
}

// describeComposite lists the children of a composite value, or its key or case.
func describeComposite(v *canon.Value) {
	switch v.Kind {
	case canon.KindRecord, canon.KindList, canon.KindKeyedList, canon.KindTable, canon.KindMap:
		fmt.Println(v.Path, v.Len())
		for _, c := range v.Children() {
			fmt.Println(" ", c.Path, c)
		}
	case canon.KindVariant:
		name, _ := v.Case()
		kind, err := v.Child(".kind")
		if err == nil {
			fmt.Println(v.Path, name, kind)
		}
	case canon.KindRef:
		key, _ := v.Key()
		fmt.Println(v.Path, key)
	case canon.KindNone, canon.KindRange:
		fmt.Println(v.Path, v, v.IsNone())
	}
}

// describeOrigin prints where a value comes from, following Via to the literal it copies.
func describeOrigin(o canon.Origin) {
	switch o.Kind {
	case canon.OriginLiteral, canon.OriginCSV, canon.OriginDefines, canon.OriginText:
		fmt.Printf("%s at %s:%d\n", o.Kind, o.File, o.Line)
	case canon.OriginJSON:
		fmt.Printf("json at %s#%s\n", o.File, o.Pointer)
	case canon.OriginLayer:
		fmt.Println("set by layer", o.Layer)
	case canon.OriginDefault, canon.OriginSpread:
		fmt.Println(o.Kind, "of")
		if o.Via != nil {
			describeOrigin(*o.Via)
		}
	case canon.OriginComputed:
		for _, f := range o.Stack {
			fmt.Printf("in %s (%s:%d)\n", f.Fn, f.File, f.Line)
		}
	}
}

// describeEditability says where an edit of the value would go, or why there is none.
func describeEditability(e canon.Editability) {
	switch e.Mode {
	case canon.EditCanon, canon.EditJSON:
		fmt.Println("edits", e.File)
	case canon.EditNone:
		describeReason(e)
	}
}

// describeReason says why a value is not editable (API.md §7.2).
func describeReason(e canon.Editability) {
	switch e.Reason {
	case canon.ReasonComputed:
		fmt.Println("computed; edit", e.Origin)
	case canon.ReasonLayered:
		fmt.Println("set by layer", e.Layer)
	case canon.ReasonFormat, canon.ReasonInput, canon.ReasonKey, canon.ReasonPseudo,
		canon.ReasonOrder, canon.ReasonLayer, canon.ReasonNone:
		fmt.Println("read-only:", e.Reason)
	}
}
