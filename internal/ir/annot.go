package ir

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// annotation is the annotation named name, or nil; the parser allows one per position (GRAMMAR.md §8.3, E1120).
func annotation(anns []*syntax.Annotation, name string) *syntax.Annotation {
	for _, a := range anns {
		if a.Name != nil && a.Name.Name == name {
			return a
		}
	}
	return nil
}

// arg is the value of the named argument key of a, or nil.
func arg(a *syntax.Annotation, key string) syntax.AnnValue {
	if a == nil {
		return nil
	}
	for _, x := range a.Args {
		if x.Name != nil && x.Name.Name == key {
			return x.Value
		}
	}
	return nil
}

// argText is a string argument's constant text, or "" when absent or not constant.
func argText(a *syntax.Annotation, key string) string {
	v := arg(a, key)
	if v == nil {
		return ""
	}
	return constString(v)
}

// argWord is a symbol argument (`access: fields`, `unit: s`).
func argWord(a *syntax.Annotation, key string) (string, bool) {
	q, ok := arg(a, key).(*syntax.QualifiedName)
	if !ok || len(q.Parts) != 1 {
		return "", false
	}
	return q.Parts[0].Name, true
}

// argInt is an integer argument (`@cpp(value: 2)`).
func argInt(a *syntax.Annotation, key string) (int64, bool) {
	i, ok := arg(a, key).(*syntax.IntLit)
	if !ok || i.Value == nil || !i.Value.IsInt64() {
		return 0, false
	}
	return i.Value.Int64(), true
}

// hasFlag reports a positional flag of a (`@ts(bigint)`).
func hasFlag(a *syntax.Annotation, flag string) bool {
	if a == nil {
		return false
	}
	for _, x := range a.Args {
		if q, ok := x.Value.(*syntax.QualifiedName); ok && x.Name == nil && len(q.Parts) == 1 && q.Parts[0].Name == flag {
			return true
		}
	}
	return false
}

// names are the @go, @cpp and @ts name overrides of one position (CODEGEN.md §3.5).
type names struct {
	goName, cpp, ts NameOptions
}

func nameOverrides(anns []*syntax.Annotation) names {
	var n names
	n.goName.Name = argText(annotation(anns, syntax.AnnGo), syntax.ArgName)
	n.cpp.Name = argText(annotation(anns, syntax.AnnCpp), syntax.ArgName)
	n.ts.Name = argText(annotation(anns, syntax.AnnTS), syntax.ArgName)
	return n
}

// recordCpp is @cpp(name:, struct:, header:, access:) on a record header (CODEGEN.md §7.8).
func recordCpp(anns []*syntax.Annotation) CppOptions {
	a := annotation(anns, syntax.AnnCpp)
	var o CppOptions
	o.Name = argText(a, syntax.ArgName)
	o.Struct = argText(a, syntax.ArgStruct)
	o.Header = argText(a, syntax.ArgHeader)
	if word, ok := argWord(a, syntax.ArgAccess); ok {
		o.Access, _ = wordIndex[Access](accessWords[:], word)
	}
	return o
}

// fieldCpp is @cpp(name:, field:, type:, unit:) on a field.
func fieldCpp(anns []*syntax.Annotation) CppFieldOptions {
	a := annotation(anns, syntax.AnnCpp)
	var o CppFieldOptions
	o.Name = argText(a, syntax.ArgName)
	o.Member = argText(a, syntax.WordField)
	o.Type = argText(a, argType)
	if word, ok := argWord(a, syntax.ArgUnit); ok {
		o.Unit, o.HasUnit = unitOf(word)
	}
	return o
}

// caseCpp is @cpp(name:, value:) on a variant case.
func caseCpp(anns []*syntax.Annotation) CppCaseOptions {
	a := annotation(anns, syntax.AnnCpp)
	var o CppCaseOptions
	o.Name = argText(a, syntax.ArgName)
	o.Value, o.HasValue = argInt(a, argValue)
	return o
}

// unitOf is the Duration unit a symbol names (GRAMMAR.md §8.3).
func unitOf(word string) (types.Unit, bool) {
	for u := types.UnitMs; u <= types.UnitD; u++ {
		if u.String() == word {
			return u, true
		}
	}
	return types.UnitMs, false
}

// wordIndex is the value whose word is w in a table of words indexed by value.
func wordIndex[T ~uint8](words []string, w string) (T, bool) {
	for i, x := range words {
		if x == w && w != "" {
			return T(i), true
		}
	}
	return 0, false
}
